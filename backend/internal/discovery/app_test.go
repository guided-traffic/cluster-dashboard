package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// ingress is an Ingress in namespace media named grafana, with one rule for
// grafana.example.com, path / to the Service grafana, changed by each opt.
func ingress(opts ...func(*networkingv1.Ingress)) *networkingv1.Ingress {
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "media", Name: "grafana", Annotations: map[string]string{}},
		Spec: networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{
			rule("grafana.example.com", path("/", networkingv1.PathTypePrefix, "grafana")),
		}},
	}
	for _, o := range opts {
		o(ing)
	}
	return ing
}

func annotate(kv ...string) func(*networkingv1.Ingress) {
	return func(ing *networkingv1.Ingress) {
		for i := 0; i+1 < len(kv); i += 2 {
			ing.Annotations[kv[i]] = kv[i+1]
		}
	}
}

func rules(r ...networkingv1.IngressRule) func(*networkingv1.Ingress) {
	return func(ing *networkingv1.Ingress) { ing.Spec.Rules = r }
}

func tls(hosts ...string) func(*networkingv1.Ingress) {
	return func(ing *networkingv1.Ingress) {
		ing.Spec.TLS = append(ing.Spec.TLS, networkingv1.IngressTLS{Hosts: hosts, SecretName: "tls"})
	}
}

func rule(host string, paths ...networkingv1.HTTPIngressPath) networkingv1.IngressRule {
	r := networkingv1.IngressRule{Host: host}
	if len(paths) > 0 {
		r.HTTP = &networkingv1.HTTPIngressRuleValue{Paths: paths}
	}
	return r
}

func path(p string, kind networkingv1.PathType, service string) networkingv1.HTTPIngressPath {
	return networkingv1.HTTPIngressPath{
		Path:     p,
		PathType: ptr.To(kind),
		Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
			Name: service, Port: networkingv1.ServiceBackendPort{Number: 80},
		}},
	}
}

func TestTheAnnotationsAreRead(t *testing.T) {
	app, why, ignored := appOf(ingress(annotate(
		KeyAppName, " Grafana ",
		KeyGroup, "monitoring",
		KeyIcon, "si:grafana",
		KeyInfo, "Dashboards",
		KeyTargetBlank, "true",
		KeyStatusCheckEnabled, "false",
		KeyBadges, "beta@bottom-left",
	), func(ing *networkingv1.Ingress) { ing.Labels = map[string]string{"app.kubernetes.io/name": "grafana"} }), Filter{DefaultEnable: true})
	require.Nil(t, why)
	assert.Empty(t, ignored)
	assert.Equal(t, App{
		Namespace: "media", Ingress: "grafana", Name: "Grafana", Group: "monitoring",
		URL: "http://grafana.example.com/", Info: "Dashboards", TargetBlank: true, Icon: "si:grafana",
		Badges: "beta@bottom-left", Labels: map[string]string{"app.kubernetes.io/name": "grafana"},
		Services: []string{"grafana"}, StatusCheck: false,
	}, app)
	assert.Equal(t, "media/grafana", app.Key())
}

func TestTheDefaultsOfAnUnannotatedIngress(t *testing.T) {
	app, why, _ := appOf(ingress(), Filter{DefaultEnable: true})
	require.Nil(t, why)
	assert.Equal(t, "grafana", app.Name, "the Ingress's name")
	assert.Equal(t, "media", app.Group, "the namespace")
	assert.False(t, app.TargetBlank)
	assert.True(t, app.StatusCheck)
	assert.Empty(t, app.Icon)
}

// Nothing under hajimari.io/ is read (docs/adr/0010): a manifest that still
// carries only Hajimari's keys shows as if it had none.
func TestHajimarisAnnotationsChangeNothing(t *testing.T) {
	hajimari := annotate(
		"hajimari.io/enable", "true", "hajimari.io/appName", "Other", "hajimari.io/group", "other",
		"hajimari.io/icon", "web", "hajimari.io/url", "https://other.example.com", "hajimari.io/info", "x",
		"hajimari.io/targetBlank", "true", "hajimari.io/instance", "home", "hajimari.io/statusCheckEnabled", "false",
	)
	for _, def := range []bool{false, true} {
		plain, plainWhy, _ := appOf(ingress(), Filter{DefaultEnable: def})
		got, gotWhy, _ := appOf(ingress(hajimari), Filter{DefaultEnable: def})
		assert.Equal(t, plainWhy == nil, gotWhy == nil, "defaultEnable %v", def)
		assert.Equal(t, plain, got, "defaultEnable %v", def)
	}
	_, why, _ := appOf(ingress(hajimari), Filter{DefaultEnable: true, InstanceName: "home"})
	assert.NotNil(t, why, "hajimari.io/instance names no instance")
}

