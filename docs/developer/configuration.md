# Configuration

How a setting travels from the chart's values to the backend, and how to add one. The decision is
[ADR 0004](../adr/0004-configuration-is-three-variables-and-one-validated-file.md); the user-facing
reference with every key and default is the README's "Full reference".

## The path of a setting

```text
values.yaml ──helm──▶ values.schema.json ──▶ templates/configmap.yaml ──▶ <fullname>-config/config.yaml
 (defaults)            (types, enums,          (toYaml of discovery, ui,        mounted at /etc/dash,
                        bounds, patterns)       apps, bookmarks, icons, badges)  read from DASH_CONFIG
                                                                                       │
backend: config.Load ── DefaultFile() ── decodeStrict ── validate ── sorted problems ──┘
          (variables)    (the defaults)   (types, unknown  (what the schema
                                           and twice-set     cannot express)
                                           keys, lines)
```

- **Two validations.** `values.schema.json` refuses a wrong type at `helm install`; the backend refuses
  what the schema cannot express — a badge rule naming a definition that does not exist, a group
  listed twice, a corner taken twice, a label selector the API server would refuse — and refuses to
  start, naming every problem at once.
- **The file's keys are the values' keys** (ADR 0004 D5): the template renders the six sections with
  `toYaml`, so a setting has one name from `values.yaml` to the struct's `yaml` tag. Only
  `icons.configMap` and `icons.existingConfigMaps` are the chart's and stay out.
- **Defaults live twice and are compared**: `values.yaml` and `config.DefaultFile`.
  `TestTheChartDefaultsAreTheBackendDefaults` renders the sections from `values.yaml` the way the
  template does and requires the result to equal `DefaultFile()`.

## The package

| File | Responsibility |
|---|---|
| [config.go](../../backend/internal/config/config.go) | The three variables, `Load`, `Parse`, the error that lists every problem |
| [file.go](../../backend/internal/config/file.go) | The file's schema as structs — a field is a known key — and `DefaultFile` |
| [strict.go](../../backend/internal/config/strict.go) | The strict decoder: unknown and twice-set keys, wrong shapes, the line of every path, the paths that failed |
| [validate.go](../../backend/internal/config/validate.go) | What the types cannot say: sets, bounds, references between keys, contradictions |

The grammar of an icon reference is `icons.ParseRef` ([icons/ref.go](../../backend/internal/icons/ref.go)),
the grammar of a badge entry `badges.ParseEntry` ([badges/badges.go](../../backend/internal/badges/badges.go));
the validation calls both, so the configuration and the annotations read one grammar.

**Problems** name the line and the key's path — `apps[0].apps[1].url`, a map key holding a `.` as
`matchLabels["app.kubernetes.io/name"]` — and never the value, which can be a URL with a token in it.
A missing key's line is its enclosing mapping's. A value the decoder could not read is not checked
again by the validation (ADR 0004 D6).

## Adding a key

1. `deploy/helm/dash/values.yaml`: the key with its default and a `# --` comment.
2. `deploy/helm/dash/values.schema.json`: its type and every bound JSON Schema can express;
   `additionalProperties: false` already refuses it until it is there.
3. `backend/internal/config/file.go`: the field with the same `yaml` name, its default in
   `DefaultFile`.
4. `backend/internal/config/validate.go`: what the schema cannot express, with a message that names
   what is allowed.
5. Tests: a case in `TestEveryValidationRule` per rule; the key in `TestAFileWithEveryKey`; the key
   in `deploy/helm/dash/ci/dashboard-values.yaml`; a refused value in the Makefile's
   `HELM_REFUSED_VALUES`; the rendered default in `tests/configmap_test.yaml`.
6. The README's values block: the key, `# default` or `# example`, a security note where one is real.

`make test-unit helm-template helm-unittest` proves the six steps agree: the defaults test fails on a
default set in one place only, `configcheck` on a value the chart renders and the backend refuses.
