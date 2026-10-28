# Attribution

Third-party material in this repository and its images, one row per item.

| Where | What | Author | Source | Licence |
|---|---|---|---|---|
| `frontend/src/app/design/glyphs.ts` | Four Material Symbols glyphs (brightness_auto, light_mode, dark_mode, arrow_outward), path data of `@iconify-json/material-symbols` 1.2.96 | Google | https://github.com/google/material-design-icons | Apache-2.0 |
| The frontend image (from `@fontsource-variable/inter` 5.3.0) | The Inter typeface | Rasmus Andersson | https://github.com/rsms/inter | SIL Open Font License 1.1 |
| The frontend image (from `@fontsource-variable/archivo` 5.3.0) | The Archivo typeface | Omnibus-Type | https://github.com/Omnibus-Type/Archivo | SIL Open Font License 1.1 |
| The frontend image (from `@fontsource/instrument-serif` 5.3.0, the italic) | The Instrument Serif typeface | Rodrigo Fuenzalida, Jordan Egstad | https://github.com/Instrument/instrument-serif | SIL Open Font License 1.1 |
| The frontend image (from `@fontsource-variable/geist-mono` 5.3.0) | The Geist Mono typeface | Andrés Briganti, Mateo Zaragoza, Guillermo Rauch, Evil Rabbit, José Rago, Facundo Santana | https://github.com/vercel/geist-font | SIL Open Font License 1.1 |
| The backend image, `mdi` pack (`@iconify-json/mdi`, pinned in `backend/icons/package.json`) | Material Design Icons | Pictogrammers | https://github.com/Templarian/MaterialDesign | Apache-2.0 |
| The backend image, `material-symbols` pack (`@iconify-json/material-symbols`) | Material Symbols | Google | https://github.com/google/material-design-icons | Apache-2.0 |
| The backend image, `si` pack (`@iconify-json/simple-icons`) | Simple Icons: brand marks | Simple Icons contributors | https://github.com/simple-icons/simple-icons | CC0-1.0 |
| The backend image, `sh` pack (pinned in `backend/icons/sources.yaml`) | selfh.st icons: app logos | selfh.st | https://github.com/selfhst/icons | CC-BY-4.0 |
| The backend image, `di` pack (pinned in `backend/icons/sources.yaml`) | Dashboard Icons: app logos | Homarr Labs and contributors | https://github.com/homarr-labs/dashboard-icons | Apache-2.0 |

**selfh.st icons** are © selfh.st, licensed under the Creative Commons Attribution 4.0 International
licence (https://creativecommons.org/licenses/by/4.0/). Dash serves their SVG files unchanged, packed
into one archive; it leaves out the files its content check refuses, and the light variant of every
logo that is not dark itself.

The Iconify sets are served as SVG documents built from each icon's path data on its grid; their
paths are unchanged. The app logos and brand marks are their owners' trademarks; the catalogues
collect them for referring to the apps, and so does Dash. This is not a legal review.

Material of the organisation's own projects — the pipeline, Makefile and chart patterns of cowork,
the envtest fetch script of Cogpit (`hack/envtest/fetch.sh`), the horizon's geometry after Cogpit's
`glow.go` (`frontend/src/app/features/dashboard/horizon.ts`), the light strip's measures, shades
and grain after Cogpit's `lightstrip_model.go` and `theme.go`
(`frontend/src/app/features/dashboard/light-strip.ts`) — is Guided Traffic's and under this
repository's Apache License 2.0.
