package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guided-traffic/dash/backend/internal/problem"
	"github.com/guided-traffic/dash/backend/internal/requestid"
)

// newAPI is the handler as the backend builds it, every response held to the
// document.
func newAPI(t *testing.T) http.Handler {
	t.Helper()
	h, err := New(Options{Version: "1.2.3", Commit: "abc1234", BuildTime: "1793000000", ValidateResponses: true})
	require.NoError(t, err)
	return h
}

func get(t *testing.T, h http.Handler, method, target string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	r = r.WithContext(requestid.With(r.Context(), requestid.New()))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, r)
	return rec
}

func problemOf(t *testing.T, rec *httptest.ResponseRecorder) problem.Body {
	t.Helper()
	assert.Equal(t, "application/problem+json; charset=utf-8", rec.Header().Get("Content-Type"))
	var body problem.Body
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, rec.Code, body.Status)
	return body
}

func TestGetVersionAnswersTheBuild(t *testing.T) {
	rec := get(t, newAPI(t), http.MethodGet, "/api/v1/version")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.JSONEq(t, `{"version":"1.2.3","commit":"abc1234","build_time":"1793000000"}`, rec.Body.String())
}

func TestGetOpenAPIAnswersTheDocumentWithTheBackendVersion(t *testing.T) {
	rec := get(t, newAPI(t), http.MethodGet, "/api/v1/openapi.json")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	doc, err := openapi3.NewLoader().LoadFromData(rec.Body.Bytes())
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", doc.Info.Version)
	assert.Equal(t, "Dash", doc.Info.Title)
	assert.NotNil(t, doc.Paths.Find("/api/v1/version"))
	assert.NoError(t, doc.Validate(t.Context()), "the served document is valid")
}

func TestAnEmptyVersionKeepsTheDocumentsOwn(t *testing.T) {
	h, err := New(Options{})
	require.NoError(t, err)
	rec := get(t, h, http.MethodGet, "/api/v1/openapi.json")
	assert.Contains(t, rec.Body.String(), `"version":"0.0.0"`)
}

func TestAnUnknownQueryParameterIsRefused(t *testing.T) {
	rec := get(t, newAPI(t), http.MethodGet, "/api/v1/version?verbose=1&a=2")
	require.Equal(t, http.StatusBadRequest, rec.Code)
	p := problemOf(t, rec)
	assert.Equal(t, "validation_failed", p.Code)
	assert.Equal(t, []problem.FieldError{
		{Pointer: "query:a", Message: "unknown parameter"},
		{Pointer: "query:verbose", Message: "unknown parameter"},
	}, p.Errors, "every unknown parameter, sorted")
	assert.Equal(t, "/api/v1/version", p.Instance)
}

func TestAnUnknownPathIsAProblem404(t *testing.T) {
	rec := get(t, newAPI(t), http.MethodGet, "/api/v1/nothing")
	require.Equal(t, http.StatusNotFound, rec.Code)
	p := problemOf(t, rec)
	assert.Equal(t, "not_found", p.Code)
	assert.Equal(t, "no route GET /api/v1/nothing", p.Detail)
	assert.NotEmpty(t, p.RequestID)
}

func TestAnotherMethodIsA405ThatNamesTheDocumentsMethods(t *testing.T) {
	rec := get(t, newAPI(t), http.MethodPost, "/api/v1/version")
	require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, "GET", rec.Header().Get("Allow"), "the router serves what the document declares, and it declares GET")
	assert.Equal(t, "method_not_allowed", problemOf(t, rec).Code)
}

