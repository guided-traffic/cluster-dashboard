package discovery

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"

	"github.com/guided-traffic/dash/backend/internal/config"
)

// within bounds every wait for the informers.
const within = 5 * time.Second

// logBuffer is a log the informers' goroutines may write while a test reads.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// start runs a discovery over client until the test ends and waits for its
// first sync.
func start(t *testing.T, client *fake.Clientset, opts Options) (*Discovery, *logBuffer) {
	t.Helper()
	logs := &logBuffer{}
	d, err := New(client, opts, slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	require.NoError(t, err)
	require.ErrorIs(t, d.Ready(t.Context()), ErrNotSynced, "not ready before the first list")
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { d.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	require.Eventually(t, func() bool { return d.Ready(ctx) == nil }, within, 10*time.Millisecond)
	return d, logs
}

func names(apps []App) []string {
	out := make([]string, 0, len(apps))
	for _, a := range apps {
		out = append(out, a.Key())
	}
	return out
}

func eventually(t *testing.T, d *Discovery, want []string) {
	t.Helper()
	assert.Eventually(t, func() bool { return assert.ObjectsAreEqual(want, names(d.Apps())) }, within, 10*time.Millisecond,
		"apps %v, want %v", names(d.Apps()), want)
}

func in(namespace, name string, opts ...func(*networkingv1.Ingress)) *networkingv1.Ingress {
	ing := ingress(opts...)
	ing.Namespace, ing.Name = namespace, name
	return ing
}

func namespace(name string, nsLabels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: nsLabels}}
}

var anyNamespace = Options{Filter: Filter{DefaultEnable: true}, Namespaces: NamespaceSelector{Any: true}}

// The discovery follows the cluster: an Ingress added, changed, opted out and
// deleted, its EndpointSlice turning ready and back.
func TestTheDiscoveryFollowsTheCluster(t *testing.T) {
	yes, no := ptr.To(true), ptr.To(false)
	client := fake.NewClientset(in("media", "grafana", annotate(KeyAppName, "Grafana")),
		endpointSlice("grafana-a", endpoint("p1", yes, nil, "10.0.0.1")))
	d, _ := start(t, client, anyNamespace)
	ctx := t.Context()

	eventually(t, d, []string{"media/grafana"})
	apps := d.Apps()
	assert.Equal(t, "Grafana", apps[0].Name)
	assert.Equal(t, Status{State: Healthy, Ready: 1, Total: 1}, apps[0].Status)

	_, err := client.DiscoveryV1().EndpointSlices("media").Update(ctx,
		endpointSlice("grafana-a", endpoint("p1", yes, nil, "10.0.0.1"), endpoint("p2", no, nil, "10.0.0.2")), metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return d.Apps()[0].Status.State == Degraded }, within, 10*time.Millisecond)

	_, err = client.NetworkingV1().Ingresses("tools").Create(ctx, in("tools", "router"), metav1.CreateOptions{})
	require.NoError(t, err)
	eventually(t, d, []string{"media/grafana", "tools/router"})

	_, err = client.NetworkingV1().Ingresses("tools").Update(ctx, in("tools", "router", annotate(KeyEnable, "false")), metav1.UpdateOptions{})
	require.NoError(t, err)
	eventually(t, d, []string{"media/grafana"})

	require.NoError(t, client.NetworkingV1().Ingresses("media").Delete(ctx, "grafana", metav1.DeleteOptions{}))
	eventually(t, d, []string{})
}

func TestTheStatusCheckCanBeSwitchedOff(t *testing.T) {
	client := fake.NewClientset(in("media", "grafana", annotate(KeyStatusCheckEnabled, "false")),
		endpointSlice("grafana-a", endpoint("p1", ptr.To(false), nil, "10.0.0.1")))
	d, _ := start(t, client, anyNamespace)
	eventually(t, d, []string{"media/grafana"})
	assert.Equal(t, Status{State: Unknown}, d.Apps()[0].Status)
}

// Named namespaces without a label selector run informers per namespace,
// which the chart's namespaced Roles allow: nothing is listed cluster-wide.
func TestNamedNamespacesAreWatchedOneByOne(t *testing.T) {
	client := fake.NewClientset(in("media", "a"), in("tools", "b"), in("other", "c"))
	d, _ := start(t, client, Options{Filter: Filter{DefaultEnable: true}, Namespaces: NamespaceSelector{Names: []string{"media", "tools"}}})
	eventually(t, d, []string{"media/a", "tools/b"})
	for _, action := range client.Actions() {
		assert.NotEmpty(t, action.GetNamespace(), "%s %s is namespaced", action.GetVerb(), action.GetResource().Resource)
		assert.NotEqual(t, "namespaces", action.GetResource().Resource, "no Namespace is read")
	}
}

