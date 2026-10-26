# Remote icons

Cases 2 and 6 of the [plan](../planning/project-plan.md) §12: an icon named by a URL is downloaded by
the backend, from inside the cluster — never by the browser — and its bytes are then served to every
viewer.

| Who | Where | Live |
|---|---|---|
| Whoever writes the values | `icon` of `apps` and `bookmarks` | While `icons.remote.enabled` (the default) |
| **Whoever may annotate an Ingress in a watched namespace** | `dash.gtrfc.com/icon` | While `icons.remote.enabled` and `icons.remote.fromAnnotations` (both default on) |

What makes it real: the backend's egress reaches what a browser outside the cluster cannot — the
cloud's metadata address, the API server, internal services. An Ingress author who could make the
backend fetch an internal address would use the dashboard as a probe (SSRF), and with every distinct
URL the backend's memory and egress grow (exhaustion).

## The rules

[backend/internal/icons/fetch.go](../../backend/internal/icons/fetch.go), a port of Cogpit's
`IconFetcher` ([ADR 0009](../adr/0009-remote-icons-are-fetched-by-the-backend-under-cogpits-rules.md)):

| Rule | Value | Where |
|---|---|---|
| Scheme | `https`; plain `http` only for a host in `icons.remote.allowedHosts` | the URL and every redirect |
| Addresses | A host at a loopback, private (RFC 1918, unique local), link-local (the metadata address 169.254.169.254 among them), carrier-grade NAT, multicast, unspecified or documentation address is refused — unless `allowPrivateNetworks` admits every host or `allowedHosts` names this one | checked when the connection is dialled, and the connection made **to the address checked**: a name that resolves elsewhere a moment later changes nothing |
| Credentials | No user or password in a URL; no cookies; the `Referer` dropped on a redirect | every request |
| Redirects | At most five, each held to the scheme, credential and address rules | `CheckRedirect` |
| Size and time | `icons.remote.maxBytes` (default 256 KiB), `icons.remote.timeout` (default 20 s) | the response |
| Content | `CheckIcon` ([served-icons.md](served-icons.md)) | before anything is kept |
| Proxy | None: the backend dials directly | the transport |

**Bounds** ([remote.go](../../backend/internal/icons/remote.go)): at most four downloads at once; at
most `icons.remote.maxEntries` URLs kept, downloaded or failed (default 512); a failure remembered
for an hour before the URL is tried again; nothing refreshed periodically. Worst case in memory:
`maxEntries × maxBytes` — 128 MiB with the defaults, beside a default memory limit of 256 MiB.

**Logs** name a URL without its user, query or fragment (`Redact`): a token in a URL stays out of the
log.

## What this does not cover

### H-6 Annotation URLs are fetched by default

`icons.remote.fromAnnotations` is on by default, so anyone who may annotate an Ingress in a watched
namespace makes the backend send requests — to public addresses only, under the rules above, but to
any public host and as often as a URL is new. An installation whose Ingress authors are not trusted
that far switches it off; the values' URLs are still fetched.

### H-7 The escape hatches are wide

`allowPrivateNetworks: true` admits every host at any address, the metadata address included, for
annotation URLs too. `allowedHosts` admits a named host at any address and over plain http; a name
the cluster's DNS resolves inside is then reachable. Prefer `allowedHosts` with
`fromAnnotations: false`.

### H-8 The memory bound is the product of two settings

`maxEntries × maxBytes` is not checked against the pod's memory limit. With the defaults it is half
the default limit; raising either needs the limit raised with it.

### H-9 No egress proxy

A cluster whose egress runs through a proxy cannot download remote icons: the fetcher dials directly
so that the address check holds; through a proxy it would not.