// A handler that drifts from the document fails the tests: the response is
// replaced by a 500 that names the mismatch.
func TestAResponseTheDocumentDoesNotDescribeIsRefusedInTests(t *testing.T) {
	h, err := New(Options{ValidateResponses: true})
	require.NoError(t, err)
	drifted := h.(*handler)
	drifted.mux = http.NewServeMux()
	drifted.mux.HandleFunc("GET /api/v1/version", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version": 1}`))
	})

	rec := get(t, drifted, http.MethodGet, "/api/v1/version")
	require.Equal(t, http.StatusInternalServerError, rec.Code)
	p := problemOf(t, rec)
	assert.Equal(t, "internal", p.Code)
	assert.Contains(t, p.Detail, "the response does not match the API document")
}

func TestWithoutResponseValidationTheHandlerAnswersDirectly(t *testing.T) {
	h, err := New(Options{Version: "1.0.0"})
	require.NoError(t, err)
	rec := get(t, h, http.MethodGet, "/api/v1/version")
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestWriteErrorPassesAProblemAndHidesAnythingElse(t *testing.T) {
	h := &handler{logger: nil}
	h.logger = newDiscardLogger()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)

	rec := httptest.NewRecorder()
	h.writeError(rec, r, problem.New(problem.NotFound, "gone"))
	assert.Equal(t, http.StatusNotFound, rec.Code)

	rec = httptest.NewRecorder()
	h.writeError(rec, r, errors.New("dial tcp 10.0.0.1:443: secret detail"))
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.NotContains(t, rec.Body.String(), "secret detail")
}

func TestLoadDocumentRefusesWhatIsNoDocument(t *testing.T) {
	_, _, err := loadDocument([]byte("not json"), "1")
	assert.Error(t, err)
	_, _, err = loadDocument([]byte(`{"openapi": 3}`), "1")
	assert.Error(t, err, "a document kin-openapi cannot load")
}

func TestValidationProblemNamesEachField(t *testing.T) {
	query := &openapi3filter.RequestError{Parameter: &openapi3.Parameter{In: openapi3.ParameterInQuery, Name: "page"}, Err: errors.New("must be at least 1")}
	header := &openapi3filter.RequestError{Parameter: &openapi3.Parameter{In: openapi3.ParameterInHeader, Name: "X-Y"}, Reason: "missing"}
	body := &openapi3filter.RequestError{Reason: "the body is not JSON"}
	other := errors.New("something else")

	p := validationProblem(openapi3.MultiError{query, openapi3.MultiError{header, body}, other})
	assert.Equal(t, problem.ValidationFailed, p.Code)
	assert.Equal(t, []problem.FieldError{
		{Pointer: "query:page", Message: "must be at least 1"},
		{Pointer: "header:X-Y", Message: "missing"},
		{Pointer: "/", Message: "the body is not JSON"},
		{Pointer: "/", Message: "something else"},
	}, p.Errors)
}

// A path parameter that breaks its pattern names nothing that can exist: it
// is answered like any unknown address.
func TestAnInvalidPathParameterIsNotFound(t *testing.T) {
	path := &openapi3filter.RequestError{Parameter: &openapi3.Parameter{In: openapi3.ParameterInPath, Name: "id"}, Reason: "pattern"}
	assert.Equal(t, problem.NotFound, validationProblem(path).Code)
}

func TestFlattenKeepsARequestErrorWhole(t *testing.T) {
	inner := &openapi3filter.RequestError{Reason: "r", Err: openapi3.MultiError{errors.New("a"), errors.New("b")}}
	assert.Len(t, flatten(inner), 1)
	assert.Nil(t, flatten(nil))
	assert.Len(t, flatten(openapi3.MultiError{errors.New("a"), errors.New("b")}), 2)
}

func TestEveryOperationOfTheDocumentHasAHandler(t *testing.T) {
	h := newAPI(t)
	doc, _, err := loadDocument(mustDocument(t), "")
	require.NoError(t, err)
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			rec := get(t, h, method, path)
			assert.NotEqual(t, http.StatusNotFound, rec.Code, "%s %s has no handler", method, path)
			assert.Less(t, rec.Code, 500, "%s %s: %s", method, path, strings.TrimSpace(rec.Body.String()))
		}
	}
}
