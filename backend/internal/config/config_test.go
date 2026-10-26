package config

import (
	"errors"
	"io/fs"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// env is a lookup over a map, standing in for os.LookupEnv.
func env(vars map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := vars[name]
		return v, ok
	}
}

// files is a readFile over a map, standing in for os.ReadFile: a path it does
// not hold does not exist.
func files(contents map[string]string) func(string) ([]byte, error) {
	return func(path string) ([]byte, error) {
		if c, ok := contents[path]; ok {
			return []byte(c), nil
		}
		return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
	}
}

func problemsOf(t *testing.T, err error) []string {
	t.Helper()
	var cfgErr *Error
	require.ErrorAs(t, err, &cfgErr)
	return cfgErr.Problems
}

func TestDefaultsWithoutVariablesOrFile(t *testing.T) {
	cfg, err := Load(env(nil), files(nil))
	require.NoError(t, err)
	assert.Equal(t, Config{IconPacks: "/usr/share/dash/icons", IconConfigMaps: "/etc/dash-icons", ListenAddr: ":8080", LogLevel: slog.LevelInfo, File: DefaultFile()}, cfg,
		"no file at the default path is no error: the defaults apply")
}

func TestTheDefaultPathIsReadWhenItExists(t *testing.T) {
	cfg, err := Load(env(nil), files(map[string]string{DefaultConfigPath: "{}\n"}))
	require.NoError(t, err)
	assert.Equal(t, DefaultConfigPath, cfg.ConfigPath)
}

func TestVariablesAreRead(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		EnvListenAddr:     "127.0.0.1:9090",
		EnvLogLevel:       "debug",
		EnvConfig:         "/tmp/cd.yaml",
		EnvIconPacks:      "/opt/icons",
		EnvIconConfigMaps: "/opt/icon-configmaps",
	}), files(map[string]string{"/tmp/cd.yaml": ""}))
	require.NoError(t, err)
	assert.Equal(t, Config{
		ConfigPath: "/tmp/cd.yaml", IconPacks: "/opt/icons", IconConfigMaps: "/opt/icon-configmaps",
		ListenAddr: "127.0.0.1:9090", LogLevel: slog.LevelDebug, File: DefaultFile(),
	}, cfg)
}

func TestEveryLogLevel(t *testing.T) {
	for name, level := range map[string]slog.Level{"debug": slog.LevelDebug, "info": slog.LevelInfo, "warn": slog.LevelWarn, "error": slog.LevelError} {
		cfg, err := Load(env(map[string]string{EnvLogLevel: name}), files(nil))
		require.NoError(t, err, name)
		assert.Equal(t, level, cfg.LogLevel, name)
	}
}

func TestListenAddresses(t *testing.T) {
	for _, ok := range []string{":8080", "0.0.0.0:80", "[::1]:8080", "localhost:0", ":65535"} {
		_, err := Load(env(map[string]string{EnvListenAddr: ok}), files(nil))
		assert.NoError(t, err, ok)
	}
	for _, bad := range []string{"8080", ":http", ":65536", ":-1", "", "local host:80"} {
		_, err := Load(env(map[string]string{EnvListenAddr: bad}), files(nil))
		require.Error(t, err, bad)
		assert.Contains(t, problemsOf(t, err)[0], EnvListenAddr+": ", bad)
	}
}

