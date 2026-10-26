package discovery

import (
	"regexp"
	"slices"

	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// App is an app an Ingress describes.
type App struct {
	// Namespace and Ingress name the Ingress.
	Namespace string
	Ingress   string
	// Name is dash.gtrfc.com/appName, else the Ingress's name.
	Name string
	// Group is dash.gtrfc.com/group, else the namespace.
	Group string
	// URL is dash.gtrfc.com/url, else derived from the rules.
	URL         string
	Info        string
	TargetBlank bool
	// Icon is the dash.gtrfc.com/icon reference as written, "" when unset;
	// resolving it is the icons package's.
	Icon string
	// Badges is dash.gtrfc.com/badges as written; parsing it is the badges
	// package's.
	Badges string
	// Labels are the Ingress's labels, for auto-match and badge rules.
	Labels map[string]string
	// Services are the Services the Ingress sends traffic to.
	Services []string
	// Names are what the app is known by, the most specific first, for
	// auto-match: its name, the Ingress's, app.kubernetes.io/name of the
	// Ingress and of its Services' EndpointSlices (which carry the Service's
	// labels), and helm.sh/chart without its version. Filled when the apps
	// are read.
	Names []string
	// StatusCheck is dash.gtrfc.com/statusCheckEnabled, true by default.
	StatusCheck bool
	// Status is the readiness at the moment the app was read.
	Status Status
}

// Key is the Ingress's namespace/name: the app's identity.
func (a App) Key() string {
	return a.Namespace + "/" + a.Ingress
}

// Filter is the part of the selection that an Ingress's annotations decide
// (docs/adr/0007 D2); which namespaces are read is NamespaceSelector's.
type Filter struct {
	// DefaultEnable shows an Ingress unless dash.gtrfc.com/enable is false;
	// off, only when it is true.
	DefaultEnable bool
	// InstanceName, when set, shows only an Ingress whose
	// dash.gtrfc.com/instance lists it.
	InstanceName string
}

// skip is why an Ingress is no app. A notable skip is the Ingress author's
// mistake and is logged as a warning; the others are the selection working.
type skip struct {
	reason  string
	notable bool
}

// appOf reads the app an Ingress describes. It returns the app, or why the
// Ingress is none; and the annotations it ignored, for the log.
func appOf(ing *networkingv1.Ingress, f Filter) (App, *skip, []ignoredKey) {
	a := readAnnotations(ing.Annotations)
	if !orDefault(a.enable, f.DefaultEnable) {
		return App{}, &skip{reason: "not enabled: dash.gtrfc.com/enable"}, a.ignored
	}
	if f.InstanceName != "" && !slices.Contains(a.instances, f.InstanceName) {
		return App{}, &skip{reason: "another instance: dash.gtrfc.com/instance does not list this one"}, a.ignored
	}
	link := ""
	if a.url != "" {
		u, err := overrideURL(a.url)
		if err != nil {
			a.ignored = append(a.ignored, ignoredKey{key: KeyURL, reason: err.Error()})
		}
		link = u
	}
	if link == "" {
		u, err := derivedURL(ing.Spec)
		if err != nil {
			return App{}, &skip{reason: err.Error(), notable: true}, a.ignored
		}
		link = u
	}
	app := App{
		Namespace:   ing.Namespace,
		Ingress:     ing.Name,
		Name:        a.appName,
		Group:       a.group,
		URL:         link,
		Info:        a.info,
		TargetBlank: orDefault(a.targetBlank, false),
		Icon:        a.icon,
		Badges:      a.badges,
		Labels:      ing.Labels,
		Services:    backendServices(ing.Spec),
		StatusCheck: orDefault(a.statusCheckEnabled, true),
	}
	if app.Name == "" {
		app.Name = ing.Name
	}
	if app.Group == "" {
		app.Group = ing.Namespace
	}
	return app, nil, a.ignored
}

// The labels auto-match reads names from.
const (
	labelName  = "app.kubernetes.io/name"
	labelChart = "helm.sh/chart"
)

// chartVersion is the version helm.sh/chart ends in: grafana-8.5.1.
var chartVersion = regexp.MustCompile(`-v?[0-9]+(\.[0-9]+)*([-+][0-9A-Za-z.-]*)?$`)

// knownAs are the names an app is known by, the most specific first, each once.
func knownAs(a App, endpointSlices []*discoveryv1.EndpointSlice) []string {
	out := make([]string, 0, 4+2*len(endpointSlices))
	out = append(out, a.Name, a.Ingress, a.Labels[labelName])
	for _, s := range endpointSlices {
		out = append(out, s.Labels[labelName])
	}
	out = append(out, chartVersion.ReplaceAllString(a.Labels[labelChart], ""))
	for _, s := range endpointSlices {
		out = append(out, chartVersion.ReplaceAllString(s.Labels[labelChart], ""))
	}
	seen := map[string]bool{"": true}
	return slices.DeleteFunc(out, func(n string) bool {
		if seen[n] {
			return true
		}
		seen[n] = true
		return false
	})
}
