# Icons

Where every icon comes from and how it reaches the page. The decisions are
[ADR 0008](../adr/0008-icons-come-from-bundled-packs-configmaps-and-urls.md) (sources, packs,
resolution) and [ADR 0009](../adr/0009-remote-icons-are-fetched-by-the-backend-under-cogpits-rules.md)
(the remote fetch); the user-facing grammar is the README's.

## The flow

```text
backend/icons/ (pins) ──iconpack──▶ /usr/share/dash/icons/<catalogue>.zip   (image build)
                                           │
mounted ConfigMaps /etc/dash-icons/<cm>/<key> ──CheckIcon──▶ blobs (memory)  (start)
icon URLs (values at start, annotations when read) ──Fetcher──CheckIcon──▶ blobs (4 at once)
                                           │
icons.Resolver: reference → {src, srcDark, tint}; every fallback decided here
                                           │
/api/v1/dashboard (icon of every item) ── /api/v1/icons/<catalogue>@<version>/<name>.svg
                                          /api/v1/icons/blob/<sha256>.<png|svg>
```

The browser loads every icon from the dashboard's own origin. Every served icon carries
`Cache-Control: public, max-age=31536000, immutable`, `X-Content-Type-Options: nosniff` and
`Content-Security-Policy: default-src 'none'; style-src 'unsafe-inline'; sandbox` — its address names
one content for good (a pack's version, a blob's SHA-256), and an SVG opened on its own runs nothing.

## The packs

| File | Responsibility |
|---|---|
| [backend/icons/package.json](../../backend/icons/package.json), `package-lock.json` | The Iconify sets `mdi`, `material-symbols`, `simple-icons`, pinned by version; the lock holds each tarball's URL and integrity. Never installed |
| [backend/icons/sources.yaml](../../backend/icons/sources.yaml) | selfh.st and Dashboard Icons, which publish no release: a commit each and the SHA-256 of the files used |
| [backend/icons/required.txt](../../backend/icons/required.txt) | What the packs must resolve: the owner's 26 MDI names, the defaults, the README's examples, the apps auto-match must find |
| [backend/tools/iconpack](../../backend/tools/iconpack/main.go) | The builder: download, verify, check, pack, then hold the packs to `required.txt` |
| [internal/icons/pack.go](../../backend/internal/icons/pack.go), [packwrite.go](../../backend/internal/icons/packwrite.go) | The pack: a zip of `icons/<name>.svg` and `manifest.json`; written reproducibly, opened with its index in memory |

**Building.** The image's build stage runs the builder (`backend/Containerfile`; `git` is the build
stage's only). `make icon-packs` builds the same packs into `.cache/icons/packs` for `make run`, which
hands them over as `DASH_ICON_PACKS`. Every download is verified before it is used: an npm tarball
against its sha512 integrity, a logo catalogue's sparse checkout — `svg/` and its index only, one
commit, blobs filtered — against the pinned SHA-256, which is
`find svg <index> -type f | LC_ALL=C sort | xargs sha256sum | sha256sum` in the checkout. Every SVG
passes `CheckSVG`; one that fails is left out and counted in the manifest.

**Glyphs and logos.** An Iconify icon becomes an SVG document of its body on the set's grid; the
builder refuses sizes, offsets and transforms per icon, which no bundled set uses (checked
2026-10-26), so an update that starts to fails loudly. A logo is kept byte for byte. Its `-light`
variant — for dark backgrounds in both catalogues (Dashboard Icons' README says so; selfh.st's
`github-light.svg` is white) — goes into the pack only where the logo is dark itself: every colour
it paints with below a relative luminance of 0.2 (`isDark` in
[logos.go](../../backend/tools/iconpack/logos.go)). The variants are single-coloured; a coloured logo
shows on a dark stage as it is. The packs hold SVG only: a logo a catalogue offers as PNG alone
(Mailu, AppDaemon in Dashboard Icons) is not bundled.

**Updating.** Renovate moves the Iconify pins (npm, grouped, minor and patch automerged) and the two
commits (weekly, by hand). A commit's pull request fails the image build until `make icon-pins` has
printed the new SHA-256 and it is put into `sources.yaml` — look at what changed first. A pin that
drops an icon of `required.txt` fails the build either way.

## Resolution

[internal/icons/resolve.go](../../backend/internal/icons/resolve.go), in this order (ADR 0008 D3):

1. The item's own reference (`dash.gtrfc.com/icon`, `icon` in the values), parsed by `ParseRef`: a
   URL (contains `//`), `configmap:<key>`, `<prefix>:<name>`, a bare name for `mdi:<name>`.
2. Auto-match, apps only (`icons.autoMatch`): the app's names, most specific first — its name, the
   Ingress's, `app.kubernetes.io/name` of the Ingress and of its Services' EndpointSlices,
   `helm.sh/chart` without its version (`discovery.App.Names`) — looked up in the manifests' `Match`
   tables of the configured catalogues, in their order; every name normalised first
   (`Home Assistant` → `home-assistant`), then compact (`Wiki.js` → `wikijs`). Exact matches only.
3. The default: `icons.defaultAppIcon` or `icons.defaultBookmarkIcon`, a catalogue icon or a
   ConfigMap key, which must resolve or the backend does not start.

A glyph (`mdi`, `material-symbols`, `si`) is `tint: true` and drawn as a mask in the page's colour; a
logo, a ConfigMap image and a downloaded image are drawn as they are.

**ConfigMaps** ([blobs.go](../../backend/internal/icons/blobs.go)): the chart mounts each icon ConfigMap
at `/etc/dash-icons/<name>` (`DASH_ICON_CONFIGMAPS`), the kubelet's layout — every key a link through
`..data`. At start every key is read, checked by `CheckIcon` (a raster image encoded again as a PNG of
at most 256 pixels a side) and kept by its SHA-256. A key in two ConfigMaps refuses the start; a key
whose content fails the check is left out with a warning. A changed existing ConfigMap arrives with
the next start; the chart's own rolls the pods through its checksum.

**URLs** ([fetch.go](../../backend/internal/icons/fetch.go), [remote.go](../../backend/internal/icons/remote.go)):
a URL is registered when it is read — the values' at start (`registerIcons` in `cmd/dash`), an
annotation's when the discovery reads the Ingress (`Options.RegisterIcon`) — and its download starts
then; the item shows its fallback until it arrives. `/readyz` waits for the downloads started before
the discovery's first list has been read, failures included. A failure is remembered for an hour;
the table holds `icons.remote.maxEntries` URLs, a failure older than the hour making room. The rules of
the fetch are [docs/security/remote-icons.md](../security/remote-icons.md)'s.

## Adding a catalogue

1. Its pin: an npm package in `backend/icons/package.json` (then `npm install --package-lock-only` in
   that folder), or a repository in `sources.yaml` with its index file.
2. Its prefix in [ref.go](../../backend/internal/icons/ref.go) (`catalogPrefixes`, and
   `GlyphCatalogs` for a monochrome set); for an Iconify set the package in `iconifyPackages`
   ([pins.go](../../backend/tools/iconpack/pins.go)); for a logo set its index reader in
   [logos.go](../../backend/tools/iconpack/logos.go) and, for auto-match, its name in
   `icons.AutoMatchCatalogs`, `values.schema.json` and `config.autoMatchCatalogs`.
3. Its row in [ATTRIBUTION.md](../../ATTRIBUTION.md) with the licence's notice, a Renovate rule for its
   pin, and a reference of it in `required.txt`.
4. The README's catalogue table.