// Hajimari's selection: with defaultEnable only "false" hides, without it
// only "true" shows; with an instance name only an Ingress that lists it.
func TestSelection(t *testing.T) {
	for _, tc := range []struct {
		defaultEnable bool
		enable        string // "" unset
		instanceName  string
		instance      string // "" unset
		shown         bool
	}{
		{defaultEnable: false, shown: false},
		{defaultEnable: false, enable: "true", shown: true},
		{defaultEnable: false, enable: "false", shown: false},
		{defaultEnable: false, enable: "yes", shown: false},
		{defaultEnable: true, shown: true},
		{defaultEnable: true, enable: "false", shown: false},
		{defaultEnable: true, enable: "FALSE", shown: false},
		{defaultEnable: true, enable: "0", shown: false},
		{defaultEnable: true, enable: "true", shown: true},
		{defaultEnable: true, enable: "maybe", shown: true},
		{defaultEnable: true, instance: "home", shown: true},
		{defaultEnable: true, instanceName: "home", shown: false},
		{defaultEnable: true, instanceName: "home", instance: "home", shown: true},
		{defaultEnable: true, instanceName: "home", instance: "office, home", shown: true},
		{defaultEnable: true, instanceName: "home", instance: "office", shown: false},
		{defaultEnable: true, instanceName: "home", instance: "homes", shown: false},
		{defaultEnable: false, enable: "true", instanceName: "home", instance: "home", shown: true},
		{defaultEnable: true, enable: "false", instanceName: "home", instance: "home", shown: false},
	} {
		ing := ingress()
		if tc.enable != "" {
			ing.Annotations[KeyEnable] = tc.enable
		}
		if tc.instance != "" {
			ing.Annotations[KeyInstance] = tc.instance
		}
		_, why, _ := appOf(ing, Filter{DefaultEnable: tc.defaultEnable, InstanceName: tc.instanceName})
		assert.Equal(t, tc.shown, why == nil, "%+v", tc)
		if why != nil {
			assert.False(t, why.notable, "the selection working is no warning: %+v", tc)
		}
	}
}

func TestAnUnreadableBooleanIsIgnoredAndNamed(t *testing.T) {
	app, _, ignored := appOf(ingress(annotate(KeyTargetBlank, "sometimes", KeyStatusCheckEnabled, "no")), Filter{DefaultEnable: true})
	assert.False(t, app.TargetBlank)
	assert.True(t, app.StatusCheck)
	assert.Equal(t, []ignoredKey{
		{key: KeyTargetBlank, reason: "not true or false"},
		{key: KeyStatusCheckEnabled, reason: "not true or false"},
	}, ignored)
}

func TestTheURLAnnotation(t *testing.T) {
	for raw, want := range map[string]string{
		"https://grafana.example.com/d/home": "https://grafana.example.com/d/home",
		"http://10.0.0.7:3000":               "http://10.0.0.7:3000",
		" https://x.example.com ":            "https://x.example.com",
	} {
		app, why, ignored := appOf(ingress(annotate(KeyURL, raw)), Filter{DefaultEnable: true})
		require.Nil(t, why, raw)
		assert.Empty(t, ignored, raw)
		assert.Equal(t, want, app.URL, raw)
	}
	// Hajimari's url.ParseRequestURI takes any scheme; Dash takes http and
	// https and falls back to the derived URL, naming the key, never the value.
	for raw, reason := range map[string]string{
		"javascript:alert(1)":           "not an absolute http or https URL",
		"ftp://files.example.com":       "not an absolute http or https URL",
		"/relative":                     "not an absolute http or https URL",
		"https://user:pw@x.example.com": "holds a user or password, which every viewer would see",
	} {
		app, why, ignored := appOf(ingress(annotate(KeyURL, raw)), Filter{DefaultEnable: true})
		require.Nil(t, why, raw)
		assert.Equal(t, "http://grafana.example.com/", app.URL, raw)
		assert.Equal(t, []ignoredKey{{key: KeyURL, reason: reason}}, ignored, raw)
	}
}

func TestAnIngressWithoutAnAddressIsSkippedNotably(t *testing.T) {
	_, why, _ := appOf(ingress(rules(rule("*.example.com"))), Filter{DefaultEnable: true})
	require.NotNil(t, why)
	assert.True(t, why.notable)
	assert.Equal(t, "no rule names a concrete host; dash.gtrfc.com/url can name the address", why.reason)

	app, why, _ := appOf(ingress(rules(rule("*.example.com")), annotate(KeyURL, "https://x.example.com")), Filter{DefaultEnable: true})
	require.Nil(t, why, "the url key names the address")
	assert.Equal(t, "https://x.example.com", app.URL)

	_, why, _ = appOf(ingress(rules(rule("*.example.com")), annotate(KeyURL, "ftp://x")), Filter{DefaultEnable: true})
	assert.NotNil(t, why, "an unusable url key and no concrete host")
}

