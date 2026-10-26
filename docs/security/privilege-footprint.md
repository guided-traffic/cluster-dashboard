# Privilege footprint

## In the cluster

| Pod | Kubernetes identity | What it can do |
|---|---|---|
| backend | its ServiceAccount (`<fullname>-backend`, or `serviceAccount.name`), token mounted | The chart grants **no role** in phase 1. The backend asks `/version`, which every client may read |
| frontend | none: `automountServiceAccountToken: false` | Nothing against the API |

Phase 2 adds `list` and `watch` on Ingresses and EndpointSlices — cluster-wide, or as namespaced Roles
when the namespace selector names namespaces only — and on IngressClasses while badge rules exist.
Never Secrets, never ConfigMaps ([plan](../planning/project-plan.md) §9).

## On the node

Both pods: `runAsNonRoot`, user and group 65532, `readOnlyRootFilesystem`, no writable volume,
`allowPrivilegeEscalation: false`, all capabilities dropped, the `RuntimeDefault` seccomp profile
([ADR 0001](../adr/0001-two-images-two-ingress-objects-one-chart.md) D3). The backend mounts its
configuration read-only.

## What this does not cover

- No NetworkPolicy ships with the chart: who may reach the pods, and where the backend may connect,
  is the cluster administrator's policy. The chart's `podLabels` carry the labels such a policy
  selects by.
- The ServiceAccount token itself: anyone who takes over the backend pod holds it; today it grants
  nothing beyond what any authenticated client may read.
