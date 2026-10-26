package discovery

import (
	"fmt"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/guided-traffic/dash/backend/internal/config"
)

// NamespaceSelector is where Ingresses are read (docs/adr/0007 D2), as
// Hajimari selects: every namespace, or the names united with the
// namespaces a label selector matches.
type NamespaceSelector struct {
	Any   bool
	Names []string
	// Labels selects namespaces by their labels; nil selects none by label.
	Labels labels.Selector
}

// mode is how the informers are laid out for a selector, which is also how
// the chart lays out the RBAC (deploy/helm/dash/templates/rbac.yaml).
type mode int

const (
	// clusterWide: one set of informers over every namespace — any.
	clusterWide mode = iota
	// perNamespace: a set of informers per named namespace, each needing a
	// Role in its namespace only — names without a label selector.
	perNamespace
	// byLabels: one set over every namespace, filtered by the names and the
	// labels of a watched Namespace — a label selector.
	byLabels
)

func (s NamespaceSelector) mode() mode {
	switch {
	case s.Any:
		return clusterWide
	case s.Labels == nil:
		return perNamespace
	default:
		return byLabels
	}
}

// selects reports whether the namespace named name, with nsLabels, is
// selected.
func (s NamespaceSelector) selects(name string, nsLabels map[string]string) bool {
	if s.Any || slices.Contains(s.Names, name) {
		return true
	}
	return s.Labels != nil && s.Labels.Matches(labels.Set(nsLabels))
}

// Options configure New.
type Options struct {
	Filter     Filter
	Namespaces NamespaceSelector
	// RegisterIcon makes an Ingress's dash.gtrfc.com/icon known when the
	// Ingress is read — a URL starts its download — and tells why it will
	// not resolve: such an annotation is ignored with a log line, and the
	// app falls back. nil takes every reference.
	RegisterIcon func(ref string) error
}

// OptionsFrom reads the discovery's options from the configuration, which
// config.Load validated: the label selector converts.
func OptionsFrom(d config.Discovery) (Options, error) {
	opts := Options{
		Filter: Filter{DefaultEnable: d.DefaultEnable, InstanceName: d.InstanceName},
		Namespaces: NamespaceSelector{
			Any:   d.NamespaceSelector.Any,
			Names: slices.Clone(d.NamespaceSelector.MatchNames),
		},
	}
	if ls := d.NamespaceSelector.LabelSelector; !opts.Namespaces.Any && !ls.IsEmpty() {
		sel, err := metav1.LabelSelectorAsSelector(LabelSelector(ls))
		if err != nil {
			return Options{}, fmt.Errorf("discovery.namespaceSelector.labelSelector: %w", err)
		}
		opts.Namespaces.Labels = sel
	}
	return opts, nil
}

// LabelSelector is the configuration's label selector as the API's.
func LabelSelector(s config.LabelSelector) *metav1.LabelSelector {
	out := &metav1.LabelSelector{MatchLabels: s.MatchLabels}
	for _, e := range s.MatchExpressions {
		out.MatchExpressions = append(out.MatchExpressions, metav1.LabelSelectorRequirement{
			Key:      e.Key,
			Operator: metav1.LabelSelectorOperator(e.Operator),
			Values:   e.Values,
		})
	}
	return out
}
