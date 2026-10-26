package icons

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Remote icons are downloaded by the backend, never by the browser
// (docs/adr/0009 D1): every URL known at start — the configuration's, and
// the annotations' of the discovery's first list — is downloaded before the
// backend turns ready (a failure does not hold it back); a URL first seen
// later is downloaded when it is seen. Until then, and after a failure, the
// item shows its fallback; a failure is tried again after an hour. Nothing
// is refreshed: a changed image arrives with the next start.

// RemoteOptions configure the remote icons (icons.remote).
type RemoteOptions struct {
	Enabled         bool
	FromAnnotations bool
	// MaxEntries bounds the URLs kept, downloaded or failed.
	MaxEntries int
	Fetch      FetchOptions
}

// The rules of the table of remote icons.
const (
	// retryAfter is how long a failure is remembered (Cogpit's
	// iconRetryAfter).
	retryAfter = time.Hour
	// maxDownloads is how many downloads run at once.
	maxDownloads = 4
)

// The errors of a remote reference that is not used.
var (
	ErrRemoteOff          = errors.New("is a URL, and icons.remote.enabled is false")
	ErrRemoteAnnotations  = errors.New("is a URL, and icons.remote.fromAnnotations is false")
	errRemoteNotYet       = errors.New("is not downloaded yet")
	errRemoteFailed       = errors.New("failed to download within the hour")
	errRemoteTableIsFull  = errors.New("is not downloaded: icons.remote.maxEntries URLs are kept already")
	errDownloadsInProcess = errors.New("the icons known at start are still being downloaded")
)

type remoteEntry struct {
	src     string // the kept icon's address, once downloaded
	failed  time.Time
	pending bool
}

// fetcher is what remote downloads with; *Fetcher is one.
type fetcher interface {
	Fetch(ctx context.Context, rawURL string) ([]byte, error)
}

type remote struct {
	opts    RemoteOptions
	fetcher fetcher
	blobs   *blobs
	logger  *slog.Logger
	ctx     context.Context
	now     func() time.Time
	slots   chan struct{}

	mu       sync.Mutex
	entries  map[string]*remoteEntry
	pending  int
	fullOnce sync.Once
}

func newRemote(ctx context.Context, opts RemoteOptions, f fetcher, b *blobs, logger *slog.Logger) *remote {
	return &remote{
		opts: opts, fetcher: f, blobs: b, logger: logger, ctx: ctx, now: time.Now,
		slots: make(chan struct{}, maxDownloads), entries: map[string]*remoteEntry{},
	}
}

// icon is the kept icon of a URL; a URL not kept yet starts a download —
// unless one runs, it failed within the hour, or the table is full — and is
// not an icon yet.
func (r *remote) icon(rawURL string) (Icon, error) {
	if !r.opts.Enabled {
		return Icon{}, ErrRemoteOff
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	e, known := r.entries[rawURL]
	switch {
	case known && e.src != "":
		return Icon{Src: e.src}, nil
	case known && e.pending:
		return Icon{}, errRemoteNotYet
	case known && r.now().Sub(e.failed) < retryAfter:
		return Icon{}, errRemoteFailed
	case !known && !r.room():
		r.fullOnce.Do(func() {
			r.logger.Warn("a remote icon is not downloaded: icons.remote.maxEntries URLs are kept already; later ones show their fallback", "max_entries", r.opts.MaxEntries)
		})
		return Icon{}, errRemoteTableIsFull
	}
	if !known {
		e = &remoteEntry{}
		r.entries[rawURL] = e
	}
	e.pending = true
	r.pending++
	go r.download(rawURL, e)
	return Icon{}, errRemoteNotYet
}

// room makes room for one more URL, forgetting failures older than the
// hour; it reports whether there is.
func (r *remote) room() bool {
	if len(r.entries) < r.opts.MaxEntries {
		return true
	}
	for u, e := range r.entries {
		if e.src == "" && !e.pending && r.now().Sub(e.failed) >= retryAfter {
			delete(r.entries, u)
		}
	}
	return len(r.entries) < r.opts.MaxEntries
}

// download fetches and checks an icon and keeps it, or remembers the
// failure.
func (r *remote) download(rawURL string, e *remoteEntry) {
	select {
	case r.slots <- struct{}{}:
	case <-r.ctx.Done():
		r.done(e, "", r.ctx.Err())
		return
	}
	defer func() { <-r.slots }()
	data, err := r.fetcher.Fetch(r.ctx, rawURL)
	var format string
	if err == nil {
		data, format, err = CheckIcon(data)
	}
	src := ""
	if err == nil {
		src = r.blobs.put(data, format)
		r.logger.Debug("a remote icon is downloaded", "url", Redact(rawURL))
	} else {
		r.logger.Warn("a remote icon failed to download; its items show their fallback, it is tried again after an hour",
			"url", Redact(rawURL), "reason", err.Error())
	}
	r.done(e, src, err)
}

func (r *remote) done(e *remoteEntry, src string, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e.pending = false
	r.pending--
	if err != nil {
		e.failed = r.now()
		return
	}
	e.src = src
}

// settled is nil while no download runs.
func (r *remote) settled() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending > 0 {
		return errDownloadsInProcess
	}
	return nil
}
