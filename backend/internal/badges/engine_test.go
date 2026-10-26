package badges

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/guided-traffic/dash/backend/internal/icons"
)

// stubGlyphs resolves material-symbols:public and :home, nothing else.
type stubGlyphs struct{}

func (stubGlyphs) Glyph(ref string) (icons.Icon, error) {
	switch ref {
	case "material-symbols:public", "material-symbols:home":
		return icons.Icon{Src: "/api/v1/icons/material-symbols@1/" + ref[len("material-symbols:"):] + ".svg", Tint: true}, nil
	case "sh:grafana":
		return icons.Icon{}, icons.ErrNotAGlyph
	}
	return icons.Icon{}, icons.ErrNotBundled
}

var (
	world = icons.Icon{Src: "/api/v1/icons/material-symbols@1/public.svg", Tint: true}
	home  = icons.Icon{Src: "/api/v1/icons/material-symbols@1/home.svg", Tint: true}
)

func defs() map[string]Definition {
	return map[string]Definition{
		"world": {Icon: "material-symbols:public", Label: "reachable worldwide", Tone: "cyan"},
		"home":  {Icon: "material-symbols:home", Label: "local network only"},
		"beta":  {Text: "beta", Label: "beta release", Tone: "amber"},
	}
}

func engine(t *testing.T, rules ...Rule) *Engine {
	t.Helper()
	e, err := New(defs(), rules, stubGlyphs{})
	require.NoError(t, err)
	return e
}

func selector(t *testing.T, s string) labels.Selector {
	t.Helper()
	sel, err := labels.Parse(s)
	require.NoError(t, err)
	return sel
}

// The owner's example: a globe for the external class, a house for the
// internal one, a beta pill an Ingress names itself.
func TestTheOwnersExample(t *testing.T) {
	e := engine(t,
		Rule{Match: Match{IngressClassNames: []string{"external"}}, Badge: "world"},
		Rule{Match: Match{IngressClassNames: []string{"internal"}}, Badge: "home"},
	)
	own, problems := e.Own(SplitList("beta@bottom-left"))
	require.Empty(t, problems)
	assert.Equal(t, []Badge{
		{Corner: TopRight, Icon: &world, Label: "reachable worldwide", Tone: Cyan},
		{Corner: BottomLeft, Text: "beta", Label: "beta release", Tone: Amber},
	}, e.For(Item{Kind: "ingress", IngressClass: "external", Own: own}))
	assert.Equal(t, []Badge{{Corner: TopRight, Icon: &home, Label: "local network only", Tone: Neutral}},
		e.For(Item{Kind: "ingress", IngressClass: "internal"}))
	assert.Empty(t, e.For(Item{Kind: "ingress", IngressClass: "other"}))
	assert.Empty(t, e.For(Item{Kind: "bookmark"}), "a bookmark has no IngressClass")
}

func TestEveryMatchField(t *testing.T) {
	ingress := Item{
		Kind: "ingress", Host: "grafana.home.arpa", Namespace: "monitoring",
		Labels: map[string]string{"tier": "web"}, Annotations: map[string]string{"example.com/tier": "gold"},
		IngressClass: "internal", IngressController: "haproxy-ingress.github.io/controller",
	}
	app := Item{Kind: "app", Host: "router.home.arpa"}
	bookmark := Item{Kind: "bookmark", Host: "kubernetes.io"}
	for name, tc := range map[string]struct {
		match Match
		want  [3]bool // ingress, app, bookmark
	}{
		"empty: everything":       {Match{}, [3]bool{true, true, true}},
		"class name":              {Match{IngressClassNames: []string{"internal", "lan"}}, [3]bool{true, false, false}},
		"another class":           {Match{IngressClassNames: []string{"external"}}, [3]bool{false, false, false}},
		"controller":              {Match{IngressControllers: []string{"haproxy-ingress.github.io/controller"}}, [3]bool{true, false, false}},
		"namespace":               {Match{Namespaces: []string{"monitoring"}}, [3]bool{true, false, false}},
		"host glob":               {Match{Hosts: []string{"*.home.arpa"}}, [3]bool{true, true, false}},
		"host, case aside":        {Match{Hosts: []string{"*.HOME.arpa"}}, [3]bool{true, true, false}},
		"exact host":              {Match{Hosts: []string{"kubernetes.io"}}, [3]bool{false, false, true}},
		"labels":                  {Match{Labels: selector(t, "tier=web")}, [3]bool{true, false, false}},
		"labels that do not hold": {Match{Labels: selector(t, "tier in (db)")}, [3]bool{false, false, false}},
		"annotations":             {Match{Annotations: map[string]string{"example.com/tier": "gold"}}, [3]bool{true, false, false}},
		"another annotation":      {Match{Annotations: map[string]string{"example.com/tier": "silver"}}, [3]bool{false, false, false}},
		"kinds":                   {Match{Kinds: []string{"app", "bookmark"}}, [3]bool{false, true, true}},
		"every field must hold": {Match{IngressClassNames: []string{"internal"}, Namespaces: []string{"media"}},
			[3]bool{false, false, false}},
		"every field holding": {Match{IngressClassNames: []string{"internal"}, Namespaces: []string{"monitoring"}, Hosts: []string{"grafana.*"},
			Kinds: []string{"ingress"}, Labels: selector(t, "tier"), Annotations: map[string]string{"example.com/tier": "gold"}},
			[3]bool{true, false, false}},
	} {
		e := engine(t, Rule{Match: tc.match, Badge: "home"})
		for i, item := range []Item{ingress, app, bookmark} {
			assert.Equal(t, tc.want[i], len(e.For(item)) == 1, "%s: %s", name, item.Kind)
		}
	}
	assert.Empty(t, engine(t, Rule{Match: Match{Hosts: []string{"*"}}, Badge: "home"}).For(Item{Kind: "app"}), "an item without a host")
}

