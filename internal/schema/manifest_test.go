package schema

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: Important constants should not have changed.
func Test_ManifestVersion_Constant_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, "1", ManifestVersion)
}

// Expectation: A new manifest is created with the constants populated.
func Test_NewManifest_Success(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(t.Context(), VersionKey, "0.1.2")
	mf := NewManifest(ctx, "test"+Par2Extension)

	require.Equal(t, "test"+Par2Extension, mf.Name)
	require.Equal(t, ManifestVersion, mf.ManifestVersion)
	require.Equal(t, "0.1.2", mf.ProgramVersion)

	require.Empty(t, mf.SHA256)
	require.Nil(t, mf.Creation)
	require.Nil(t, mf.Verification)
	require.Nil(t, mf.Repair)
}
