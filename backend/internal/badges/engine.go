package badges

import (
	"errors"
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/guided-traffic/dash/backend/internal/icons"
)

// Badge is a pill on an item's icon, resolved: a glyph or a text, its
// label, its tone, its corner.
type Badge struct {
	Corner Corner
	// Icon is the glyph, tinted in the tone's colour; nil for a text badge.
	Icon *icons.Icon
	// Text is at most MaxText characters; empty for a glyph badge.
	Text string
	// Label is the tooltip and the accessible name.
	Label string
	Tone  Tone
}

// Definition is a named badge as the configuration writes it
// (badges.definitions).
type Definition struct {
	Icon, Text, Label, Tone string
}

// Match is a rule's condition (badges.rules[].match): every field set must
// hold, and a field an item does not have does not hold.
type Match struct {
	IngressClassNames  []string
	IngressControllers []string
	Namespaces         []string
	Hosts              []string
	// Labels is nil when the rule names no label.
	Labels      labels.Selector
	Annotations map[string]string
	Kinds       []string
}

// Rule puts a definition's badge on every item its match holds for.
type Rule struct {
	Match  Match
	Badge  string
	Corner Corner
}

// Item is what a rule is matched against.
type Item struct {
	// Kind is ingress, app or bookmark.
	Kind string
	// Host is the host of the item's URL.
	Host string
	// Namespace, Labels, Annotations, IngressClass and IngressController
	// are an Ingress's: an app or a bookmark has none.
	Namespace         string
	Labels            map[string]string
	Annotations       map[string]string
	IngressClass      string
	IngressController string
	// Own are the item's own entries, parsed by Engine.Own.
	Own []Entry
}

// Glyphs resolves a definition's glyph; icons.Resolver does.
type Glyphs interface {
	Glyph(ref string) (icons.Icon, error)
}

// Engine puts badges on items (docs/adr/0011).
type Engine struct {
	defs  map[string]Badge
	rules []Rule
}

// New resolves every definition's glyph — one that does not resolve, or is
// no glyph, refuses the start, as the configuration's other problems do —
// and keeps the rules, which config.Load checked against the definitions.
func New(defs map[string]Definition, rules []Rule, glyphs Glyphs) (*Engine, error) {
	e := &Engine{defs: map[string]Badge{}, rules: rules}
	var problems []string
	for _, name := range slices.Sorted(maps.Keys(defs)) {
		d := defs[name]
		b := Badge{Text: strings.TrimSpace(d.Text), Label: d.Label, Tone: Tone(d.Tone)}
		if b.Tone == "" {
			b.Tone = DefaultTone
		}
		if d.Icon != "" {
			icon, err := glyphs.Glyph(d.Icon)
			if err != nil {
				problems = append(problems, fmt.Sprintf("badges.definitions.%s.icon: %v", name, err))
				continue
			}
			b.Icon = &icon
		}
		e.defs[name] = b
	}
	for i, r := range rules {
		if _, defined := defs[r.Badge]; !defined {
			problems = append(problems, fmt.Sprintf("badges.rules[%d].badge: names no definition", i))
		}
	}
	if len(problems) > 0 {
		return nil, errors.New(strings.Join(problems, "; "))
	}
	return e, nil
}

// Own parses an item's own entries — the annotation's comma-separated
// list, or the configuration's list — and returns those to use and the
// problems with the others: an entry that does not parse, names no
// definition, or takes a corner an earlier entry took. An Ingress author's
// mistake must not stop the dashboard: the caller logs the problems.
func (e *Engine) Own(raw []string) ([]Entry, []string) {
	var out []Entry
	var problems []string
	taken := map[Corner]bool{}
	for i, s := range raw {
		entry, err := ParseEntry(s)
		switch {
		case err != nil:
			problems = append(problems, fmt.Sprintf("entry %d %v", i+1, err))
			continue
		case entry.Name != "":
			if _, ok := e.defs[entry.Name]; !ok {
				problems = append(problems, fmt.Sprintf("entry %d names no badge definition", i+1))
				continue
			}
		}
		if taken[entry.Corner] {
			problems = append(problems, fmt.Sprintf("entry %d takes the corner %s an earlier entry took", i+1, entry.Corner))
			continue
		}
		taken[entry.Corner] = true
		out = append(out, entry)
	}
	return out, problems
}

// SplitList splits the annotation's value into its entries.
func SplitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	return strings.Split(raw, ",")
}

// For is an item's badges (docs/adr/0011 D3): its own entries take their
// corners first, then each rule in order whose match holds takes its corner
// if it is free; one badge per corner, four at most, in the order of
// Corners.
func (e *Engine) For(item Item) []Badge {
	byCorner := map[Corner]Badge{}
	for _, entry := range item.Own {
		b := Badge{Text: entry.Text, Label: entry.Text, Tone: DefaultTone}
		if entry.Name != "" {
			b = e.defs[entry.Name]
		}
		b.Corner = entry.Corner
		byCorner[entry.Corner] = b
	}
	for _, r := range e.rules {
		corner := r.Corner
		if corner == "" {
			corner = DefaultCorner
		}
		if _, taken := byCorner[corner]; taken || !r.Match.holds(item) {
			continue
		}
		b := e.defs[r.Badge]
		b.Corner = corner
		byCorner[corner] = b
	}
	out := make([]Badge, 0, len(byCorner))
	for _, c := range Corners {
		if b, ok := byCorner[c]; ok {
			out = append(out, b)
		}
	}
	return out
}

// holds reports whether every field the match sets holds for the item.
func (m Match) holds(item Item) bool {
	return within(m.IngressClassNames, item.IngressClass) &&
		within(m.IngressControllers, item.IngressController) &&
		within(m.Namespaces, item.Namespace) &&
		within(m.Kinds, item.Kind) &&
		m.hostHolds(item.Host) &&
		(m.Labels == nil || (item.Labels != nil && m.Labels.Matches(labels.Set(item.Labels)))) &&
		m.annotationsHold(item)
}

// within holds for a field the match does not set, and else when the item
// has a value the list names: an item without the field does not match.
func within(list []string, value string) bool {
	return len(list) == 0 || (value != "" && slices.Contains(list, value))
}

func (m Match) hostHolds(host string) bool {
	if len(m.Hosts) == 0 {
		return true
	}
	host = strings.ToLower(host)
	for _, glob := range m.Hosts {
		if ok, _ := path.Match(strings.ToLower(glob), host); ok && host != "" {
			return true
		}
	}
	return false
}

func (m Match) annotationsHold(item Item) bool {
	for k, v := range m.Annotations {
		if got, ok := item.Annotations[k]; !ok || got != v {
			return false
		}
	}
	return true
}

// AnnotationKeys are the annotation keys the rules read: the discovery
// keeps them on the Ingresses it caches.
func (e *Engine) AnnotationKeys() []string {
	var keys []string
	for _, r := range e.rules {
		for k := range r.Match.Annotations {
			if !slices.Contains(keys, k) {
				keys = append(keys, k)
			}
		}
	}
	slices.Sort(keys)
	return keys
}
