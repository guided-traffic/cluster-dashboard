package config

import "time"

// File is the schema of the YAML file: a key the structs do not name is
// refused. The chart renders it from its values of the same names
// (deploy/helm/dash/templates/configmap.yaml); a key the file leaves out
// keeps the default of DefaultFile.
type File struct {
	Discovery Discovery       `yaml:"discovery"`
	UI        UI              `yaml:"ui"`
	Apps      []AppGroup      `yaml:"apps"`
	Bookmarks []BookmarkGroup `yaml:"bookmarks"`
	Icons     Icons           `yaml:"icons"`
	Badges    Badges          `yaml:"badges"`
}

// Discovery is which Ingresses become apps (docs/adr/0007).
type Discovery struct {
	// Enabled runs the discovery; off, the dashboard shows the custom apps
	// and the bookmarks only.
	Enabled bool `yaml:"enabled"`
	// DefaultEnable shows every selected Ingress that does not opt out with
	// dash.gtrfc.com/enable: "false"; off, only those that opt in with "true".
	DefaultEnable bool `yaml:"defaultEnable"`
	// InstanceName, when set, shows only the Ingresses whose
	// dash.gtrfc.com/instance lists it.
	InstanceName string `yaml:"instanceName"`
	// NamespaceSelector is where Ingresses are read.
	NamespaceSelector NamespaceSelector `yaml:"namespaceSelector"`
}

// NamespaceSelector selects namespaces as Hajimari does: every namespace, or
// those named united with those a label selector matches.
type NamespaceSelector struct {
	Any           bool          `yaml:"any"`
	MatchNames    []string      `yaml:"matchNames"`
	LabelSelector LabelSelector `yaml:"labelSelector"`
}

// LabelSelector is a Kubernetes label selector, in its manifest form.
type LabelSelector struct {
	MatchLabels      map[string]string          `yaml:"matchLabels"`
	MatchExpressions []LabelSelectorRequirement `yaml:"matchExpressions"`
}

// IsEmpty reports whether the selector names no label: it selects nothing
// where it is optional and everything where Kubernetes reads it.
func (s LabelSelector) IsEmpty() bool {
	return len(s.MatchLabels) == 0 && len(s.MatchExpressions) == 0
}

// LabelSelectorRequirement is one expression of a label selector.
type LabelSelectorRequirement struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values"`
}

// UI is what the page shows and how.
type UI struct {
	Title    string   `yaml:"title"`
	Greeting Greeting `yaml:"greeting"`
	// LightStrip is the strip of frosted glass over the greeting, lit in a
	// colour (Cogpit's light strip).
	LightStrip LightStrip `yaml:"lightStrip"`
	// DefaultTheme is the colour scheme until the person picks one: system,
	// light or dark.
	DefaultTheme string `yaml:"defaultTheme"`
	// RefreshSeconds is how often the page asks for the dashboard again.
	RefreshSeconds     int  `yaml:"refreshSeconds"`
	ShowApps           bool `yaml:"showApps"`
	ShowAppGroups      bool `yaml:"showAppGroups"`
	ShowAppURLs        bool `yaml:"showAppUrls"`
	ShowAppInfo        bool `yaml:"showAppInfo"`
	ShowAppStatus      bool `yaml:"showAppStatus"`
	ShowBookmarks      bool `yaml:"showBookmarks"`
	ShowBookmarkGroups bool `yaml:"showBookmarkGroups"`
	// AlwaysTargetBlank opens every app and bookmark in a new tab.
	AlwaysTargetBlank bool `yaml:"alwaysTargetBlank"`
	// GroupOrder names the groups that come first, in this order; the others
	// follow by name.
	GroupOrder []string `yaml:"groupOrder"`
}

// Greeting is the page's first line: the time of day and the environment,
// "Good morning, welcome to production."
type Greeting struct {
	Enabled     bool   `yaml:"enabled"`
	Environment string `yaml:"environment"`
}

// LightStrip is the light strip's colour and how its light moves.
type LightStrip struct {
	// Color is empty for none (the glass alone), a name of lightStripColors,
	// or #rrggbb.
	Color string `yaml:"color"`
	// Animation is one of lightStripAnimations.
	Animation string `yaml:"animation"`
}

// AppGroup is a group of custom apps: apps no Ingress describes.
type AppGroup struct {
	Group string `yaml:"group"`
	Apps  []App  `yaml:"apps"`
}

// App is a custom app.
type App struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	// Icon is an icon reference (docs/adr/0008 D2); empty: auto-match, then
	// the default.
	Icon        string `yaml:"icon"`
	Info        string `yaml:"info"`
	TargetBlank bool   `yaml:"targetBlank"`
	// Badges are the app's own badges, each an entry of the annotation's
	// grammar: name, name@corner, text:<value>, text:<value>@corner.
	Badges []string `yaml:"badges"`
}

// BookmarkGroup is a group of bookmarks.
type BookmarkGroup struct {
	Group     string     `yaml:"group"`
	Bookmarks []Bookmark `yaml:"bookmarks"`
}

// Bookmark is a link with a name and an icon.
type Bookmark struct {
	Name        string   `yaml:"name"`
	URL         string   `yaml:"url"`
	Icon        string   `yaml:"icon"`
	TargetBlank bool     `yaml:"targetBlank"`
	Badges      []string `yaml:"badges"`
}

