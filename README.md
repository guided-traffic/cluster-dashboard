# Dash

A customizable browser dashboard with Kubernetes application discovery.

<!-- problem-codes:start -->
| `code` | Status | Meaning |
|---|---|---|
| `validation_failed` | 400 | The request does not match the API document: an unknown query parameter, or a value the document refuses; `errors[]` names each |
| `not_found` | 404 | No such route. The frontend's file server answers the same for an `/api/` path that reached it instead of the backend |
| `method_not_allowed` | 405 | The path exists with other methods; `Allow` names them |
| `internal` | 500 | Something failed inside the backend; the `request_id` finds it in the log |
| `not_ready` | 503 | The backend has not reached the Kubernetes API server since it started (`/readyz`) |
<!-- problem-codes:end -->
