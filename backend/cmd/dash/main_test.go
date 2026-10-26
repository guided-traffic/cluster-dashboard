package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/icons"
)

// syncBuffer is a bytes.Buffer the server's goroutines and the test share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type answering struct{}

func (answering) ServerVersion() (*k8sversion.Info, error) {
	return &k8sversion.Info{GitVersion: "v1.37.0", Platform: "linux/arm64"}, nil
}

// testPacks is a folder with the icon packs the defaults need:
// material-symbols with apps and bookmark.
func testPacks(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	glyph := []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg>`)
	require.NoError(t, icons.WritePack(filepath.Join(dir, "material-symbols.zip"),
		icons.Manifest{Catalog: icons.CatalogMaterialSymbols, Version: "1.2.96"},
		map[string][]byte{"apps": glyph, "bookmark": glyph}))
	return dir
}

// testDeps runs without the process's environment, files or cluster; the
// icon packs are testPacks unless env names others.
func testDeps(t *testing.T, env map[string]string, files map[string]string) (deps, *syncBuffer, *syncBuffer) {
	t.Helper()
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	packs := testPacks(t)
	return deps{
		lookup: func(name string) (string, bool) {
			if name == config.EnvIconPacks {
				if v, ok := env[name]; ok {
					return v, true
				}
				return packs, true
			}
			v, ok := env[name]
			return v, ok
		},
		readFile: func(path string) ([]byte, error) {
			if c, ok := files[path]; ok {
				return []byte(c), nil
			}
			return nil, &fs.PathError{Op: "open", Path: path, Err: fs.ErrNotExist}
		},
		clients: func() (clients, error) { return clients{version: answering{}, kube: fake.NewClientset()}, nil },
		listen:  func(addr string) (net.Listener, error) { return net.Listen("tcp", addr) },
		stdout:  stdout,
		stderr:  stderr,
	}, stdout, stderr
}

func TestVersionPrintsTheBuild(t *testing.T) {
	d, stdout, _ := testDeps(t, nil, nil)
	assert.Equal(t, 0, run(context.Background(), []string{"version"}, d))
	assert.Equal(t, "dash dev (commit unknown, built unknown)\n", stdout.String())
}

func TestHelpPrintsTheUsage(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		d, stdout, _ := testDeps(t, nil, nil)
		assert.Equal(t, 0, run(context.Background(), []string{arg}, d), arg)
		assert.Contains(t, stdout.String(), "DASH_CONFIG", arg)
	}
}

func TestNoCommandIsAUsageError(t *testing.T) {
	d, _, stderr := testDeps(t, nil, nil)
	assert.Equal(t, 2, run(context.Background(), nil, d))
	assert.Contains(t, stderr.String(), "Usage: dash <command>")
}

func TestAnUnknownCommandIsAUsageError(t *testing.T) {
	d, _, stderr := testDeps(t, nil, nil)
	assert.Equal(t, 2, run(context.Background(), []string{"migrate"}, d))
	assert.Contains(t, stderr.String(), `dash: unknown command "migrate"`)
}

// An invalid configuration refuses the start and names every problem.
func TestServeRefusesAnInvalidConfiguration(t *testing.T) {
	d, _, stderr := testDeps(t, map[string]string{"DASH_LOG_LEVEL": "loud", "DASH_CONFIG": "/c.yaml"}, map[string]string{"/c.yaml": "title: x\n"})
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), "DASH_LOG_LEVEL: must be one of debug, info, warn, error")
	assert.Contains(t, stderr.String(), "/c.yaml: line 1: title: is not a known key")
}

func TestServeRefusesToStartWithoutAKubernetesClient(t *testing.T) {
	d, _, stderr := testDeps(t, nil, nil)
	d.clients = func() (clients, error) { return clients{}, errors.New("no configuration has been provided") }
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), "no Kubernetes client")
}

// The icon packs open at start and the defaults must resolve in them: a
// dashboard whose fallback icon is missing does not start.
func TestServeRefusesDefaultsThatDoNotResolve(t *testing.T) {
	d, _, stderr := testDeps(t, map[string]string{config.EnvIconPacks: t.TempDir()}, nil)
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), `"msg":"the icons cannot be served"`)
	assert.Contains(t, stderr.String(), "icons.defaultAppIcon: names no icon of the bundled catalogues")

	broken := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(broken, "x.zip"), []byte("not a zip"), 0o600))
	d, _, stderr = testDeps(t, map[string]string{config.EnvIconPacks: broken}, nil)
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), "the icon pack x.zip")
}

// An icon of the configuration that does not resolve is a warning at start;
// a URL is downloaded at start — here refused, loopback — and the backend
// turns ready once that ended.
func TestServeRegistersTheConfigurationsIcons(t *testing.T) {
	d, _, stderr := testDeps(t, map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0", "DASH_CONFIG": "/c.yaml"}, map[string]string{"/c.yaml": `
discovery: {enabled: false}
apps:
  - group: tools
    apps:
      - {name: Router, url: "https://192.168.1.1", icon: "mdi:nothing"}
bookmarks:
  - group: docs
    bookmarks:
      - {name: Local, url: "https://127.0.0.1", icon: "https://127.0.0.1:1/logo.png"}
`})
	base, cancel, exit := serving(t, d)
	require.Eventually(t, func() bool { return status(t, base+"/readyz") == http.StatusOK }, 5*time.Second, 10*time.Millisecond)
	stopped(t, cancel, exit)
	logs := stderr.String()
	assert.Contains(t, logs, `"msg":"an icon of the configuration does not resolve; its item falls back","key":"apps[0].apps[0].icon","reason":"names no icon of the bundled catalogues"`)
	assert.Contains(t, logs, `"msg":"a remote icon failed to download; its items show their fallback, it is tried again after an hour","url":"https://127.0.0.1:1/logo.png"`)
}

// A badge definition whose glyph does not resolve refuses the start, as the
// configuration's other problems do; with one that does, a rule on the
// IngressClass puts its badge on the discovered app.
func TestServeDrawsTheBadges(t *testing.T) {
	d, _, stderr := testDeps(t, map[string]string{"DASH_CONFIG": "/c.yaml"}, map[string]string{"/c.yaml": `
badges:
  definitions:
    world: {icon: "material-symbols:public", label: reachable worldwide}
`})
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), `"msg":"the badges cannot be drawn","error":"badges.definitions.world.icon: names no icon of the bundled catalogues"`)

	d, _, _ = testDeps(t, map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0", "DASH_CONFIG": "/c.yaml"}, map[string]string{"/c.yaml": `
discovery: {defaultEnable: true}
badges:
  definitions:
    home: {icon: "material-symbols:apps", label: local network only}
  rules:
    - {match: {ingressControllers: [haproxy-ingress.github.io/controller]}, badge: home}
`})
	internal := &networkingv1.IngressClass{
		ObjectMeta: metav1.ObjectMeta{Name: "internal", Annotations: map[string]string{networkingv1.AnnotationIsDefaultIngressClass: "true"}},
		Spec:       networkingv1.IngressClassSpec{Controller: "haproxy-ingress.github.io/controller"},
	}
	app := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "media", Name: "jellyfin"},
		Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "jellyfin.home.arpa"}}},
	}
	d.clients = func() (clients, error) {
		return clients{version: answering{}, kube: fake.NewClientset(internal, app)}, nil
	}
	base, cancel, exit := serving(t, d)
	require.Eventually(t, func() bool { return status(t, base+"/readyz") == http.StatusOK }, 5*time.Second, 10*time.Millisecond)
	res, err := http.Get(base + "/api/v1/dashboard")
	require.NoError(t, err)
	var page struct {
		Groups []struct {
			Apps []struct {
				Badges []struct {
					Corner string `json:"corner"`
					Label  string `json:"label"`
				} `json:"badges"`
			} `json:"apps"`
		} `json:"groups"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	_ = res.Body.Close()
	stopped(t, cancel, exit)
	require.Len(t, page.Groups, 1)
	require.Len(t, page.Groups[0].Apps[0].Badges, 1, "the default class's controller matched")
	assert.Equal(t, "local network only", page.Groups[0].Apps[0].Badges[0].Label)
}

