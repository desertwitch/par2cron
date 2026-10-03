package specs

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: The embedded bundle specification should match the file it is embedded from.
func Test_BundleSpecification_MatchesFile_Success(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("bundle_specification.txt")
	require.NoError(t, err)

	require.Equal(t, string(data), BundleSpecification)
}

// Expectation: The embedded bundle specification should be newline-terminated.
func Test_BundleSpecification_TrailingNewline_Success(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, BundleSpecification)
	require.True(t, strings.HasSuffix(BundleSpecification, "\n"))
}
