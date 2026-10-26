# ADR 0002: Every Statement Has One Home, and the Project Plan Is Consumed

## Status

Accepted 2026-10-26, after the owner's documentation standard and cowork's ADR 0002. **Built**: the
homes exist.

## Context

A statement kept in two places drifts. cowork learned to give each kind of statement one home and to
consume planning documents instead of maintaining them beside the code.

## Decision

**D1 — The homes.**

| Kind | Home |
|---|---|
| A decision, what was rejected | an ADR here |
| How the code works, how to contribute | [docs/developer/](../developer/README.md) |
| The threat model and each mechanism's gaps | [docs/security/](../security/README.md), one page per perspective, each ending with `## What this does not cover`, open gaps as `H-<n>` headings there |
| How to report a vulnerability | [SECURITY.md](../../SECURITY.md) |
| The reference tables: values, annotations, variables, endpoints | [README.md](../../README.md) |
| The plan | `docs/planning/project-plan.md`, until its phases close |

**D2 — The plan is consumed.** A phase that closes moves its facts into these homes and leaves the
plan; the plan is deleted when its last phase closes.

**D3 — A change that moves documented behaviour updates its page in the same change.**

## Consequences

- No `DEVELOPER.md` and no `SECURITY_ARCHITECTURE.md` at the root.
- A reviewer checks the pages with the code.

## References

- [docs/planning/project-plan.md](../planning/project-plan.md)
