package config

import (
	"fmt"
	"maps"
	"net"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/guided-traffic/dash/backend/internal/badges"
	"github.com/guided-traffic/dash/backend/internal/icons"
)

// The bounds of the values the schema of the YAML file cannot hold: the same
// as values.schema.json's.
const (
	maxTextLength     = 64
	maxInfoLength     = 120
	minRefreshSeconds = 5
	maxRefreshSeconds = 3600
	minRemoteBytes    = 1 << 10
	maxRemoteBytes    = 4 << 20
	minRemoteTimeout  = time.Second
	maxRemoteTimeout  = 2 * time.Minute
	maxRemoteEntries  = 10000
)

// validator holds the decoded file to what its types cannot say: values from
// a set, bounds, names that must exist, combinations that contradict each
// other. A problem names the key's path and its line; it never echoes the
// value, which can be a URL with a token in it.
type validator struct {
	lines    lines
	failed   map[string]bool
	problems []problem
	// remoteOff refuses an icon URL: icons.remote.enabled is false.
	remoteOff bool
}

// validate checks f, which d decoded (nil: no file was read).
func validate(f *File, d *decoder) []problem {
	v := &validator{lines: lines{}, failed: map[string]bool{}}
	if d != nil {
		v.lines, v.failed = d.lines, d.failed
	}
	v.discovery(&f.Discovery)
	v.ui(&f.UI)
	v.remoteOff = !f.Icons.Remote.Enabled
	v.apps(f.Apps, f.Badges.Definitions)
	v.bookmarks(f.Bookmarks, f.Badges.Definitions)
	v.icons(&f.Icons)
	v.badges(&f.Badges)
	return v.problems
}

// add records a problem at p, unless the decoder could not read p or a key
// that holds it: that problem is reported already.
func (v *validator) add(p, message string) {
	for q := p; ; q = parent(q) {
		if v.failed[q] {
			return
		}
		if q == "" {
			break
		}
	}
	v.problems = append(v.problems, problem{line: v.lines.of(p), path: p, message: message})
}

func (v *validator) discovery(d *Discovery) {
	const p = "discovery.namespaceSelector"
	if strings.Contains(d.InstanceName, ",") {
		v.add("discovery.instanceName", "must not hold ',', which separates the names of dash.gtrfc.com/instance")
	}
	sel := d.NamespaceSelector
	v.each(p+".matchNames", sel.MatchNames, isDNS1123Label, "must be a namespace name: lower case letters, digits and '-', at most 63 characters")
	v.labelSelector(p+".labelSelector", sel.LabelSelector)
	byNameOrLabel := len(sel.MatchNames) > 0 || !sel.LabelSelector.IsEmpty()
	switch {
	case sel.Any && byNameOrLabel:
		v.add(p, "any is true, which would ignore matchNames and labelSelector: set any to false to select namespaces by name or label")
	case !sel.Any && !byNameOrLabel && d.Enabled:
		v.add(p, "selects no namespace: set any to true, name namespaces in matchNames or select them with labelSelector, or set discovery.enabled to false")
	}
}

// labelSelector checks a selector as the API server would, without echoing
// a key or a value.
func (v *validator) labelSelector(p string, s LabelSelector) {
	for _, key := range slices.Sorted(maps.Keys(s.MatchLabels)) {
		kp := mapKey(p+".matchLabels", key)
		v.labelKey(kp, key)
		if len(validation.IsValidLabelValue(s.MatchLabels[key])) > 0 {
			v.add(kp, "must be a label value: at most 63 letters, digits, '-', '_' or '.', starting and ending with a letter or digit")
		}
	}
	for i, e := range s.MatchExpressions {
		ep := fmt.Sprintf("%s.matchExpressions[%d]", p, i)
		v.labelKey(ep+".key", e.Key)
		switch e.Operator {
		case "In", "NotIn":
			if len(e.Values) == 0 {
				v.add(ep+".values", "must name at least one value for In and NotIn")
			}
		case "Exists", "DoesNotExist":
			if len(e.Values) > 0 {
				v.add(ep+".values", "must be empty for Exists and DoesNotExist")
			}
		default:
			v.add(ep+".operator", "must be In, NotIn, Exists or DoesNotExist")
		}
		for j, value := range e.Values {
			if len(validation.IsValidLabelValue(value)) > 0 {
				v.add(fmt.Sprintf("%s.values[%d]", ep, j), "must be a label value: at most 63 letters, digits, '-', '_' or '.', starting and ending with a letter or digit")
			}
		}
	}
}

