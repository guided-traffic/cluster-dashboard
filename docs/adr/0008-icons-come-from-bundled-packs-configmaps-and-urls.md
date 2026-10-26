# ADR 0008: Icons Come From Bundled Packs, ConfigMaps and URLs, and Most Apps Get Their Own Logo by Name

## Status

Accepted 2026-10-26 (the plan's §7). **Not built** — phase 2.

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

**D4 — ConfigMap icons** from the release namespace only, mounted, keys unique.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §7
