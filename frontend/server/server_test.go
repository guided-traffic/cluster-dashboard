package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

// A bundle as Angular's production build writes it: the shell, hashed
// scripts, styles and a font, and the files of public/ under their own names.
var (
	indexHTML = `<!doctype html><html><head><script src="theme-init.js"></script></head><body><app-root></app-root>` +
		strings.Repeat("<!-- padding so the shell is worth compressing -->", 40) + `</body></html>`
	mainJS = "console.log('main');\n" + strings.Repeat("export const x = 'a long enough chunk';\n", 60)
)

func writeBundle(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"index.html":                     indexHTML,
		"main-WRY5R6RT.js":               mainJS,
		"chunk-ZD6FVBYZ.js":              "export const y = 1;\n",
		"styles-5INURTSO.css":            strings.Repeat(":root{--stage:#0d121a}\n", 80),
		"theme-init.js":                  "(function(){})();\n",
		"favicon.svg":                    `<svg xmlns="http://www.w3.org/2000/svg"/>`,
		"media/inter-NRMW37G5.woff2":     "wOF2-binary",
		"assets/data.bin":                "\x00\x01",
		"3rdpartylicenses-A1B2C3D4.json": "{}",
	}
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	b, err := loadBundle(writeBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	return newServer(b, nil)
}

func do(h http.Handler, method, target string, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func expect(t *testing.T, what string, got, want any) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", what, got, want)
	}
}

func TestEveryRouterPathIsTheShellNeverCached(t *testing.T) {
	h := newTestServer(t)
	for _, target := range []string{"/", "/index.html", "/apps", "/apps/grafana", "/media/", "/apis", "/a/b/c/"} {
		rec := do(h, http.MethodGet, target, nil)
		expect(t, target+" status", rec.Code, http.StatusOK)
		expect(t, target+" body", rec.Body.String(), indexHTML)
		expect(t, target+" Cache-Control", rec.Header().Get("Cache-Control"), "no-store")
		expect(t, target+" Content-Type", rec.Header().Get("Content-Type"), "text/html; charset=utf-8")
	}
}

func TestHashedFilesAreImmutable(t *testing.T) {
	h := newTestServer(t)
	for target, typ := range map[string]string{
		"/main-WRY5R6RT.js":           "text/javascript; charset=utf-8",
		"/styles-5INURTSO.css":        "text/css; charset=utf-8",
		"/media/inter-NRMW37G5.woff2": "font/woff2",
	} {
		rec := do(h, http.MethodGet, target, nil)
		expect(t, target+" status", rec.Code, http.StatusOK)
		expect(t, target+" Cache-Control", rec.Header().Get("Cache-Control"), "public, max-age=31536000, immutable")
		expect(t, target+" Content-Type", rec.Header().Get("Content-Type"), typ)
	}
}

// A file that keeps its name across builds must reach the browser when it
// changes: revalidated by its ETag, never immutable.
func TestFilesWithoutAHashAreRevalidated(t *testing.T) {
	h := newTestServer(t)
	for target, typ := range map[string]string{
		"/theme-init.js":   "text/javascript; charset=utf-8",
		"/favicon.svg":     "image/svg+xml",
		"/assets/data.bin": "application/octet-stream",
	} {
		rec := do(h, http.MethodGet, target, nil)
		expect(t, target+" Cache-Control", rec.Header().Get("Cache-Control"), "no-cache")
		expect(t, target+" Content-Type", rec.Header().Get("Content-Type"), typ)
	}
}

func TestTheShellsPolicyIsOnEveryUIResponse(t *testing.T) {
	h := newTestServer(t)
	for _, target := range []string{"/", "/apps", "/main-WRY5R6RT.js", "/theme-init.js", "/missing-AAAAAAAA.js"} {
		rec := do(h, http.MethodGet, target, nil)
		expect(t, target+" CSP", rec.Header().Get("Content-Security-Policy"), contentSecurityPolicy)
		expect(t, target+" nosniff", rec.Header().Get("X-Content-Type-Options"), "nosniff")
	}
	rec := do(h, http.MethodPost, "/", nil)
	expect(t, "405 CSP", rec.Header().Get("Content-Security-Policy"), contentSecurityPolicy)
}

func TestThePolicyIsCoworksString(t *testing.T) {
	expect(t, "policy", contentSecurityPolicy, "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'")
}

