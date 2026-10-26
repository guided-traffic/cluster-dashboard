# ADR 0011: Badges on Icons, in the Style of macOS, From Definitions and Rules

## Status

Accepted 2026-10-26, the owner's requirement (the plan's §7.9). **Built** 2026-10-26 (phase 2, step 5)
in `backend/internal/badges`, the discovery's IngressClass watch and the chart's
`<namespace>-<fullname>-ingressclasses` ClusterRole; amended the same day with D5. Verified by the rule
matrix of `TestEveryMatchField` and its neighbours, the discovery's class tests on a fake clientset,
the chart's tests, and the integration tier, which reads the default class and its controller under
the RBAC `helm template` renders with a rule.

## Decision

**D1 — Definitions** in the values: a glyph or a text of at most eight characters, a required label, a
tone from the palette's lights.

**D2 — Rules** match on IngressClass names, IngressClass controllers, namespaces, hosts, labels,
annotations and kinds; the annotation `dash.gtrfc.com/badges` adds an item's own, comma-separated. A
custom app or a bookmark lists its own under `badges`, one entry per list item, in the annotation's
grammar (`name`, `name@corner`, `text:<value>`, `text:<value>@corner`). In the values an unknown
name or a corner taken twice refuses the start; in an annotation it is ignored with a log line.

**D3 — One badge per corner**, four at most; the item's own first, then the first matching rule.

**D4 — The effective IngressClass**: the field, the legacy annotation, the cluster's default class;
`ingressclasses` are watched only while rules exist. With several default classes there is none: the
API server admits no Ingress without a class then.

**D5 — Badges are resolved when the page asks, entries when they are read.** Definitions and their
glyphs at start — a glyph that does not resolve refuses it; an item's own entries when its source is
read, the annotation's problems logged once per change; the match per request, against the effective
class and controller of that moment. A text entry is labelled with its text and toned neutral.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §7.9
- [docs/developer/badges.md](../developer/badges.md)
