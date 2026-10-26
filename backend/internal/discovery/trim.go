package discovery

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The transforms trim each object before the informer's cache keeps it, to
// what discovery reads — as Cogpit's cache trims (internal/cache/trim.go):
// no managed fields, no last-applied configuration, no status of an Ingress,
// no ports or topology of an endpoint. The caches of a large cluster stay
// small, and the backend holds nothing it does not use: of an Ingress's
// annotations, only Dash's own, the legacy class and those the badge rules
// read; of an IngressClass, its controller and whether it is the default.

// trimmer is the informers' transform: it keeps of an Ingress's
// annotations Dash's own, the legacy class and those the badge rules read
// (keep). An object of another type — a deleted object whose last state is
// unknown — passes unchanged.
func trimmer(keep []string) func(obj any) (any, error) {
	return func(obj any) (any, error) {
		switch o := obj.(type) {
		case *networkingv1.Ingress:
			return trimIngress(o, keep), nil
		case *discoveryv1.EndpointSlice:
			return trimEndpointSlice(o), nil
		case *corev1.Namespace:
			return &corev1.Namespace{ObjectMeta: trimMeta(o.ObjectMeta, nil)}, nil
		case *networkingv1.IngressClass:
			return trimIngressClass(o), nil
		default:
			return obj, nil
		}
	}
}

// trim is trimmer without keys of the rules.
var trim = trimmer(nil)

func trimIngress(in *networkingv1.Ingress, keep []string) *networkingv1.Ingress {
	kept := map[string]string{}
	for k, v := range in.Annotations {
		if strings.HasPrefix(k, Prefix) || k == LegacyIngressClass || slices.Contains(keep, k) {
			kept[k] = v
		}
	}
	return &networkingv1.Ingress{ObjectMeta: trimMeta(in.ObjectMeta, kept), Spec: in.Spec}
}

// trimIngressClass keeps a class's name, whether it is the default, and its
// controller.
func trimIngressClass(in *networkingv1.IngressClass) *networkingv1.IngressClass {
	var kept map[string]string
	if v, ok := in.Annotations[networkingv1.AnnotationIsDefaultIngressClass]; ok {
		kept = map[string]string{networkingv1.AnnotationIsDefaultIngressClass: v}
	}
	return &networkingv1.IngressClass{ObjectMeta: trimMeta(in.ObjectMeta, kept), Spec: networkingv1.IngressClassSpec{Controller: in.Spec.Controller}}
}

func trimEndpointSlice(in *discoveryv1.EndpointSlice) *discoveryv1.EndpointSlice {
	out := &discoveryv1.EndpointSlice{ObjectMeta: trimMeta(in.ObjectMeta, nil), AddressType: in.AddressType}
	if len(in.Endpoints) > 0 {
		out.Endpoints = make([]discoveryv1.Endpoint, len(in.Endpoints))
	}
	for i, ep := range in.Endpoints {
		out.Endpoints[i] = discoveryv1.Endpoint{Addresses: ep.Addresses, Conditions: ep.Conditions}
		if ref := ep.TargetRef; ref != nil {
			out.Endpoints[i].TargetRef = &corev1.ObjectReference{Kind: ref.Kind, Namespace: ref.Namespace, Name: ref.Name}
		}
	}
	return out
}

// trimMeta keeps what identifies an object, its labels and the annotations
// given.
func trimMeta(in metav1.ObjectMeta, annotations map[string]string) metav1.ObjectMeta {
	out := metav1.ObjectMeta{
		Name:            in.Name,
		Namespace:       in.Namespace,
		UID:             in.UID,
		ResourceVersion: in.ResourceVersion,
		Labels:          in.Labels,
	}
	if len(annotations) > 0 {
		out.Annotations = annotations
	}
	return out
}
