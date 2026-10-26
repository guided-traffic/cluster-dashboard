# Privilege footprint

## In the cluster

| Pod | Kubernetes identity | What it can do |
|---|---|---|
| backend | its ServiceAccount (`<fullname>-backend`, or `serviceAccount.name`), token mounted | `list` and `watch` on Ingresses and EndpointSlices of the selected namespaces; on Namespaces with a label selector; `/version`, which every client may read |
| frontend | none: `automountServiceAccountToken: false` | Nothing against the API |

The chart's grant follows `discovery.namespaceSelector`
([templates/rbac.yaml](../../deploy/helm/dash/templates/rbac.yaml); verified per mode by the chart's
unit tests and by the integration tier, which runs the discovery under exactly the rendered RBAC and
asserts the refusals):

| Selector | Objects | Scope of the read |
|---|---|---|
| `any: true` (the default) | ClusterRole and ClusterRoleBinding `<namespace>-<fullname>` | Ingresses and EndpointSlices of **every** namespace |
| `matchNames` only | Role and RoleBinding `<namespace>-<fullname>` in each named namespace | Those namespaces only; nothing cluster-wide |
| `labelSelector` | The ClusterRole, plus Namespaces | Ingresses and EndpointSlices of every namespace — the filter by label is the backend's, not the API server's — and every Namespace's labels |
| `discovery.enabled: false` or `rbac.create: false` | None | Nothing; with `rbac.create: false` the administrator grants it |
| Any, while `badges.rules` holds a rule | ClusterRole and ClusterRoleBinding `<namespace>-<fullname>-ingressclasses` | list, watch on IngressClasses: their names, controllers, parameter references and default marks — `matchNames` alone included |

Never `get`, never a verb that writes, never Secrets or ConfigMaps: icon ConfigMaps will be mounted,
not read through the API.

**What a taken-over backend pod reads with the token** (plan §12, case 5): every Ingress of the
scope — hosts, TLS secret names and every annotation, where some controllers keep authentication and
allowlist settings — and every EndpointSlice — pod addresses and names; with badge rules every IngressClass. The backend itself keeps less:
its informers trim each object before the cache holds it, to `dash.gtrfc.com/*` and the legacy class
annotation, the spec, and the endpoints' addresses, conditions and targets
([trim.go](../../backend/internal/discovery/trim.go)). The API it serves will carry derived fields
only, never an annotation.

## On the node

Both pods: `runAsNonRoot`, user and group 65532, `readOnlyRootFilesystem`, no writable volume,
`allowPrivilegeEscalation: false`, all capabilities dropped, the `RuntimeDefault` seccomp profile
([ADR 0001](../adr/0001-two-images-two-ingress-objects-one-chart.md) D3). The backend mounts its
configuration read-only.

## What this does not cover

- No NetworkPolicy ships with the chart: who may reach the pods, and where the backend may connect,
  is the cluster administrator's policy. The chart's `podLabels` carry the labels such a policy
  selects by.

### H-4 A label selector reads every namespace

Kubernetes cannot grant a read by a namespace's labels, so `labelSelector` needs the cluster-wide
ClusterRole: the token reads the Ingresses and EndpointSlices of namespaces the selector does not
take. Only `matchNames` alone keeps the grant to the selected namespaces.