// Every problem is reported at once: a broken configuration is fixed in one
// round, not one key per restart.
func TestEveryProblemAtOnce(t *testing.T) {
	_, err := Load(env(map[string]string{
		EnvListenAddr: "nope",
		EnvLogLevel:   "verbose",
		EnvConfig:     "/etc/cd.yaml",
	}), files(map[string]string{"/etc/cd.yaml": "ui:\n  title: x\n  defaultTheme: sepia\nsearch: {}\ndiscovery:\n  defaultEnable: maybe\n"}))
	require.Error(t, err)
	assert.Equal(t, []string{
		"DASH_LISTEN_ADDR: must be host:port, such as :8080 or 127.0.0.1:8080",
		"DASH_LOG_LEVEL: must be one of debug, info, warn, error",
		"/etc/cd.yaml: line 3: ui.defaultTheme: must be system, light or dark",
		"/etc/cd.yaml: line 4: search: is not a known key",
		"/etc/cd.yaml: line 6: discovery.defaultEnable: must be true or false",
	}, problemsOf(t, err), "the decoder's problems and the validation's, in the order of their lines")
	assert.Contains(t, err.Error(), "invalid configuration:\n  DASH_LISTEN_ADDR")
	assert.NotContains(t, err.Error(), "verbose", "a value is never echoed")
}

func TestAnExplicitFileMustExist(t *testing.T) {
	_, err := Load(env(map[string]string{EnvConfig: "/nowhere.yaml"}), files(nil))
	assert.Equal(t, []string{"DASH_CONFIG: cannot read /nowhere.yaml: file does not exist"}, problemsOf(t, err))
}

func TestAnEmptyFileVariableIsRefused(t *testing.T) {
	_, err := Load(env(map[string]string{EnvConfig: "", EnvIconPacks: "", EnvIconConfigMaps: ""}), files(nil))
	assert.Equal(t, []string{
		"DASH_ICON_CONFIGMAPS: must name a folder when it is set",
		"DASH_ICON_PACKS: must name a folder when it is set",
		"DASH_CONFIG: must name a file when it is set",
	}, problemsOf(t, err))
}

func TestAnUnreadableDefaultFileIsAProblem(t *testing.T) {
	denied := func(string) ([]byte, error) {
		return nil, &fs.PathError{Op: "open", Path: DefaultConfigPath, Err: fs.ErrPermission}
	}
	_, err := Load(env(nil), denied)
	assert.Equal(t, []string{"DASH_CONFIG: cannot read /etc/dash/config.yaml: permission denied"}, problemsOf(t, err))
}

func TestAnotherReadErrorIsReported(t *testing.T) {
	_, err := Load(env(map[string]string{EnvConfig: "/x"}), func(string) ([]byte, error) { return nil, errors.New("boom") })
	assert.Equal(t, []string{"DASH_CONFIG: cannot read /x: boom"}, problemsOf(t, err))
}

func TestTheFileShapes(t *testing.T) {
	for name, tc := range map[string]struct {
		content  string
		problems []string
	}{
		"empty":            {content: ""},
		"only a comment":   {content: "# nothing yet\n"},
		"empty mapping":    {content: "{}\n"},
		"explicit null":    {content: "~\n"},
		"a list":           {content: "- a\n", problems: []string{"/c.yaml: line 1: the document: must be a mapping of keys"}},
		"a single value":   {content: "hello\n", problems: []string{"/c.yaml: line 1: the document: must be a mapping of keys"}},
		"not yaml":         {content: "a: [\n", problems: []string{"/c.yaml: line 1: did not find expected node content"}},
		"an unknown key":   {content: "title: x\n", problems: []string{"/c.yaml: line 1: title: is not a known key"}},
		"hajimari's names": {content: "defaultEnable: true\nshowAppUrls: false\n", problems: []string{"/c.yaml: line 1: defaultEnable: is not a known key", "/c.yaml: line 2: showAppUrls: is not a known key"}},
		"hajimari's keys":  {content: "ui:\n  showSearch: true\n  lightTheme: x\n", problems: []string{"/c.yaml: line 2: ui.showSearch: is not a known key", "/c.yaml: line 3: ui.lightTheme: is not a known key"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Load(env(map[string]string{EnvConfig: "/c.yaml"}), files(map[string]string{"/c.yaml": tc.content}))
			if tc.problems == nil {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, tc.problems, problemsOf(t, err))
		})
	}
}

