package flags

import (
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/stretchr/testify/assert/yaml"
	"github.com/stretchr/testify/require"
)

// Expectation: The function should take an empty string.
func Test_Duration_Set_Empty_Success(t *testing.T) {
	t.Parallel()

	f := &Duration{}

	err := f.Set("")

	require.NoError(t, err)
	require.Equal(t, time.Duration(0), f.Value)
	require.Empty(t, f.Raw)
}

// Expectation: The function should take a zero string.
func Test_Duration_Set_Zero_Success(t *testing.T) {
	t.Parallel()

	f := &Duration{}

	err := f.Set("0")

	require.NoError(t, err)
	require.Zero(t, f.Value)
	require.Equal(t, "0", f.Raw)
}

// Expectation: The function should take a valid duration string.
func Test_Duration_Set_ValidDuration_Success(t *testing.T) {
	t.Parallel()

	f := &Duration{}

	err := f.Set(" 1H30m ")

	require.NoError(t, err)
	require.Equal(t, 90*time.Minute, f.Value)
	require.Equal(t, "1h30m", f.Raw)
}

// Expectation: The function should reject an invalid duration string.
func Test_Duration_Set_InvalidDuration_Error(t *testing.T) {
	t.Parallel()

	f := &Duration{}

	err := f.Set("invalid")

	require.Error(t, err)
}

// Expectation: The function should return it's type as string.
func Test_Duration_Type(t *testing.T) {
	t.Parallel()

	f := &Duration{}

	require.Equal(t, "duration", f.Type())
}

// Expectation: The function should return an empty string.
func Test_Duration_String_Empty_Success(t *testing.T) {
	t.Parallel()

	f := &Duration{}

	require.Empty(t, f.String())
}

// Expectation: The function should return the contained raw string.
func Test_Duration_String_WithValue_Success(t *testing.T) {
	t.Parallel()

	f := &Duration{Raw: "1h"}

	require.Equal(t, "1h", f.String())
}

// Expectation: The function should take a valid log level string.
func Test_LogLevel_Set_Table_Success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantLevel slog.Level
		wantRaw   string
	}{
		{
			name:      "debug",
			input:     "debug",
			wantLevel: slog.LevelDebug,
			wantRaw:   "debug",
		},
		{
			name:      "info",
			input:     "info",
			wantLevel: slog.LevelInfo,
			wantRaw:   "info",
		},
		{
			name:      "warn",
			input:     "warn",
			wantLevel: slog.LevelWarn,
			wantRaw:   "warn",
		},
		{
			name:      "warning",
			input:     "warning",
			wantLevel: slog.LevelWarn,
			wantRaw:   "warning",
		},
		{
			name:      "error",
			input:     "error",
			wantLevel: slog.LevelError,
			wantRaw:   "error",
		},
		{
			name:      "case insensitive",
			input:     "INFO",
			wantLevel: slog.LevelInfo,
			wantRaw:   "info",
		},
		{
			name:      "with whitespace",
			input:     "  debug  ",
			wantLevel: slog.LevelDebug,
			wantRaw:   "debug",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			f := &LogLevel{}

			err := f.Set(tt.input)

			require.NoError(t, err)
			require.Equal(t, tt.wantLevel, f.Value)

			if tt.wantRaw != "" {
				require.Equal(t, tt.wantRaw, f.Raw)
			}
		})
	}
}

// Expectation: The function should marshal an empty duration to an empty JSON string.
func Test_Duration_MarshalJSON_Empty_Success(t *testing.T) {
	t.Parallel()

	f := Duration{}

	data, err := json.Marshal(f)

	require.NoError(t, err)
	require.Equal(t, `""`, string(data))
}

// Expectation: The function should marshal a duration to its raw JSON string.
func Test_Duration_MarshalJSON_WithValue_Success(t *testing.T) {
	t.Parallel()

	f := Duration{Raw: "1h30m", Value: 90 * time.Minute}

	data, err := json.Marshal(f)

	require.NoError(t, err)
	require.Equal(t, `"1h30m"`, string(data))
}

// Expectation: The function should unmarshal a valid JSON duration string.
func Test_Duration_UnmarshalJSON_Valid_Success(t *testing.T) {
	t.Parallel()

	var f Duration

	err := json.Unmarshal([]byte(`"1h30m"`), &f)

	require.NoError(t, err)
	require.Equal(t, 90*time.Minute, f.Value)
	require.Equal(t, "1h30m", f.Raw)
}

// Expectation: The function should unmarshal an empty JSON string.
func Test_Duration_UnmarshalJSON_Empty_Success(t *testing.T) {
	t.Parallel()

	var f Duration

	err := json.Unmarshal([]byte(`""`), &f)

	require.NoError(t, err)
	require.Zero(t, f.Value)
	require.Empty(t, f.Raw)
}

