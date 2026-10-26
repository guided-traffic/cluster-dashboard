// Package problem is the error shape of the API: RFC 9457 problem details
// with a stable code and the request id (docs/adr/0001 D5).
//
// The catalogue below is the one place a code is defined; `make generate`
// writes the OpenAPI enum of the codes and the README's code table from it,
// so the three cannot disagree.
package problem

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/guided-traffic/dash/backend/internal/requestid"
)

// TypeBase is the prefix of every problem type URI. The URI is an identifier,
// not a link that must resolve (RFC 9457, section 3.1.1).
const TypeBase = "https://dash.gtrfc.com/problems/"

// Code is one entry of the catalogue.
type Code struct {
	// Code is the stable snake_case identifier clients switch on.
	Code string
	// Status is the HTTP status the code is answered with.
	Status int
	// Title is the code in words.
	Title string
	// Meaning says when the code is answered; it is the README's text.
	Meaning string
}

// Type is the code's type URI.
func (c Code) Type() string {
	return TypeBase + strings.ReplaceAll(c.Code, "_", "-")
}

// The catalogue, in the order of the README table.
var (
	ValidationFailed = Code{"validation_failed", http.StatusBadRequest, "Validation failed", "The request does not match the API document: an unknown query parameter, or a value the document refuses; `errors[]` names each"}
	NotFound         = Code{"not_found", http.StatusNotFound, "Not found", "No such route. The frontend's file server answers the same for an `/api/` path that reached it instead of the backend"}
	MethodNotAllowed = Code{"method_not_allowed", http.StatusMethodNotAllowed, "Method not allowed", "The path exists with other methods; `Allow` names them"}
	Internal         = Code{"internal", http.StatusInternalServerError, "Internal error", "Something failed inside the backend; the `request_id` finds it in the log"}
	NotReady         = Code{"not_ready", http.StatusServiceUnavailable, "Not ready", "The backend has not reached the Kubernetes API server since it started, or its discovery has not listed the Ingresses yet (`/readyz`, `/api/v1/dashboard`)"}
)

// Catalogue lists every code; the generators read it.
var Catalogue = []Code{ValidationFailed, NotFound, MethodNotAllowed, Internal, NotReady}

// FieldError is one entry of errors[].
type FieldError struct {
	// Pointer is a JSON pointer into the body, or query:<name>.
	Pointer string `json:"pointer"`
	Message string `json:"message"`
}

// Body is the problem details body.
type Body struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Instance  string       `json:"instance,omitempty"`
	Code      string       `json:"code"`
	RequestID string       `json:"request_id,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// Error is a problem a handler returns instead of a response; the API
// renders it. Detail is for a person and never carries a secret, a stack
// trace or an internal path.
type Error struct {
	Code    Code
	Detail  string
	Errors  []FieldError
	Headers map[string]string
}

// New returns a problem with a detail.
func New(code Code, detail string) *Error {
	return &Error{Code: code, Detail: detail}
}

func (e *Error) Error() string {
	if e.Detail == "" {
		return e.Code.Code
	}
	return e.Code.Code + ": " + e.Detail
}

// Write answers r with the problem.
func Write(w http.ResponseWriter, r *http.Request, e *Error) {
	for k, v := range e.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(e.Code.Status)
	_ = json.NewEncoder(w).Encode(BodyOf(r, e))
}

// BodyOf is the body Write answers r with.
func BodyOf(r *http.Request, e *Error) Body {
	body := Body{
		Type:   e.Code.Type(),
		Title:  e.Code.Title,
		Status: e.Code.Status,
		Detail: e.Detail,
		Code:   e.Code.Code,
		Errors: e.Errors,
	}
	if r != nil {
		body.Instance = r.URL.Path
		body.RequestID = requestid.From(r.Context())
	}
	return body
}