// One badge per corner: the item's own first, then the first rule that
// matches; a later rule for a taken corner gives way.
func TestCorners(t *testing.T) {
	e := engine(t,
		Rule{Match: Match{}, Badge: "world"},
		Rule{Match: Match{}, Badge: "home"},
		Rule{Match: Match{}, Badge: "home", Corner: BottomRight},
		Rule{Match: Match{}, Badge: "beta", Corner: TopLeft},
	)
	own, problems := e.Own([]string{"text:new@top-left"})
	require.Empty(t, problems)
	got := e.For(Item{Kind: "app", Own: own})
	require.Len(t, got, 3)
	assert.Equal(t, Badge{Corner: TopRight, Icon: &world, Label: "reachable worldwide", Tone: Cyan}, got[0], "the first rule takes top-right")
	assert.Equal(t, Badge{Corner: TopLeft, Text: "new", Label: "new", Tone: Neutral}, got[1], "the item's own entry wins its corner")
	assert.Equal(t, BottomRight, got[2].Corner)

	all, _ := e.Own([]string{"beta", "beta@top-left", "home@bottom-right", "text:4@bottom-left"})
	assert.Len(t, e.For(Item{Kind: "app", Own: all}), 4, "four at most, never a stack")
}

func TestOwnEntries(t *testing.T) {
	e := engine(t)
	own, problems := e.Own(SplitList(" beta , nope, text:toolongtext, home@top-right, world@middle, text:1@bottom-left"))
	assert.Equal(t, []Entry{{Name: "beta", Corner: TopRight}, {Text: "1", Corner: BottomLeft}}, own)
	assert.Equal(t, []string{
		"entry 2 names no badge definition",
		"entry 3 must be text: followed by one to eight characters, without ',' or '@'",
		"entry 4 takes the corner top-right an earlier entry took",
		"entry 5 names no corner after '@': top-right, top-left, bottom-right or bottom-left",
	}, problems)
	assert.Nil(t, SplitList("  "))
	none, problems := e.Own(nil)
	assert.Empty(t, none)
	assert.Empty(t, problems)
}

func TestNewRefusesWhatDoesNotResolve(t *testing.T) {
	d := defs()
	d["logo"] = Definition{Icon: "sh:grafana", Label: "l"}
	d["gone"] = Definition{Icon: "mdi:nothing", Label: "l"}
	_, err := New(d, []Rule{{Badge: "world"}, {Badge: "absent"}}, stubGlyphs{})
	require.Error(t, err)
	assert.Equal(t, "badges.definitions.gone.icon: "+icons.ErrNotBundled.Error()+
		"; badges.definitions.logo.icon: "+icons.ErrNotAGlyph.Error()+
		"; badges.rules[1].badge: names no definition", err.Error())
}

func TestAnnotationKeys(t *testing.T) {
	e := engine(t,
		Rule{Match: Match{Annotations: map[string]string{"b.example.com/y": "1", "a.example.com/x": "2"}}, Badge: "home"},
		Rule{Match: Match{Annotations: map[string]string{"a.example.com/x": "3"}}, Badge: "home"},
		Rule{Badge: "home"},
	)
	assert.Equal(t, []string{"a.example.com/x", "b.example.com/y"}, e.AnnotationKeys())
}
