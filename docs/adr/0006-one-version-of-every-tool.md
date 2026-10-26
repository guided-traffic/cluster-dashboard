# ADR 0006: The Toolchains Track the Newest Release, and Every Tool Is Pinned Once

## Status

Accepted 2026-10-26. **Built** in the Makefile and `renovate.json`. **Not verified:** Renovate has not
run against the repository yet.

## Context

On the owner's machine on 2026-10-26, `helm` on the PATH was v3.21.3 while cowork's CI pinned v4.3.0,
and two kind binaries of different versions sat on the PATH. Two versions of one tool are a finding,
never a reason to support both.

## Decision

**D1 — Go 1.27.3 and Angular 22.2, the newest releases**; TypeScript inside Angular's peer range;
Node.js 26.

**D2 — Every tool is pinned once** with a `# renovate:` comment in the Makefile, built or downloaded into
`bin/` under a file name that carries its version, and rebuilt when its pin or the Go toolchain changes.
CI runs the same targets.

**D3 — Nothing runs a pinned tool from the PATH**: not Helm, not kind, not kubeconform.

**D4 — Renovate** (cowork's ADR 0062): Go patches at once, Go minors after seven days, as one group
across both `go.mod` files, both Containerfiles, the workflows and the release template; the Makefile
pins automerge minor and patch; Angular moves as one group; distroless digests automerge; majors wait
for a person, except GitHub Actions.

## Consequences

- A pin bump is a missing file that installs itself.
- `ENVTEST_K8S_VERSION` moves by hand: its SHA-512 per platform changes with it.

## References

- [Makefile](../../Makefile), [renovate.json](../../renovate.json)
