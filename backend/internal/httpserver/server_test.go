package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func do(t *testing.T, h http.Handler, method, target string) *http.Response {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
	return rec.Result()
}

func decode(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	defer res.Body.Close()
	var body map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	return body
}

// problemOf decodes an RFC 9457 problem details body and checks its envelope:
// the media type, the standard members and that status matches the response.
func problemOf(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	assert.Equal(t, "application/problem+json; charset=utf-8", res.Header.Get("Content-Type"))
	body := decode(t, res)
	assert.EqualValues(t, res.StatusCode, body["status"])
	assert.NotEmpty(t, body["title"])
	typ, _ := body["type"].(string)
	code, _ := body["code"].(string)
	assert.Equal(t, "https://dash.gtrfc.com/problems/"+strings.ReplaceAll(code, "_", "-"), typ)
	assert.Equal(t, res.Header.Get("X-Request-Id"), body["request_id"], "the body names the request the header names")
	assert.NotEmpty(t, body["request_id"])
	return body
}

func TestHealthz(t *testing.T) {
	res := do(t, New(Options{}), http.MethodGet, "/healthz")
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "application/json; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))
	assert.Equal(t, map[string]any{"status": "ok"}, decode(t, res))
}

func TestHealthzAnswersHead(t *testing.T) {
	res := do(t, New(Options{}), http.MethodHead, "/healthz")
	assert.Equal(t, http.StatusOK, res.StatusCode)
}

func TestHealthzRefusesOtherMethods(t *testing.T) {
	res := do(t, New(Options{}), http.MethodPost, "/healthz")
	assert.Equal(t, http.StatusMethodNotAllowed, res.StatusCode)
	assert.Equal(t, "GET, HEAD", res.Header.Get("Allow"))
	assert.Equal(t, "method_not_allowed", problemOf(t, res)["code"])
}

func TestReadyzWithoutCheckIsReady(t *testing.T) {
	res := do(t, New(Options{}), http.MethodGet, "/readyz")
	assert.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, map[string]any{"status": "ready"}, decode(t, res))
}

// The check's error can name an internal address: it goes to the log, never
// into the answer.
func TestReadyzReportsAFailingCheck(t *testing.T) {
	h := New(Options{Ready: func(context.Context) error { return errors.New("dial tcp 10.96.0.1:443: refused") }})
	res := do(t, h, http.MethodGet, "/readyz")
	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)
	p := problemOf(t, res)
	assert.Equal(t, "not_ready", p["code"])
	assert.Equal(t, "the Kubernetes API server has not answered since the backend started", p["detail"])
	assert.Equal(t, "/readyz", p["instance"])
}

func TestAPIPathsReachTheAPIHandler(t *testing.T) {
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	res := do(t, New(Options{API: api}), http.MethodPost, "/api/v1/anything")
	assert.Equal(t, http.StatusTeapot, res.StatusCode)
}

func TestEveryResponseCarriesARequestID(t *testing.T) {
	res := do(t, New(Options{}), http.MethodGet, "/healthz")
	assert.Len(t, res.Header.Get("X-Request-Id"), 36)
}

func TestAnInboundRequestIDIsNotTrusted(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	r.Header.Set("X-Request-Id", "chosen-by-the-client")
	New(Options{}).ServeHTTP(rec, r)
	assert.NotEqual(t, "chosen-by-the-client", rec.Header().Get("X-Request-Id"))
}

func TestAPanicIsAnInternalProblem(t *testing.T) {
	api := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("secret internal state") })
	res := do(t, New(Options{API: api}), http.MethodGet, "/api/v1/boom")
	assert.Equal(t, http.StatusInternalServerError, res.StatusCode)
	p := problemOf(t, res)
	assert.Equal(t, "internal", p["code"])
	assert.NotContains(t, p["detail"], "secret")
}

func TestAnAbortedHandlerStaysAborted(t *testing.T) {
	api := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })
	assert.PanicsWithValue(t, http.ErrAbortHandler, func() {
		do(t, New(Options{API: api}), http.MethodGet, "/api/v1/abort")
	})
}

// The backend serves no UI: every path it does not know is a problem+json
// 404, the API ones and the ones the frontend would have resolved alike.
func TestUnknownRouteIsProblem404(t *testing.T) {
	h := New(Options{})
	for _, target := range []string{"/", "/index.html", "/api/v1/nothing", "/apps", "/api"} {
		t.Run(target, func(t *testing.T) {
			res := do(t, h, http.MethodGet, target)
			assert.Equal(t, http.StatusNotFound, res.StatusCode)
			p := problemOf(t, res)
			assert.Equal(t, "not_found", p["code"])
			assert.Contains(t, p["detail"], "GET "+target)
			assert.Equal(t, target, p["instance"])
		})
	}
}

