# ADR 0008: Icons Come From Bundled Packs, ConfigMaps and URLs, and Most Apps Get Their Own Logo by Name

## Status

Accepted 2026-10-26 (the plan's §7). **Built** 2026-10-26 (phase 2, step 4) in `backend/internal/icons`,
`backend/tools/iconpack` and the pins in `backend/icons/`; amended the same day with D5 to D8. Verified
by the unit tests of both packages, by `make icon-packs`, which builds the five packs from the pins and
holds them to `backend/icons/required.txt`, and by the end-to-end suite, which fetches an auto-matched
logo of the image's packs and a ConfigMap icon through the backend's Ingress.

## Decision

**D1 — Five bundled catalogues**: `mdi`, `material-symbols`, `si` (Simple Icons), `sh` (selfh.st),
`di` (Dashboard Icons), served by the backend from icon packs built at image build; no browser request
leaves the origin.

**D2 — The reference grammar**: a URL (contains `//`), `configmap:<key>`, `<prefix>:<name>`, or a bare
name meaning `mdi:<name>`.

**D3 — Resolution**: the icon set, else a logo matched by name (auto-match), else the default; every app
and bookmark has an icon. The defaults (`icons.defaultAppIcon`, `icons.defaultBookmarkIcon`) are a
catalogue icon or a ConfigMap key, never a URL: the last resort must not depend on a download (the
grammar and this rule built with the configuration, phase 2 step 1).

**D4 — ConfigMap icons** from the release namespace only, mounted, keys unique. The chart mounts each at
`/etc/dash-icons/<name>`; a key in two ConfigMaps refuses the start, a key whose content fails the check
is left out with a warning.

**D5 — Packs hold SVG only.** A logo a catalogue offers as PNG alone is not bundled: the PNG folders are
hundreds of megabytes. Of the owner's apps, Mailu and AppDaemon lose their logo to this (measured
2026-10-26); a ConfigMap or a URL gives them one.

**D6 — The dark variant replaces a logo only where the logo is dark itself.** Both catalogues name the
variant for dark backgrounds `<name>-light` (verified 2026-10-26); the variants are single-coloured, and
selfh.st has one for most logos, so using it everywhere would turn coloured logos white. The pack
builder keeps it where every colour of the logo is below a relative luminance of 0.2.

**D7 — Auto-match compares two forms**: normalised (`Home Assistant` → `home-assistant`), then compact
(`Wiki.js` → `wikijs`), each name of an app in turn, the most specific first, in each catalogue in the
configured order. Still exact: no prefix, no distance.

**D8 — What must resolve is a list the build checks** (`backend/icons/required.txt`): the owner's 26 MDI
names, the defaults, the README's examples and the apps auto-match must find. A pin that drops one fails
the image build.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §7
- [docs/developer/icons.md](../developer/icons.md), [docs/security/served-icons.md](../security/served-icons.md),
  [docs/security/supply-chain.md](../security/supply-chain.md)