// An old chunk after a rollout must fail as a missing file, not arrive as
// HTML a script tag cannot run.
func TestAMissingFileIsA404NotTheShell(t *testing.T) {
	h := newTestServer(t)
	for _, target := range []string{"/main-OLDHASH1.js", "/favicon.ico", "/media/x.woff2"} {
		rec := do(h, http.MethodGet, target, nil)
		expect(t, target+" status", rec.Code, http.StatusNotFound)
		expect(t, target+" Cache-Control", rec.Header().Get("Cache-Control"), "no-store")
		expect(t, target+" Content-Type", rec.Header().Get("Content-Type"), "text/plain; charset=utf-8")
	}
}

func TestAnAPIPathIsAProblem404WhateverTheMethod(t *testing.T) {
	h := newTestServer(t)
	for _, tc := range []struct{ method, target string }{
		{http.MethodGet, "/api"}, {http.MethodGet, "/api/"}, {http.MethodGet, "/api/v1/version"},
		{http.MethodPost, "/api/v1/version"}, {http.MethodDelete, "/api/v1/x.png"},
	} {
		rec := do(h, tc.method, tc.target, nil)
		where := tc.method + " " + tc.target
		expect(t, where+" status", rec.Code, http.StatusNotFound)
		expect(t, where+" Content-Type", rec.Header().Get("Content-Type"), "application/problem+json; charset=utf-8")
		expect(t, where+" Cache-Control", rec.Header().Get("Cache-Control"), "no-store")
		var p problemBody
		if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
			t.Fatalf("%s: %v", where, err)
		}
		expect(t, where+" problem", p, problemBody{
			Type: "https://dash.gtrfc.com/problems/not-found", Title: "Not found", Status: 404,
			Detail: misroutedDetail, Instance: tc.target, Code: "not_found",
		})
	}
}

func TestHealthz(t *testing.T) {
	h := newTestServer(t)
	rec := do(h, http.MethodGet, "/healthz", nil)
	expect(t, "status", rec.Code, http.StatusOK)
	expect(t, "body", rec.Body.String(), "{\"status\":\"ok\"}\n")
	expect(t, "Cache-Control", rec.Header().Get("Cache-Control"), "no-store")
	expect(t, "HEAD", do(h, http.MethodHead, "/healthz", nil).Code, http.StatusOK)
	rec = do(h, http.MethodPost, "/healthz", nil)
	expect(t, "POST", rec.Code, http.StatusMethodNotAllowed)
	expect(t, "Allow", rec.Header().Get("Allow"), "GET, HEAD")
}

func TestOnlyGetAndHeadReachTheUI(t *testing.T) {
	h := newTestServer(t)
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		rec := do(h, method, "/main-WRY5R6RT.js", nil)
		expect(t, method, rec.Code, http.StatusMethodNotAllowed)
		expect(t, method+" Allow", rec.Header().Get("Allow"), "GET, HEAD")
	}
}

func TestHeadAnswersHeadersWithoutABody(t *testing.T) {
	srv := httptest.NewServer(newTestServer(t))
	defer srv.Close()
	res, err := http.Head(srv.URL + "/main-WRY5R6RT.js")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	expect(t, "status", res.StatusCode, http.StatusOK)
	expect(t, "body length", len(body), 0)
	expect(t, "Content-Length", res.ContentLength, int64(len(mainJS)))
}

func TestARangeIsServedFromTheIdentityBytes(t *testing.T) {
	h := newTestServer(t)
	rec := do(h, http.MethodGet, "/main-WRY5R6RT.js", map[string]string{"Range": "bytes=0-6", "Accept-Encoding": "gzip"})
	expect(t, "status", rec.Code, http.StatusPartialContent)
	expect(t, "body", rec.Body.String(), "console")
	expect(t, "Content-Range", rec.Header().Get("Content-Range"), "bytes 0-6/"+strconv.Itoa(len(mainJS)))
	expect(t, "Content-Encoding", rec.Header().Get("Content-Encoding"), "")
	rec = do(h, http.MethodGet, "/main-WRY5R6RT.js", map[string]string{"Range": "bytes=999999-"})
	expect(t, "unsatisfiable", rec.Code, http.StatusRequestedRangeNotSatisfiable)
}

