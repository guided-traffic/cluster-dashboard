package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

// contentSecurityPolicy is the shell's policy, cowork's string (its
// frontend/nginx/default.conf): everything the shell loads and every request
// it makes stays on this origin. Its scripts are the bundle's files — the
// production build inlines no critical CSS ("inlineCritical": false in
// angular.json), and theme-init.js is a file, because this policy refuses
// inline scripts; its styles are files plus the <style> elements Angular
// writes at run time, hence 'unsafe-inline' for styles and never for scripts;
// its requests go to /api/ on the same origin, which the Ingress sends to the
// backend.
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'"

// misroutedDetail says why an /api/ path answers 404 here.
const misroutedDetail = "the frontend serves the UI only; the Ingress must route /api/ to the backend Service"

// problemBody is the RFC 9457 body of the misrouted answer, in the backend's
// shape (backend/internal/problem).
type problemBody struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
	Code     string `json:"code"`
}

// server answers every request from the bundle in memory.
type server struct {
	bundle *bundle
	logger *slog.Logger
}

func newServer(b *bundle, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	s := &server{bundle: b, logger: logger}
	return requestLog(logger, http.HandlerFunc(s.serve))
}

func (s *server) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	p := r.URL.Path
	switch {
	case p == "/healthz":
		healthz(w, r)
	case p == "/api" || strings.HasPrefix(p, "/api/"):
		misrouted(w, r)
	default:
		s.ui(w, r)
	}
}

// healthz is the frontend's own health: it says the server serves, nothing
// about the backend.
func healthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
}

// misrouted answers an /api/ path, whatever the method: it belongs to the
// backend, and reaching this server means the Ingress sent it here. A
// problem that names the cause, never the shell with a 200.
func misrouted(w http.ResponseWriter, r *http.Request) {
	body, _ := json.Marshal(problemBody{
		Type:     "https://dash.gtrfc.com/problems/not-found",
		Title:    "Not found",
		Status:   http.StatusNotFound,
		Detail:   misroutedDetail,
		Instance: r.URL.Path,
		Code:     "not_found",
	})
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write(append(body, '\n'))
}

// ui serves the bundle: a file under its own path, else the shell for every
// path the router owns. A path whose last segment has an extension and names
// no file is a missing file, not a route — an old chunk after a rollout, a
// typo — and answers 404 rather than HTML a script tag would choke on.
func (s *server) ui(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Security-Policy", contentSecurityPolicy)
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	clean := path.Clean("/" + r.URL.Path)
	if a, ok := s.bundle.assets[clean]; ok {
		serveAsset(w, r, a)
		return
	}
	if strings.Contains(path.Base(clean), ".") {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found\n"))
		return
	}
	serveAsset(w, r, s.bundle.index)
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, HEAD")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusMethodNotAllowed)
	_, _ = w.Write([]byte("method not allowed\n"))
}

// serveAsset answers with a's bytes — gzip when the client takes it and asks
// no range — and leaves conditional requests, ranges and HEAD to
// http.ServeContent.
func serveAsset(w http.ResponseWriter, r *http.Request, a *asset) {
	h := w.Header()
	h.Set("Content-Type", a.contentType)
	h.Set("Cache-Control", a.cache)
	content, etag := a.content, a.etag
	if a.gzipped != nil {
		h.Add("Vary", "Accept-Encoding")
		if r.Header.Get("Range") == "" && acceptsGzip(r.Header.Values("Accept-Encoding")) {
			content, etag = a.gzipped, a.gzipETag
			h.Set("Content-Encoding", "gzip")
		}
	}
	h.Set("ETag", etag)
	http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(content))
}

// acceptsGzip reads Accept-Encoding: gzip, or *, with a quality above zero.
func acceptsGzip(values []string) bool {
	gzipQ, starQ := -1.0, -1.0
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
			q := 1.0
			if k, val, ok := strings.Cut(strings.TrimSpace(params), "="); ok && strings.TrimSpace(k) == "q" {
				if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
					q = f
				}
			}
			switch strings.ToLower(strings.TrimSpace(name)) {
			case "gzip":
				gzipQ = q
			case "*":
				starQ = q
			}
		}
	}
	if gzipQ >= 0 {
		return gzipQ > 0
	}
	return starQ > 0
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

// requestLog writes one line per request: method, path, status and duration
// — never the query, a header or a body, the backend's rule. The kubelet's
// /healthz is logged at debug.
func requestLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		level := slog.LevelInfo
		if r.URL.Path == "/healthz" {
			level = slog.LevelDebug
		}
		logger.Log(r.Context(), level, "request", "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration", time.Since(start))
	})
}
