// Package kube is the backend's connection to the Kubernetes API server: the
// REST configuration — the pod's ServiceAccount in a cluster, client-go's
// default loading rules elsewhere — and the probe that tells the readiness
// endpoint whether the API server has answered since the start
// (docs/adr/0001 D6).
//
// Ready means "answered once": the probe stops asking after the first answer,
// so a short outage of the API server later does not take every replica out
// of its Service at once. Discovery's informers will hold the same meaning
// when they arrive: ready once their first sync is done.
package kube

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// ProbeTimeout bounds one request of the probe: an API server that accepts
// the connection and never answers must not hold it forever.
const ProbeTimeout = 10 * time.Second

// RESTConfig returns the configuration of the backend's client. In a pod it
// is the pod's ServiceAccount (in-cluster configuration); elsewhere it is
// what client-go's default loading rules find — the files KUBECONFIG names,
// else ~/.kube/config — and its current context. userAgent names the backend
// in the API server's audit log.
func RESTConfig(userAgent string) (*rest.Config, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load the Kubernetes client configuration: %w", err)
	}
	cfg.UserAgent = userAgent
	return cfg, nil
}

// VersionGetter asks the API server for its version, GET /version — which
// every client may read, authenticated or not (the system:public-info-viewer
// role), so the probe needs no RBAC. client-go's discovery client implements
// it.
type VersionGetter interface {
	ServerVersion() (*version.Info, error)
}

// NewVersionGetter returns the discovery client of cfg with ProbeTimeout on
// each request. It is a client of its own: the timeout must not reach the
// long-lived watches of a later client built from the same configuration.
func NewVersionGetter(cfg *rest.Config) (VersionGetter, error) {
	probe := rest.CopyConfig(cfg)
	probe.Timeout = ProbeTimeout
	clientset, err := kubernetes.NewForConfig(probe)
	if err != nil {
		return nil, fmt.Errorf("create the Kubernetes client: %w", err)
	}
	return clientset.Discovery(), nil
}

// ErrNotReached is Ready's answer until the API server has answered.
var ErrNotReached = errors.New("the Kubernetes API server has not answered since the start")

// Probe tells whether the API server has answered since the start.
type Probe struct {
	getter VersionGetter
	// first and max bound the wait between two attempts: it starts at first
	// and doubles up to max.
	first, max time.Duration
	reached    atomic.Pointer[version.Info]
}

// NewProbe returns a probe that asks getter, waiting one second after the
// first failure and up to thirty seconds after later ones.
func NewProbe(getter VersionGetter) *Probe {
	return &Probe{getter: getter, first: time.Second, max: 30 * time.Second}
}

// Run asks the API server for its version until it answers or ctx ends. It
// logs the first answer and every failure; the error of a failure can name
// the API server's address, so it goes to the log only.
func (p *Probe) Run(ctx context.Context, logger *slog.Logger) {
	wait := p.first
	for {
		info, err := p.getter.ServerVersion()
		if err == nil {
			p.reached.Store(info)
			logger.Info("the Kubernetes API server answered", "version", info.GitVersion, "platform", info.Platform)
			return
		}
		logger.Warn("the Kubernetes API server did not answer", "error", err, "retry_in", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		wait = min(2*wait, p.max)
	}
}

// Ready is nil once the API server has answered, ErrNotReached before.
func (p *Probe) Ready(context.Context) error {
	if p.reached.Load() == nil {
		return ErrNotReached
	}
	return nil
}

// Version is the API server's version once it has answered, else nil.
func (p *Probe) Version() *version.Info {
	return p.reached.Load()
}