func TestETagsAnswerConditionalRequests(t *testing.T) {
	h := newTestServer(t)
	first := do(h, http.MethodGet, "/theme-init.js", nil)
	etag := first.Header().Get("ETag")
	if !strings.HasPrefix(etag, `"`) || strings.HasPrefix(etag, "W/") {
		t.Fatalf("a strong ETag, got %q", etag)
	}
	again := do(h, http.MethodGet, "/theme-init.js", map[string]string{"If-None-Match": etag})
	expect(t, "revalidated", again.Code, http.StatusNotModified)
	expect(t, "no body", again.Body.Len(), 0)
	changed := do(h, http.MethodGet, "/theme-init.js", map[string]string{"If-None-Match": `"something-else"`})
	expect(t, "another ETag", changed.Code, http.StatusOK)
}

func TestGzipIsServedToWhoTakesIt(t *testing.T) {
	h := newTestServer(t)
	rec := do(h, http.MethodGet, "/main-WRY5R6RT.js", map[string]string{"Accept-Encoding": "br, gzip;q=0.8"})
	expect(t, "Content-Encoding", rec.Header().Get("Content-Encoding"), "gzip")
	expect(t, "Vary", rec.Header().Get("Vary"), "Accept-Encoding")
	zr, err := gzip.NewReader(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	plain, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	expect(t, "decompressed", string(plain), mainJS)

	identity := do(h, http.MethodGet, "/main-WRY5R6RT.js", nil)
	expect(t, "identity Content-Encoding", identity.Header().Get("Content-Encoding"), "")
	expect(t, "identity Vary", identity.Header().Get("Vary"), "Accept-Encoding")
	if identity.Header().Get("ETag") == rec.Header().Get("ETag") {
		t.Error("each representation has its own ETag")
	}
	refused := do(h, http.MethodGet, "/main-WRY5R6RT.js", map[string]string{"Accept-Encoding": "gzip;q=0, *"})
	expect(t, "gzip;q=0", refused.Header().Get("Content-Encoding"), "")
	shell := do(h, http.MethodGet, "/apps", map[string]string{"Accept-Encoding": "gzip"})
	expect(t, "the shell compressed too", shell.Header().Get("Content-Encoding"), "gzip")
}

func TestASmallFileIsNotCompressed(t *testing.T) {
	h := newTestServer(t)
	rec := do(h, http.MethodGet, "/chunk-ZD6FVBYZ.js", map[string]string{"Accept-Encoding": "gzip"})
	expect(t, "Content-Encoding", rec.Header().Get("Content-Encoding"), "")
	expect(t, "Vary", rec.Header().Get("Vary"), "")
}

func TestAcceptsGzip(t *testing.T) {
	for header, want := range map[string]bool{
		"":                 false,
		"gzip":             true,
		"GZIP":             true,
		"deflate, gzip":    true,
		"gzip;q=0":         false,
		"gzip; q=0.5":      true,
		"*":                true,
		"*;q=0":            false,
		"gzip;q=0, *;q=1":  false,
		"identity":         false,
		"gzip;q=nonsense":  true,
		"br;q=1, *;q=0.1":  true,
		"gzip;level=1":     true,
		"x, gzip ; q=0.01": true,
	} {
		var values []string
		if header != "" {
			values = []string{header}
		}
		if got := acceptsGzip(values); got != want {
			t.Errorf("Accept-Encoding %q: got %v, want %v", header, got, want)
		}
	}
}

// Nothing outside the bundle is reachable: a path that climbs is cleaned to
// one inside, and a symbolic link out of it is no file of the bundle.
func TestNoPathOutsideTheBundle(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "www")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	must(t, os.WriteFile(filepath.Join(dir, "index.html"), []byte("shell"), 0o644))
	must(t, os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("outside"), 0o644))
	must(t, os.Symlink(filepath.Join(parent, "secret.txt"), filepath.Join(dir, "link.txt")))
	must(t, os.Symlink(parent, filepath.Join(dir, "up")))
	b, err := loadBundle(dir)
	if err != nil {
		t.Fatal(err)
	}
	h := newServer(b, nil)
	for _, target := range []string{"/../secret.txt", "/link.txt", "/up/secret.txt", "/%2e%2e/secret.txt"} {
		rec := do(h, http.MethodGet, target, nil)
		if strings.Contains(rec.Body.String(), "outside") {
			t.Errorf("%s served a file outside the bundle", target)
		}
		expect(t, target, rec.Code, http.StatusNotFound)
	}
	if _, ok := b.assets["/link.txt"]; ok {
		t.Error("a symbolic link was loaded")
	}
}

