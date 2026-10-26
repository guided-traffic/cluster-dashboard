//go:build integration

package integration

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/guided-traffic/dash/backend/internal/httpserver"
	"github.com/guided-traffic/dash/backend/internal/kube"
)

// probeWithin runs the probe until the API server answers, within a bound.
func probeWithin(t *testing.T, getter kube.VersionGetter) *kube.Probe {
	t.Helper()
	probe := kube.NewProbe(getter)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	probe.Run(ctx, slog.New(slog.DiscardHandler))
	require.NoError(t, probe.Ready(ctx), "the API server answered within 30 s")
	return probe
}

// The backend's client, configured as the backend configures it outside a
// cluster — client-go's loading rules reading the file KUBECONFIG names —
// reaches the API server and reads its version: the one envtest runs.
func TestTheBackendClientReachesTheAPIServer(t *testing.T) {
	t.Setenv("KUBECONFIG", kubeconfigPath)
	cfg, err := kube.RESTConfig("dash/integration")
	require.NoError(t, err)
	getter, err := kube.NewVersionGetter(cfg)
	require.NoError(t, err)

	probe := probeWithin(t, getter)

	assert.Equal(t, "v"+os.Getenv(envVersion), probe.Version().GitVersion)
}

// The readiness endpoint, as serve wires it, answers 503 until the probe has
// reached the API server and 200 after.
func TestReadyzTurnsReadyOnceTheAPIServerAnswered(t *testing.T) {
	t.Setenv("KUBECONFIG", kubeconfigPath)
	cfg, err := kube.RESTConfig("dash/integration")
	require.NoError(t, err)
	getter, err := kube.NewVersionGetter(cfg)
	require.NoError(t, err)
	probe := kube.NewProbe(getter)
	h := httpserver.New(httpserver.Options{Ready: probe.Ready})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code, "not ready before the first answer")

	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	probe.Run(ctx, slog.New(slog.DiscardHandler))

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
}

// The probe asks GET /version, which every client may read: the backend's
// ServiceAccount needs no RBAC for it. The same identity is refused what RBAC
// does not grant it — which also proves this API server authorises by RBAC,
// the ground the discovery's RBAC tests will stand on.
func TestTheProbeNeedsNoRBACWhileRBACIsEnforced(t *testing.T) {
	sa := rest.CopyConfig(adminConfig)
	sa.Impersonate = rest.ImpersonationConfig{
		UserName: "system:serviceaccount:dash:dash-backend",
		Groups:   []string{"system:serviceaccounts", "system:serviceaccounts:dash", "system:authenticated"},
	}
	getter, err := kube.NewVersionGetter(sa)
	require.NoError(t, err)
	probeWithin(t, getter)

	client, err := kubernetes.NewForConfig(sa)
	require.NoError(t, err)
	_, err = client.NetworkingV1().Ingresses("").List(t.Context(), metav1.ListOptions{})
	require.Error(t, err)
	assert.True(t, apierrors.IsForbidden(err), "a ServiceAccount without a binding is forbidden to list Ingresses, got %v", err)
}