func (v *validator) labelKey(p, key string) {
	if len(validation.IsQualifiedName(key)) > 0 {
		v.add(p, "must be a label key such as app or app.kubernetes.io/name")
	}
}

func (v *validator) ui(u *UI) {
	v.text("ui.title", u.Title, true)
	v.text("ui.greeting.name", u.Greeting.Name, false)
	if !slices.Contains(themes, u.DefaultTheme) {
		v.add("ui.defaultTheme", "must be system, light or dark")
	}
	if u.RefreshSeconds < minRefreshSeconds || u.RefreshSeconds > maxRefreshSeconds {
		v.add("ui.refreshSeconds", fmt.Sprintf("must be from %d to %d", minRefreshSeconds, maxRefreshSeconds))
	}
	v.unique("ui.groupOrder", u.GroupOrder, "a group")
}

// text checks a name or a title: at most maxTextLength characters, and not
// empty when required.
func (v *validator) text(p, s string, required bool) {
	switch {
	case required && strings.TrimSpace(s) == "":
		v.add(p, "must not be empty")
	case utf8.RuneCountInString(s) > maxTextLength:
		v.add(p, fmt.Sprintf("must be at most %d characters", maxTextLength))
	}
}

// unique checks a list of names: none empty, none twice.
func (v *validator) unique(p string, names []string, what string) {
	first := map[string]int{}
	for i, name := range names {
		ip := fmt.Sprintf("%s[%d]", p, i)
		if strings.TrimSpace(name) == "" {
			v.add(ip, "must name "+what)
			continue
		}
		if j, seen := first[name]; seen {
			v.add(ip, fmt.Sprintf("repeats %s[%d]", p, j))
			continue
		}
		first[name] = i
	}
}

func (v *validator) apps(groups []AppGroup, defs map[string]BadgeDefinition) {
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Group
		gp := fmt.Sprintf("apps[%d]", i)
		if len(g.Apps) == 0 {
			v.add(gp+".apps", "must list at least one app")
		}
		for j, a := range g.Apps {
			ap := fmt.Sprintf("%s.apps[%d]", gp, j)
			v.item(ap, a.Name, a.URL, a.Icon, a.Badges, defs)
			if utf8.RuneCountInString(a.Info) > maxInfoLength {
				v.add(ap+".info", fmt.Sprintf("must be at most %d characters", maxInfoLength))
			}
		}
	}
	v.groups("apps", names)
}

func (v *validator) bookmarks(groups []BookmarkGroup, defs map[string]BadgeDefinition) {
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Group
		gp := fmt.Sprintf("bookmarks[%d]", i)
		if len(g.Bookmarks) == 0 {
			v.add(gp+".bookmarks", "must list at least one bookmark")
		}
		for j, b := range g.Bookmarks {
			v.item(fmt.Sprintf("%s.bookmarks[%d]", gp, j), b.Name, b.URL, b.Icon, b.Badges, defs)
		}
	}
	v.groups("bookmarks", names)
}

// groups checks the group names of apps or bookmarks: each named, none
// twice — a group's items are listed under one entry.
func (v *validator) groups(p string, names []string) {
	first := map[string]int{}
	for i, name := range names {
		gp := fmt.Sprintf("%s[%d].group", p, i)
		v.text(gp, name, true)
		if j, seen := first[name]; seen && strings.TrimSpace(name) != "" {
			v.add(gp, fmt.Sprintf("repeats the group of %s[%d]: list its items there", p, j))
			continue
		}
		first[name] = i
	}
}

