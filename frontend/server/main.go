// Command server is the frontend image's file server (docs/adr/0001 D4). It
// loads the Angular bundle into memory at start and serves it with the rules
// of cowork's nginx configuration, without nginx: every path the router owns
// answered with index.html, never cached; hashed files immutable; the shell's
// Content-Security-Policy on everything of the UI; its own /healthz; a 404
// problem for an /api/ path that reached it instead of the backend. It never
// reaches the backend, and the image runs it with its defaults: the flags
// exist for a run outside the image. Standard library only.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// shutdownTimeout is how long the requests in flight get once a signal ends
// the server.
const shutdownTimeout = 15 * time.Second

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stderr, nil))
}

// run serves until a signal or ctx ends it. ready, when set, receives the
// bound address once the server listens.
func run(ctx context.Context, args []string, stderr io.Writer, ready chan<- string) int {
	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(stderr)
	addr := flags.String("addr", ":8080", "the address to listen on")
	root := flags.String("root", "/app/www", "the directory of the Angular bundle, read once at start")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	logger := slog.New(slog.NewJSONHandler(stderr, nil))

	b, err := loadBundle(*root)
	if err != nil {
		logger.Error("cannot load the bundle", "root", *root, "error", err)
		return 1
	}
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		logger.Error("cannot listen", "addr", *addr, "error", err)
		return 1
	}
	logger.Info("listening", "addr", ln.Addr().String(), "files", len(b.assets))
	if ready != nil {
		ready <- ln.Addr().String()
	}

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := serve(ctx, ln, newServer(b, logger)); err != nil {
		logger.Error("server stopped with error", "error", err)
		return 1
	}
	logger.Info("stopped")
	return 0
}

// serve answers on ln until ctx ends, then lets the requests in flight finish
// within shutdownTimeout. It closes ln.
func serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 120 * time.Second}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shut down: %w", err)
	}
	// After a clean shutdown Serve answers ErrServerClosed; anything else
	// went wrong while it ran.
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