func TestServeFailsWhenItCannotListen(t *testing.T) {
	d, _, stderr := testDeps(t, nil, nil)
	d.listen = func(string) (net.Listener, error) { return nil, errors.New("address already in use") }
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), "cannot listen")
}

// serving runs serve on a free port and returns its base URL, the function
// that ends it and the channel of its exit code.
func serving(t *testing.T, d deps) (string, context.CancelFunc, chan int) {
	t.Helper()
	addr := make(chan string, 1)
	d.listen = func(a string) (net.Listener, error) {
		ln, err := net.Listen("tcp", a)
		if err == nil {
			addr <- ln.Addr().String()
		}
		return ln, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	exit := make(chan int, 1)
	go func() { exit <- run(ctx, []string{"serve"}, d) }()
	return "http://" + <-addr, cancel, exit
}

func stopped(t *testing.T, cancel context.CancelFunc, exit chan int) {
	t.Helper()
	cancel()
	select {
	case code := <-exit:
		assert.Equal(t, 0, code)
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after its context ended")
	}
}

// The whole serve path: the configuration, the probe, the discovery, the
// API and the health endpoints on a real listener, and a graceful end when
// the context ends.
func TestServeServesUntilItsContextEnds(t *testing.T) {
	d, _, stderr := testDeps(t, map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0", "DASH_LOG_LEVEL": "debug"}, nil)
	grafana := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Namespace: "monitoring", Name: "grafana", Annotations: map[string]string{"dash.gtrfc.com/enable": "true"}},
		Spec:       networkingv1.IngressSpec{Rules: []networkingv1.IngressRule{{Host: "grafana.example.com"}}},
	}
	d.clients = func() (clients, error) { return clients{version: answering{}, kube: fake.NewClientset(grafana)}, nil }
	base, cancel, exit := serving(t, d)
	assert.Equal(t, http.StatusOK, status(t, base+"/healthz"))
	require.Eventually(t, func() bool { return status(t, base+"/readyz") == http.StatusOK }, 5*time.Second, 10*time.Millisecond,
		"ready once the API server has answered")
	res, err := http.Get(base + "/api/v1/version")
	require.NoError(t, err)
	var v map[string]string
	require.NoError(t, json.NewDecoder(res.Body).Decode(&v))
	_ = res.Body.Close()
	assert.Equal(t, map[string]string{"version": "dev", "commit": "unknown", "build_time": "unknown"}, v)

	res, err = http.Get(base + "/api/v1/dashboard")
	require.NoError(t, err)
	var page struct {
		Groups []struct {
			Name string `json:"name"`
			Apps []struct {
				ID   string `json:"id"`
				URL  string `json:"url"`
				Icon struct {
					Src string `json:"src"`
				} `json:"icon"`
			} `json:"apps"`
		} `json:"groups"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&page))
	_ = res.Body.Close()
	require.Len(t, page.Groups, 1, "the discovered Ingress, in the group of its namespace")
	assert.Equal(t, "monitoring", page.Groups[0].Name)
	assert.Equal(t, "ingress/monitoring/grafana", page.Groups[0].Apps[0].ID)
	assert.Equal(t, "http://grafana.example.com/", page.Groups[0].Apps[0].URL)
	icon := page.Groups[0].Apps[0].Icon.Src
	assert.Equal(t, "/api/v1/icons/material-symbols@1.2.96/apps.svg", icon, "no logo pack here: the default")
	assert.Equal(t, http.StatusOK, status(t, base+icon), "the icon is served")

	stopped(t, cancel, exit)
	logs := stderr.String()
	for _, want := range []string{`"msg":"starting"`, `"msg":"an icon pack is open","catalog":"material-symbols","version":"1.2.96","icons":2`,
		`"msg":"the Kubernetes API server answered"`, `"msg":"the discovery listed every Ingress"`, `"msg":"listening"`, `"msg":"stopped"`} {
		assert.Contains(t, logs, want)
	}
	assert.Contains(t, logs, `"config":"none: /etc/dash/config.yaml does not exist, the defaults apply"`)
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		assert.True(t, json.Valid([]byte(line)), "every log line is JSON: %s", line)
	}
}

// Ready means the discovery has listed too: a backend whose ServiceAccount
// may not list Ingresses never turns ready, and the log says why.
func TestServeIsNotReadyUntilTheDiscoveryListed(t *testing.T) {
	d, _, stderr := testDeps(t, map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0"}, nil)
	client := fake.NewClientset()
	client.PrependReactor("list", "ingresses", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, "", nil)
	})
	d.clients = func() (clients, error) { return clients{version: answering{}, kube: client}, nil }
	base, cancel, exit := serving(t, d)
	require.Eventually(t, func() bool { return strings.Contains(stderr.String(), "forbidden") }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return strings.Contains(stderr.String(), `"the Kubernetes API server answered"`) }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, http.StatusServiceUnavailable, status(t, base+"/readyz"), "the API server answered, the discovery has not listed")
	assert.Equal(t, http.StatusServiceUnavailable, status(t, base+"/api/v1/dashboard"), "no page from a discovery that has not listed")
	stopped(t, cancel, exit)
}

// Without the discovery the dashboard serves the configured apps and
// bookmarks: ready once the API server has answered.
func TestServeWithoutTheDiscovery(t *testing.T) {
	d, _, stderr := testDeps(t, map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0", "DASH_CONFIG": "/c.yaml"},
		map[string]string{"/c.yaml": "discovery:\n  enabled: false\n"})
	client := fake.NewClientset()
	d.clients = func() (clients, error) { return clients{version: answering{}, kube: client}, nil }
	base, cancel, exit := serving(t, d)
	require.Eventually(t, func() bool { return status(t, base+"/readyz") == http.StatusOK }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, http.StatusOK, status(t, base+"/api/v1/dashboard"), "the configured apps and bookmarks, at once")
	stopped(t, cancel, exit)
	assert.Contains(t, stderr.String(), "the discovery is off")
	assert.Empty(t, client.Actions(), "nothing is listed or watched")
}

func status(t *testing.T, url string) int {
	t.Helper()
	res, err := http.Get(url)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, res.Body)
	_ = res.Body.Close()
	return res.StatusCode
}

func TestConfigSourceNamesTheFile(t *testing.T) {
	assert.Equal(t, "/c.yaml", configSource(config.Config{ConfigPath: "/c.yaml"}))
}

func TestMemoryLimitReadsGOMEMLIMIT(t *testing.T) {
	before := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(before) })
	debug.SetMemoryLimit(268435456)
	assert.Equal(t, "268435456 bytes", memoryLimit())
	debug.SetMemoryLimit(1<<63 - 1)
	assert.Equal(t, "none", memoryLimit())
}