// item checks what a custom app and a bookmark share.
func (v *validator) item(p, name, link, icon string, entries []string, defs map[string]BadgeDefinition) {
	v.text(p+".name", name, true)
	v.link(p+".url", link)
	if icon != "" {
		ref, err := icons.ParseRef(icon)
		switch {
		case err != nil:
			v.add(p+".icon", err.Error())
		case ref.Kind == icons.RefRemote && v.remoteOff:
			v.add(p+".icon", "is a URL, and icons.remote.enabled is false")
		}
	}
	v.itemBadges(p+".badges", entries, defs)
}

// link checks a URL the page links to: http or https only — a javascript:
// or data: link would run on the dashboard's origin — and no user or
// password, which every viewer would see.
func (v *validator) link(p, raw string) {
	u, err := url.Parse(strings.TrimSpace(raw))
	switch {
	case raw == "":
		v.add(p, "must not be empty")
	case err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "":
		v.add(p, "must be an absolute http or https URL")
	case u.User != nil:
		v.add(p, "must not hold a user or password: every viewer of the dashboard would see it")
	}
}

// itemBadges checks an item's own badges: each entry parses, names a
// definition, and takes a corner no earlier entry took.
func (v *validator) itemBadges(p string, entries []string, defs map[string]BadgeDefinition) {
	corners := map[badges.Corner]int{}
	for i, raw := range entries {
		ep := fmt.Sprintf("%s[%d]", p, i)
		e, err := badges.ParseEntry(raw)
		if err != nil {
			v.add(ep, err.Error())
			continue
		}
		if _, defined := defs[e.Name]; e.Name != "" && !defined {
			v.add(ep, "names no definition of badges.definitions")
			continue
		}
		if j, taken := corners[e.Corner]; taken {
			v.add(ep, fmt.Sprintf("takes the corner %s of %s[%d]: one badge per corner", e.Corner, p, j))
			continue
		}
		corners[e.Corner] = i
	}
}

func (v *validator) icons(c *Icons) {
	v.defaultIcon("icons.defaultAppIcon", c.DefaultAppIcon)
	v.defaultIcon("icons.defaultBookmarkIcon", c.DefaultBookmarkIcon)

	const am = "icons.autoMatch.catalogs"
	seen := map[string]int{}
	for i, name := range c.AutoMatch.Catalogs {
		ip := fmt.Sprintf("%s[%d]", am, i)
		if j, twice := seen[name]; twice {
			v.add(ip, fmt.Sprintf("repeats %s[%d]", am, j))
			continue
		}
		seen[name] = i
		if !slices.Contains(autoMatchCatalogs, name) {
			v.add(ip, "must be selfhst or dashboard-icons, the catalogues of app logos")
		}
	}
	if c.AutoMatch.Enabled && len(c.AutoMatch.Catalogs) == 0 {
		v.add(am, "must name a catalogue while icons.autoMatch.enabled is true")
	}

	r := c.Remote
	if r.MaxBytes < minRemoteBytes || r.MaxBytes > maxRemoteBytes {
		v.add("icons.remote.maxBytes", fmt.Sprintf("must be from %d to %d", minRemoteBytes, maxRemoteBytes))
	}
	if r.Timeout < minRemoteTimeout || r.Timeout > maxRemoteTimeout {
		v.add("icons.remote.timeout", fmt.Sprintf("must be from %s to %s", minRemoteTimeout, maxRemoteTimeout))
	}
	if r.MaxEntries < 1 || r.MaxEntries > maxRemoteEntries {
		v.add("icons.remote.maxEntries", fmt.Sprintf("must be from 1 to %d", maxRemoteEntries))
	}
	v.each("icons.remote.allowedHosts", r.AllowedHosts, isHost, "must be a host name in lower case or an IP address, without a scheme or port")
}

// defaultIcon checks a fallback icon: it must not fail, so it is a catalogue
// icon or a ConfigMap key, never a URL.
func (v *validator) defaultIcon(p, raw string) {
	ref, err := icons.ParseRef(raw)
	switch {
	case err != nil:
		v.add(p, err.Error())
	case ref.Kind == icons.RefRemote:
		v.add(p, "must be a catalogue icon or configmap:<key>: a URL could fail to download, and every tile falls back to this icon")
	}
}

