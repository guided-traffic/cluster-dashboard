//go:build integration

// Package integration is the backend's integration tier (docs/adr/0003 D2):
// the backend's code against a real kube-apiserver and etcd, which
// controller-runtime's envtest starts from the pinned binaries — no Docker, no
// kubelet, no controller manager. It fails, never skips, without them:
// make test-integration downloads them (make envtest-bin) and names their
// folder in DASH_TEST_ENVTEST_ASSETS.
package integration

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/client-go/rest"
	crenvtest "sigs.k8s.io/controller-runtime/pkg/envtest"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

// The variables make test-integration sets.
const (
	envAssets  = "DASH_TEST_ENVTEST_ASSETS"
	envVersion = "DASH_TEST_ENVTEST_VERSION"
	// envRBAC names the folder of the chart's rendered RBAC, a file per mode
	// of the namespace selector (make integration-rbac).
	envRBAC = "DASH_TEST_RBAC_DIR"
)

var (
	// adminConfig is a cluster administrator's (system:masters).
	adminConfig *rest.Config
	// kubeconfigPath is the administrator's kubeconfig as a file, for the
	// path the backend takes outside a cluster: client-go's loading rules.
	kubeconfigPath string
)

func TestMain(m *testing.M) {
	os.Exit(run(m))
}

func run(m *testing.M) int {
	assets := os.Getenv(envAssets)
	if assets == "" {
		fmt.Fprintf(os.Stderr, "FAIL: %s is not set. Run make test-integration: it downloads the pinned envtest binaries into .cache/envtest (make envtest-bin) and sets it.\n", envAssets)
		return 1
	}
	for _, bin := range []string{"etcd", "kube-apiserver"} {
		if _, err := os.Stat(filepath.Join(assets, bin)); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: %s names %s, which holds no %s: run make envtest-bin (%v)\n", envAssets, assets, bin, err)
			return 1
		}
	}
	if os.Getenv(envVersion) == "" {
		fmt.Fprintf(os.Stderr, "FAIL: %s is not set: run make test-integration, which sets it to ENVTEST_K8S_VERSION\n", envVersion)
		return 1
	}
	for _, mode := range rbacModes {
		if _, err := os.Stat(filepath.Join(os.Getenv(envRBAC), mode+".yaml")); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: %s holds no %s.yaml: run make test-integration, which renders the chart's RBAC (make integration-rbac) (%v)\n", envRBAC, mode, err)
			return 1
		}
	}
	// envtest logs through controller-runtime's global logger, which warns
	// once when nothing set it.
	crlog.SetLogger(logr.Discard())

	env := &crenvtest.Environment{
		BinaryAssetsDirectory:    assets,
		ControlPlaneStartTimeout: time.Minute,
		ControlPlaneStopTimeout:  30 * time.Second,
	}
	cfg, err := env.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: start envtest from %s: %v\n", assets, err)
		return 1
	}
	defer func() {
		if err := env.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "stop envtest: %v\n", err)
		}
	}()
	dir, err := os.MkdirTemp("", "dash-integration-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: %v\n", err)
		return 1
	}
	defer os.RemoveAll(dir)
	kubeconfigPath = filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(kubeconfigPath, env.KubeConfig, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "FAIL: write the kubeconfig: %v\n", err)
		return 1
	}
	adminConfig = cfg
	return m.Run()
}
