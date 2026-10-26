package icons

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// iconServer serves a small PNG at /icon.png, a large body at /big, a slow
// one at /slow, an error at /missing and redirects at /to?url=…
func iconServer(t *testing.T, tlsServer bool) *httptest.Server {
	t.Helper()
	icon := pngOf(t, 4, 4)
	mux := http.NewServeMux()
	mux.HandleFunc("/icon.png", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" {
			http.Error(w, "credentials reached the host", http.StatusTeapot)
			return
		}
		_, _ = w.Write(icon)
	})
	mux.HandleFunc("/big", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(make([]byte, 4096)) })
	mux.HandleFunc("/big-declared", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "4096")
		_, _ = w.Write(make([]byte, 4096))
	})
	mux.HandleFunc("/slow", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	})
	mux.HandleFunc("/missing", http.NotFound)
	mux.HandleFunc("/to", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referer", "x")
		http.Redirect(w, r, r.URL.Query().Get("url"), http.StatusFound)
	})
	var srv *httptest.Server
	if tlsServer {
		srv = httptest.NewTLSServer(mux)
	} else {
		srv = httptest.NewServer(mux)
	}
	t.Cleanup(srv.Close)
	return srv
}

// fetcherFor fetches from srv, trusting its certificate, under opts.
func fetcherFor(srv *httptest.Server, opts FetchOptions) *Fetcher {
	if opts.MaxBytes == 0 {
		opts.MaxBytes = 1024
	}
	if opts.Timeout == 0 {
		opts.Timeout = 5 * time.Second
	}
	f := NewFetcher(opts)
	if srv.TLS != nil {
		f.client.Transport.(*http.Transport).TLSClientConfig = srv.Client().Transport.(*http.Transport).TLSClientConfig
	}
	return f
}

// The test server listens on 127.0.0.1: loopback is refused unless the
// configuration admits it.
func TestTheFetcherRefusesNonPublicAddresses(t *testing.T) {
	srv := iconServer(t, true)
	_, err := fetcherFor(srv, FetchOptions{}).Fetch(t.Context(), srv.URL+"/icon.png")
	assert.ErrorIs(t, err, ErrNotPublic)

	data, err := fetcherFor(srv, FetchOptions{AllowPrivateNetworks: true}).Fetch(t.Context(), srv.URL+"/icon.png")
	require.NoError(t, err, "allowPrivateNetworks admits it")
	assert.NotEmpty(t, data)

	data, err = fetcherFor(srv, FetchOptions{AllowedHosts: []string{"127.0.0.1"}}).Fetch(t.Context(), srv.URL+"/icon.png")
	require.NoError(t, err, "an allowed host admits it")
	assert.NotEmpty(t, data)
}

// The check is made on the address dialled: a name is resolved once, by the
// fetcher, and a name that resolves to a private address is refused however
// it resolves elsewhere.
func TestTheFetcherDialsTheAddressItChecked(t *testing.T) {
	srv := iconServer(t, true)
	f := fetcherFor(srv, FetchOptions{})
	lookups := 0
	f.lookup = func(context.Context, string) ([]netip.Addr, error) {
		lookups++
		return []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("169.254.169.254"), netip.MustParseAddr("127.0.0.1")}, nil
	}
	port := srv.URL[strings.LastIndex(srv.URL, ":"):]
	_, err := f.Fetch(t.Context(), "https://icons.example.com"+port+"/icon.png")
	assert.ErrorIs(t, err, ErrNotPublic)
	assert.Equal(t, 1, lookups)

	f.lookup = func(context.Context, string) ([]netip.Addr, error) { return nil, errors.New("no such host") }
	_, err = f.Fetch(t.Context(), "https://icons.example.com/icon.png")
	assert.ErrorContains(t, err, "no such host")
}

func TestTheFetcherRules(t *testing.T) {
	srv := iconServer(t, true)
	plain := iconServer(t, false)
	allowed := FetchOptions{AllowedHosts: []string{"127.0.0.1"}, MaxBytes: 1024, Timeout: 300 * time.Millisecond}
	f := fetcherFor(srv, allowed)
	for name, tc := range map[string]struct {
		url  string
		want string
	}{
		"too large":                    {srv.URL + "/big", "larger than 1024 bytes"},
		"declared too large":           {srv.URL + "/big-declared", "larger than 1024 bytes"},
		"too slow":                     {srv.URL + "/slow", "deadline exceeded"},
		"not found":                    {srv.URL + "/missing", "the host answered 404 Not Found"},
		"a user":                       {strings.Replace(srv.URL, "https://", "https://u:p@", 1) + "/icon.png", ErrCredential.Error()},
		"ftp":                          {"ftp://127.0.0.1/icon.png", ErrNotHTTPS.Error()},
		"no host":                      {"https:///icon.png", "the URL does not parse"},
		"a redirect to http elsewhere": {srv.URL + "/to?url=http://example.com/icon.png", ErrNotHTTPS.Error()},
		"a redirect with a user":       {srv.URL + "/to?url=" + strings.Replace(srv.URL, "https://", "https://u:p@", 1) + "/icon.png", ErrCredential.Error()},
		"six redirects": {srv.URL + "/to?url=" + srv.URL + "/to?url=" + srv.URL + "/to?url=" + srv.URL + "/to?url=" + srv.URL +
			"/to?url=" + srv.URL + "/to?url=" + srv.URL + "/icon.png", "too many redirects"},
	} {
		_, err := f.Fetch(t.Context(), tc.url)
		require.Error(t, err, name)
		assert.Contains(t, err.Error(), tc.want, name)
		assert.NotContains(t, err.Error(), "127.0.0.1", "%s: the error names no URL", name)
	}

	data, err := f.Fetch(t.Context(), srv.URL+"/to?url="+srv.URL+"/icon.png")
	require.NoError(t, err, "a redirect on https, the referrer dropped")
	assert.NotEmpty(t, data)

	_, err = f.Fetch(t.Context(), plain.URL+"/icon.png")
	require.NoError(t, err, "plain http for an allowed host")
	_, err = fetcherFor(plain, FetchOptions{AllowPrivateNetworks: true}).Fetch(t.Context(), plain.URL+"/icon.png")
	assert.ErrorIs(t, err, ErrNotHTTPS, "plain http only for an allowed host, private networks or not")
}

func TestPublicAddress(t *testing.T) {
	for addr, public := range map[string]bool{
		"93.184.216.34": true, "2606:4700::1111": true, "1.1.1.1": true,
		"127.0.0.1": false, "::1": false, "10.1.2.3": false, "172.16.0.1": false, "192.168.1.1": false,
		"169.254.169.254": false, "fe80::1": false, "fd00::1": false, "100.64.0.1": false, "0.0.0.0": false,
		"224.0.0.1": false, "198.18.0.1": false, "203.0.113.5": false, "2001:db8::1": false, "::ffff:127.0.0.1": false,
		"240.0.0.1": false,
	} {
		assert.Equal(t, public, PublicAddress(netip.MustParseAddr(addr)), addr)
	}
	assert.False(t, PublicAddress(netip.Addr{}))
}

func TestRedact(t *testing.T) {
	assert.Equal(t, "https://icons.example.com/a/b.png", Redact("https://user:pw@icons.example.com/a/b.png?token=s3cret#x"))
	assert.Equal(t, "(a URL that does not parse)", Redact("::"))
}
