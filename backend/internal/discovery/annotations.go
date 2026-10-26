// Package discovery finds the apps a cluster serves through its Ingresses,
// with Hajimari's semantics where they hold and fixed where they do not
// (docs/adr/0007): it watches Ingresses and EndpointSlices — and Namespaces
// for a label selector — through informers, reads each Ingress's
// dash.gtrfc.com/* annotations (docs/adr/0010), selects, derives the app's
// URL and counts its ready endpoints. The API reads the informers' caches,
// so a page view costs the API server nothing.
package discovery

import (
	"strconv"
	"strings"
)

// Prefix is the only annotation prefix discovery reads (docs/adr/0010):
// hajimari.io/* is not read.
const Prefix = "dash.gtrfc.com/"

// The annotation keys, Hajimari's names under Dash's prefix, plus badges.
const (
	KeyEnable             = Prefix + "enable"
	KeyAppName            = Prefix + "appName"
	KeyGroup              = Prefix + "group"
	KeyIcon               = Prefix + "icon"
	KeyURL                = Prefix + "url"
	KeyInfo               = Prefix + "info"
	KeyTargetBlank        = Prefix + "targetBlank"
	KeyInstance           = Prefix + "instance"
	KeyStatusCheckEnabled = Prefix + "statusCheckEnabled"
	KeyBadges             = Prefix + "badges"
)

// LegacyIngressClass is the annotation that named an Ingress's class before
// spec.ingressClassName; badge rules read it (docs/adr/0011 D4).
const LegacyIngressClass = "kubernetes.io/ingress.class"

// annotations are an Ingress's dash.gtrfc.com/* keys, read. A key that is
// absent, or whose value does not parse, is nil or empty, and its default
// applies.
type annotations struct {
	enable             *bool
	appName            string
	group              string
	icon               string
	url                string
	info               string
	targetBlank        *bool
	instances          []string
	statusCheckEnabled *bool
	badges             string
	// ignored names each key whose value was not used, and why: for the log,
	// never for the API.
	ignored []ignoredKey
}

type ignoredKey struct {
	key    string
	reason string
}

// readAnnotations reads the dash.gtrfc.com/* keys of an Ingress. Text is
// trimmed; a boolean is what strconv.ParseBool reads (true, false, 1, 0, …)
// and is ignored otherwise.
func readAnnotations(a map[string]string) annotations {
	var r annotations
	r.enable = r.boolean(a, KeyEnable)
	r.targetBlank = r.boolean(a, KeyTargetBlank)
	r.statusCheckEnabled = r.boolean(a, KeyStatusCheckEnabled)
	r.appName = strings.TrimSpace(a[KeyAppName])
	r.group = strings.TrimSpace(a[KeyGroup])
	r.icon = strings.TrimSpace(a[KeyIcon])
	r.url = strings.TrimSpace(a[KeyURL])
	r.info = strings.TrimSpace(a[KeyInfo])
	r.badges = strings.TrimSpace(a[KeyBadges])
	if v, ok := a[KeyInstance]; ok {
		for name := range strings.SplitSeq(v, ",") {
			if name = strings.TrimSpace(name); name != "" {
				r.instances = append(r.instances, name)
			}
		}
	}
	return r
}

func (r *annotations) boolean(a map[string]string, key string) *bool {
	v, ok := a[key]
	if !ok {
		return nil
	}
	b, err := strconv.ParseBool(strings.TrimSpace(v))
	if err != nil {
		r.ignored = append(r.ignored, ignoredKey{key: key, reason: "not true or false"})
		return nil
	}
	return &b
}

// orDefault is b's value, or def when b is nil.
func orDefault(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}
