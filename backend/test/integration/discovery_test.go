//go:build integration

package integration

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/guided-traffic/dash/backend/internal/discovery"
)

// rbacModes are the files make integration-rbac renders: the chart's RBAC
// for every namespace, for two named namespaces, for a named namespace
// united with a label selector, and for two named namespaces with a badge
// rule, which adds the IngressClasses.
var rbacModes = []string{"any", "names", "labels", "classes"}

// within bounds how long the discovery may take to follow a change.
const within = 15 * time.Second

// The world every mode looks at: an app in each namespace, one namespace
// carrying the label the label selector asks for.
var world = map[string]map[string]string{
	"team-a":   nil,
	"team-b":   nil,
	"other":    nil,
	"static":   nil,
	"labelled": {"dash": "listed"},
}

// asBackend is the chart's ServiceAccount, dash-backend in namespace dash,
// as the API server sees the backend's pod: impersonated by the
// administrator, so RBAC decides.
func asBackend() *rest.Config {
	cfg := rest.CopyConfig(adminConfig)
	cfg.Impersonate = rest.ImpersonationConfig{
		UserName: "system:serviceaccount:dash:dash-backend",
		Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:dash", "system:authenticated"},
	}
	return cfg
}

func adminClient(t *testing.T) kubernetes.Interface {
	t.Helper()
	c, err := kubernetes.NewForConfig(adminConfig)
	require.NoError(t, err)
	return c
}

// createWorld creates the namespaces and, in each, an Ingress app with a
// ready endpoint. envtest runs no controllers: the EndpointSlices are
// written here, as the EndpointSlice controller would.
func createWorld(t *testing.T, c kubernetes.Interface) {
	t.Helper()
	ctx := t.Context()
	for _, ns := range append(slices.Sorted(maps.Keys(world)), "dash") {
		_, err := c.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns, Labels: world[ns]}}, metav1.CreateOptions{})
		if !apierrors.IsAlreadyExists(err) {
			require.NoError(t, err)
		}
	}
	for ns := range world {
		createApp(t, c, ns, "web", true)
		// A Secret and a ConfigMap the backend must never read.
		_, err := c.CoreV1().Secrets(ns).Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "token"}}, metav1.CreateOptions{})
		if !apierrors.IsAlreadyExists(err) {
			require.NoError(t, err)
		}
	}
}

func createApp(t *testing.T, c kubernetes.Interface, ns, name string, ready bool) {
	t.Helper()
	ctx := t.Context()
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: map[string]string{discovery.KeyAppName: ns + " " + name}},
		Spec: networkingv1.IngressSpec{
			TLS: []networkingv1.IngressTLS{{Hosts: []string{name + "." + ns + ".example.com"}, SecretName: "tls"}},
			Rules: []networkingv1.IngressRule{{Host: name + "." + ns + ".example.com", IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{Paths: []networkingv1.HTTPIngressPath{{
					Path: "/", PathType: ptr.To(networkingv1.PathTypePrefix),
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: name, Port: networkingv1.ServiceBackendPort{Number: 80},
					}},
				}}},
			}}},
		},
	}
	_, err := c.NetworkingV1().Ingresses(ns).Create(ctx, ing, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		return
	}
	require.NoError(t, err)
	_, err = c.DiscoveryV1().EndpointSlices(ns).Create(ctx, endpointSlice(name, ready), metav1.CreateOptions{})
	require.NoError(t, err)
}

func endpointSlice(service string, ready bool) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta:  metav1.ObjectMeta{Name: service + "-1", Labels: map[string]string{discoveryv1.LabelServiceName: service}},
		AddressType: discoveryv1.AddressTypeIPv4,
		Endpoints: []discoveryv1.Endpoint{{
			Addresses:  []string{"10.0.0.1"},
			Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(ready)},
			TargetRef:  &corev1.ObjectReference{Kind: "Pod", Name: service + "-pod"},
		}},
	}
}

// applyRBAC creates the chart's RBAC of a mode and deletes it when the test
// ends.
func applyRBAC(t *testing.T, mode string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(os.Getenv(envRBAC), mode+".yaml"))
	require.NoError(t, err)
	c, err := client.New(adminConfig, client.Options{})
	require.NoError(t, err)
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var objs []*unstructured.Unstructured
	for {
		obj := &unstructured.Unstructured{}
		err := dec.Decode(&obj.Object)
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err)
		if len(obj.Object) == 0 {
			continue
		}
		require.NoError(t, c.Create(t.Context(), obj), "%s %s", obj.GetKind(), obj.GetName())
		objs = append(objs, obj)
	}
	require.NotEmpty(t, objs, "the chart renders RBAC for %s", mode)
	t.Cleanup(func() {
		for _, obj := range objs {
			assert.NoError(t, client.IgnoreNotFound(c.Delete(context.Background(), obj)))
		}
	})
}