// recordingHandler collects slog records as maps, for asserting on the
// request log without parsing text.
type recordingHandler struct {
	mu    *sync.Mutex
	lines *[]map[string]any
}

func newRecordingLogger() (*slog.Logger, func() []map[string]any) {
	var mu sync.Mutex
	var lines []map[string]any
	logger := slog.New(recordingHandler{mu: &mu, lines: &lines})
	return logger, func() []map[string]any {
		mu.Lock()
		defer mu.Unlock()
		return append([]map[string]any(nil), lines...)
	}
}

func (recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h recordingHandler) Handle(_ context.Context, r slog.Record) error {
	line := map[string]any{"msg": r.Message, "level": r.Level}
	r.Attrs(func(a slog.Attr) bool {
		line[a.Key] = a.Value.Any()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.lines = append(*h.lines, line)
	return nil
}

func (h recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h recordingHandler) WithGroup(string) slog.Handler      { return h }

// The request log carries method, path, status, duration and the request id
// — never the query, a header or a body.
func TestTheRequestLogLineCarriesNoQueryHeaderOrBody(t *testing.T) {
	logger, lines := newRecordingLogger()
	api := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, "a body the log must not hold")
	})
	h := New(Options{API: api, Logger: logger})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/x?token=secret", strings.NewReader("request-body-secret"))
	r.Header.Set("Authorization", "Bearer header-secret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)

	got := lines()
	require.Len(t, got, 1)
	line := got[0]
	assert.Equal(t, "request", line["msg"])
	assert.Equal(t, slog.LevelInfo, line["level"])
	assert.Equal(t, http.MethodPost, line["method"])
	assert.Equal(t, "/api/v1/x", line["path"])
	assert.EqualValues(t, http.StatusAccepted, line["status"])
	assert.Equal(t, rec.Header().Get("X-Request-Id"), line["request_id"])
	assert.Contains(t, line, "duration")
	for key, value := range line {
		text, _ := value.(string)
		assert.NotContains(t, text, "secret", key)
	}
	assert.Len(t, line, 7, "msg, level and the five fields, nothing else")
}

func TestProbesAreLoggedAtDebug(t *testing.T) {
	logger, lines := newRecordingLogger()
	h := New(Options{Logger: logger})
	do(t, h, http.MethodGet, "/healthz")
	do(t, h, http.MethodGet, "/readyz")
	do(t, h, http.MethodGet, "/nothing")
	got := lines()
	require.Len(t, got, 3)
	assert.Equal(t, slog.LevelDebug, got[0]["level"])
	assert.Equal(t, slog.LevelDebug, got[1]["level"])
	assert.Equal(t, slog.LevelInfo, got[2]["level"])
	assert.EqualValues(t, http.StatusNotFound, got[2]["status"])
}

func TestAHandlerThatWritesNothingIsLoggedAs200(t *testing.T) {
	logger, lines := newRecordingLogger()
	api := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	do(t, New(Options{API: api, Logger: logger}), http.MethodGet, "/api/v1/empty")
	assert.EqualValues(t, http.StatusOK, lines()[0]["status"])
}

func TestServeShutsDownGracefully(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	release := make(chan struct{})
	started := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "finished")
	})
	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- Serve(ctx, ln, h, 5*time.Second) }()

	answer := make(chan string, 1)
	go func() {
		res, err := http.Get("http://" + ln.Addr().String() + "/slow")
		if err != nil {
			answer <- "error: " + err.Error()
			return
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		answer <- string(b)
	}()
	<-started
	cancel() // the shutdown begins while the request is in flight
	time.Sleep(50 * time.Millisecond)
	close(release)

	assert.Equal(t, "finished", <-answer, "the request in flight is answered")
	require.NoError(t, <-served)
	_, err = net.Dial("tcp", ln.Addr().String())
	assert.Error(t, err, "the listener is closed")
}

func TestServeReturnsTheErrorOfAListenerThatFails(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	require.NoError(t, ln.Close())
	err = Serve(context.Background(), ln, http.NotFoundHandler(), time.Second)
	assert.Error(t, err)
}

func TestListenAndServeRefusesAnAddressInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	err = ListenAndServe(context.Background(), ln.Addr().String(), http.NotFoundHandler(), time.Second)
	assert.Error(t, err)
}

func TestListenAndServeStopsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- ListenAndServe(ctx, "127.0.0.1:0", http.NotFoundHandler(), time.Second) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("ListenAndServe did not return after its context ended")
	}
}
