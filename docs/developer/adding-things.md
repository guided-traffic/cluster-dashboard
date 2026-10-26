# Adding things

Ordered checklists for the changes that touch more than one place. Each ends where the tests prove
the places agree; the subsystem's page has the detail.

| To add | Checklist |
|---|---|
| A configuration key | [configuration.md](configuration.md#adding-a-key) |
| An annotation key | [discovery.md](discovery.md#adding-an-annotation-key) |
| An icon catalogue | [icons.md](icons.md#adding-a-catalogue) |
| A badge match field | [badges.md](badges.md#adding-a-match-field) |
| A frontend component | [frontend.md](frontend.md#adding-a-component) |
| An API operation | below |
| A problem code | below |

## An API operation

1. The path in [backend/api/openapi.yaml](../../backend/api/openapi.yaml), its operation in the file of
   its family (`meta.yaml`, `dashboard.yaml`, `icons.yaml`), its schemas in `components/schemas.yaml`
   with an example; errors as `$ref: './components/responses.yaml#/Problem'`.
2. `make generate`: the bundled document, the Go server interface. The build fails until the strict
   server implements the operation — a method on `server` in
   [backend/internal/api](../../backend/internal/api/server.go), returning a `*problem.Error` for what
   the client did wrong.
3. Tests in `internal/api` against `newAPI`, which validates every response against the document;
   `TestEveryOperationOfTheDocumentHasAHandler` calls every path (add its parameters to the replacer).
4. `make frontend-generate`: the Angular client; a service in `frontend/src/app/core/` imports the one
   function from `api/fn/<tag>/` (the lint refuses the barrel).
5. The README's endpoint table; the security pages if the operation shows something new.

## A problem code

1. The code in the catalogue of [backend/internal/problem/problem.go](../../backend/internal/problem/problem.go):
   the snake_case code, its status, title and meaning (the README's text), appended to `Catalogue`.
2. `make generate`: `components/problem-codes.yaml` and the README's code table between its markers.
3. A test that answers it; `make generate-check` proves the generated files are current.