func TestABundleNeedsItsShell(t *testing.T) {
	dir := t.TempDir()
	must(t, os.WriteFile(filepath.Join(dir, "main.js"), []byte("x"), 0o644))
	if _, err := loadBundle(dir); err == nil || !strings.Contains(err.Error(), "no index.html") {
		t.Errorf("got %v, want the missing shell named", err)
	}
	if _, err := loadBundle(filepath.Join(dir, "absent")); err == nil {
		t.Error("a missing directory must fail")
	}
}

// failingFS is a bundle whose one file cannot be read. It offers Open alone,
// so every read goes through it.
type failingFS struct {
	files  fstest.MapFS
	locked string
}

func (f failingFS) Open(name string) (fs.File, error) {
	if name == f.locked {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return f.files.Open(name)
}

func TestAnUnreadableFileFailsTheLoad(t *testing.T) {
	files := failingFS{files: fstest.MapFS{
		"index.html": {Data: []byte("shell")},
		"locked.js":  {Data: []byte("x")},
	}, locked: "locked.js"}
	if _, err := loadFS(files); err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("got %v, want the unreadable file to fail the load", err)
	}
}

// recorder is a slog handler that keeps each record's attributes.
type recorder struct {
	mu    sync.Mutex
	lines []map[string]any
}

func (r *recorder) Enabled(context.Context, slog.Level) bool { return true }
func (r *recorder) Handle(_ context.Context, rec slog.Record) error {
	line := map[string]any{"msg": rec.Message, "level": rec.Level}
	rec.Attrs(func(a slog.Attr) bool { line[a.Key] = a.Value.Any(); return true })
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
	return nil
}
func (r *recorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *recorder) WithGroup(string) slog.Handler      { return r }

func TestTheRequestLogHoldsNoQuery(t *testing.T) {
	b, err := loadBundle(writeBundle(t))
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	h := newServer(b, slog.New(rec))
	do(h, http.MethodGet, "/apps?token=secret", map[string]string{"Cookie": "secret"})
	do(h, http.MethodGet, "/healthz", nil)
	if len(rec.lines) != 2 {
		t.Fatalf("got %d lines", len(rec.lines))
	}
	line := rec.lines[0]
	expect(t, "path", line["path"], "/apps")
	expect(t, "method", line["method"], "GET")
	expect(t, "status", line["status"], int64(200))
	expect(t, "level", line["level"], slog.LevelInfo)
	if _, ok := line["duration"]; !ok {
		t.Error("no duration")
	}
	for k, v := range line {
		if s, ok := v.(string); ok && strings.Contains(s, "secret") {
			t.Errorf("%s carries the query or a header", k)
		}
	}
	expect(t, "healthz level", rec.lines[1]["level"], slog.LevelDebug)
}

func TestRunServesAndShutsDown(t *testing.T) {
	dir := writeBundle(t)
	var stderr safeBuffer
	ready := make(chan string, 1)
	ctx, cancel := context.WithCancel(context.Background())
	exit := make(chan int, 1)
	go func() { exit <- run(ctx, []string{"-addr", "127.0.0.1:0", "-root", dir}, &stderr, ready) }()
	addr := <-ready
	res, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	expect(t, "healthz", res.StatusCode, http.StatusOK)
	cancel()
	select {
	case code := <-exit:
		expect(t, "exit", code, 0)
	case <-time.After(10 * time.Second):
		t.Fatal("run did not return after its context ended")
	}
	for _, want := range []string{`"msg":"listening"`, `"files":9`, `"msg":"stopped"`} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("the log lacks %s: %s", want, stderr.String())
		}
	}
}

func TestRunRefusesWhatItCannotServe(t *testing.T) {
	var stderr safeBuffer
	expect(t, "bad flag", run(context.Background(), []string{"-nope"}, &stderr, nil), 2)
	expect(t, "no bundle", run(context.Background(), []string{"-root", filepath.Join(t.TempDir(), "absent")}, &stderr, nil), 1)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	expect(t, "address in use", run(context.Background(), []string{"-addr", ln.Addr().String(), "-root", writeBundle(t)}, &stderr, nil), 1)
}

func TestServeLetsARequestInFlightFinish(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}), make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "finished")
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serve(ctx, ln, h) }()
	answer := make(chan string, 1)
	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/")
		if err != nil {
			answer <- err.Error()
			return
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		answer <- string(b)
	}()
	<-started
	cancel()
	time.Sleep(50 * time.Millisecond)
	close(release)
	expect(t, "answer", <-answer, "finished")
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestServeReturnsAListenerError(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()
	if err := serve(context.Background(), ln, http.NotFoundHandler()); err == nil {
		t.Error("a closed listener must fail")
	}
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
