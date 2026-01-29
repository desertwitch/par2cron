package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: Important constants should not have changed.
func Test_ManifestVersion_Constant_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, "2", ManifestVersion)
}

// Expectation: A new manifest is created with the constants populated.
func Test_NewManifest_Success(t *testing.T) {
	t.Parallel()

	mf := NewManifest("test" + Par2Extension)

	require.Equal(t, "test"+Par2Extension, mf.Name)
	require.Equal(t, ProgramVersion, mf.ProgramVersion)
	require.Equal(t, ManifestVersion, mf.ManifestVersion)

	require.Empty(t, mf.SHA256)
	require.Nil(t, mf.Creation)
	require.Nil(t, mf.Verification)
	require.Nil(t, mf.Repair)
}

// Expectation: The unmarshalling should work according to expectations.
func Test_CreationManifest_UnmarshalJSON_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		jsonData string
		check    func(*CreationManifest)
	}{
		{
			name: "Uses modern 'elements' key",
			jsonData: `{
				"time": "2023-10-01T10:00:00Z",
				"duration_ns": 1000000,
				"elements": [{"name": "test.txt"}]
			}`,
			check: func(m *CreationManifest) {
				require.Len(t, m.Elements, 1)
				require.Equal(t, "test.txt", m.Elements[0].Name)
			},
		},
		{
			name: "Fallback to legacy 'files' key",
			jsonData: `{
				"time": "2023-10-01T10:00:00Z",
				"duration_ns": 1000000,
				"files": [{"name": "test.txt"}]
			}`,
			check: func(m *CreationManifest) {
				require.Len(t, m.Elements, 1, "Should have migrated 'files' to 'elements'")
				require.Equal(t, "test.txt", m.Elements[0].Name)
			},
		},
		{
			name: "Modern key takes precedence over legacy",
			jsonData: `{
				"elements": [{"name": "new.txt"}],
				"files": [{"name": "old.txt"}]
			}`,
			check: func(m *CreationManifest) {
				require.Len(t, m.Elements, 1)
				require.Equal(t, "new.txt", m.Elements[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var m CreationManifest
			err := json.Unmarshal([]byte(tt.jsonData), &m)
			require.NoError(t, err)

			tt.check(&m)
		})
	}
}
