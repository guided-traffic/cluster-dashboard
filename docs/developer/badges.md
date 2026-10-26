# Badges

Pills on an item's icon in the style of macOS dock badges. The decision is
[ADR 0011](../adr/0011-badges-on-icons-from-definitions-and-rules.md); the user-facing reference is the
README's.

## The flow

```text
badges.definitions ──┐                                   ┌── an item's own entries
badges.rules ────────┼─▶ badges.New (glyphs resolved) ──▶│   (dash.gtrfc.com/badges, badges: in values)
                     │      at start; a glyph that      │   parsed by Engine.Own when read
icons.Resolver.Glyph ┘      does not resolve refuses    │
                            the start                   └── Engine.For(Item) per request
                                                             own corners first, then the rules in order
```

| File | Responsibility |
|---|---|
| [internal/badges/badges.go](../../backend/internal/badges/badges.go) | Corners, tones, the grammar of an entry (`name`, `name@corner`, `text:<value>`, `text:<value>@corner`) |
| [internal/badges/engine.go](../../backend/internal/badges/engine.go) | Definitions resolved, `Own` (entries kept and the problems with the others), `For` (an item's badges), the rule match |
| [cmd/dash](../../backend/cmd/dash/main.go) `newBadges` | The configuration's definitions and rules as the engine's, label selectors converted |
| [internal/discovery](../../backend/internal/discovery/discovery.go) | An Ingress's own entries parsed when it is read, the problems logged; the effective IngressClass and its controller; the annotations the rules read kept in the cache |
| [internal/dashboard](../../backend/internal/dashboard/dashboard.go) | Every item's `badges.Item`: its kind, the host of its URL, and of an Ingress its namespace, labels, annotations, class and controller |

## The rules

- **A match holds** when every field it sets holds; a field the item does not have — an IngressClass
  for a bookmark, labels for a custom app — does not hold. An empty match holds for every item.
- **Hosts** are globs (`path.Match`) on the host of the item's URL, case aside: `*.home.arpa`.
- **Corners**: the item's own entries take theirs first; then each rule in order takes its corner
  (`top-right` by default) if it is free. One badge per corner, four at most, returned in the order
  top-right, top-left, bottom-right, bottom-left.
- **The effective IngressClass** (ADR 0011 D4): `spec.ingressClassName`, else
  `kubernetes.io/ingress.class`, else the cluster's default class — the one IngressClass annotated
  `ingressclass.kubernetes.io/is-default-class: "true"`; with several, none, as the API server then
  admits no Ingress without a class. The controller is that IngressClass's `spec.controller`. The
  discovery watches IngressClasses only while a rule exists, and the chart grants the read only then
  (`<namespace>-<fullname>-ingressclasses`, a ClusterRole in every selector mode).
- **An Ingress author's mistake** — an entry that does not parse, names no definition, or takes a
  corner an earlier entry took — is dropped with a warning naming the Ingress and the key; the
  values' entries are refused at start by `config.Load` instead.

## Adding a match field

1. The field in `config.BadgeMatch` ([file.go](../../backend/internal/config/file.go)), its check in
   `validator.match`, its schema in `values.schema.json`.
2. The field in `badges.Match` and its test in `Match.holds`; what it reads in `badges.Item`.
3. The value on the item: `dashboard.appGroups` for an Ingress — from `discovery.App`, the trim in
   [trim.go](../../backend/internal/discovery/trim.go) keeping what it needs — and `configured` for
   custom apps and bookmarks.
4. A case in `TestEveryMatchField`; the README's badge reference.
