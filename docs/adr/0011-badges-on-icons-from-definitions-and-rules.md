# ADR 0011: Badges on Icons, in the Style of macOS, From Definitions and Rules

## Status

Accepted 2026-10-26, the owner's requirement (the plan's §7.9). **Not built** — phase 2.

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
`ingressclasses` are watched only while rules exist.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md) §7.9
