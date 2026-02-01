package util

import (
	"log/slog"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: Successfully parse valid PAR2 file and populate manifest.
func Test_Par2ToManifest_ValidFile_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewOsFs()
	var buf testutil.SafeBuffer
	log := &logging.Logger{
		Logger:  slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Options: logging.Options{},
	}

	manifest := &schema.Manifest{}
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     testTime,
		Path:     "testdata/simple_par2cmdline.par2",
		Manifest: manifest,
	}, log)

	require.NotNil(t, manifest.Par2Data)
	require.Equal(t, testTime, manifest.Par2Data.Time)
	require.NotNil(t, manifest.Par2Data.Set)
	require.Contains(t, buf.String(), "Parsed PAR2 set to manifest")
}

// Expectation: Reuse existing Par2Data pointer when updating existing data.
func Test_Par2ToManifest_ReuseExistingPointer_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewOsFs()
	var buf testutil.SafeBuffer
	log := &logging.Logger{
		Logger:  slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Options: logging.Options{},
	}

	existingData := &schema.Par2DataManifest{}
	manifest := &schema.Manifest{Par2Data: existingData}
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     testTime,
		Path:     "testdata/simple_par2cmdline.par2",
		Manifest: manifest,
	}, log)

	require.Same(t, existingData, manifest.Par2Data)
	require.Equal(t, testTime, manifest.Par2Data.Time)
}

// Expectation: Preserve existing data when parse fails.
func Test_Par2ToManifest_ParseError_PreservesData_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	var buf testutil.SafeBuffer
	log := &logging.Logger{
		Logger:  slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Options: logging.Options{},
	}

	existingTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	existingData := &schema.Par2DataManifest{
		Time: existingTime,
		Set: &par2.FileSet{
			Sets: []par2.Set{{SetID: par2.Hash{1, 2, 3}}},
		},
	}
	manifest := &schema.Manifest{Par2Data: existingData}

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     time.Now(),
		Path:     "/notexist.par2",
		Manifest: manifest,
	}, log)

	require.Same(t, existingData, manifest.Par2Data)
	require.Equal(t, existingTime, manifest.Par2Data.Time)
	require.NotNil(t, manifest.Par2Data.Set)
	require.Contains(t, buf.String(), "Failed to parse PAR2 set for par2cron manifest")
}

// Expectation: Update manifest even when file has no datasets.
func Test_Par2ToManifest_EmptyDatasets_UpdatesManifest_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/empty.par2", []byte{}, 0o644))

	var buf testutil.SafeBuffer
	log := &logging.Logger{
		Logger:  slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
		Options: logging.Options{},
	}

	manifest := &schema.Manifest{}
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     testTime,
		Path:     "/empty.par2",
		Manifest: manifest,
	}, log)

	require.NotNil(t, manifest.Par2Data)
	require.Equal(t, testTime, manifest.Par2Data.Time)
	require.NotNil(t, manifest.Par2Data.Set)
	require.Empty(t, manifest.Par2Data.Set.Sets)
	require.Contains(t, buf.String(), "PAR2 set is syntactically valid, but seems to contain no datasets")
}