// Expectation: The function should reject an invalid JSON duration string.
func Test_Duration_UnmarshalJSON_InvalidDuration_Error(t *testing.T) {
	t.Parallel()

	var f Duration

	err := json.Unmarshal([]byte(`"invalid"`), &f)

	require.Error(t, err)
}

// Expectation: The function should reject non-string JSON.
func Test_Duration_UnmarshalJSON_InvalidType_Error(t *testing.T) {
	t.Parallel()

	var f Duration

	err := json.Unmarshal([]byte(`123`), &f)

	require.Error(t, err)
}

// Expectation: The function should roundtrip through JSON correctly.
func Test_Duration_JSON_Roundtrip_Success(t *testing.T) {
	t.Parallel()

	original := Duration{Raw: "2h45m", Value: 165 * time.Minute}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored Duration
	err = json.Unmarshal(data, &restored)

	require.NoError(t, err)
	require.Equal(t, original.Raw, restored.Raw)
	require.Equal(t, original.Value, restored.Value)
}

// Expectation: The function should unmarshal a valid duration from YAML.
func Test_Duration_UnmarshalYAML_Success(t *testing.T) {
	t.Parallel()

	var f Duration

	err := yaml.Unmarshal([]byte(`1h30m`), &f)

	require.NoError(t, err)
	require.Equal(t, 90*time.Minute, f.Value)
	require.Equal(t, "1h30m", f.Raw)
}

// Expectation: The function should reject an invalid log level string.
func Test_LogLevel_Set_InvalidLevel_Error(t *testing.T) {
	t.Parallel()

	f := &LogLevel{}

	err := f.Set("invalid")

	require.ErrorIs(t, err, errInvalidValue)
}

// Expectation: The function should return it's type as string.
func Test_LogLevel_Type_Success(t *testing.T) {
	t.Parallel()

	f := &LogLevel{}

	require.Equal(t, "level", f.Type())
}

// Expectation: The function should return an empty string.
func Test_LogLevel_String_Empty_Success(t *testing.T) {
	t.Parallel()

	f := &LogLevel{}

	require.Empty(t, f.String())
}

// Expectation: The function should return the contained raw string.
func Test_LogLevel_String_WithValue_Success(t *testing.T) {
	t.Parallel()

	f := &LogLevel{Raw: "info"}

	require.Equal(t, "info", f.String())
}

// Expectation: The function should unmarshal a valid log level from YAML.
func Test_LogLevel_UnmarshalYAML_Success(t *testing.T) {
	t.Parallel()

	var f LogLevel

	err := yaml.Unmarshal([]byte(`info`), &f)

	require.NoError(t, err)
	require.Equal(t, slog.LevelInfo, f.Value)
	require.Equal(t, "info", f.Raw)
}

// Expectation: The function should accept a valid mode string.
func Test_CreateMode_Set_FileMode_Success(t *testing.T) {
	t.Parallel()

	f := &CreateMode{}

	err := f.Set(schema.CreateFileMode)
	require.NoError(t, err)

	require.Equal(t, schema.CreateFileMode, f.Raw)
	require.Equal(t, schema.CreateFileMode, f.Value)
}

// Expectation: The function should accept a valid mode string.
func Test_CreateMode_Set_FolderMode_Success(t *testing.T) {
	t.Parallel()

	f := &CreateMode{}

	err := f.Set(schema.CreateFolderMode)
	require.NoError(t, err)

	require.Equal(t, schema.CreateFolderMode, f.Raw)
	require.Equal(t, schema.CreateFolderMode, f.Value)
}

// Expectation: The function should reject an invalid mode string.
func Test_CreateMode_Set_InvalidMode_Error(t *testing.T) {
	t.Parallel()

	f := &CreateMode{}

	err := f.Set("invalid")

	require.ErrorIs(t, err, errInvalidValue)
}

// Expectation: The function should return it's type as string.
func Test_CreateMode_Type_Success(t *testing.T) {
	t.Parallel()

	f := &CreateMode{}

	require.Equal(t, "mode", f.Type())
}

// Expectation: The function should return an empty string.
func Test_CreateMode_String_Empty_Success(t *testing.T) {
	t.Parallel()

	f := &CreateMode{}

	require.Empty(t, f.String())
}

// Expectation: The function should return the contained raw string.
func Test_CreateMode_String_WithValue_Success(t *testing.T) {
	t.Parallel()

	f := &CreateMode{Raw: schema.CreateFileMode}

	require.Equal(t, schema.CreateFileMode, f.String())
}

// Expectation: The function should unmarshal a valid mode from YAML.
func Test_CreateMode_UnmarshalYAML_Success(t *testing.T) {
	t.Parallel()

	var f CreateMode

	err := yaml.Unmarshal([]byte(schema.CreateFileMode), &f)

	require.NoError(t, err)
	require.Equal(t, schema.CreateFileMode, f.Value)
	require.Equal(t, schema.CreateFileMode, f.Raw)
}
