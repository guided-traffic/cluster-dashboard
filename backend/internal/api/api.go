// Package api serves the API under /api/v1 from the generated server
// interface (docs/adr/0001 D5). Every request runs one pipeline before its
// handler: the operation is found in the API document; a query parameter the
// operation does not declare is refused; the request is validated against
// the document by kin-openapi; only then does the handler run. Every error is
// a problem details body.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	apispec "github.com/guided-traffic/dash/backend/api"
	"github.com/guided-traffic/dash/backend/internal/api/apigen"
	"github.com/guided-traffic/dash/backend/internal/dashboard"
	"github.com/guided-traffic/dash/backend/internal/problem"
	"github.com/guided-traffic/dash/backend/internal/requestid"
)

// Options configures New.
type Options struct {
	Logger    *slog.Logger
	Version   string
	Commit    string
	BuildTime string
	// Dashboard answers GET /api/v1/dashboard with the page's content now.
	// An error that is a *problem.Error is answered as it is — not_ready
	// while the discovery has not listed. nil answers not_ready.
	Dashboard func(ctx context.Context) (dashboard.Dashboard, error)
	// ValidateResponses checks every response against the document and
	// answers 500 when one does not match: on in the tests, so a handler that
	// drifts from the document fails a test.
	ValidateResponses bool
}

// handler is the API: the router over the document, the generated mux and
// the pipeline in front of them.
type handler struct {
	opts   Options
	served []byte
	router routers.Router
	mux    *http.ServeMux
	logger *slog.Logger
}

// New builds the API handler. It fails only when the embedded document does
// not load, which is a build defect.
func New(opts Options) (http.Handler, error) {
	if opts.Logger == nil {
		opts.Logger = slog.New(slog.DiscardHandler)
	}
	doc, served, err := loadDocument(apispec.Document, opts.Version)
	if err != nil {
		return nil, err
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		return nil, fmt.Errorf("route the API document: %w", err)
	}
	h := &handler{opts: opts, served: served, router: router, mux: http.NewServeMux(), logger: opts.Logger}
	strict := apigen.NewStrictHandlerWithOptions(&server{h: h}, nil, apigen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, _ error) {
			problem.Write(w, r, problem.New(problem.ValidationFailed, "the request does not match the API document"))
		},
		ResponseErrorHandlerFunc: h.writeError,
	})
	apigen.HandlerWithOptions(strict, apigen.StdHTTPServerOptions{
		BaseRouter: h.mux,
		ErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			problem.Write(w, r, problem.New(problem.ValidationFailed, err.Error()))
		},
	})
	return h, nil
}

// loadDocument loads the document and returns it with the backend version as
// info.version, and its JSON as served.
func loadDocument(document []byte, version string) (*openapi3.T, []byte, error) {
	var raw map[string]any
	if err := json.Unmarshal(document, &raw); err != nil {
		return nil, nil, fmt.Errorf("decode the API document: %w", err)
	}
	if info, ok := raw["info"].(map[string]any); ok && version != "" {
		info["version"] = version
	}
	served, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("encode the API document: %w", err)
	}
	doc, err := openapi3.NewLoader().LoadFromData(served)
	if err != nil {
		return nil, nil, fmt.Errorf("load the API document: %w", err)
	}
	// The server is mounted at /, so the document's paths are matched as
	// written whatever host the request names.
	doc.Servers = nil
	return doc, served, nil
}

// allMethods are the methods probed for the Allow header of a 405.
var allMethods = []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}

// ServeHTTP runs the pipeline.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	route, pathParams, err := h.router.FindRoute(r)
	if err != nil {
		h.writeRouteError(w, r, err)
		return
	}
	if perr := validateRequest(r, route, pathParams); perr != nil {
		problem.Write(w, r, perr)
		return
	}
	if h.opts.ValidateResponses {
		h.serveValidated(w, r, route, pathParams)
		return
	}
	h.mux.ServeHTTP(w, r)
}