// runDiscovery runs the discovery as the backend's ServiceAccount until the
// test ends, and waits for its first sync.
func runDiscovery(t *testing.T, opts discovery.Options) *discovery.Discovery {
	t.Helper()
	c, err := kubernetes.NewForConfig(asBackend())
	require.NoError(t, err)
	d, err := discovery.New(c, opts, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { d.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	require.Eventually(t, func() bool { return d.Ready(ctx) == nil }, within, 50*time.Millisecond,
		"the discovery lists under the chart's RBAC")
	return d
}

func keys(d *discovery.Discovery) []string {
	apps := d.Apps()
	out := make([]string, 0, len(apps))
	for _, a := range apps {
		out = append(out, a.Key())
	}
	return out
}

func eventuallyApps(t *testing.T, d *discovery.Discovery, want ...string) {
	t.Helper()
	assert.Eventually(t, func() bool { return slices.Equal(want, keys(d)) }, within, 50*time.Millisecond,
		"apps %v, want %v", keys(d), want)
}

// forbidden asserts that the backend's ServiceAccount may not list a
// resource in a namespace ("" for every namespace).
func forbidden(t *testing.T, list func(ctx context.Context) error, what string) {
	t.Helper()
	err := list(t.Context())
	require.Error(t, err, what)
	assert.True(t, apierrors.IsForbidden(err), "%s: forbidden, got %v", what, err)
}

// The discovery runs as the chart's ServiceAccount with exactly the RBAC the
// chart renders, in each mode of the namespace selector; it sees the
// namespaces the mode selects and follows them, and the API server refuses
// what the chart does not grant.
func TestTheDiscoveryUnderTheChartsRBAC(t *testing.T) {
	admin := adminClient(t)
	createWorld(t, admin)
	sel, err := labels.Parse("dash=listed")
	require.NoError(t, err)
	filter := discovery.Filter{DefaultEnable: true}

	t.Run("every namespace", func(t *testing.T) {
		applyRBAC(t, "any")
		d := runDiscovery(t, discovery.Options{Filter: filter, Namespaces: discovery.NamespaceSelector{Any: true}})
		eventuallyApps(t, d, "labelled/web", "other/web", "static/web", "team-a/web", "team-b/web")
		app := d.Apps()[0]
		assert.Equal(t, "labelled web", app.Name)
		assert.Equal(t, "https://web.labelled.example.com/", app.URL)
		assert.Equal(t, discovery.Status{State: discovery.Healthy, Ready: 1, Total: 1}, app.Status)

		c, err := kubernetes.NewForConfig(asBackend())
		require.NoError(t, err)
		forbidden(t, func(ctx context.Context) error {
			_, err := c.CoreV1().Secrets("").List(ctx, metav1.ListOptions{})
			return err
		}, "secrets")
		forbidden(t, func(ctx context.Context) error {
			_, err := c.CoreV1().ConfigMaps("").List(ctx, metav1.ListOptions{})
			return err
		}, "configmaps")
		forbidden(t, func(ctx context.Context) error {
			_, err := c.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
			return err
		}, "namespaces without a label selector")
		forbidden(t, func(ctx context.Context) error {
			_, err := c.NetworkingV1().Ingresses("other").Get(ctx, "web", metav1.GetOptions{})
			return err
		}, "get, a verb the discovery does not use")
	})

	t.Run("named namespaces", func(t *testing.T) {
		applyRBAC(t, "names")
		d := runDiscovery(t, discovery.Options{Filter: filter, Namespaces: discovery.NamespaceSelector{Names: []string{"team-a", "team-b"}}})
		eventuallyApps(t, d, "team-a/web", "team-b/web")

		c, err := kubernetes.NewForConfig(asBackend())
		require.NoError(t, err)
		forbidden(t, func(ctx context.Context) error {
			_, err := c.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
			return err
		}, "ingresses cluster-wide")
		forbidden(t, func(ctx context.Context) error {
			_, err := c.NetworkingV1().Ingresses("other").List(ctx, metav1.ListOptions{})
			return err
		}, "ingresses of a namespace not named")
		forbidden(t, func(ctx context.Context) error {
			_, err := c.CoreV1().Secrets("team-a").List(ctx, metav1.ListOptions{})
			return err
		}, "secrets of a named namespace")
	})

	t.Run("badge rules read the IngressClasses", func(t *testing.T) {
		for _, c := range []*networkingv1.IngressClass{
			{ObjectMeta: metav1.ObjectMeta{Name: "internal", Annotations: map[string]string{networkingv1.AnnotationIsDefaultIngressClass: "true"}},
				Spec: networkingv1.IngressClassSpec{Controller: "haproxy-ingress.github.io/controller"}},
		} {
			_, err := admin.NetworkingV1().IngressClasses().Create(t.Context(), c, metav1.CreateOptions{})
			if !apierrors.IsAlreadyExists(err) {
				require.NoError(t, err)
			}
		}
		applyRBAC(t, "classes")
		d := runDiscovery(t, discovery.Options{
			Filter: filter, Namespaces: discovery.NamespaceSelector{Names: []string{"team-a", "team-b"}}, IngressClasses: true,
		})
		eventuallyApps(t, d, "team-a/web", "team-b/web")
		assert.Eventually(t, func() bool {
			a := d.Apps()[0]
			return a.IngressClass == "internal" && a.IngressController == "haproxy-ingress.github.io/controller"
		}, within, 50*time.Millisecond, "the default class and its controller")

		c, err := kubernetes.NewForConfig(asBackend())
		require.NoError(t, err)
		forbidden(t, func(ctx context.Context) error {
			_, err := c.NetworkingV1().Ingresses("").List(ctx, metav1.ListOptions{})
			return err
		}, "ingresses cluster-wide, a ClusterRole on IngressClasses notwithstanding")
		forbidden(t, func(ctx context.Context) error {
			_, err := c.NetworkingV1().IngressClasses().Get(ctx, "internal", metav1.GetOptions{})
			return err
		}, "get on IngressClasses")
	})

	t.Run("a label selector", func(t *testing.T) {
		applyRBAC(t, "labels")
		d := runDiscovery(t, discovery.Options{Filter: filter, Namespaces: discovery.NamespaceSelector{Names: []string{"static"}, Labels: sel}})
		eventuallyApps(t, d, "labelled/web", "static/web")

		// A namespace that gains the label joins.
		ns, err := admin.CoreV1().Namespaces().Get(t.Context(), "team-b", metav1.GetOptions{})
		require.NoError(t, err)
		ns.Labels = map[string]string{"dash": "listed"}
		_, err = admin.CoreV1().Namespaces().Update(t.Context(), ns, metav1.UpdateOptions{})
		require.NoError(t, err)
		t.Cleanup(func() {
			ns, err := admin.CoreV1().Namespaces().Get(context.Background(), "team-b", metav1.GetOptions{})
			if assert.NoError(t, err) {
				ns.Labels = nil
				_, err = admin.CoreV1().Namespaces().Update(context.Background(), ns, metav1.UpdateOptions{})
				assert.NoError(t, err)
			}
		})
		eventuallyApps(t, d, "labelled/web", "static/web", "team-b/web")
	})
}

// The discovery follows the cluster within a bound: an Ingress created,
// opted out and deleted; its endpoints turning not ready, as a Deployment
// scaled to zero leaves them.
func TestTheDiscoveryFollowsChanges(t *testing.T) {
	admin := adminClient(t)
	createWorld(t, admin)
	applyRBAC(t, "any")
	d := runDiscovery(t, discovery.Options{Filter: discovery.Filter{DefaultEnable: true}, Namespaces: discovery.NamespaceSelector{Any: true}})
	ctx := t.Context()
	app := func(key string) discovery.App {
		for _, a := range d.Apps() {
			if a.Key() == key {
				return a
			}
		}
		return discovery.App{}
	}

	createApp(t, admin, "other", "fresh", false)
	assert.Eventually(t, func() bool { return app("other/fresh").Status.State == discovery.Down }, within, 50*time.Millisecond,
		"a new Ingress appears, its only endpoint not ready")

	_, err := admin.DiscoveryV1().EndpointSlices("other").Update(ctx, endpointSlice("fresh", true), metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return app("other/fresh").Status.State == discovery.Healthy }, within, 50*time.Millisecond)

	scaledToZero := endpointSlice("fresh", true)
	scaledToZero.Endpoints = nil
	_, err = admin.DiscoveryV1().EndpointSlices("other").Update(ctx, scaledToZero, metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return app("other/fresh").Status == discovery.Status{State: discovery.Down} }, within, 50*time.Millisecond,
		"a Service whose slice holds no endpoint is down")

	ing, err := admin.NetworkingV1().Ingresses("other").Get(ctx, "fresh", metav1.GetOptions{})
	require.NoError(t, err)
	ing.Annotations[discovery.KeyEnable] = "false"
	ing, err = admin.NetworkingV1().Ingresses("other").Update(ctx, ing, metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return app("other/fresh").Key() == "/" }, within, 50*time.Millisecond, "an opt-out removes the app")

	ing.Annotations[discovery.KeyEnable] = "true"
	_, err = admin.NetworkingV1().Ingresses("other").Update(ctx, ing, metav1.UpdateOptions{})
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return app("other/fresh").Key() == "other/fresh" }, within, 50*time.Millisecond)

	require.NoError(t, admin.NetworkingV1().Ingresses("other").Delete(ctx, "fresh", metav1.DeleteOptions{}))
	require.NoError(t, admin.DiscoveryV1().EndpointSlices("other").Delete(ctx, "fresh-1", metav1.DeleteOptions{}))
	assert.Eventually(t, func() bool { return app("other/fresh").Key() == "/" }, within, 50*time.Millisecond, "a deleted Ingress leaves")
}