// A label selector selects namespaces by their labels, united with the
// names, and follows a namespace whose labels change.
func TestALabelSelectorFollowsTheNamespaces(t *testing.T) {
	client := fake.NewClientset(
		namespace("media", map[string]string{"dash": "listed"}), namespace("tools", nil), namespace("static", nil),
		in("media", "a"), in("tools", "b"), in("static", "c"),
	)
	sel, err := labels.Parse("dash=listed")
	require.NoError(t, err)
	d, _ := start(t, client, Options{Filter: Filter{DefaultEnable: true}, Namespaces: NamespaceSelector{Names: []string{"static"}, Labels: sel}})
	eventually(t, d, []string{"media/a", "static/c"})

	_, err = client.CoreV1().Namespaces().Update(t.Context(), namespace("tools", map[string]string{"dash": "listed"}), metav1.UpdateOptions{})
	require.NoError(t, err)
	eventually(t, d, []string{"media/a", "static/c", "tools/b"})
}

// An Ingress author's mistake is a warning that names the Ingress and the
// key, never the value; the selection at work is no warning.
func TestTheLogNamesWhatWasIgnored(t *testing.T) {
	client := fake.NewClientset(
		in("media", "wild", rules(rule("*.example.com"))),
		in("media", "badurl", annotate(KeyURL, "javascript:alert(1)")),
		in("media", "off", annotate(KeyEnable, "false")),
	)
	d, logs := start(t, client, anyNamespace)
	eventually(t, d, []string{"media/badurl"})
	out := logs.String()
	assert.Contains(t, out, `level=WARN msg="an Ingress is no app" namespace=media ingress=wild reason="no rule names a concrete host`)
	assert.Contains(t, out, `level=WARN msg="an annotation is ignored" namespace=media ingress=badurl key=dash.gtrfc.com/url reason="not an absolute http or https URL"`)
	assert.Contains(t, out, `level=DEBUG msg="an Ingress is no app" namespace=media ingress=off`)
	assert.NotContains(t, out, "alert(1)")
}

// With a label selector the Ingresses of every namespace are read; those of
// a namespace it does not select are not the log's business.
func TestUnselectedNamespacesAreNotLogged(t *testing.T) {
	sel, err := labels.Parse("dash=listed")
	require.NoError(t, err)
	client := fake.NewClientset(namespace("media", map[string]string{"dash": "listed"}), namespace("other", nil))
	d, logs := start(t, client, Options{Filter: Filter{DefaultEnable: true}, Namespaces: NamespaceSelector{Labels: sel}})
	_, err = client.NetworkingV1().Ingresses("other").Create(t.Context(), in("other", "wild", rules(rule("*.example.com"))), metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = client.NetworkingV1().Ingresses("media").Create(t.Context(), in("media", "a"), metav1.CreateOptions{})
	require.NoError(t, err)
	eventually(t, d, []string{"media/a"})
	assert.NotContains(t, logs.String(), "ingress=wild")
}

// A list the API server forbids — a missing RBAC grant — is logged as a
// warning that says so, and the discovery does not turn ready.
func TestAForbiddenListIsLoggedAndNeverReady(t *testing.T) {
	client := fake.NewClientset()
	client.PrependReactor("list", "ingresses", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, "", nil)
	})
	logs := &logBuffer{}
	d, err := New(client, anyNamespace, slog.New(slog.NewTextHandler(logs, nil)))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() { d.Run(ctx); close(done) }()
	assert.Eventually(t, func() bool { return bytes.Contains([]byte(logs.String()), []byte("forbidden")) }, within, 10*time.Millisecond)
	assert.Contains(t, logs.String(), `level=WARN msg="the list or watch failed" resource=ingresses`)
	assert.ErrorIs(t, d.Ready(ctx), ErrNotSynced)
	cancel()
	<-done
	assert.ErrorIs(t, d.Ready(ctx), ErrNotSynced, "a cancelled wait does not turn it ready")
}

