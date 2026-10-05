package configs

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: The embedded example configuration should match the file it is embedded from.
func Test_ExampleConfiguration_MatchesFile_Success(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("par2cron.yaml")
	require.NoError(t, err)

	require.Equal(t, string(data), ExampleConfiguration)
}

// Expectation: The embedded example configuration should be newline-terminated.
func Test_ExampleConfiguration_TrailingNewline_Success(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, ExampleConfiguration)
	require.True(t, strings.HasSuffix(ExampleConfiguration, "\n"))
}

// Expectation: The embedded example configuration should contain a section for every configurable command.
func Test_ExampleConfiguration_AllSections_Success(t *testing.T) {
	t.Parallel()

	for _, section := range []string{"create:", "verify:", "repair:", "info:"} {
		require.Contains(t, ExampleConfiguration, "\n"+section+"\n", "missing section %s", section)
	}
}
