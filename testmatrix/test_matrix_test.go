package testmatrix

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTestMatrix(t *testing.T) {
	testMatrix := NewTestMatrix()

	dimensionA := NewDimension("fieldA", []any{"a.1", "a.2"})
	dimensionB := NewDimension("fieldB", []any{"b.1", "b.2", "b.3"})
	dimensionC := NewDimension("fieldC", []any{"c.1"})

	testMatrix.AddDimension(dimensionA)
	testMatrix.AddDimension(dimensionB)
	testMatrix.AddDimension(dimensionC)

	testConfigurations := testMatrix.GenerateTestConfigurations()

	require.Equal(t, 6, len(testConfigurations))

	fmt.Print(TestConfigurationsToString(testConfigurations))
}
