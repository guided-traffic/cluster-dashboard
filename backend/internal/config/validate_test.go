package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// parse runs Parse on content over the defaults, as Load does.
func parse(content string) (File, []string) {
	f := DefaultFile()
	return f, Parse([]byte(content), &f)
}

func TestTheDefaultsAreValid(t *testing.T) {
	f := DefaultFile()
	assert.Empty(t, validate(&f, nil))
}

// A file that sets every key the README's reference names, each to a
// realistic value, is valid and lands where it should.
func TestAFileWithEveryKey(t *testing.T) {
	f, problems := parse(`
discovery:
  enabled: true
  defaultEnable: true
  instanceName: home
  namespaceSelector:
    any: false
    matchNames: [monitoring, media]
    labelSelector:
      matchLabels: {dash.example.com/listed: "true"}
      matchExpressions:
        - {key: tier, operator: In, values: [web, tools]}
        - {key: legacy, operator: DoesNotExist}
ui:
  title: Home lab
  greeting: {enabled: false, name: Hans}
  defaultTheme: dark
  refreshSeconds: 30
  showSummary: false
  showApps: true
  showAppGroups: false
  showAppUrls: false
  showAppInfo: true
  showAppStatus: false
  showBookmarks: false
  showBookmarkGroups: false
  alwaysTargetBlank: true
  groupOrder: [monitoring, media]
apps:
  - group: tools
    apps:
      - name: Router
        url: https://192.168.1.1
        icon: material-symbols:router
        info: The edge router
        targetBlank: true
        badges: [home, "text:lan@bottom-left"]
bookmarks:
  - group: docs
    bookmarks:
      - {name: Kubernetes, url: "https://kubernetes.io/docs", icon: "si:kubernetes", targetBlank: true, badges: [world]}
icons:
  defaultAppIcon: mdi:application
  defaultBookmarkIcon: configmap:bookmark.png
  autoMatch: {enabled: true, catalogs: [dashboard-icons]}
  remote:
    enabled: true
    fromAnnotations: false
    allowPrivateNetworks: true
    allowedHosts: [icons.home.arpa, 10.0.0.7]
    maxBytes: 131072
    timeout: 5s
    maxEntries: 64
badges:
  definitions:
    world: {icon: "material-symbols:public", label: reachable worldwide, tone: cyan}
    home: {icon: "material-symbols:home", label: local network only}
    beta: {text: beta, label: beta release, tone: amber}
  rules:
    - match: {ingressClassNames: [external]}
      badge: world
    - match:
        ingressControllers: [haproxy-ingress.github.io/controller]
        namespaces: [media]
        hosts: ["*.home.arpa"]
        labels: {matchLabels: {app.kubernetes.io/part-of: media}}
        annotations: {example.com/tier: gold}
        kinds: [ingress, app, bookmark]
      badge: home
      corner: bottom-right
`)
	require.Empty(t, problems)
	assert.Equal(t, NamespaceSelector{
		MatchNames: []string{"monitoring", "media"},
		LabelSelector: LabelSelector{
			MatchLabels: map[string]string{"dash.example.com/listed": "true"},
			MatchExpressions: []LabelSelectorRequirement{
				{Key: "tier", Operator: "In", Values: []string{"web", "tools"}},
				{Key: "legacy", Operator: "DoesNotExist"},
			},
		},
	}, f.Discovery.NamespaceSelector)
	assert.Equal(t, "Home lab", f.UI.Title)
	assert.Equal(t, 30, f.UI.RefreshSeconds)
	assert.Equal(t, []string{"home", "text:lan@bottom-left"}, f.Apps[0].Apps[0].Badges)
	assert.Equal(t, 5*time.Second, f.Icons.Remote.Timeout)
	assert.Equal(t, int64(131072), f.Icons.Remote.MaxBytes)
	assert.Equal(t, BadgeDefinition{Text: "beta", Label: "beta release", Tone: "amber"}, f.Badges.Definitions["beta"])
	assert.Equal(t, "bottom-right", f.Badges.Rules[1].Corner)
}

