# Discovery and the dashboard model

How the backend turns Ingresses into apps, and apps into a page. The decisions are
[ADR 0007](../adr/0007-discovery-with-hajimaris-semantics.md) and
[ADR 0010](../adr/0010-annotations-are-read-under-dash-gtrfc-com-only.md); the annotation reference is
the README's.

## The flow

```text
Kubernetes API ──list/watch──▶ informers (trimmed) ──Ingress added/changed──▶ appOf ──▶ apps map
                                   │                                         (selection, name,
                                   │ EndpointSlice cache,                     group, URL, logs
                                   │ indexed by Service                       the author's mistakes)
                                   ▼                                              │
                     Discovery.Apps(): selected namespaces, status counted now ◀──┘
                                   │
                                   ▼
                     dashboard.Build(apps, config.File): groups, order, summary, what ui.* hides left out
```

- **An Ingress is read when it changes**, not when the page asks: `appOf` runs in the informer's
  event handler, so a warning about an Ingress is logged once per change, not once per request. The
  informers run without a resync period.
- **The status is counted when the page asks**, from the EndpointSlice cache: endpoints change far
  more often than Ingresses, and counting a handful of slices is cheap.
- **Ready means listed once** (`Discovery.Ready`): `/readyz` answers 503 until every informer has
  its first list and then stays ready, the same meaning as the API server probe
  ([internal/kube](../../backend/internal/kube/kube.go)). A ServiceAccount without the grant never
  turns ready, and the log says `the list or watch failed … forbidden`.

## The informers follow the namespace selector, and so does the chart's RBAC

| Selector | Informers | RBAC the chart renders ([templates/rbac.yaml](../../deploy/helm/dash/templates/rbac.yaml)) |
|---|---|---|
| `any: true` | Ingresses and EndpointSlices of every namespace | ClusterRole `<namespace>-<fullname>`: list, watch on both |
| `matchNames` only | One pair per named namespace | A Role `<namespace>-<fullname>` in each named namespace, nothing cluster-wide |
| `labelSelector` (with or without names) | Every namespace, plus Namespaces; filtered by name or label when read | The ClusterRole, plus list, watch on Namespaces |

`NamespaceSelector.mode` in [options.go](../../backend/internal/discovery/options.go) and the
template decide the same way; the integration tier runs the discovery as the chart's ServiceAccount
under the RBAC `helm template` renders for each mode (`make integration-rbac`). With a label
selector, an Ingress of a namespace the selector does not take is read all the same — the
namespace's labels may change — but nothing about it is logged.

## The package

| File | Responsibility |
|---|---|
| [annotations.go](../../backend/internal/discovery/annotations.go) | The `dash.gtrfc.com/*` keys; reading them, booleans through `strconv.ParseBool`, the ignored ones named |
| [app.go](../../backend/internal/discovery/app.go) | `App`; `appOf`: enable and instance, name and group defaults, the URL key or the derived URL |
| [url.go](../../backend/internal/discovery/url.go) | The derived URL: the first concrete host, https when a TLS block names it, the first plain `Prefix` or `Exact` path |
| [status.go](../../backend/internal/discovery/status.go) | The backend Services of an Ingress; the status from their EndpointSlices |
| [options.go](../../backend/internal/discovery/options.go) | `Options` from the configuration; the selector's mode |
| [trim.go](../../backend/internal/discovery/trim.go) | The transforms that trim each object before the cache keeps it |
| [discovery.go](../../backend/internal/discovery/discovery.go) | The informers, the event handlers, `Run`, `Ready`, `Apps` |

**Status.** A pod counts once (by its endpoint's `targetRef`), a terminating endpoint not at all.
`unknown`: the status check is off, or no backend Service has an EndpointSlice (an `ExternalName`
Service, a Service without a selector, a backend that is no Service). `down`: slices exist and no
endpoint in them is ready — a Deployment scaled to zero leaves its slice empty. `degraded`: some
ready. `healthy`: all ready.

**What the cache keeps** ([trim.go](../../backend/internal/discovery/trim.go)): of an Ingress its
name, labels, spec and only the annotations `dash.gtrfc.com/*` and `kubernetes.io/ingress.class`;
of an EndpointSlice its labels and each endpoint's addresses, conditions and target; of a Namespace
its name and labels.

## The dashboard model

[internal/dashboard](../../backend/internal/dashboard/dashboard.go) puts discovered and custom apps
into groups — one group per name, whichever side it comes from — and orders them: the groups
`ui.groupOrder` names first, in its order, then by name, case aside; the apps of a group by name.
Bookmark groups keep the configuration's order. `ui.alwaysTargetBlank` is applied here.

**What the configuration hides is not served**: `ui.showApps` and `ui.showBookmarks` off leave the
apps or the bookmarks out, `ui.showAppInfo` off leaves every description out, `ui.showAppStatus` off
makes every status `unknown`. The other `ui.*` switches are the page's to apply.

## Adding an annotation key

1. The key's constant in [annotations.go](../../backend/internal/discovery/annotations.go), read in
   `readAnnotations`; a value that does not parse is named in `ignored` and the default applies.
2. Its effect in `appOf` and a field of `App`; a case in [app_test.go](../../backend/internal/discovery/app_test.go).
3. The trim keeps every `dash.gtrfc.com/*` key already; another prefix needs `trimIngress`.
4. The README's annotation table; a security note where the value reaches the page or a request.
