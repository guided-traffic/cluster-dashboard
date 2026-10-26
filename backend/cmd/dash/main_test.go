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
	"runtime/debug"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/guided-traffic/dash/backend/internal/config"
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

// testDeps runs without the process's environment, files or cluster.
func testDeps(env map[string]string, files map[string]string) (deps, *syncBuffer, *syncBuffer) {
	stdout, stderr := &syncBuffer{}, &syncBuffer{}
	return deps{
		lookup: func(name string) (string, bool) { v, ok := env[name]; return v, ok },
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
	d, stdout, _ := testDeps(nil, nil)
	assert.Equal(t, 0, run(context.Background(), []string{"version"}, d))
	assert.Equal(t, "dash dev (commit unknown, built unknown)\n", stdout.String())
}

func TestHelpPrintsTheUsage(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		d, stdout, _ := testDeps(nil, nil)
		assert.Equal(t, 0, run(context.Background(), []string{arg}, d), arg)
		assert.Contains(t, stdout.String(), "DASH_CONFIG", arg)
	}
}

func TestNoCommandIsAUsageError(t *testing.T) {
	d, _, stderr := testDeps(nil, nil)
	assert.Equal(t, 2, run(context.Background(), nil, d))
	assert.Contains(t, stderr.String(), "Usage: dash <command>")
}

func TestAnUnknownCommandIsAUsageError(t *testing.T) {
	d, _, stderr := testDeps(nil, nil)
	assert.Equal(t, 2, run(context.Background(), []string{"migrate"}, d))
	assert.Contains(t, stderr.String(), `dash: unknown command "migrate"`)
}

// An invalid configuration refuses the start and names every problem.
func TestServeRefusesAnInvalidConfiguration(t *testing.T) {
	d, _, stderr := testDeps(map[string]string{"DASH_LOG_LEVEL": "loud", "DASH_CONFIG": "/c.yaml"}, map[string]string{"/c.yaml": "title: x\n"})
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), "DASH_LOG_LEVEL: must be one of debug, info, warn, error")
	assert.Contains(t, stderr.String(), "/c.yaml: line 1: title: is not a known key")
}

func TestServeRefusesToStartWithoutAKubernetesClient(t *testing.T) {
	d, _, stderr := testDeps(nil, nil)
	d.clients = func() (clients, error) { return clients{}, errors.New("no configuration has been provided") }
	assert.Equal(t, 1, run(context.Background(), []string{"serve"}, d))
	assert.Contains(t, stderr.String(), "no Kubernetes client")
}

func TestServeFailsWhenItCannotListen(t *testing.T) {
	d, _, stderr := testDeps(nil, nil)
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
	d, _, stderr := testDeps(map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0", "DASH_LOG_LEVEL": "debug"}, nil)
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

	stopped(t, cancel, exit)
	logs := stderr.String()
	for _, want := range []string{`"msg":"starting"`, `"msg":"the Kubernetes API server answered"`, `"msg":"the discovery listed every Ingress"`, `"msg":"listening"`, `"msg":"stopped"`} {
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
	d, _, stderr := testDeps(map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0"}, nil)
	client := fake.NewClientset()
	client.PrependReactor("list", "ingresses", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "ingresses"}, "", nil)
	})
	d.clients = func() (clients, error) { return clients{version: answering{}, kube: client}, nil }
	base, cancel, exit := serving(t, d)
	require.Eventually(t, func() bool { return strings.Contains(stderr.String(), "forbidden") }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return strings.Contains(stderr.String(), `"the Kubernetes API server answered"`) }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, http.StatusServiceUnavailable, status(t, base+"/readyz"), "the API server answered, the discovery has not listed")
	stopped(t, cancel, exit)
}

// Without the discovery the dashboard serves the configured apps and
// bookmarks: ready once the API server has answered.
func TestServeWithoutTheDiscovery(t *testing.T) {
	d, _, stderr := testDeps(map[string]string{"DASH_LISTEN_ADDR": "127.0.0.1:0", "DASH_CONFIG": "/c.yaml"},
		map[string]string{"/c.yaml": "discovery:\n  enabled: false\n"})
	client := fake.NewClientset()
	d.clients = func() (clients, error) { return clients{version: answering{}, kube: client}, nil }
	base, cancel, exit := serving(t, d)
	require.Eventually(t, func() bool { return status(t, base+"/readyz") == http.StatusOK }, 5*time.Second, 10*time.Millisecond)
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