func TestTheHandlersIgnoreOtherObjects(t *testing.T) {
	d, err := New(fake.NewClientset(), anyNamespace, nil)
	require.NoError(t, err)
	d.upsert(&corev1.Pod{})
	d.remove(&corev1.Pod{})
	d.upsert(in("media", "a"))
	assert.Len(t, d.apps, 1)
	d.remove(cache.DeletedFinalStateUnknown{Key: "media/a", Obj: in("media", "a")})
	assert.Empty(t, d.apps, "a deletion whose final state is unknown removes the app")
	keys, err := byService(&corev1.Pod{})
	assert.NoError(t, err)
	assert.Nil(t, keys)
	keys, err = byService(&discoveryv1.EndpointSlice{})
	assert.NoError(t, err)
	assert.Nil(t, keys, "a slice without a Service")
}

func TestTrim(t *testing.T) {
	ing := in("media", "a", annotate(KeyAppName, "A", "kubectl.kubernetes.io/last-applied-configuration", "{...}", LegacyIngressClass, "internal"))
	ing.Labels = map[string]string{"app": "a"}
	ing.ManagedFields = []metav1.ManagedFieldsEntry{{Manager: "kubectl"}}
	ing.Status.LoadBalancer.Ingress = []networkingv1.IngressLoadBalancerIngress{{IP: "1.2.3.4"}}
	got, err := trim(ing)
	require.NoError(t, err)
	out := got.(*networkingv1.Ingress)
	assert.Equal(t, map[string]string{KeyAppName: "A", LegacyIngressClass: "internal"}, out.Annotations)
	assert.Equal(t, map[string]string{"app": "a"}, out.Labels)
	assert.Empty(t, out.ManagedFields)
	assert.Empty(t, out.Status.LoadBalancer.Ingress)
	assert.Equal(t, ing.Spec, out.Spec)

	slice := endpointSlice("s", endpoint("p1", ptr.To(true), nil, "10.0.0.1"))
	slice.Endpoints[0].NodeName = ptr.To("node-1")
	slice.Endpoints[0].TargetRef.UID = "uid"
	slice.Ports = []discoveryv1.EndpointPort{{Port: ptr.To[int32](80)}}
	got, err = trim(slice)
	require.NoError(t, err)
	outSlice := got.(*discoveryv1.EndpointSlice)
	assert.Empty(t, outSlice.Ports)
	assert.Nil(t, outSlice.Endpoints[0].NodeName)
	assert.Equal(t, &corev1.ObjectReference{Kind: "Pod", Name: "p1"}, outSlice.Endpoints[0].TargetRef)
	assert.Equal(t, "grafana", outSlice.Labels[discoveryv1.LabelServiceName])

	got, err = trim(namespace("media", map[string]string{"a": "b"}))
	require.NoError(t, err)
	assert.Equal(t, namespace("media", map[string]string{"a": "b"}), got)

	tomb := cache.DeletedFinalStateUnknown{Key: "x"}
	got, err = trim(tomb)
	require.NoError(t, err)
	assert.Equal(t, tomb, got)
}

func TestOptionsFrom(t *testing.T) {
	f := config.DefaultFile()
	opts, err := OptionsFrom(f.Discovery)
	require.NoError(t, err)
	assert.Equal(t, Options{Namespaces: NamespaceSelector{Any: true, Names: nil}}, opts)
	assert.Equal(t, clusterWide, opts.Namespaces.mode())

	opts, err = OptionsFrom(config.Discovery{
		DefaultEnable: true, InstanceName: "home",
		NamespaceSelector: config.NamespaceSelector{MatchNames: []string{"media"}},
	})
	require.NoError(t, err)
	assert.Equal(t, Filter{DefaultEnable: true, InstanceName: "home"}, opts.Filter)
	assert.Equal(t, perNamespace, opts.Namespaces.mode())

	opts, err = OptionsFrom(config.Discovery{NamespaceSelector: config.NamespaceSelector{LabelSelector: config.LabelSelector{
		MatchLabels:      map[string]string{"dash": "listed"},
		MatchExpressions: []config.LabelSelectorRequirement{{Key: "tier", Operator: "In", Values: []string{"web"}}},
	}}})
	require.NoError(t, err)
	assert.Equal(t, byLabels, opts.Namespaces.mode())
	assert.Equal(t, "dash=listed,tier in (web)", opts.Namespaces.Labels.String())
	assert.True(t, opts.Namespaces.selects("x", map[string]string{"dash": "listed", "tier": "web"}))
	assert.False(t, opts.Namespaces.selects("x", map[string]string{"dash": "listed"}))

	_, err = OptionsFrom(config.Discovery{NamespaceSelector: config.NamespaceSelector{LabelSelector: config.LabelSelector{
		MatchExpressions: []config.LabelSelectorRequirement{{Key: "tier", Operator: "Like"}},
	}}})
	assert.Error(t, err, "config.Load refuses it first; the conversion does not trust that")
}