func (v *validator) badges(b *Badges) {
	for _, name := range slices.Sorted(maps.Keys(b.Definitions)) {
		v.definition(mapKey("badges.definitions", name), name, b.Definitions[name])
	}
	for i, r := range b.Rules {
		rp := fmt.Sprintf("badges.rules[%d]", i)
		if _, defined := b.Definitions[r.Badge]; !defined {
			v.add(rp+".badge", "must name a definition of badges.definitions")
		}
		if r.Corner != "" && !badges.IsCorner(r.Corner) {
			v.add(rp+".corner", "must be top-right, top-left, bottom-right or bottom-left")
		}
		v.match(rp+".match", r.Match)
	}
}

func (v *validator) definition(p, name string, d BadgeDefinition) {
	if !badges.IsDefinitionName(name) {
		v.add(p, "must be named in lower case letters, digits and '-', at most 63 characters")
	}
	switch {
	case d.Icon != "" && d.Text != "":
		v.add(p, "must set icon or text, not both")
	case d.Icon == "" && d.Text == "":
		v.add(p, "must set an icon or a text")
	case d.Icon != "":
		ref, err := icons.ParseRef(d.Icon)
		if err != nil {
			v.add(p+".icon", err.Error())
		} else if ref.Kind != icons.RefCatalog || !icons.IsGlyphCatalog(ref.Catalog) {
			v.add(p+".icon", "must be a glyph of mdi, material-symbols or si: a logo or an image is unreadable at the size of a badge")
		}
	default:
		if !badges.ValidText(strings.TrimSpace(d.Text)) {
			v.add(p+".text", "must be one to eight characters, without ',' or '@'")
		}
	}
	v.text(p+".label", d.Label, true)
	switch {
	case d.Tone == "violet":
		v.add(p+".tone", "must not be violet, the agent's light in Cogpit: neutral, blue, cyan, mint, amber or coral")
	case d.Tone != "" && !badges.IsTone(d.Tone):
		v.add(p+".tone", "must be neutral, blue, cyan, mint, amber or coral")
	}
}

func (v *validator) match(p string, m BadgeMatch) {
	v.each(p+".ingressClassNames", m.IngressClassNames, isDNS1123Subdomain, "must be an IngressClass name")
	v.each(p+".ingressControllers", m.IngressControllers, isController, "must be an IngressClass controller such as haproxy-ingress.github.io/controller")
	v.each(p+".namespaces", m.Namespaces, isDNS1123Label, "must be a namespace name")
	v.each(p+".hosts", m.Hosts, isHostGlob, "must be a host name, '*' standing for any part of it, such as *.home.arpa")
	v.labelSelector(p+".labels", m.Labels)
	for _, key := range slices.Sorted(maps.Keys(m.Annotations)) {
		if len(validation.IsQualifiedName(key)) > 0 {
			v.add(mapKey(p+".annotations", key), "must be an annotation key such as example.com/tier")
		}
	}
	v.each(p+".kinds", m.Kinds, isKind, "must be ingress, app or bookmark")
}

// each adds message at every item of list that ok refuses.
func (v *validator) each(p string, list []string, ok func(string) bool, message string) {
	for i, s := range list {
		if !ok(s) {
			v.add(fmt.Sprintf("%s[%d]", p, i), message)
		}
	}
}

func isDNS1123Subdomain(s string) bool { return len(validation.IsDNS1123Subdomain(s)) == 0 }

func isDNS1123Label(s string) bool { return len(validation.IsDNS1123Label(s)) == 0 }

func isHost(s string) bool { return isDNS1123Subdomain(s) || net.ParseIP(s) != nil }

func isController(s string) bool { return s != "" && !strings.ContainsFunc(s, isSpace) }

// isHostGlob accepts a host name in which '*' stands for any part, and
// nothing a URL adds to a host: no scheme, path or port.
func isHostGlob(s string) bool {
	_, err := path.Match(s, "")
	return err == nil && s != "" && !strings.ContainsAny(s, "/:")
}

func isKind(s string) bool { return s == KindIngress || s == KindApp || s == KindBookmark }

func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}