func TestDerivedURL(t *testing.T) {
	prefix, exact, impl := networkingv1.PathTypePrefix, networkingv1.PathTypeExact, networkingv1.PathTypeImplementationSpecific
	for name, tc := range map[string]struct {
		ing  *networkingv1.Ingress
		want string
	}{
		"http without TLS":        {ingress(), "http://grafana.example.com/"},
		"https when TLS names it": {ingress(tls("grafana.example.com")), "https://grafana.example.com/"},
		"https by a wildcard":     {ingress(tls("*.example.com")), "https://grafana.example.com/"},
		"a wildcard of another domain": {
			ingress(tls("*.other.com")), "http://grafana.example.com/",
		},
		"a wildcard covers one label only": {
			ingress(rules(rule("a.b.example.com")), tls("*.example.com")), "http://a.b.example.com/",
		},
		"TLS blocks in another order than the rules": {
			ingress(rules(rule("a.example.com"), rule("b.example.com")), tls("b.example.com"), tls("a.example.com")),
			"https://a.example.com/",
		},
		"Hajimari's tls[0].hosts[0] would take the other host": {
			ingress(rules(rule("a.example.com")), tls("b.example.com", "a.example.com")), "https://a.example.com/",
		},
		"a wildcard rule is passed over": {
			ingress(rules(rule("*.example.com"), rule("c.example.com"))), "http://c.example.com/",
		},
		"a rule without a host is passed over": {
			ingress(rules(rule("", path("/", prefix, "s")), rule("d.example.com"))), "http://d.example.com/",
		},
		"the first Prefix path": {
			ingress(rules(rule("e.example.com", path("/grafana", prefix, "s"), path("/other", prefix, "s")))), "http://e.example.com/grafana",
		},
		"an Exact path": {
			ingress(rules(rule("e.example.com", path("/login", exact, "s")))), "http://e.example.com/login",
		},
		"a trailing slash is kept": {
			ingress(rules(rule("e.example.com", path("/grafana/", prefix, "s")))), "http://e.example.com/grafana/",
		},
		"an ImplementationSpecific path is passed over": {
			ingress(rules(rule("e.example.com", path("/x", impl, "s"), path("/y", prefix, "s")))), "http://e.example.com/y",
		},
		"only ImplementationSpecific paths": {
			ingress(rules(rule("e.example.com", path("/x", impl, "s")))), "http://e.example.com/",
		},
		"a regular expression": {
			ingress(rules(rule("e.example.com", path("/api(/|$)(.*)", prefix, "s")))), "http://e.example.com/",
		},
		"a rule without paths": {ingress(rules(rule("f.example.com"))), "http://f.example.com/"},
		"a host in upper case in TLS": {
			ingress(rules(rule("g.example.com")), tls("G.Example.com")), "https://g.example.com/",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := derivedURL(tc.ing.Spec)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
	for name, spec := range map[string]networkingv1.IngressSpec{
		"no rules":             {},
		"only a wildcard":      ingress(rules(rule("*.example.com"))).Spec,
		"only a hostless rule": ingress(rules(rule("", path("/", prefix, "s")))).Spec,
		"a default backend only": {DefaultBackend: &networkingv1.IngressBackend{
			Service: &networkingv1.IngressServiceBackend{Name: "s"},
		}},
	} {
		_, err := derivedURL(spec)
		assert.ErrorIs(t, err, errNoHost, name)
	}
}

func TestBackendServices(t *testing.T) {
	ing := ingress(rules(
		rule("a.example.com", path("/", networkingv1.PathTypePrefix, "web"), path("/api", networkingv1.PathTypePrefix, "api")),
		rule("b.example.com", path("/", networkingv1.PathTypePrefix, "web")),
		rule("c.example.com"),
	))
	ing.Spec.DefaultBackend = &networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{Name: "fallback"}}
	ing.Spec.Rules[1].HTTP.Paths = append(ing.Spec.Rules[1].HTTP.Paths, networkingv1.HTTPIngressPath{
		Path: "/bucket", Backend: networkingv1.IngressBackend{Resource: &corev1.TypedLocalObjectReference{Kind: "Bucket", Name: "b"}},
	})
	assert.Equal(t, []string{"api", "fallback", "web"}, backendServices(ing.Spec))
	assert.Empty(t, backendServices(networkingv1.IngressSpec{}))
}

// endpoint is a pod's endpoint; ready nil leaves the condition unset.
func endpoint(pod string, ready, terminating *bool, addresses ...string) discoveryv1.Endpoint {
	ep := discoveryv1.Endpoint{
		Addresses:  addresses,
		Conditions: discoveryv1.EndpointConditions{Ready: ready, Terminating: terminating},
	}
	if pod != "" {
		ep.TargetRef = &corev1.ObjectReference{Kind: "Pod", Name: pod}
	}
	return ep
}

func endpointSlice(name string, eps ...discoveryv1.Endpoint) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Namespace: "media", Name: name, Labels: map[string]string{discoveryv1.LabelServiceName: "grafana"}},
		Endpoints:  eps,
	}
}

