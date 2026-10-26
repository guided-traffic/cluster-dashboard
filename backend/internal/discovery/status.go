package discovery

import (
	"slices"
	"strings"

	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

// State is an app's readiness, in Cogpit's terms: a healthy app draws no
// colour, a degraded or down one lights up (docs/adr/0005).
type State string

// The states of an app.
const (
	// Healthy: every endpoint ready.
	Healthy State = "healthy"
	// Degraded: some endpoints ready, not all.
	Degraded State = "degraded"
	// Down: the app's Services have EndpointSlices, and no endpoint in them
	// is ready — a Deployment scaled to zero among them.
	Down State = "down"
	// Unknown: the status check is off, or none of the app's Services has
	// an EndpointSlice (an ExternalName Service, a Service without a
	// selector, a backend that is no Service).
	Unknown State = "unknown"
)

// Status is an app's readiness and the counts it is made of.
type Status struct {
	State State
	// Ready and Total count endpoints, each pod once.
	Ready, Total int
}

// backendServices are the Services an Ingress sends traffic to: its default
// backend's and every path's, each once, sorted.
func backendServices(spec networkingv1.IngressSpec) []string {
	var names []string
	if b := spec.DefaultBackend; b != nil && b.Service != nil {
		names = append(names, b.Service.Name)
	}
	for _, rule := range spec.Rules {
		if rule.HTTP == nil {
			continue
		}
		for _, p := range rule.HTTP.Paths {
			if p.Backend.Service != nil {
				names = append(names, p.Backend.Service.Name)
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// statusOf counts the endpoints of the EndpointSlices of an app's Services
// (docs/adr/0007 D4). Hajimari sums every slice and rounds the ratio to 0 or
// 1; here a pod is counted once — a dual-stack Service has a slice per
// address family, so the same pod twice — by the endpoint's targetRef, and a
// terminating endpoint, which a rollout leaves behind for a moment, is not
// counted at all.
func statusOf(endpointSlices []*discoveryv1.EndpointSlice) Status {
	if len(endpointSlices) == 0 {
		return Status{State: Unknown}
	}
	ready := map[string]bool{}
	for _, slice := range endpointSlices {
		for _, ep := range slice.Endpoints {
			if ep.Conditions.Terminating != nil && *ep.Conditions.Terminating {
				continue
			}
			key := endpointKey(slice, ep)
			// A nil ready condition is ready (the API's reading of it).
			ready[key] = ready[key] || ep.Conditions.Ready == nil || *ep.Conditions.Ready
		}
	}
	s := Status{Total: len(ready)}
	for _, r := range ready {
		if r {
			s.Ready++
		}
	}
	switch {
	case s.Ready == 0:
		s.State = Down
	case s.Ready < s.Total:
		s.State = Degraded
	default:
		s.State = Healthy
	}
	return s
}

// endpointKey is what makes two endpoints one: the object behind them (the
// pod), else the addresses, within the slice's namespace.
func endpointKey(slice *discoveryv1.EndpointSlice, ep discoveryv1.Endpoint) string {
	if ref := ep.TargetRef; ref != nil {
		ns := ref.Namespace
		if ns == "" {
			ns = slice.Namespace
		}
		return ref.Kind + "/" + ns + "/" + ref.Name
	}
	return "address/" + slice.Namespace + "/" + strings.Join(ep.Addresses, ",")
}
