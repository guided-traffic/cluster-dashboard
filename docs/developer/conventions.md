# Conventions

- **English** for all code, comments, commits and documentation.
- **Conventional commits**, a scope where one exists, **no apostrophe anywhere in a message**.
- **Errors** wrap with `%w` and a verb; a configuration error names the variable or key and never
  echoes a URL, a key or a secret.
- **Every API error is an RFC 9457 problem body** written through `backend/internal/problem`, with a
  code from its catalogue; never an ad-hoc JSON error.
- **The request log** carries method, path, status, duration and the request id — no bodies, no
  headers, no query.
- **Generated code is committed** and written only by its generator (`make generate`,
  `make frontend-generate`); the `-check` targets fail on drift.
- **Kubeconfig safety**: no target, test or script reads or writes `~/.kube/config`; clusters are the
  e2e cluster's own file under `.cache/kind/`, and the Go tests run with `KUBECONFIG=/dev/null`.
- **Distroless**: nothing goes into an image that its process does not need; a change that adds a
  shell or a package manager fails CI.
- **Pins**: a tool or image is pinned once, in the Makefile, a lock file or a Containerfile, with
  what Renovate needs to move it.
- **A page describing behaviour changes with the behaviour**, in the same change.
