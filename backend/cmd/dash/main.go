// Command dash is the backend binary: it serves the API under /api/v1 and the
// health endpoints. The web UI is the frontend container's (docs/adr/0001).
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/guided-traffic/dash/backend/internal/api"
	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/dashboard"
	"github.com/guided-traffic/dash/backend/internal/discovery"
	"github.com/guided-traffic/dash/backend/internal/httpserver"
	"github.com/guided-traffic/dash/backend/internal/kube"
	"github.com/guided-traffic/dash/backend/internal/problem"
)

// Set by the linker; see the Containerfile and the Makefile.
var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

const usageText = `Usage: dash <command>

Commands:
  serve     Serve the API and the health endpoints until SIGTERM or SIGINT.
  version   Print the build version and exit.

Configuration: DASH_CONFIG names the YAML file (default /etc/dash/config.yaml),
DASH_LISTEN_ADDR the address (default :8080), DASH_LOG_LEVEL the log level
(default info); the reference is the configuration section of README.md.
`

// shutdownTimeout is how long the requests in flight get once a signal ends
// the server; the chart's terminationGracePeriodSeconds is longer.
const shutdownTimeout = 15 * time.Second

func main() {
	os.Exit(run(context.Background(), os.Args[1:], defaultDeps()))
}

// deps is what run reaches outside the process; a test replaces each.
type deps struct {
	lookup   func(string) (string, bool)
	readFile func(string) ([]byte, error)
	// clients builds the Kubernetes clients: the one the readiness probe
	// asks, and the one the discovery watches with.
	clients func() (clients, error)
	listen  func(addr string) (net.Listener, error)
	stdout  io.Writer
	stderr  io.Writer
}

// clients are the backend's connections to the Kubernetes API server.
type clients struct {
	version kube.VersionGetter
	kube    kubernetes.Interface
}

func defaultDeps() deps {
	return deps{
		lookup:   os.LookupEnv,
		readFile: os.ReadFile,
		clients: func() (clients, error) {
			cfg, err := kube.RESTConfig("dash/" + version)
			if err != nil {
				return clients{}, err
			}
			getter, err := kube.NewVersionGetter(cfg)
			if err != nil {
				return clients{}, err
			}
			client, err := kubernetes.NewForConfig(cfg)
			if err != nil {
				return clients{}, fmt.Errorf("create the Kubernetes client: %w", err)
			}
			return clients{version: getter, kube: client}, nil
		},
		listen: func(addr string) (net.Listener, error) { return net.Listen("tcp", addr) },
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
}

// run dispatches the command line and returns the exit code.
func run(ctx context.Context, args []string, d deps) int {
	if len(args) == 0 {
		fmt.Fprint(d.stderr, usageText)
		return 2
	}
	switch args[0] {
	case "version":
		fmt.Fprintf(d.stdout, "dash %s (commit %s, built %s)\n", version, commit, buildTime)
		return 0
	case "help", "-h", "--help":
		fmt.Fprint(d.stdout, usageText)
		return 0
	case "serve":
		return serve(ctx, d)
	default:
		fmt.Fprintf(d.stderr, "dash: unknown command %q\n\n%s", args[0], usageText)
		return 2
	}
}

// serve loads the configuration, refusing to start on any problem with it,
// connects to the Kubernetes API server and starts the discovery in the
// background, and serves until a signal or ctx ends it.
func serve(ctx context.Context, d deps) int {
	cfg, err := config.Load(d.lookup, d.readFile)
	if err != nil {
		fmt.Fprintf(d.stderr, "dash: %v\n", err)
		return 1
	}
	logger := slog.New(slog.NewJSONHandler(d.stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	// client-go's own messages, through klog, in the same log and format.
	klog.SetSlogLogger(logger)
	logger.Info("starting", "version", version, "commit", commit, "config", configSource(cfg), "memory_limit", memoryLimit())

	cl, err := d.clients()
	if err != nil {
		logger.Error("no Kubernetes client", "error", err)
		return 1
	}

	// serve returns once the work it started in the background has stopped:
	// the deferred stop ends it, then this wait — deferred first, run last.
	var background sync.WaitGroup
	defer background.Wait()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	probe := kube.NewProbe(cl.version)
	background.Go(func() { probe.Run(ctx, logger) })
	ready := []func(context.Context) error{probe.Ready}

	var disc *discovery.Discovery
	if cfg.File.Discovery.Enabled {
		disc, err = newDiscovery(cl.kube, cfg.File.Discovery, logger)
		if err != nil {
			logger.Error("discovery setup failed", "error", err)
			return 1
		}
		background.Go(func() { disc.Run(ctx) })
		ready = append(ready, disc.Ready)
	} else {
		logger.Info("the discovery is off: the dashboard shows the configured apps and bookmarks only")
	}

	apiHandler, err := api.New(api.Options{
		Logger: logger, Version: version, Commit: commit, BuildTime: buildTime,
		Dashboard: dashboardSource(disc, cfg.File),
	})
	if err != nil {
		logger.Error("API setup failed", "error", err)
		return 1
	}
	root := httpserver.New(httpserver.Options{Ready: allReady(ready), API: apiHandler, Logger: logger})

	ln, err := d.listen(cfg.ListenAddr)
	if err != nil {
		logger.Error("cannot listen", "addr", cfg.ListenAddr, "error", err)
		return 1
	}
	logger.Info("listening", "addr", ln.Addr().String())
	if err := httpserver.Serve(ctx, ln, root, shutdownTimeout); err != nil {
		logger.Error("server stopped with error", "error", err)
		return 1
	}
	logger.Info("stopped")
	return 0
}

// newDiscovery sets the discovery up as the configuration selects.
func newDiscovery(client kubernetes.Interface, c config.Discovery, logger *slog.Logger) (*discovery.Discovery, error) {
	opts, err := discovery.OptionsFrom(c)
	if err != nil {
		return nil, err
	}
	return discovery.New(client, opts, logger)
}

// dashboardSource answers the page's content now: the discovery's apps — once
// it has listed, not_ready before — and the configuration's.
func dashboardSource(disc *discovery.Discovery, f config.File) func(context.Context) (dashboard.Dashboard, error) {
	return func(ctx context.Context) (dashboard.Dashboard, error) {
		var apps []discovery.App
		if disc != nil {
			if err := disc.Ready(ctx); err != nil {
				return dashboard.Dashboard{}, problem.New(problem.NotReady, "the discovery has not listed the Ingresses yet")
			}
			apps = disc.Apps()
		}
		return dashboard.Build(apps, f), nil
	}
}

// allReady is ready when every check is: the API server has answered, and
// the discovery, when it runs, has listed.
func allReady(checks []func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		for _, check := range checks {
			if err := check(ctx); err != nil {
				return err
			}
		}
		return nil
	}
}

// configSource names the file the configuration came from, for the log.
func configSource(cfg config.Config) string {
	if cfg.ConfigPath == "" {
		return "none: " + config.DefaultConfigPath + " does not exist, the defaults apply"
	}
	return cfg.ConfigPath
}

// memoryLimit is the Go runtime's soft memory limit — GOMEMLIMIT, which the
// chart sets to the container's memory limit — or "none".
func memoryLimit() string {
	limit := debug.SetMemoryLimit(-1)
	if limit == math.MaxInt64 {
		return "none"
	}
	return fmt.Sprintf("%d bytes", limit)
}
