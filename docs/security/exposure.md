# Exposure without a login

Dash has no login (the owner's brief). Whoever reaches its host sees what it shows.

## What it shows

| Where | What |
|---|---|
| `GET /api/v1/version` | The backend's version, commit and build time |
| `GET /api/v1/dashboard` | The inventory of the cluster's web applications: the name, URL, group (the namespace by default), description and readiness (ready and total endpoints) of every discovered app; every custom app and bookmark; the namespace selector and the instance name in effect |
| The UI | The same, drawn |

Not in it: an Ingress's annotations, its labels, its Services, its namespace apart from the group
it names by default, the pods behind it ([ADR 0007](../adr/0007-discovery-with-hajimaris-semantics.md);
the response's schema is `/api/v1/openapi.json`).

**What the configuration hides is not served** (ADR 0007 D7): `ui.showApps: false` leaves every app
out of the answer, `ui.showBookmarks: false` every bookmark, `ui.showAppInfo: false` every
description, `ui.showAppStatus: false` every readiness count. An installation that must not show
readiness to its viewers switches the status off rather than only hiding it on the page.

## How an installation protects it

- **An internal Ingress class**, reachable only from the networks that should see Dash — the owner's
  practice with Hajimari.
- **Authentication at the controller** (forward auth, basic auth, an IP allowlist), as annotations
  under **`ingress.annotations`**, which both Ingress objects carry.

### The two-object trap

An authentication annotation under `ingress.frontend.annotations` protects the UI only: the backend's
object still serves `/api/` — the whole inventory at `/api/v1/dashboard` — to anyone.
The chart's notes and the README say so; the chart cannot tell an authentication annotation from any
other, so it does not enforce it.

## What this does not cover

### H-1 The chart does not refuse an authentication annotation on one object alone

A controller-specific annotation under one object's own set is valid configuration; Dash cannot know
which annotations authenticate. The notes after install are the only warning.
