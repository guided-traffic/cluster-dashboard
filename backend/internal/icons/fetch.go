package icons

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strings"
	"time"
)

// The remote fetch (docs/adr/0009 D2): a port of Cogpit's IconFetcher
// (internal/helm/icon.go). https only — plain http only for a host the
// configuration allows; no credentials, no cookies, no referrer; at most
// MaxBytes in Timeout and five redirects, each checked again; a host at a
// loopback, private, link-local or otherwise non-public address refused
// unless the configuration admits private networks or names the host; the
// connection dialled at the address that was checked, so a name that
// resolves elsewhere a moment later changes nothing. No proxy: the chart
// sets none, and through a proxy the address check would not apply.

// FetchOptions are the rules of the remote fetch (icons.remote).
type FetchOptions struct {
	// AllowPrivateNetworks dials hosts at non-public addresses too.
	AllowPrivateNetworks bool
	// AllowedHosts are fetched at any address and over plain http.
	AllowedHosts []string
	MaxBytes     int64
	Timeout      time.Duration
}

// maxRedirects is how many redirects a download follows.
const maxRedirects = 5

// The errors of a refused download; none names the URL.
var (
	ErrNotPublic  = errors.New("the host is not at a public address, and icons.remote admits neither private networks nor this host")
	ErrNotHTTPS   = errors.New("not https, and the host is not in icons.remote.allowedHosts")
	ErrCredential = errors.New("a URL with a user or password is not fetched")
)

// Fetcher downloads remote icons under the rules.
type Fetcher struct {
	opts   FetchOptions
	client *http.Client
	// lookup resolves a host name; a test replaces it.
	lookup func(ctx context.Context, host string) ([]netip.Addr, error)
}

// NewFetcher returns a fetcher under opts.
func NewFetcher(opts FetchOptions) *Fetcher {
	f := &Fetcher{opts: opts, lookup: func(ctx context.Context, host string) ([]netip.Addr, error) {
		return net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	}}
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.Proxy = nil
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	t.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return f.dial(ctx, dialer, network, addr)
	}
	f.client = &http.Client{Transport: t, Timeout: opts.Timeout, CheckRedirect: f.checkRedirect}
	return f
}

// allowed reports whether a host is one of icons.remote.allowedHosts.
func (f *Fetcher) allowed(host string) bool {
	return slices.ContainsFunc(f.opts.AllowedHosts, func(h string) bool { return strings.EqualFold(h, host) })
}

// checkURL holds a URL to the scheme and credential rules.
func (f *Fetcher) checkURL(u *url.URL) error {
	switch {
	case u.User != nil:
		return ErrCredential
	case u.Scheme == "https":
		return nil
	case u.Scheme == "http" && f.allowed(u.Hostname()):
		return nil
	}
	return ErrNotHTTPS
}

func (f *Fetcher) checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return errors.New("too many redirects")
	}
	if err := f.checkURL(req.URL); err != nil {
		return fmt.Errorf("a redirect: %w", err)
	}
	req.Header.Del("Referer")
	return nil
}

// dial connects to addr: an allowed host, or any host while private
// networks are admitted, as it is; else the first of its addresses that is
// public, by that address, so the name is resolved once.
func (f *Fetcher) dial(ctx context.Context, d *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if f.allowed(host) || f.opts.AllowPrivateNetworks {
		return d.DialContext(ctx, network, addr)
	}
	ips, err := f.lookup(ctx, host)
	if err != nil {
		return nil, err
	}
	last := ErrNotPublic
	for _, ip := range ips {
		if !PublicAddress(ip) {
			continue
		}
		c, err := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return c, nil
		}
		last = err
	}
	return nil, last
}

// Fetch downloads the icon at rawURL, unchecked: the caller holds the bytes
// to CheckIcon. Its errors name no URL.
func (f *Fetcher) Fetch(ctx context.Context, rawURL string) ([]byte, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return nil, errors.New("the URL does not parse")
	}
	if err := f.checkURL(u); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, f.opts.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, errors.New("the URL does not make a request")
	}
	req.Header.Set("User-Agent", "Dash")
	req.Header.Set("Accept", "image/png, image/jpeg, image/gif, image/webp, image/svg+xml")
	resp, err := f.client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			return nil, ue.Err // the error without the URL
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch {
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("the host answered %s", resp.Status)
	case resp.ContentLength > f.opts.MaxBytes:
		return nil, fmt.Errorf("larger than %d bytes", f.opts.MaxBytes)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, f.opts.MaxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > f.opts.MaxBytes {
		return nil, fmt.Errorf("larger than %d bytes", f.opts.MaxBytes)
	}
	return data, nil
}

// PublicAddress reports whether an address is on the internet: not
// loopback, private (RFC 1918, unique local), link-local, carrier-grade NAT
// (100.64.0.0/10), multicast, unspecified or reserved for documentation and
// benchmarks.
func PublicAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return false
	}
	for _, p := range nonPublicPrefixes {
		if p.Contains(ip) {
			return false
		}
	}
	return true
}

// nonPublicPrefixes are the ranges PublicAddress refuses beyond the standard
// library's tests.
var nonPublicPrefixes = func() []netip.Prefix {
	ranges := []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15",
		"198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "64:ff9b:1::/48", "2001:db8::/32"}
	out := make([]netip.Prefix, 0, len(ranges))
	for _, s := range ranges {
		out = append(out, netip.MustParsePrefix(s))
	}
	return out
}()

// Redact is a URL as the log may show it: scheme, host and path, without
// user, query or fragment, which can carry a token.
func Redact(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "(a URL that does not parse)"
	}
	return (&url.URL{Scheme: u.Scheme, Host: u.Host, Path: u.Path}).String()
}
