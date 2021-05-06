package resource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourceCreate_noState(t *testing.T) {
	require := require.New(t)

	var actual int
	r := NewResource(
		WithName("test"),
		WithCreate(func(v int) error {
			actual = v
			return nil
		}),
	)

	// Create
	require.NoError(r.Create(int(42)))

	// Ensure we were called with the proper value
	require.Equal(actual, int(42))
}
