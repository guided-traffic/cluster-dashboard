# ADR 0004: The Configuration Is Few Variables and One Strict YAML File, Validated Twice

## Status

Accepted 2026-10-26. **Built** in `backend/internal/config` with unit tests. Amended 2026-10-26 (phase
2, step 1): the file holds the dashboard's keys — `discovery`, `ui`, `apps`, `bookmarks`, `icons`,
`badges` — and D5 and D6 are added. Amended 2026-10-26 (phase 2, step 4): D1 names five variables —
the two icon folders are paths of the image and the chart, like the file's. Verified by the unit tests of `backend/internal/config`, the
chart's `tests/configmap_test.yaml`, and `make helm-template`, which hands the `config.yaml` rendered
from every `ci/*-values.yaml` to the backend's validation (`backend/tools/configcheck`).

## Context

Every setting is the chart's (the brief). Bookmark groups, custom apps and label selectors are nested
lists, which environment variables carry badly; cowork's rule of variables only (its ADR 0001 D7)
does not fit here.

## Decision

**D1 — Five variables**: `DASH_CONFIG` (the file, default `/etc/dash/config.yaml`), `DASH_ICON_PACKS`
(the icon packs, default `/usr/share/dash/icons`, where the image carries them), `DASH_ICON_CONFIGMAPS`
(the mounted icon ConfigMaps, default `/etc/dash-icons`, where the chart mounts them),
`DASH_LISTEN_ADDR` (default `:8080`), `DASH_LOG_LEVEL` (`debug`, `info`, `warn`, `error`; default
`info`). A variable names where the image or the chart put something; a setting is a key of the file.

**D2 — One YAML file for everything else**, decoded strictly: an unknown key is refused. A missing file
at the default path means the defaults; a missing file `DASH_CONFIG` names is an error.

**D3 — Every problem at once.** Load reports every invalid variable and key in one error, naming each,
never echoing a secret.

**D4 — The chart renders the file** into `<fullname>-config`; the backend's pods carry its checksum, so a
changed configuration rolls them. `values.schema.json` refuses wrong types at install; the backend
refuses what the schema cannot express.

**D5 — The file's keys are the values' keys.** The chart renders the values `discovery`, `ui`, `apps`,
`bookmarks`, `icons` and `badges` with `toYaml`, unchanged; only the chart's own keys
(`icons.configMap`, `icons.existingConfigMaps`) stay out. One name per setting, from `values.yaml`
to the struct field; the backend's defaults (`config.DefaultFile`) are the values' defaults, which a
unit test compares.

**D6 — A problem names its line and its key's path**, the decoder's and the validation's alike, sorted
by line; a value the decoder could not read is reported once, not again by the validation.

## Consequences

- Two validations to keep in step; the chart's tests and the config tests hold each, and
  `make helm-template` runs the backend's validation on what the chart renders.
- A key added to the values flows into the file by itself; the backend refuses it until `File` names
  it, and `configcheck` fails the chart job until then.

## References

- `backend/internal/config/`, `backend/tools/configcheck/`, `deploy/helm/dash/templates/configmap.yaml`,
  `deploy/helm/dash/values.schema.json`, [docs/developer/configuration.md](../developer/configuration.md)