// writeRouteError answers a request no operation of the document matches:
// 405 with Allow when the path exists with other methods, 404 otherwise.
func (h *handler) writeRouteError(w http.ResponseWriter, r *http.Request, err error) {
	if !errors.Is(err, routers.ErrMethodNotAllowed) {
		problem.Write(w, r, problem.New(problem.NotFound, "no route "+r.Method+" "+r.URL.Path))
		return
	}
	var allowed []string
	for _, m := range allMethods {
		probe := r.Clone(r.Context())
		probe.Method = m
		if _, _, err := h.router.FindRoute(probe); err == nil {
			allowed = append(allowed, m)
		}
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	problem.Write(w, r, problem.New(problem.MethodNotAllowed, r.Method+" is not allowed on "+r.URL.Path))
}

// writeError answers a handler's error: a problem as it is, anything else as
// 500 with the details in the log only.
func (h *handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	var perr *problem.Error
	if errors.As(err, &perr) {
		problem.Write(w, r, perr)
		return
	}
	h.logger.Error("request failed", "request_id", requestid.From(r.Context()), "method", r.Method, "path", r.URL.Path, "error", err)
	problem.Write(w, r, problem.New(problem.Internal, "internal error"))
}

// validateRequest holds the request to the API document and refuses a query
// parameter the operation does not declare, which the validator would let
// pass.
func validateRequest(r *http.Request, route *routers.Route, pathParams map[string]string) *problem.Error {
	if perr := unknownQueryParameters(r, route); perr != nil {
		return perr
	}
	if err := openapi3filter.ValidateRequest(r.Context(), requestInput(r, route, pathParams)); err != nil {
		return validationProblem(err)
	}
	return nil
}

func requestInput(r *http.Request, route *routers.Route, pathParams map[string]string) *openapi3filter.RequestValidationInput {
	return &openapi3filter.RequestValidationInput{
		Request:    r,
		PathParams: pathParams,
		Route:      route,
		Options: &openapi3filter.Options{
			AuthenticationFunc: openapi3filter.NoopAuthenticationFunc,
			MultiError:         true,
			// The handlers apply the defaults; a default written into the
			// request would look like a parameter the client sent.
			SkipSettingDefaults: true,
		},
	}
}

func unknownQueryParameters(r *http.Request, route *routers.Route) *problem.Error {
	declared := map[string]bool{}
	for _, params := range []openapi3.Parameters{route.PathItem.Parameters, route.Operation.Parameters} {
		for _, p := range params {
			if p.Value != nil && p.Value.In == openapi3.ParameterInQuery {
				declared[p.Value.Name] = true
			}
		}
	}
	var unknown []problem.FieldError
	for _, name := range slices.Sorted(maps.Keys(r.URL.Query())) {
		if !declared[name] {
			unknown = append(unknown, problem.FieldError{Pointer: "query:" + name, Message: "unknown parameter"})
		}
	}
	if len(unknown) == 0 {
		return nil
	}
	return &problem.Error{Code: problem.ValidationFailed, Detail: "the request has parameters this route does not know", Errors: unknown}
}

// validationProblem turns the validator's errors into errors[] entries. A
// path parameter that breaks its pattern names nothing that can exist and is
// answered like any unknown address, 404.
func validationProblem(err error) *problem.Error {
	var fields []problem.FieldError
	for _, e := range flatten(err) {
		var reqErr *openapi3filter.RequestError
		if !errors.As(e, &reqErr) {
			fields = append(fields, problem.FieldError{Pointer: "/", Message: e.Error()})
			continue
		}
		if reqErr.Parameter != nil {
			if reqErr.Parameter.In == openapi3.ParameterInPath {
				return problem.New(problem.NotFound, "")
			}
			fields = append(fields, problem.FieldError{Pointer: reqErr.Parameter.In + ":" + reqErr.Parameter.Name, Message: reasonOf(reqErr)})
			continue
		}
		fields = append(fields, problem.FieldError{Pointer: "/", Message: reasonOf(reqErr)})
	}
	return &problem.Error{Code: problem.ValidationFailed, Detail: "the request does not match the API document", Errors: fields}
}

// reasonOf is a request error's reason, its cause's text where it has one.
func reasonOf(e *openapi3filter.RequestError) string {
	if e.Err != nil {
		return e.Err.Error()
	}
	return e.Reason
}

// flatten unrolls the validator's multi-errors, and only those: a
// RequestError unwraps to its cause and must stay whole.
func flatten(err error) []error {
	if multi, ok := err.(openapi3.MultiError); ok { //nolint:errorlint // only the error itself, not its causes
		out := make([]error, 0, len(multi))
		for _, e := range multi {
			out = append(out, flatten(e)...)
		}
		return out
	}
	if err == nil {
		return nil
	}
	return []error{err}
}
