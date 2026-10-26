# Validation of what reaches the page

Case 4 of the [plan](../planning/project-plan.md) §12: a link or a text that an operator or an
Ingress author writes ends up in every viewer's browser, on the dashboard's origin. Who writes it:

| Source | Who | Read from |
|---|---|---|
| The chart's values | Whoever installs or upgrades the release | `apps`, `bookmarks`, `icons`, `badges` in `<fullname>-config` |
| Ingress annotations | Whoever may annotate an Ingress in a watched namespace | `dash.gtrfc.com/*` — read once the discovery is built (phase 2) |

## The configuration (built)

The backend validates its file before it serves anything and refuses to start on any problem
([ADR 0004](../adr/0004-configuration-is-three-variables-and-one-validated-file.md);
[backend/internal/config/validate.go](../../backend/internal/config/validate.go)). The schema refuses
the same at `helm install` where JSON Schema can express it.

| Value | Rule | What it forecloses |
|---|---|---|
| `apps[].apps[].url`, `bookmarks[].bookmarks[].url` | An absolute `http` or `https` URL with a host | A `javascript:` or `data:` link that runs on the dashboard's origin when clicked |
| The same | No user or password in the URL | A credential shown to every viewer and kept in every browser's history |
| `icons.defaultAppIcon`, `icons.defaultBookmarkIcon` | A catalogue icon or `configmap:<key>`, never a URL | A fallback that depends on a download; every tile's last resort must not fail |
| An icon reference anywhere | The grammar of `icons.ParseRef`: a URL is `http` or `https` without a user or password | A scheme the fetcher was never meant to speak |
| `badges.definitions.*.text`, `text:` entries | One to eight characters, no control character, no `,` or `@` | Markup or a long string in a pill; the text is rendered as text in any case |
| `badges.definitions.*.icon` | A glyph of `mdi`, `material-symbols` or `si` | A remote or ConfigMap image in a badge |
| `badges.definitions.*.label` | Required, at most 64 characters | A badge without an accessible name |
| Every problem | Names the key and its line, never the value | A token in a URL copied into the log |

## What this does not cover

### H-2 Values that pass validation are still the operator's to trust

An `http` URL to any host passes: whoever writes the values decides where a click goes. The rules
above refuse what runs code on the dashboard's origin, not what the operator chose to link.

### H-3 Annotations are not validated yet

The `dash.gtrfc.com/*` keys are read once the discovery is built. Until then this page covers the
configuration only; the annotation rules — the same URL rule for `url`, an unknown badge name ignored
with a log line instead of refusing to start — arrive with it.
