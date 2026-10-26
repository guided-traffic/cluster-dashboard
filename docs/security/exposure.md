# Exposure without a login

Dash has no login (the owner's brief). Whoever reaches its host sees what it shows.

## Today (phase 1)

The UI shell and the backend's version, commit and build time (`GET /api/v1/version`).

## With the dashboard (phase 2)

The name, URL, group (the namespace by default) and readiness of every discovered app, and every
bookmark — the inventory of the cluster's web applications.

## How an installation protects it

- **An internal Ingress class**, reachable only from the networks that should see Dash — the owner's
  practice with Hajimari.
- **Authentication at the controller** (forward auth, basic auth, an IP allowlist), as annotations
  under **`ingress.annotations`**, which both Ingress objects carry.

### The two-object trap

An authentication annotation under `ingress.frontend.annotations` protects the UI only: the backend's
object still serves `/api/` — today the version, with the dashboard the whole inventory — to anyone.
The chart's notes and the README say so; the chart cannot tell an authentication annotation from any
other, so it does not enforce it.

## What this does not cover

### H-1 The chart does not refuse an authentication annotation on one object alone

A controller-specific annotation under one object's own set is valid configuration; Dash cannot know
which annotations authenticate. The notes after install are the only warning.
