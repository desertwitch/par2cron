package schema

import (
	"encoding/json"
	"testing"
	"time"

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

// Expectation: NormalizeTimes should convert all timestamps to UTC without changing the instants.
func Test_Manifest_NormalizeTimes_Success(t *testing.T) {
	t.Parallel()

	cest := time.FixedZone("CEST", 2*60*60)

	creationTime := time.Date(2026, 9, 18, 15, 57, 1, 123456789, cest)
	modTime := time.Date(2026, 9, 17, 10, 0, 0, 987654321, cest)
	verifyTime := time.Date(2026, 9, 18, 16, 0, 0, 0, cest)
	healthyTime := time.Date(2026, 9, 18, 16, 5, 0, 0, cest)
	repairTime := time.Date(2026, 9, 18, 17, 0, 0, 0, cest)

	mf := NewManifest("test" + Par2Extension)
	mf.Creation = NewCreationManifest()
	mf.Creation.Time = creationTime
	mf.Creation.Elements = []FsElement{{Name: "test.txt", ModTime: modTime}}
	mf.Verification = NewVerificationManifest()
	mf.Verification.Time = verifyTime
	mf.Verification.TimeLastHealthy = healthyTime
	mf.Repair = NewRepairManifest()
	mf.Repair.Time = repairTime

	mf.NormalizeTimes()

	require.Equal(t, time.UTC, mf.Creation.Time.Location())
	require.True(t, creationTime.Equal(mf.Creation.Time))

	require.Equal(t, time.UTC, mf.Creation.Elements[0].ModTime.Location())
	require.True(t, modTime.Equal(mf.Creation.Elements[0].ModTime))
	require.Equal(t, 987654321, mf.Creation.Elements[0].ModTime.Nanosecond())

	require.Equal(t, time.UTC, mf.Verification.Time.Location())
	require.True(t, verifyTime.Equal(mf.Verification.Time))

	require.Equal(t, time.UTC, mf.Verification.TimeLastHealthy.Location())
	require.True(t, healthyTime.Equal(mf.Verification.TimeLastHealthy))

	require.Equal(t, time.UTC, mf.Repair.Time.Location())
	require.True(t, repairTime.Equal(mf.Repair.Time))
}

// Expectation: NormalizeTimes should not panic when manifest sections are missing.
func Test_Manifest_NormalizeTimes_NilSections_Success(t *testing.T) {
	t.Parallel()

	mf := NewManifest("test" + Par2Extension)

	require.NotPanics(t, mf.NormalizeTimes)

	require.Nil(t, mf.Creation)
	require.Nil(t, mf.Verification)
	require.Nil(t, mf.Repair)
}

// Expectation: NormalizeTimes should keep zero timestamps as zero.
func Test_Manifest_NormalizeTimes_ZeroTimes_Success(t *testing.T) {
	t.Parallel()

	mf := NewManifest("test" + Par2Extension)
	mf.Verification = NewVerificationManifest()

	mf.NormalizeTimes()

	require.True(t, mf.Verification.Time.IsZero())
	require.True(t, mf.Verification.TimeLastHealthy.IsZero())
}

// Expectation: A normalized manifest should marshal all timestamps in UTC and round-trip to the same instants.
func Test_Manifest_NormalizeTimes_MarshalJSON_Success(t *testing.T) {
	t.Parallel()

	cest := time.FixedZone("CEST", 2*60*60)

	creationTime := time.Date(2026, 9, 18, 15, 57, 1, 0, cest)
	modTime := time.Date(2026, 9, 17, 10, 0, 0, 0, cest)
	verifyTime := time.Date(2026, 9, 18, 16, 0, 0, 0, cest)

	mf := NewManifest("test" + Par2Extension)
	mf.Creation = NewCreationManifest()
	mf.Creation.Time = creationTime
	mf.Creation.Elements = []FsElement{{Name: "test.txt", ModTime: modTime}}
	mf.Verification = NewVerificationManifest()
	mf.Verification.Time = verifyTime

	mf.NormalizeTimes()

	data, err := json.Marshal(mf)
	require.NoError(t, err)

	require.NotContains(t, string(data), "+02:00")
	require.Contains(t, string(data), `"2026-09-18T13:57:01Z"`)
	require.Contains(t, string(data), `"2026-09-17T08:00:00Z"`)
	require.Contains(t, string(data), `"2026-09-18T14:00:00Z"`)

	var decoded Manifest
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.True(t, creationTime.Equal(decoded.Creation.Time))
	require.True(t, modTime.Equal(decoded.Creation.Elements[0].ModTime))
	require.True(t, verifyTime.Equal(decoded.Verification.Time))
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

// Expectation: UnmarshalJSON should return error on wrong type for time.
func Test_CreationManifest_UnmarshalJSON_InvalidTime_Error(t *testing.T) {
	t.Parallel()

	jsonData := `{
		"time": "not-a-valid-time",
		"duration_ns": 1000000,
		"elements": []
	}`

	var m CreationManifest
	err := json.Unmarshal([]byte(jsonData), &m)

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to unmarshal")
}

// Expectation: A new manifest is created with the constants populated.
func Test_NewCreationManifest_Success(t *testing.T) {
	t.Parallel()

	mf := NewCreationManifest()

	require.Equal(t, ProgramVersion, mf.ProgramVersion)
	require.Equal(t, Par2Version, mf.Par2Version)
}

// Expectation: A new manifest is created with the constants populated.
func Test_NewVerificationManifest_Success(t *testing.T) {
	t.Parallel()

	mf := NewVerificationManifest()

	require.Equal(t, ProgramVersion, mf.ProgramVersion)
	require.Equal(t, Par2Version, mf.Par2Version)
}

// Expectation: A new manifest is created with the constants populated.
func Test_NewRepairManifest_Success(t *testing.T) {
	t.Parallel()

	mf := NewRepairManifest()

	require.Equal(t, ProgramVersion, mf.ProgramVersion)
	require.Equal(t, Par2Version, mf.Par2Version)
}
