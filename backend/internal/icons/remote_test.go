package icons

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gate is a fetcher whose downloads wait until the test answers them.
type gate struct {
	mu      sync.Mutex
	answers map[string]chan answer
	calls   map[string]int
}

type answer struct {
	data []byte
	err  error
}

func newGate() *gate {
	return &gate{answers: map[string]chan answer{}, calls: map[string]int{}}
}

func (g *gate) ch(url string) chan answer {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.answers[url] == nil {
		g.answers[url] = make(chan answer, 1)
	}
	return g.answers[url]
}

func (g *gate) Fetch(ctx context.Context, url string) ([]byte, error) {
	g.mu.Lock()
	g.calls[url]++
	g.mu.Unlock()
	select {
	case a := <-g.ch(url):
		return a.data, a.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *gate) count(url string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.calls[url]
}

func remoteResolver(t *testing.T, opts RemoteOptions, f fetcher, log *bytes.Buffer) *Resolver {
	t.Helper()
	r, err := NewResolver(Options{
		Packs: testPacks(t), DefaultApp: "material-symbols:apps", DefaultBookmark: "material-symbols:bookmark",
		Remote: opts, Context: t.Context(), Logger: slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug})), fetcher: f,
	})
	require.NoError(t, err)
	return r
}

var remoteOn = RemoteOptions{Enabled: true, FromAnnotations: true, MaxEntries: 2}

func TestARemoteIconIsDownloadedOnceAndServed(t *testing.T) {
	g := newGate()
	var log bytes.Buffer
	r := remoteResolver(t, remoteOn, g, &log)
	const u = "https://icons.example.com/a.png?token=s3cret"

	require.NoError(t, r.Register(u, false), "a download that runs is no problem")
	assert.ErrorIs(t, r.Settled(), errDownloadsInProcess, "not settled while it runs")
	assert.Equal(t, defaultApp, r.App(u, nil), "the fallback until it arrives")

	g.ch(u) <- answer{data: pngOf(t, 8, 8)}
	require.Eventually(t, func() bool { return r.Settled() == nil }, 5*time.Second, 5*time.Millisecond)
	icon := r.App(u, nil)
	assert.Regexp(t, `^/api/v1/icons/blob/[0-9a-f]{64}\.png$`, icon.Src)
	data, contentType, err := r.Blob(icon.Src[len(PathPrefix+"blob/"):])
	require.NoError(t, err)
	assert.Equal(t, "image/png", contentType)
	assert.NotEmpty(t, data)
	assert.Equal(t, icon, r.Bookmark(u))
	assert.Equal(t, 1, g.count(u), "downloaded once")
	assert.NotContains(t, log.String(), "s3cret", "the log names no query")
}

func TestAFailureIsRememberedForAnHour(t *testing.T) {
	g := newGate()
	var log bytes.Buffer
	r := remoteResolver(t, remoteOn, g, &log)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	r.remote.now = func() time.Time { return now }
	const u = "https://icons.example.com/broken.png"

	require.NoError(t, r.Register(u, true))
	g.ch(u) <- answer{err: errors.New("the host answered 500 Internal Server Error")}
	require.Eventually(t, func() bool { return r.Settled() == nil }, 5*time.Second, 5*time.Millisecond)
	assert.Contains(t, log.String(), `msg="a remote icon failed to download; its items show their fallback, it is tried again after an hour" url=https://icons.example.com/broken.png reason="the host answered 500`)

	assert.NoError(t, r.Register(u, true), "a failure is the log's, not the annotation's")
	assert.Equal(t, defaultApp, r.App(u, nil))
	assert.Equal(t, 1, g.count(u), "not again within the hour")

	now = now.Add(retryAfter)
	g.ch(u) <- answer{data: []byte(logo)}
	assert.Equal(t, defaultApp, r.App(u, nil), "tried again after the hour; the fallback while it runs")
	require.Eventually(t, func() bool { return r.Settled() == nil }, 5*time.Second, 5*time.Millisecond)
	assert.Regexp(t, `\.svg$`, r.App(u, nil).Src)
	assert.Equal(t, 2, g.count(u))
}

func TestContentTheCheckRefusesIsAFailure(t *testing.T) {
	g := newGate()
	var log bytes.Buffer
	r := remoteResolver(t, remoteOn, g, &log)
	const u = "https://icons.example.com/x.svg"
	require.NoError(t, r.Register(u, false))
	g.ch(u) <- answer{data: []byte(`<svg onload="alert(1)"/>`)}
	require.Eventually(t, func() bool { return r.Settled() == nil }, 5*time.Second, 5*time.Millisecond)
	assert.Equal(t, defaultApp, r.App(u, nil))
	assert.Contains(t, log.String(), "event handler")
}

func TestTheTableIsBounded(t *testing.T) {
	g := newGate()
	var log bytes.Buffer
	r := remoteResolver(t, remoteOn, g, &log)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC)
	r.remote.now = func() time.Time { return now }
	for _, u := range []string{"https://a.example.com/1.png", "https://a.example.com/2.png"} {
		require.NoError(t, r.Register(u, true))
		g.ch(u) <- answer{err: errors.New("down")}
	}
	require.Eventually(t, func() bool { return r.Settled() == nil }, 5*time.Second, 5*time.Millisecond)
	assert.ErrorIs(t, r.Register("https://a.example.com/3.png", true), errRemoteTableIsFull)
	assert.ErrorIs(t, r.Register("https://a.example.com/4.png", true), errRemoteTableIsFull)
	assert.Equal(t, 1, bytes.Count(log.Bytes(), []byte("icons.remote.maxEntries URLs are kept already")), "said once")

	now = now.Add(retryAfter)
	assert.NoError(t, r.Register("https://a.example.com/3.png", true), "an old failure makes room")
}

func TestTheRemoteSwitches(t *testing.T) {
	var log bytes.Buffer
	off := remoteResolver(t, RemoteOptions{MaxEntries: 1}, newGate(), &log)
	assert.ErrorIs(t, off.Register("https://a.example.com/x.png", false), ErrRemoteOff)
	assert.Equal(t, defaultApp, off.App("https://a.example.com/x.png", nil), "remote icons off: the fallback")

	noAnnotations := remoteResolver(t, RemoteOptions{Enabled: true, MaxEntries: 1}, newGate(), &log)
	assert.ErrorIs(t, noAnnotations.Register("https://a.example.com/x.png", true), ErrRemoteAnnotations)
	assert.NoError(t, noAnnotations.Register("https://a.example.com/x.png", false), "the configuration's URLs are fetched")
	assert.ErrorIs(t, noAnnotations.Register("mdi:", false), ErrRefName)
}

// A download waiting for a slot ends with the backend.
func TestADownloadEndsWithTheContext(t *testing.T) {
	g := newGate()
	ctx, cancel := context.WithCancel(t.Context())
	r, err := NewResolver(Options{
		Packs: testPacks(t), DefaultApp: "material-symbols:apps", DefaultBookmark: "material-symbols:bookmark",
		Remote: RemoteOptions{Enabled: true, FromAnnotations: true, MaxEntries: 10}, Context: ctx, fetcher: g,
	})
	require.NoError(t, err)
	for _, u := range []string{"https://a/1", "https://a/2", "https://a/3", "https://a/4", "https://a/5"} {
		require.NoError(t, r.Register(u, true))
	}
	cancel()
	require.Eventually(t, func() bool { return r.Settled() == nil }, 5*time.Second, 5*time.Millisecond, "every download ends")
}