func TestStatus(t *testing.T) {
	yes, no := ptr.To(true), ptr.To(false)
	for name, tc := range map[string]struct {
		slices []*discoveryv1.EndpointSlice
		want   Status
	}{
		"no slice": {nil, Status{State: Unknown}},
		"a slice without endpoints, scaled to zero": {
			[]*discoveryv1.EndpointSlice{endpointSlice("a")}, Status{State: Down},
		},
		"all ready": {
			[]*discoveryv1.EndpointSlice{endpointSlice("a", endpoint("p1", yes, nil, "10.0.0.1"), endpoint("p2", yes, nil, "10.0.0.2"))},
			Status{State: Healthy, Ready: 2, Total: 2},
		},
		"some ready": {
			[]*discoveryv1.EndpointSlice{endpointSlice("a", endpoint("p1", yes, nil, "10.0.0.1"), endpoint("p2", no, nil, "10.0.0.2"), endpoint("p3", no, nil, "10.0.0.3"))},
			Status{State: Degraded, Ready: 1, Total: 3},
		},
		"none ready": {
			[]*discoveryv1.EndpointSlice{endpointSlice("a", endpoint("p1", no, nil, "10.0.0.1"))},
			Status{State: Down, Total: 1},
		},
		"a nil ready condition is ready": {
			[]*discoveryv1.EndpointSlice{endpointSlice("a", endpoint("p1", nil, nil, "10.0.0.1"))},
			Status{State: Healthy, Ready: 1, Total: 1},
		},
		// Hajimari sums every slice: one pod of a dual-stack Service counts twice.
		"dual stack counts each pod once": {
			[]*discoveryv1.EndpointSlice{
				endpointSlice("v4", endpoint("p1", yes, nil, "10.0.0.1"), endpoint("p2", no, nil, "10.0.0.2")),
				endpointSlice("v6", endpoint("p1", yes, nil, "fd00::1"), endpoint("p2", no, nil, "fd00::2")),
			},
			Status{State: Degraded, Ready: 1, Total: 2},
		},
		"a terminating endpoint is not counted": {
			[]*discoveryv1.EndpointSlice{endpointSlice("a", endpoint("p1", yes, nil, "10.0.0.1"), endpoint("old", no, yes, "10.0.0.9"))},
			Status{State: Healthy, Ready: 1, Total: 1},
		},
		"endpoints without a target count by address": {
			[]*discoveryv1.EndpointSlice{endpointSlice("a", endpoint("", yes, nil, "10.0.0.1"), endpoint("", no, nil, "10.0.0.2"))},
			Status{State: Degraded, Ready: 1, Total: 2},
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, statusOf(tc.slices))
		})
	}
}

func TestKnownAs(t *testing.T) {
	slice := endpointSlice("a")
	slice.Labels["app.kubernetes.io/name"] = "grafana"
	slice.Labels["helm.sh/chart"] = "kube-prometheus-stack-61.3.2"
	app := App{Name: "Dashboards", Ingress: "grafana-ui", Labels: map[string]string{
		"app.kubernetes.io/name": "grafana", "helm.sh/chart": "grafana-8.5.1",
	}}
	assert.Equal(t, []string{"Dashboards", "grafana-ui", "grafana", "kube-prometheus-stack"},
		knownAs(app, []*discoveryv1.EndpointSlice{slice}), "each once, the most specific first; the chart without its version")
	assert.Equal(t, []string{"web"}, knownAs(App{Name: "web", Ingress: "web"}, nil))
	for chart, want := range map[string]string{
		"grafana-8.5.1":            "grafana",
		"app-template-v3.2.1":      "app-template",
		"loki-6.0.0-rc.1":          "loki",
		"cert-manager-v1.21.3+abc": "cert-manager",
		"no-version":               "no-version",
	} {
		assert.Equal(t, want, chartVersion.ReplaceAllString(chart, ""), chart)
	}
}
