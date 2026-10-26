# Architecture decision records

Every durable decision about Dash is a record here: what was decided, why, and what was rejected.
A record is written in the session the decision is taken; changing behaviour a record describes
means updating that record in the same change, and a superseded rule is marked in place, never
silently removed. Every claim in a record is verified against the code; what is not verified
says so.

## Form

`NNNN-<slug>.md`, numbered without gaps, never reused. Sections:

- **Status** — accepted, amended (with the date and what changed), superseded; and whether it is
  built, verified how, or not built yet.
- **Context** — the forces behind the decision.
- **Decision** — numbered rules `D1`, `D2`, …; code and pages cite them as `docs/adr/NNNN Dn`.
- **Consequences**, **Alternatives considered**, **References**.

## Index

| Record | Decision | Built |
|---|---|---|
| [0001](0001-two-images-two-ingress-objects-one-chart.md) | Two images, two Ingress objects, one chart; distroless; spec-first API; readiness | Yes |
| [0002](0002-documentation-homes-and-the-plan-consumed.md) | Where a statement lives; the project plan is consumed | Yes |
| [0003](0003-test-and-ci-policy.md) | `make` as the entry point, the test tiers, required gates | Yes |
| [0004](0004-configuration-is-three-variables-and-one-validated-file.md) | Three variables and one strict YAML file, the values' keys | Yes |
| [0005](0005-cogpits-dark-cockpit-on-the-web-without-a-component-library.md) | Cogpit's design language, no component library, the theme preference | Shell and theme |
| [0006](0006-one-version-of-every-tool.md) | Toolchains track the newest release; every tool pinned once | Yes |
| [0007](0007-discovery-with-hajimaris-semantics.md) | Ingress discovery with Hajimari's semantics, watched not polled; RBAC by selector | Yes |
| [0008](0008-icons-come-from-bundled-packs-configmaps-and-urls.md) | Icon sources, packs, auto-match, resolution | No |
| [0009](0009-remote-icons-are-fetched-by-the-backend-under-cogpits-rules.md) | The remote icon fetch policy | No |
| [0010](0010-annotations-are-read-under-dash-gtrfc-com-only.md) | Annotations under `dash.gtrfc.com/` only | Yes |
| [0011](0011-badges-on-icons-from-definitions-and-rules.md) | Badges on icons, macOS style, from definitions and rules | No |