// A key the file leaves out keeps its default; a nested key overrides only
// itself.
func TestTheFileOverridesKeyByKey(t *testing.T) {
	f, problems := parse("ui:\n  title: Lab\nicons:\n  remote:\n    timeout: 3s\n")
	require.Empty(t, problems)
	want := DefaultFile()
	want.UI.Title = "Lab"
	want.Icons.Remote.Timeout = 3 * time.Second
	assert.Equal(t, want, f)
}

func TestEveryValidationRule(t *testing.T) {
	for name, tc := range map[string]struct {
		content  string
		problems []string
	}{
		"instance name with a comma": {
			content:  "discovery:\n  instanceName: a,b\n",
			problems: []string{"line 2: discovery.instanceName: must not hold ',', which separates the names of dash.gtrfc.com/instance"},
		},
		"any with names": {
			content:  "discovery:\n  namespaceSelector:\n    matchNames: [media]\n",
			problems: []string{"line 2: discovery.namespaceSelector: any is true, which would ignore matchNames and labelSelector: set any to false to select namespaces by name or label"},
		},
		"any with a label selector": {
			content:  "discovery:\n  namespaceSelector:\n    labelSelector: {matchLabels: {a: b}}\n",
			problems: []string{"line 2: discovery.namespaceSelector: any is true, which would ignore matchNames and labelSelector: set any to false to select namespaces by name or label"},
		},
		"no namespace": {
			content:  "discovery:\n  namespaceSelector:\n    any: false\n    labelSelector: {}\n",
			problems: []string{"line 2: discovery.namespaceSelector: selects no namespace: set any to true, name namespaces in matchNames or select them with labelSelector, or set discovery.enabled to false"},
		},
		"no namespace while discovery is off": {
			content: "discovery:\n  enabled: false\n  namespaceSelector:\n    any: false\n",
		},
		"namespace names": {
			content: "discovery:\n  namespaceSelector:\n    any: false\n    matchNames: [Media, ok, a_b]\n",
			problems: []string{
				"line 4: discovery.namespaceSelector.matchNames[0]: must be a namespace name: lower case letters, digits and '-', at most 63 characters",
				"line 4: discovery.namespaceSelector.matchNames[2]: must be a namespace name: lower case letters, digits and '-', at most 63 characters",
			},
		},
		"label selector": {
			content: `discovery:
  namespaceSelector:
    any: false
    labelSelector:
      matchLabels:
        app.kubernetes.io/name: "not a value"
        "bad key!": x
      matchExpressions:
        - {key: tier, operator: In}
        - {key: tier, operator: Exists, values: [a]}
        - {key: tier, operator: Like, values: [a]}
        - {key: "", operator: NotIn, values: ["-x"]}
`,
			problems: []string{
				"line 6: discovery.namespaceSelector.labelSelector.matchLabels[\"app.kubernetes.io/name\"]: must be a label value: at most 63 letters, digits, '-', '_' or '.', starting and ending with a letter or digit",
				"line 7: discovery.namespaceSelector.labelSelector.matchLabels.bad key!: must be a label key such as app or app.kubernetes.io/name",
				"line 9: discovery.namespaceSelector.labelSelector.matchExpressions[0].values: must name at least one value for In and NotIn",
				"line 10: discovery.namespaceSelector.labelSelector.matchExpressions[1].values: must be empty for Exists and DoesNotExist",
				"line 11: discovery.namespaceSelector.labelSelector.matchExpressions[2].operator: must be In, NotIn, Exists or DoesNotExist",
				"line 12: discovery.namespaceSelector.labelSelector.matchExpressions[3].key: must be a label key such as app or app.kubernetes.io/name",
				"line 12: discovery.namespaceSelector.labelSelector.matchExpressions[3].values[0]: must be a label value: at most 63 letters, digits, '-', '_' or '.', starting and ending with a letter or digit",
			},
		},
		"ui": {
			content: "ui:\n  title: \"  \"\n  greeting: {name: " + strings.Repeat("a", 65) + "}\n  defaultTheme: sepia\n  refreshSeconds: 4\n  groupOrder: [a, \"\", a]\n",
			problems: []string{
				"line 2: ui.title: must not be empty",
				"line 3: ui.greeting.name: must be at most 64 characters",
				"line 4: ui.defaultTheme: must be system, light or dark",
				"line 5: ui.refreshSeconds: must be from 5 to 3600",
				"line 6: ui.groupOrder[1]: must name a group",
				"line 6: ui.groupOrder[2]: repeats ui.groupOrder[0]",
			},
		},
		"refresh too slow": {
			content:  "ui:\n  refreshSeconds: 3601\n",
			problems: []string{"line 2: ui.refreshSeconds: must be from 5 to 3600"},
		},
		"apps": {
			content: `apps:
  - group: tools
    apps:
      - name: ""
        url: javascript:alert(1)
        icon: fa:home
        badges: [nope, "text:toolongtext", "text:a", "text:b"]
      - name: Mail
        url: https://user:secret@mail.example.com
        info: ` + strings.Repeat("a", 121) + `
  - group: tools
    apps: []
  - apps:
      - {name: x}
`,
			problems: []string{
				"line 4: apps[0].apps[0].name: must not be empty",
				"line 5: apps[0].apps[0].url: must be an absolute http or https URL",
				"line 6: apps[0].apps[0].icon: names no catalogue: the prefixes are mdi, material-symbols, si (simple-icons), sh (selfhst) and di (dashboard-icons)",
				"line 7: apps[0].apps[0].badges[0]: names no definition of badges.definitions",
				"line 7: apps[0].apps[0].badges[1]: must be text: followed by one to eight characters, without ',' or '@'",
				"line 7: apps[0].apps[0].badges[3]: takes the corner top-right of apps[0].apps[0].badges[2]: one badge per corner",
				"line 9: apps[0].apps[1].url: must not hold a user or password: every viewer of the dashboard would see it",
				"line 10: apps[0].apps[1].info: must be at most 120 characters",
				"line 11: apps[1].group: repeats the group of apps[0]: list its items there",
				"line 12: apps[1].apps: must list at least one app",
				"line 13: apps[2].group: must not be empty",
				"line 14: apps[2].apps[0].url: must not be empty",
			},
		},
		"bookmarks": {
			content: `bookmarks:
  - group: docs
    bookmarks:
      - {name: Docs, url: "ftp://docs.example.com", icon: "mdi:"}
  - group: docs
    bookmarks: []
`,
			problems: []string{
				"line 4: bookmarks[0].bookmarks[0].url: must be an absolute http or https URL",
				"line 4: bookmarks[0].bookmarks[0].icon: must name an icon in lower case letters, digits and single '-', '_' or '.' between them",
				"line 5: bookmarks[1].group: repeats the group of bookmarks[0]: list its items there",
				"line 6: bookmarks[1].bookmarks: must list at least one bookmark",
			},
		},
		"default icons": {
			content: "icons:\n  defaultAppIcon: https://example.com/a.png\n  defaultBookmarkIcon: \"\"\n",
			problems: []string{
				"line 2: icons.defaultAppIcon: must be a catalogue icon or configmap:<key>: a URL could fail to download, and every tile falls back to this icon",
				"line 3: icons.defaultBookmarkIcon: is empty",
			},
		},
		"auto-match": {
			content: "icons:\n  autoMatch:\n    catalogs: [selfhst, mdi, selfhst]\n",
			problems: []string{
				"line 3: icons.autoMatch.catalogs[1]: must be selfhst or dashboard-icons, the catalogues of app logos",
				"line 3: icons.autoMatch.catalogs[2]: repeats icons.autoMatch.catalogs[0]",
			},
		},
		"auto-match without a catalogue": {
			content:  "icons:\n  autoMatch:\n    catalogs: []\n",
			problems: []string{"line 3: icons.autoMatch.catalogs: must name a catalogue while icons.autoMatch.enabled is true"},
		},
		"a URL icon while remote icons are off": {
			content:  "icons:\n  remote: {enabled: false}\nbookmarks:\n  - group: g\n    bookmarks:\n      - {name: n, url: \"https://x\", icon: \"https://x/a.png\"}\n",
			problems: []string{"line 6: bookmarks[0].bookmarks[0].icon: is a URL, and icons.remote.enabled is false"},
		},
		"auto-match off without a catalogue": {
			content: "icons:\n  autoMatch: {enabled: false, catalogs: []}\n",
		},
		"remote": {
			content: "icons:\n  remote:\n    maxBytes: 1023\n    timeout: 500ms\n    maxEntries: 0\n    allowedHosts: [Icons.example.com, \"example.com:8080\", \"https://x\", ok.example.com, \"::1\"]\n",
			problems: []string{
				"line 3: icons.remote.maxBytes: must be from 1024 to 4194304",
				"line 4: icons.remote.timeout: must be from 1s to 2m0s",
				"line 5: icons.remote.maxEntries: must be from 1 to 10000",
				"line 6: icons.remote.allowedHosts[0]: must be a host name in lower case or an IP address, without a scheme or port",
				"line 6: icons.remote.allowedHosts[1]: must be a host name in lower case or an IP address, without a scheme or port",
				"line 6: icons.remote.allowedHosts[2]: must be a host name in lower case or an IP address, without a scheme or port",
			},
		},
		"remote upper bounds": {
			content: "icons:\n  remote: {maxBytes: 4194305, timeout: 3m, maxEntries: 10001}\n",
			problems: []string{
				"line 2: icons.remote.maxBytes: must be from 1024 to 4194304",
				"line 2: icons.remote.timeout: must be from 1s to 2m0s",
				"line 2: icons.remote.maxEntries: must be from 1 to 10000",
			},
		},
		"badge definitions": {
			content: `badges:
  definitions:
    both: {icon: mdi:home, text: x, label: l}
    neither: {label: l}
    logo: {icon: "sh:grafana", label: l}
    image: {icon: "https://example.com/a.png", label: l}
    broken: {icon: "fa:x", label: l}
    long: {text: "123456789", label: l}
    unlabelled: {text: "1"}
    violet: {text: "v", label: l, tone: violet}
    red: {text: "r", label: l, tone: red}
    Bad_Name: {text: "b", label: l}
    bare: {icon: web, label: "a bare name is mdi's"}
`,
			problems: []string{
				"line 3: badges.definitions.both: must set icon or text, not both",
				"line 4: badges.definitions.neither: must set an icon or a text",
				"line 5: badges.definitions.logo.icon: must be a glyph of mdi, material-symbols or si: a logo or an image is unreadable at the size of a badge",
				"line 6: badges.definitions.image.icon: must be a glyph of mdi, material-symbols or si: a logo or an image is unreadable at the size of a badge",
				"line 7: badges.definitions.broken.icon: names no catalogue: the prefixes are mdi, material-symbols, si (simple-icons), sh (selfhst) and di (dashboard-icons)",
				"line 8: badges.definitions.long.text: must be one to eight characters, without ',' or '@'",
				"line 9: badges.definitions.unlabelled.label: must not be empty",
				"line 10: badges.definitions.violet.tone: must not be violet, the agent's light in Cogpit: neutral, blue, cyan, mint, amber or coral",
				"line 11: badges.definitions.red.tone: must be neutral, blue, cyan, mint, amber or coral",
				"line 12: badges.definitions.Bad_Name: must be named in lower case letters, digits and '-', at most 63 characters",
			},
		},
		"badge rules": {
			content: `badges:
  definitions:
    world: {icon: "material-symbols:public", label: worldwide}
  rules:
    - badge: nowhere
      corner: middle
      match:
        ingressClassNames: [External]
        ingressControllers: ["", "a b"]
        namespaces: [a.b]
        hosts: ["[", "", "a.example.com:443", "*.ok.example.com"]
        labels: {matchExpressions: [{key: a, operator: Has}]}
        annotations: {"bad key!": x, example.com/ok: y}
        kinds: [ingress, route]
    - match: {}
      badge: world
`,
			problems: []string{
				"line 5: badges.rules[0].badge: must name a definition of badges.definitions",
				"line 6: badges.rules[0].corner: must be top-right, top-left, bottom-right or bottom-left",
				"line 8: badges.rules[0].match.ingressClassNames[0]: must be an IngressClass name",
				"line 9: badges.rules[0].match.ingressControllers[0]: must be an IngressClass controller such as haproxy-ingress.github.io/controller",
				"line 9: badges.rules[0].match.ingressControllers[1]: must be an IngressClass controller such as haproxy-ingress.github.io/controller",
				"line 10: badges.rules[0].match.namespaces[0]: must be a namespace name",
				"line 11: badges.rules[0].match.hosts[0]: must be a host name, '*' standing for any part of it, such as *.home.arpa",
				"line 11: badges.rules[0].match.hosts[1]: must be a host name, '*' standing for any part of it, such as *.home.arpa",
				"line 11: badges.rules[0].match.hosts[2]: must be a host name, '*' standing for any part of it, such as *.home.arpa",
				"line 12: badges.rules[0].match.labels.matchExpressions[0].operator: must be In, NotIn, Exists or DoesNotExist",
				"line 13: badges.rules[0].match.annotations.bad key!: must be an annotation key such as example.com/tier",
				"line 14: badges.rules[0].match.kinds[1]: must be ingress, app or bookmark",
			},
		},
		// A value the decoder could not read is reported once: the validation
		// says nothing more about it or what it holds.
		"a decoding problem is not reported twice": {
			content: "ui:\n  defaultTheme: [dark]\napps:\n  - just a string\n  - group: x\n    apps: nope\n",
			problems: []string{
				"line 2: ui.defaultTheme: must be a single value",
				"line 4: apps[0]: must be a mapping of keys",
				"line 6: apps[1].apps: must be a list",
			},
		},
		"a document that is no mapping": {
			content:  "[]\n",
			problems: []string{"line 1: the document: must be a mapping of keys"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, problems := parse(tc.content)
			assert.Equal(t, tc.problems, problems)
		})
	}
}

