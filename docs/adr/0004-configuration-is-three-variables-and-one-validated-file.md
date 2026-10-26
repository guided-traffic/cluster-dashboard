# ADR 0004: The Configuration Is Three Variables and One Strict YAML File, Validated Twice

## Status

Accepted 2026-10-26. **Built** in `backend/internal/config` with unit tests; the file holds **no key
yet** — the dashboard's settings arrive with phase 2 of the plan, so today every key a file sets is
refused.

## Context

Every setting is the chart's (the brief). Bookmark groups, custom apps and label selectors are nested
lists, which environment variables carry badly; cowork's rule of variables only (its ADR 0001 D7)
does not fit here.

## Decision

**D1 — Three variables**: `DASH_CONFIG` (the file, default `/etc/dash/config.yaml`), `DASH_LISTEN_ADDR`
(default `:8080`), `DASH_LOG_LEVEL` (`debug`, `info`, `warn`, `error`; default `info`).

**D2 — One YAML file for everything else**, decoded strictly: an unknown key is refused. A missing file
at the default path means the defaults; a missing file `DASH_CONFIG` names is an error.

**D3 — Every problem at once.** Load reports every invalid variable and key in one error, naming each,
never echoing a secret.

**D4 — The chart renders the file** into `<fullname>-config`; the backend's pods carry its checksum, so a
changed configuration rolls them. `values.schema.json` refuses wrong types at install; the backend
refuses what the schema cannot express.

## Consequences

- Two validations to keep in step; the chart's tests and the config tests hold each.

## References

- `backend/internal/config/`, `deploy/helm/dash/templates/configmap.yaml`, `deploy/helm/dash/values.schema.json`