// probe is a schema with a field of every kind the decoder walks, for the
// branches the File does not reach.
type probe struct {
	Title    string            `yaml:"title"`
	Enabled  bool              `yaml:"enabled"`
	Count    int               `yaml:"count"`
	Max      uint              `yaml:"max"`
	Ratio    float64           `yaml:"ratio"`
	Timeout  time.Duration     `yaml:"timeout"`
	Hosts    []string          `yaml:"hosts"`
	Labels   map[string]string `yaml:"labels"`
	Nested   probeNested       `yaml:"nested"`
	Items    []probeNested     `yaml:"items"`
	Optional *probeNested      `yaml:"optional"`
	Ignored  string            `yaml:"-"`
	Untagged string
	hidden   string //nolint:unused // proves an unexported field is no key
}

type probeNested struct {
	Name string `yaml:"name"`
}

func TestTheDecoderFillsEveryKind(t *testing.T) {
	var p probe
	d := decodeStrict([]byte(`
title: Dash
enabled: true
count: -3
max: 7
ratio: 0.5
timeout: 20s
hosts: [a.example.com, b.example.com]
labels: {team: platform}
nested: {name: inner}
items:
  - name: one
  - name: two
optional: {name: set}
`), &p)
	require.Empty(t, d.problems)
	assert.Equal(t, probe{
		Title: "Dash", Enabled: true, Count: -3, Max: 7, Ratio: 0.5, Timeout: 20 * time.Second,
		Hosts: []string{"a.example.com", "b.example.com"}, Labels: map[string]string{"team": "platform"},
		Nested: probeNested{Name: "inner"}, Items: []probeNested{{Name: "one"}, {Name: "two"}},
		Optional: &probeNested{Name: "set"},
	}, p)
}

func TestTheDecoderReportsEveryProblem(t *testing.T) {
	var p probe
	d := decodeStrict([]byte(`title: [not, a, string]
enabled: maybe
count: many
max: -1
ratio: half
timeout: soon
hosts: a.example.com
labels: [x]
nested:
  name: inner
  extra: 1
items:
  - name: one
  - nme: two
  - just a string
optional: 3
title: again
Untagged: x
Ignored: x
labels2: {}
`), &p)
	assert.Equal(t, []string{
		"line 1: title: must be a single value",
		"line 2: enabled: must be true or false",
		"line 3: count: must be a whole number",
		"line 4: max: must be a whole number of zero or more",
		"line 5: ratio: must be a number",
		"line 6: timeout: must be a duration such as 20s or 5m",
		"line 7: hosts: must be a list",
		"line 8: labels: must be a mapping",
		"line 11: nested.extra: is not a known key",
		"line 14: items[1].nme: is not a known key",
		"line 15: items[2]: must be a mapping of keys",
		"line 16: optional: must be a mapping of keys",
		"line 17: title: is set twice",
		"line 18: Untagged: is not a known key",
		"line 19: Ignored: is not a known key",
		"line 20: labels2: is not a known key",
	}, d.messages())
}

// yaml.v3 parses a key set twice into a node without complaint; the walk
// refuses it, in a map as in a struct.
func TestTheDecoderRefusesADuplicateMapKey(t *testing.T) {
	var p probe
	d := decodeStrict([]byte("labels:\n  a: x\n  a: y\n"), &p)
	assert.Equal(t, []string{"line 3: labels.a: is set twice"}, d.messages())
	assert.Equal(t, map[string]string{"a": "x"}, p.Labels, "the first value stays")
}

func TestTheDecoderFollowsAnchorsAndLeavesNullAlone(t *testing.T) {
	var p probe
	p.Title = "kept"
	d := decodeStrict([]byte("title: ~\nnested: &n {name: shared}\noptional: *n\n"), &p)
	require.Empty(t, d.problems)
	assert.Equal(t, "kept", p.Title, "an explicit null leaves the default")
	assert.Equal(t, &probeNested{Name: "shared"}, p.Optional)
}
