package apispec

import (
	"context"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The embedded document is the contract: it loads, it is valid, and every
// operation is named, tagged and answers its errors as problem details.
func TestTheEmbeddedDocumentIsTheContract(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromData(Document)
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))
	assert.Equal(t, "3.1.0", doc.OpenAPI)

	ids := map[string]bool{}
	for path, item := range doc.Paths.Map() {
		for method, op := range item.Operations() {
			where := method + " " + path
			assert.NotEmpty(t, op.OperationID, where)
			assert.False(t, ids[op.OperationID], "%s: operationId %s is used twice", where, op.OperationID)
			ids[op.OperationID] = true
			assert.NotEmpty(t, op.Tags, where)
			assert.NotEmpty(t, op.Summary, where)
			def := op.Responses.Default()
			require.NotNil(t, def, "%s answers its errors with the default response", where)
			assert.NotNil(t, def.Value.Content.Get("application/problem+json"), where)
		}
		assert.Regexp(t, `^/api/v1/`, path, "every path is under /api/v1/, the backend Ingress's path")
	}
}
