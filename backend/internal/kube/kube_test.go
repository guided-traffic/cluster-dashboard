package kube

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes/fake"
)

// noCluster takes the test out of any cluster it might run in: CI's runners
// are pods, and client-go would find their ServiceAccount.
func noCluster(t *testing.T) {
	t.Helper()
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	t.Setenv("KUBERNETES_SERVICE_PORT", "")
}

const kubeconfig = `apiVersion: v1
kind: Config
clusters:
  - name: test
    cluster:
      server: https://203.0.113.10:6443
      insecure-skip-tls-verify: true
users:
  - name: test
    user:
      token: not-a-real-token
contexts:
  - name: test
    context: {cluster: test, user: test}
current-context: test
`

func TestRESTConfigReadsTheKubeconfigKUBECONFIGNames(t *testing.T) {
	noCluster(t)
	path := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(path, []byte(kubeconfig), 0o600))
	t.Setenv("KUBECONFIG", path)

	cfg, err := RESTConfig("dash/test")
	require.NoError(t, err)
	assert.Equal(t, "https://203.0.113.10:6443", cfg.Host)
	assert.Equal(t, "dash/test", cfg.UserAgent)
	assert.Equal(t, "not-a-real-token", cfg.BearerToken)
}

// Outside a cluster and without a kubeconfig there is nothing to connect to:
// RESTConfig says so instead of guessing a server.
func TestRESTConfigFailsWithoutAnyConfiguration(t *testing.T) {
	noCluster(t)
	t.Setenv("KUBECONFIG", filepath.Join(t.TempDir(), "absent"))
	_, err := RESTConfig("dash/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load the Kubernetes client configuration")
}

func TestNewVersionGetterBoundsEachRequest(t *testing.T) {
	noCluster(t)
	path := filepath.Join(t.TempDir(), "kubeconfig")
	require.NoError(t, os.WriteFile(path, []byte(kubeconfig), 0o600))
	t.Setenv("KUBECONFIG", path)
	cfg, err := RESTConfig("dash/test")
	require.NoError(t, err)

	getter, err := NewVersionGetter(cfg)
	require.NoError(t, err)
	assert.NotNil(t, getter)
	assert.Zero(t, cfg.Timeout, "the caller's configuration keeps no timeout: later watches must not inherit one")
}

// flaky fails a number of times, then answers.
type flaky struct {
	mu       sync.Mutex
	failures int
	calls    int
}

func (f *flaky) ServerVersion() (*version.Info, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.calls <= f.failures {
		return nil, errors.New("dial tcp 10.96.0.1:443: connect: connection refused")
	}
	return &version.Info{GitVersion: "v1.37.0", Platform: "linux/amd64"}, nil
}

func fastProbe(getter VersionGetter) *Probe {
	p := NewProbe(getter)
	p.first, p.max = time.Millisecond, 4*time.Millisecond
	return p
}

func TestTheProbeIsNotReadyBeforeTheFirstAnswer(t *testing.T) {
	p := NewProbe(&flaky{})
	assert.ErrorIs(t, p.Ready(context.Background()), ErrNotReached)
	assert.Nil(t, p.Version())
}

func TestTheProbeRetriesUntilTheAPIServerAnswers(t *testing.T) {
	getter := &flaky{failures: 3}
	p := fastProbe(getter)

	p.Run(context.Background(), slog.New(slog.DiscardHandler))

	require.NoError(t, p.Ready(context.Background()))
	assert.Equal(t, "v1.37.0", p.Version().GitVersion)
	assert.Equal(t, 4, getter.calls, "three failures, then the answer, then no more requests")
}

func TestTheProbeStopsWhenItsContextEnds(t *testing.T) {
	p := fastProbe(&flaky{failures: 1 << 30})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		p.Run(ctx, slog.New(slog.DiscardHandler))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context ended")
	}
	assert.ErrorIs(t, p.Ready(context.Background()), ErrNotReached)
}

func TestTheWaitDoublesUpToItsBound(t *testing.T) {
	getter := &flaky{failures: 6}
	p := NewProbe(getter)
	p.first, p.max = time.Millisecond, 8*time.Millisecond
	start := time.Now()
	p.Run(context.Background(), slog.New(slog.DiscardHandler))
	// 1 + 2 + 4 + 8 + 8 + 8 ms: the bound holds the later waits.
	assert.GreaterOrEqual(t, time.Since(start), 31*time.Millisecond)
	require.NoError(t, p.Ready(context.Background()))
}

// client-go's own fake discovery is a VersionGetter: the probe works with the
// client the backend builds.
func TestTheFakeDiscoveryIsAVersionGetter(t *testing.T) {
	clientset := fake.NewClientset()
	disco, ok := clientset.Discovery().(*fakediscovery.FakeDiscovery)
	require.True(t, ok)
	disco.FakedServerVersion = &version.Info{GitVersion: "v1.37.0"}

	p := NewProbe(clientset.Discovery())
	p.Run(context.Background(), slog.New(slog.DiscardHandler))
	assert.Equal(t, "v1.37.0", p.Version().GitVersion)
}
