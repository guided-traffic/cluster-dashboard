package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"

	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"

	"github.com/guided-traffic/dash/backend/internal/api/apigen"
	"github.com/guided-traffic/dash/backend/internal/problem"
)

// server implements the generated strict server interface: one method per
// operation of the document.
type server struct {
	h *handler
}

var _ apigen.StrictServerInterface = (*server)(nil)

// GetVersion answers the build.
func (s *server) GetVersion(context.Context, apigen.GetVersionRequestObject) (apigen.GetVersionResponseObject, error) {
	return apigen.GetVersion200JSONResponse{
		Version:   s.h.opts.Version,
		Commit:    s.h.opts.Commit,
		BuildTime: s.h.opts.BuildTime,
	}, nil
}

// GetOpenAPI answers the document with the backend version as info.version.
func (s *server) GetOpenAPI(context.Context, apigen.GetOpenAPIRequestObject) (apigen.GetOpenAPIResponseObject, error) {
	var doc apigen.GetOpenAPI200JSONResponse
	if err := json.Unmarshal(s.h.served, &doc); err != nil {
		return nil, fmt.Errorf("decode the API document: %w", err)
	}
	return doc, nil
}

// serveValidated serves the request and holds the response to the document
// before it leaves: a response the document does not describe is answered as
// a 500 that names the mismatch. On in the tests only.
func (h *handler) serveValidated(w http.ResponseWriter, r *http.Request, route *routers.Route, pathParams map[string]string) {
	rec := httptest.NewRecorder()
	maps.Copy(rec.Header(), w.Header())
	h.mux.ServeHTTP(rec, r)

	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: requestInput(r.Clone(r.Context()), route, pathParams),
		Status:                 rec.Code,
		Header:                 rec.Header(),
		Body:                   io.NopCloser(bytes.NewReader(rec.Body.Bytes())),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	if err := openapi3filter.ValidateResponse(r.Context(), in); err != nil {
		h.logger.Error("the response does not match the API document", "method", r.Method, "path", r.URL.Path, "error", err)
		problem.Write(w, r, problem.New(problem.Internal, "the response does not match the API document: "+err.Error()))
		return
	}
	maps.Copy(w.Header(), rec.Header())
	w.WriteHeader(rec.Code)
	_, _ = w.Write(rec.Body.Bytes())
}
