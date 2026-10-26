// Command dash is the backend binary: it serves the API under /api/v1 and the
// health endpoints. The web UI is the frontend container's (docs/adr/0001).
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"math"
	"net"
	"os"
	"os/signal"
	"runtime/debug"
	"slices"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"

	"github.com/guided-traffic/dash/backend/internal/api"
	"github.com/guided-traffic/dash/backend/internal/badges"
	"github.com/guided-traffic/dash/backend/internal/config"
	"github.com/guided-traffic/dash/backend/internal/dashboard"
	"github.com/guided-traffic/dash/backend/internal/discovery"
	"github.com/guided-traffic/dash/backend/internal/httpserver"
	"github.com/guided-traffic/dash/backend/internal/icons"
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

	// serve returns once the work it started in the background has stopped:
	// the deferred stop ends it, then this wait — deferred first, run last.
	var background sync.WaitGroup
	defer background.Wait()
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	resolver, closeIcons, err := openIcons(ctx, cfg, logger)
	if err != nil {
		logger.Error("the icons cannot be served", "error", err)
		return 1
	}
	defer closeIcons()
	registerIcons(resolver, cfg.File, logger)
	engine, err := newBadges(cfg.File.Badges, resolver)
	if err != nil {
		logger.Error("the badges cannot be drawn", "error", err)
		return 1
	}

	cl, err := d.clients()
	if err != nil {
		logger.Error("no Kubernetes client", "error", err)
		return 1
	}

	probe := kube.NewProbe(cl.version)
	background.Go(func() { probe.Run(ctx, logger) })
	ready := []func(context.Context) error{probe.Ready}

	var disc *discovery.Discovery
	if cfg.File.Discovery.Enabled {
		disc, err = newDiscovery(cl.kube, cfg.File, resolver, engine, logger)
		if err != nil {
			logger.Error("discovery setup failed", "error", err)
			return 1
		}
		background.Go(func() { disc.Run(ctx) })
		ready = append(ready, disc.Ready)
	} else {
		logger.Info("the discovery is off: the dashboard shows the configured apps and bookmarks only")
	}
	// Last: the downloads of the URLs known by now, the discovery's first
	// list included, have ended — downloaded or failed.
	ready = append(ready, once(func(context.Context) error { return resolver.Settled() }))

	apiHandler, err := api.New(api.Options{
		Logger: logger, Version: version, Commit: commit, BuildTime: buildTime,
		Dashboard: dashboardSource(disc, cfg.File, resolver, engine),
		Icons:     resolver,
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

// newDiscovery sets the discovery up as the configuration selects: an
// Ingress's icon is registered with the resolver, its badges parsed by the
// engine; IngressClasses and the annotations the rules read are kept while
// badge rules exist.
func newDiscovery(client kubernetes.Interface, f config.File, r *icons.Resolver, engine *badges.Engine, logger *slog.Logger) (*discovery.Discovery, error) {
	opts, err := discovery.OptionsFrom(f.Discovery)
	if err != nil {
		return nil, err
	}
	opts.RegisterIcon = func(ref string) error { return r.Register(ref, true) }
	opts.OwnBadges = func(raw string) ([]badges.Entry, []string) { return engine.Own(badges.SplitList(raw)) }
	opts.IngressClasses = len(f.Badges.Rules) > 0
	opts.KeepAnnotations = engine.AnnotationKeys()
	return discovery.New(client, opts, logger)
}

// newBadges sets the badges up from the configuration: every definition's
// glyph must resolve, or the backend does not start.
func newBadges(c config.Badges, glyphs badges.Glyphs) (*badges.Engine, error) {
	defs := make(map[string]badges.Definition, len(c.Definitions))
	for name, d := range c.Definitions {
		defs[name] = badges.Definition{Icon: d.Icon, Text: d.Text, Label: d.Label, Tone: d.Tone}
	}
	rules := make([]badges.Rule, 0, len(c.Rules))
	for i, r := range c.Rules {
		m := r.Match
		rule := badges.Rule{Badge: r.Badge, Corner: badges.Corner(r.Corner), Match: badges.Match{
			IngressClassNames: m.IngressClassNames, IngressControllers: m.IngressControllers, Namespaces: m.Namespaces,
			Hosts: m.Hosts, Annotations: m.Annotations, Kinds: m.Kinds,
		}}
		if !m.Labels.IsEmpty() {
			sel, err := metav1.LabelSelectorAsSelector(discovery.LabelSelector(m.Labels))
			if err != nil {
				return nil, fmt.Errorf("badges.rules[%d].match.labels: %w", i, err)
			}
			rule.Match.Labels = sel
		}
		rules = append(rules, rule)
	}
	return badges.New(defs, rules, glyphs)
}

// openIcons opens the icon packs and the icon ConfigMaps and sets the
// resolution up: the defaults must resolve, or the backend does not start.
// Downloads end with ctx. It returns what closes the packs.
func openIcons(ctx context.Context, cfg config.Config, logger *slog.Logger) (*icons.Resolver, func(), error) {
	packs, err := icons.OpenPacks(cfg.IconPacks)
	if err != nil {
		return nil, nil, err
	}
	for _, catalog := range slices.Sorted(maps.Keys(packs)) {
		p := packs[catalog]
		logger.Info("an icon pack is open", "catalog", catalog, "version", p.Version, "icons", p.Count)
	}
	var autoMatch []string
	if cfg.File.Icons.AutoMatch.Enabled {
		for _, name := range cfg.File.Icons.AutoMatch.Catalogs {
			autoMatch = append(autoMatch, icons.AutoMatchCatalogs[name])
		}
	}
	remote := cfg.File.Icons.Remote
	r, err := icons.NewResolver(icons.Options{
		Packs: packs, AutoMatch: autoMatch,
		DefaultApp: cfg.File.Icons.DefaultAppIcon, DefaultBookmark: cfg.File.Icons.DefaultBookmarkIcon,
		ConfigMaps: cfg.IconConfigMaps,
		Remote: icons.RemoteOptions{
			Enabled: remote.Enabled, FromAnnotations: remote.FromAnnotations, MaxEntries: remote.MaxEntries,
			Fetch: icons.FetchOptions{
				AllowPrivateNetworks: remote.AllowPrivateNetworks, AllowedHosts: remote.AllowedHosts,
				MaxBytes: remote.MaxBytes, Timeout: remote.Timeout,
			},
		},
		Context: ctx, Logger: logger,
	})
	if err != nil {
		packs.Close()
		return nil, nil, fmt.Errorf("%w (icon packs in %s)", err, cfg.IconPacks)
	}
	return r, packs.Close, nil
}

// registerIcons makes the configuration's icons known at start: a URL starts
// its download, so the backend turns ready once it arrived or failed; an
// icon that does not resolve is a warning, and its item falls back.
func registerIcons(r *icons.Resolver, f config.File, logger *slog.Logger) {
	register := func(key, ref string) {
		if ref == "" {
			return
		}
		if err := r.Register(ref, false); err != nil {
			logger.Warn("an icon of the configuration does not resolve; its item falls back", "key", key, "reason", err.Error())
		}
	}
	for gi, g := range f.Apps {
		for ai, a := range g.Apps {
			register(fmt.Sprintf("apps[%d].apps[%d].icon", gi, ai), a.Icon)
		}
	}
	for gi, g := range f.Bookmarks {
		for bi, b := range g.Bookmarks {
			register(fmt.Sprintf("bookmarks[%d].bookmarks[%d].icon", gi, bi), b.Icon)
		}
	}
}

// once is ready from the first time check is, and stays ready.
func once(check func(context.Context) error) func(context.Context) error {
	var ok atomic.Bool
	return func(ctx context.Context) error {
		if ok.Load() {
			return nil
		}
		if err := check(ctx); err != nil {
			return err
		}
		ok.Store(true)
		return nil
	}
}

// dashboardSource answers the page's content now: the discovery's apps — once
// it has listed, not_ready before — and the configuration's, with their icons.
func dashboardSource(disc *discovery.Discovery, f config.File, ic dashboard.Icons, bg dashboard.Badges) func(context.Context) (dashboard.Dashboard, error) {
	return func(ctx context.Context) (dashboard.Dashboard, error) {
		var apps []discovery.App
		if disc != nil {
			if err := disc.Ready(ctx); err != nil {
				return dashboard.Dashboard{}, problem.New(problem.NotReady, "the discovery has not listed the Ingresses yet")
			}
			apps = disc.Apps()
		}
		return dashboard.Build(apps, f, ic, bg), nil
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
