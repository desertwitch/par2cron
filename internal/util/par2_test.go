package util

import (
	"log/slog"
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: Par2ToManifest should populate manifest when parsing succeeds.
func Test_Par2ToManifest_ValidPar2_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewOsFs()

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	manifest := &schema.Manifest{}
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     testTime,
		Path:     "testdata/simple_par2cmdline.par2",
		Manifest: manifest,
	}, log)

	require.NotNil(t, manifest.Archive)
	require.Equal(t, testTime, manifest.Archive.Time)
	require.NotNil(t, manifest.Archive.Content)
	require.Contains(t, buf.String(), "Succeeded to parse PAR2 to manifest")
}

// Expectation: Par2ToManifest should preserve existing archive and update fields.
func Test_Par2ToManifest_ExistingArchive_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewOsFs()

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	existingArchive := &schema.ArchiveManifest{}
	manifest := &schema.Manifest{Archive: existingArchive}
	testTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     testTime,
		Path:     "testdata/simple_par2cmdline.par2",
		Manifest: manifest,
	}, log)

	require.Same(t, existingArchive, manifest.Archive)
	require.Equal(t, testTime, manifest.Archive.Time)
}

// Expectation: Par2ToManifest should log warning when file does not exist.
func Test_Par2ToManifest_FileNotFound_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	manifest := &schema.Manifest{}

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     time.Now(),
		Path:     "/nonexistent/file.par2",
		Manifest: manifest,
	}, log)

	require.Nil(t, manifest.Archive)
	require.Contains(t, buf.String(), "Failed to parse PAR2 for par2cron manifest")
}

// Expectation: Par2ToManifest should log warning when file is invalid PAR2.
func Test_Par2ToManifest_InvalidPar2_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/test.par2", []byte("not a valid par2 file content here"), 0o644))

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	manifest := &schema.Manifest{}

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     time.Now(),
		Path:     "/test.par2",
		Manifest: manifest,
	}, log)

	require.Nil(t, manifest.Archive)
	require.Contains(t, buf.String(), "Failed to parse PAR2 for par2cron manifest")
}

// Expectation: Par2ToManifest should not modify manifest on parse failure.
func Test_Par2ToManifest_ParseFailure_ManifestUnchanged_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/invalid.par2", []byte("invalid par2 content"), 0o644))

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	manifest := &schema.Manifest{}

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     time.Now(),
		Path:     "/invalid.par2",
		Manifest: manifest,
	}, log)

	require.Nil(t, manifest.Archive)
}

// Expectation: Par2ToManifest should log warning and set archive to nil when file is empty.
func Test_Par2ToManifest_EmptyFile_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/empty.par2", []byte{}, 0o644))

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	manifest := &schema.Manifest{}

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     time.Now(),
		Path:     "/empty.par2",
		Manifest: manifest,
	}, log)

	require.Nil(t, manifest.Archive)
	require.Contains(t, buf.String(), "PAR2 file parsed as containing no datasets")
}

// Expectation: Par2ToManifest should clear existing archive when file parses as empty.
func Test_Par2ToManifest_EmptyFile_ClearsExistingArchive_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fsys, "/empty.par2", []byte{}, 0o644))

	var buf testutil.SafeBuffer
	log := slog.New(slog.NewTextHandler(&buf, nil))

	existingArchive := &schema.ArchiveManifest{
		Time: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	manifest := &schema.Manifest{Archive: existingArchive}

	Par2ToManifest(fsys, Par2ToManifestOptions{
		Time:     time.Now(),
		Path:     "/empty.par2",
		Manifest: manifest,
	}, log)

	require.Nil(t, manifest.Archive)
	require.Contains(t, buf.String(), "PAR2 file parsed as containing no datasets")
}
