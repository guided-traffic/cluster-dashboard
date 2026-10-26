package requestid

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewIsAVersion7UUID(t *testing.T) {
	id := New()
	assert.Equal(t, uuid.Version(7), id.Version())
	assert.NotEqual(t, id, New(), "every request gets its own id")
}

func TestFromReturnsTheIDTheContextCarries(t *testing.T) {
	id := New()
	ctx := With(context.Background(), id)
	require.Equal(t, id.String(), From(ctx))
}

func TestFromIsEmptyWithoutAnID(t *testing.T) {
	assert.Empty(t, From(context.Background()))
}