// A problem never echoes the value: a URL can hold a token.
func TestAProblemNamesNoValue(t *testing.T) {
	_, problems := parse("apps:\n  - group: g\n    apps:\n      - {name: n, url: \"ftp://host/?token=s3cret\"}\n")
	require.Len(t, problems, 1)
	assert.NotContains(t, problems[0], "s3cret")
}

func TestTheLineOfAMissingKeyIsItsMapping(t *testing.T) {
	_, problems := parse("bookmarks:\n  - group: g\n    bookmarks:\n      - name: n\n        icon: web\n")
	assert.Equal(t, []string{"line 4: bookmarks[0].bookmarks[0].url: must not be empty"}, problems)
}

// The chart's values.yaml and DefaultFile hold the same defaults: the chart
// renders the file from its values (templates/configmap.yaml), so a default
// changed in one place only would differ between a file the chart rendered
// and none.
func TestTheChartDefaultsAreTheBackendDefaults(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "deploy", "helm", "dash", "values.yaml"))
	require.NoError(t, err)
	var values map[string]any
	require.NoError(t, yaml.Unmarshal(data, &values))

	// What templates/configmap.yaml renders: these sections, without the
	// chart's own keys of icons.
	icons, ok := values["icons"].(map[string]any)
	require.True(t, ok, "values.yaml has icons")
	delete(icons, "configMap")
	delete(icons, "existingConfigMaps")
	rendered, err := yaml.Marshal(map[string]any{
		"discovery": values["discovery"], "ui": values["ui"], "apps": values["apps"],
		"bookmarks": values["bookmarks"], "icons": icons, "badges": values["badges"],
	})
	require.NoError(t, err)

	var f File // no defaults: the values set every key
	d := decodeStrict(rendered, &f)
	require.Empty(t, d.messages())
	require.Empty(t, validate(&f, d))
	got, err := yaml.Marshal(f)
	require.NoError(t, err)
	want, err := yaml.Marshal(DefaultFile())
	require.NoError(t, err)
	assert.Equal(t, string(want), string(got))
}

func TestLinesFallBackToTheEnclosingKey(t *testing.T) {
	l := lines{"": 1, "a": 2, "a.b[0]": 3}
	assert.Equal(t, 3, l.of("a.b[0]"))
	assert.Equal(t, 3, l.of("a.b[0].c"))
	assert.Equal(t, 2, l.of("a.x"))
	assert.Equal(t, 1, l.of("z"))
	assert.Equal(t, 0, lines{}.of("z"))
	assert.Equal(t, "a.b", parent("a.b[0]"))
	assert.Empty(t, parent("a"))
}
