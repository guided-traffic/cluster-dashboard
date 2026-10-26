package problem

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guided-traffic/dash/backend/internal/requestid"
)

func TestTypeIsTheCodeUnderTheBase(t *testing.T) {
	assert.Equal(t, "https://dash.gtrfc.com/problems/method-not-allowed", MethodNotAllowed.Type())
}

// Every code is snake_case, unique, has a title and a meaning, and an error
// status: the generators and the README table read the catalogue as it is.
func TestTheCatalogueIsWellFormed(t *testing.T) {
	snake := regexp.MustCompile(`^[a-z]+(_[a-z]+)*$`)
	seen := map[string]bool{}
	for _, c := range Catalogue {
		assert.Regexp(t, snake, c.Code)
		assert.False(t, seen[c.Code], "code %s is listed twice", c.Code)
		seen[c.Code] = true
		assert.NotEmpty(t, c.Title, c.Code)
		assert.NotEmpty(t, c.Meaning, c.Code)
		assert.GreaterOrEqual(t, c.Status, 400, c.Code)
	}
}

func TestWriteAnswersAProblemBody(t *testing.T) {
	id := requestid.New()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/nothing?secret=1", nil)
	r = r.WithContext(requestid.With(r.Context(), id))
	rec := httptest.NewRecorder()

	Write(rec, r, &Error{
		Code:    ValidationFailed,
		Detail:  "the request does not match the API document",
		Errors:  []FieldError{{Pointer: "query:x", Message: "unknown parameter"}},
		Headers: map[string]string{"Allow": "GET"},
	})

	res := rec.Result()
	assert.Equal(t, http.StatusBadRequest, res.StatusCode)
	assert.Equal(t, "application/problem+json; charset=utf-8", res.Header.Get("Content-Type"))
	assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))
	assert.Equal(t, "nosniff", res.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "GET", res.Header.Get("Allow"))
	var body Body
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	assert.Equal(t, Body{
		Type:      "https://dash.gtrfc.com/problems/validation-failed",
		Title:     "Validation failed",
		Status:    http.StatusBadRequest,
		Detail:    "the request does not match the API document",
		Instance:  "/api/v1/nothing",
		Code:      "validation_failed",
		RequestID: id.String(),
		Errors:    []FieldError{{Pointer: "query:x", Message: "unknown parameter"}},
	}, body, "the instance is the path without its query")
}

func TestBodyOfWithoutARequest(t *testing.T) {
	body := BodyOf(nil, New(Internal, ""))
	assert.Equal(t, "internal", body.Code)
	assert.Empty(t, body.Instance)
	assert.Empty(t, body.RequestID)
}

func TestErrorText(t *testing.T) {
	assert.Equal(t, "not_found", New(NotFound, "").Error())
	assert.Equal(t, "not_found: no route GET /x", New(NotFound, "no route GET /x").Error())
}
