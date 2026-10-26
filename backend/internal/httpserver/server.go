// Package httpserver assembles the backend's HTTP handler — the health
// endpoints and the API under /api/ — and runs the server. The web UI is not
// served here; it is the frontend container's, and the Ingress
// <fullname>-backend routes /api/ to this one (docs/adr/0001 D2). Every
// request gets an id (X-Request-Id), a recovery from panics and one line in
// the request log; every error is an RFC 9457 problem details body.
package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/guided-traffic/dash/backend/internal/problem"
	"github.com/guided-traffic/dash/backend/internal/requestid"
)

// Options configures New.
type Options struct {
	// Ready reports whether the backend can do its work: whether the
	// Kubernetes API server has answered since the start. nil means always
	// ready.
	Ready func(ctx context.Context) error
	// API serves every path under /api/. nil answers them with 404.
	API http.Handler
	// Logger receives one line per request. nil discards them.
	Logger *slog.Logger
}

// New builds the handler.
func New(opts Options) http.Handler {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	mux := http.NewServeMux()
	handleGet(mux, "/healthz", handleHealthz)
	handleGet(mux, "/readyz", handleReadyz(opts.Ready, opts.Logger))
	if opts.API != nil {
		mux.Handle("/api/", opts.API)
	}
	mux.HandleFunc("/", handleNotFound)
	return withRequestID(requestLog(opts.Logger, recoverer(opts.Logger, mux)))
}

// handleGet registers h for GET (and HEAD) on pattern and answers every other
// method on the same path with 405. Without the second registration the
// catch-all "/" would match those requests and answer 404.
func handleGet(mux *http.ServeMux, pattern string, h http.HandlerFunc) {
	mux.HandleFunc("GET "+pattern, h)
	mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "GET, HEAD")
		problem.Write(w, r, problem.New(problem.MethodNotAllowed, r.Method+" is not allowed on "+r.URL.Path))
	})
}

// ListenAndServe serves handler on addr until ctx is cancelled, then shuts
// down gracefully within shutdownTimeout. A clean shutdown returns nil.
func ListenAndServe(ctx context.Context, addr string, handler http.Handler, shutdownTimeout time.Duration) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return Serve(ctx, ln, handler, shutdownTimeout)
}

// Serve is ListenAndServe on an existing listener, which it closes. The
// requests in flight when ctx ends are answered before it returns, within
// shutdownTimeout.
func Serve(ctx context.Context, ln net.Listener, handler http.Handler, shutdownTimeout time.Duration) error {
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
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
		return err
	}
	// After a clean shutdown Serve answers ErrServerClosed; anything else
	// went wrong while it ran.
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// handleHealthz answers that the process serves: the liveness probe.
func handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleReadyz answers whether the backend can do its work: the readiness
// probe. The check's error goes to the log only: it can name an internal
// address.
func handleReadyz(ready func(ctx context.Context) error, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ready != nil {
			if err := ready(r.Context()); err != nil {
				logger.Debug("not ready", "request_id", requestid.From(r.Context()), "error", err)
				problem.Write(w, r, problem.New(problem.NotReady, "the Kubernetes API server has not answered since the backend started"))
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	problem.Write(w, r, problem.New(problem.NotFound, "no route "+r.Method+" "+r.URL.Path))
}

// withRequestID gives every request an id of the backend's making — an
// inbound X-Request-Id is not trusted — and answers it in X-Request-Id.
func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := requestid.New()
		w.Header().Set("X-Request-Id", id.String())
		next.ServeHTTP(w, r.WithContext(requestid.With(r.Context(), id)))
	})
}

// recoverer answers a panic with a 500 that names the request id and logs
// the panic; nothing internal reaches the client.
func recoverer(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				logger.Error("panic", "request_id", requestid.From(r.Context()), "method", r.Method, "path", r.URL.Path, "panic", v)
				problem.Write(w, r, problem.New(problem.Internal, "internal error"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the status code for the request log.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	if s.status == 0 {
		s.status = code
	}
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the connection.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// probePaths are the kubelet's: a line every few seconds per probe and pod,
// logged at debug so the log at info holds what people and clients did.
var probePaths = map[string]bool{"/healthz": true, "/readyz": true}

// requestLog writes one line per request: method, path, status, duration and
// the request id — never a body, a header or a query.
func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		level := slog.LevelInfo
		if probePaths[r.URL.Path] {
			level = slog.LevelDebug
		}
		logger.Log(r.Context(), level, "request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start),
			"request_id", requestid.From(r.Context()),
		)
	})
}
