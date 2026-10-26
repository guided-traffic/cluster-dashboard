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
	"syscall"
	"time"

	"github.com/guided-traffic/dash/backend/internal/api"
	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/httpserver"
	"github.com/guided-traffic/dash/backend/internal/kube"
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
	// versionGetter builds the Kubernetes client the readiness probe asks.
	versionGetter func() (kube.VersionGetter, error)
	listen        func(addr string) (net.Listener, error)
	stdout        io.Writer
	stderr        io.Writer
}

func defaultDeps() deps {
	return deps{
		lookup:   os.LookupEnv,
		readFile: os.ReadFile,
		versionGetter: func() (kube.VersionGetter, error) {
			cfg, err := kube.RESTConfig("dash/" + version)
			if err != nil {
				return nil, err
			}
			return kube.NewVersionGetter(cfg)
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
// connects to the Kubernetes API server in the background, and serves until
// a signal or ctx ends it.
func serve(ctx context.Context, d deps) int {
	cfg, err := config.Load(d.lookup, d.readFile)
	if err != nil {
		fmt.Fprintf(d.stderr, "dash: %v\n", err)
		return 1
	}
	logger := slog.New(slog.NewJSONHandler(d.stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))
	logger.Info("starting", "version", version, "commit", commit, "config", configSource(cfg), "memory_limit", memoryLimit())

	getter, err := d.versionGetter()
	if err != nil {
		logger.Error("no Kubernetes client", "error", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	probe := kube.NewProbe(getter)
	go probe.Run(ctx, logger)

	apiHandler, err := api.New(api.Options{Logger: logger, Version: version, Commit: commit, BuildTime: buildTime})
	if err != nil {
		logger.Error("API setup failed", "error", err)
		return 1
	}
	root := httpserver.New(httpserver.Options{Ready: probe.Ready, API: apiHandler, Logger: logger})

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
