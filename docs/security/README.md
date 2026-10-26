# Security architecture

One page per perspective, never one all-covering document. The file names are the index. Every page
ends with `## What this does not cover`; an open gap lives there, under a stable `H-<n>` heading, on
the page whose mechanism has it. Reporting a vulnerability: [SECURITY.md](../../SECURITY.md).

| Page | Perspective |
|---|---|
| [trust-boundaries.md](trust-boundaries.md) | Who talks to whom, and what each side trusts |
| [privilege-footprint.md](privilege-footprint.md) | What the pods may do in the cluster and on their node |
| [exposure.md](exposure.md) | Who can reach Dash without a login, and what they see |

The pages describe phase 1. The cases the dashboard adds — remote icons, served SVG, links and badge
text from annotations, the discovery's read — get their pages with the code that brings them
([plan](../planning/project-plan.md) §12).