// Icons configures where icons come from (docs/adr/0008, 0009).
type Icons struct {
	// DefaultAppIcon and DefaultBookmarkIcon are the icons of an item whose
	// own does not resolve and that auto-match does not find: a catalogue
	// icon or a ConfigMap key, never a URL, which could fail.
	DefaultAppIcon      string    `yaml:"defaultAppIcon"`
	DefaultBookmarkIcon string    `yaml:"defaultBookmarkIcon"`
	AutoMatch           AutoMatch `yaml:"autoMatch"`
	Remote              Remote    `yaml:"remote"`
}

// AutoMatch looks an app without an icon up by name in the logo catalogues.
type AutoMatch struct {
	Enabled bool `yaml:"enabled"`
	// Catalogs are the catalogues searched, in this order: selfhst,
	// dashboard-icons.
	Catalogs []string `yaml:"catalogs"`
}

// Remote is how icons named by a URL are fetched (docs/adr/0009).
type Remote struct {
	Enabled bool `yaml:"enabled"`
	// FromAnnotations fetches URLs that Ingress annotations name, not only
	// those of the configuration.
	FromAnnotations bool `yaml:"fromAnnotations"`
	// AllowPrivateNetworks fetches from hosts at loopback, private,
	// link-local and other non-public addresses.
	AllowPrivateNetworks bool `yaml:"allowPrivateNetworks"`
	// AllowedHosts are host names fetched at any address, over http too.
	AllowedHosts []string      `yaml:"allowedHosts"`
	MaxBytes     int64         `yaml:"maxBytes"`
	Timeout      time.Duration `yaml:"timeout"`
	MaxEntries   int           `yaml:"maxEntries"`
}

// Badges are the pills on icons (docs/adr/0011).
type Badges struct {
	// Definitions are the named badges rules and items refer to.
	Definitions map[string]BadgeDefinition `yaml:"definitions"`
	// Rules put a definition's badge on every item they match, in order.
	Rules []BadgeRule `yaml:"rules"`
}

// BadgeDefinition is a named badge: a glyph or a text, a label, a tone.
type BadgeDefinition struct {
	// Icon is a glyph of mdi, material-symbols or si.
	Icon string `yaml:"icon"`
	// Text is at most eight characters; a number is a text.
	Text string `yaml:"text"`
	// Label is the tooltip and the accessible name.
	Label string `yaml:"label"`
	Tone  string `yaml:"tone"`
}

// BadgeRule puts a badge on every item its match holds for.
type BadgeRule struct {
	Match  BadgeMatch `yaml:"match"`
	Badge  string     `yaml:"badge"`
	Corner string     `yaml:"corner"`
}

// BadgeMatch is a rule's condition: every field set must hold, and a field
// an item does not have (an IngressClass for a bookmark) does not hold.
type BadgeMatch struct {
	IngressClassNames  []string          `yaml:"ingressClassNames"`
	IngressControllers []string          `yaml:"ingressControllers"`
	Namespaces         []string          `yaml:"namespaces"`
	Hosts              []string          `yaml:"hosts"`
	Labels             LabelSelector     `yaml:"labels"`
	Annotations        map[string]string `yaml:"annotations"`
	Kinds              []string          `yaml:"kinds"`
}

// The kinds of item a badge rule can match.
const (
	KindIngress  = "ingress"
	KindApp      = "app"
	KindBookmark = "bookmark"
)

// The colour schemes ui.defaultTheme takes.
var themes = []string{"system", "light", "dark"}

// The colours the light strip takes by name, each with a shade per colour
// scheme in the frontend's tokens: Cogpit's accents without violet, which
// Dash leaves to Cogpit's agent (docs/adr/0005).
var lightStripColors = []string{"cyan", "blue", "magenta", "red", "orange", "yellow", "green"}

// How the light strip's light moves.
var lightStripAnimations = []string{"static", "breathing", "scanner", "flow"}

// The logo catalogues auto-match searches, by the names the configuration
// uses.
var autoMatchCatalogs = []string{"selfhst", "dashboard-icons"}

// DefaultFile is the file's content when the file sets nothing: the defaults
// of every key, the same as the chart's values.yaml.
func DefaultFile() File {
	return File{
		Discovery: Discovery{
			Enabled:           true,
			NamespaceSelector: NamespaceSelector{Any: true},
		},
		UI: UI{
			Title:              "Dash",
			Greeting:           Greeting{Enabled: true},
			LightStrip:         LightStrip{Animation: "static"},
			DefaultTheme:       "system",
			RefreshSeconds:     60,
			ShowApps:           true,
			ShowAppGroups:      true,
			ShowAppURLs:        true,
			ShowAppStatus:      true,
			ShowBookmarks:      true,
			ShowBookmarkGroups: true,
		},
		Icons: Icons{
			DefaultAppIcon:      "material-symbols:apps",
			DefaultBookmarkIcon: "material-symbols:bookmark",
			AutoMatch:           AutoMatch{Enabled: true, Catalogs: []string{"selfhst", "dashboard-icons"}},
			Remote: Remote{
				Enabled:         true,
				FromAnnotations: true,
				MaxBytes:        256 << 10,
				Timeout:         20 * time.Second,
				MaxEntries:      512,
			},
		},
	}
}
