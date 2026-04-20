package create

import (
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The function should pack PAR2 files into a bundle and clean up originals.
func Test_Service_packAsBundle_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{
						MainPacket: &par2.MainPacket{
							SetID: setID,
						},
					},
				},
			}, nil
		},
	}

	var packCalled bool
	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			packCalled = true

			require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, bundlePath)
			require.Equal(t, setID, recoverySetID)
			require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, manifest.Name)
			require.NotEmpty(t, manifest.Bytes)
			require.NotEmpty(t, files)

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, bundler, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.NoError(t, prog.packAsBundle(t.Context(), job, mf))
	require.True(t, packCalled)

	// Original PAR2 files should be cleaned up.
	indexExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, indexExists)

	vol1Exists, _ := afero.Exists(fs, "/data/folder/test.vol00+01"+schema.Par2Extension)
	require.False(t, vol1Exists)

	vol2Exists, _ := afero.Exists(fs, "/data/folder/test.vol01+02"+schema.Par2Extension)
	require.False(t, vol2Exists)

	// Bundle should exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)

	// Job fields should be updated to point to the bundle.
	require.Equal(t, "test"+schema.BundleExtension+schema.Par2Extension, job.par2Name)
	require.Equal(t, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, job.par2Path)
}

// Expectation: The file list and manifest bytes passed to Pack should be correct and complete.
func Test_Service_packAsBundle_CorrectFilesAndManifest_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/other"+schema.Par2Extension, []byte("unrelated"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/file.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	var capturedFiles []bundle.FileInput
	var capturedManifest bundle.ManifestInput
	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			capturedFiles = files
			capturedManifest = manifest

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, bundler, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()
	mf.Creation.Mode = schema.CreateFolderMode
	mf.Creation.Glob = "*.txt"
	mf.Creation.Args = []string{"-r10", "-n5"}

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.NoError(t, prog.packAsBundle(t.Context(), job, mf))

	// Only PAR2 index and volume files for "test" should be included, not unrelated files.
	require.Len(t, capturedFiles, 3)

	fileNames := make([]string, 0, len(capturedFiles))
	for _, f := range capturedFiles {
		fileNames = append(fileNames, f.Name)
		require.Equal(t, filepath.Join("/data/folder", f.Name), f.Path)
	}
	require.Contains(t, fileNames, "test"+schema.Par2Extension)
	require.Contains(t, fileNames, "test.vol00+01"+schema.Par2Extension)
	require.Contains(t, fileNames, "test.vol01+02"+schema.Par2Extension)
	require.NotContains(t, fileNames, "other"+schema.Par2Extension)
	require.NotContains(t, fileNames, "file.txt")

	// Manifest name should match the job's manifest name.
	require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, capturedManifest.Name)

	// Manifest bytes should be valid JSON that unmarshals to the original manifest.
	var unmarshaled schema.Manifest
	require.NoError(t, json.Unmarshal(capturedManifest.Bytes, &unmarshaled))
	require.Equal(t, "test"+schema.Par2Extension, unmarshaled.Name)
	require.Equal(t, schema.CreateFolderMode, unmarshaled.Creation.Mode)
	require.Equal(t, "*.txt", unmarshaled.Creation.Glob)
	require.Equal(t, []string{"-r10", "-n5"}, unmarshaled.Creation.Args)
	require.Equal(t, schema.ProgramVersion, unmarshaled.ProgramVersion)
	require.Equal(t, schema.ManifestVersion, unmarshaled.ManifestVersion)
}

// Expectation: The function should return an error when no bundleable files are found.
func Test_Service_packAsBundle_NoFilesFound_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/unrelated.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &testutil.MockBundleHandler{}, &testutil.MockPar2Handler{})

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.ErrorContains(t, prog.packAsBundle(t.Context(), job, mf), "no files to bundle")
}

// Expectation: The function should return an error when the PAR2 index file cannot be parsed.
func Test_Service_packAsBundle_ParseFileFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return nil, errors.New("parse error")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &testutil.MockBundleHandler{}, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.ErrorContains(t, prog.packAsBundle(t.Context(), job, mf), "failed to parse index par2")
}

// Expectation: The function should return an error when the PAR2 file has no sets.
func Test_Service_packAsBundle_NoSets_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{Sets: []par2.Set{}}, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &testutil.MockBundleHandler{}, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.ErrorContains(t, prog.packAsBundle(t.Context(), job, mf), "malformed file")
}

// Expectation: The function should return an error when the PAR2 file has a nil main packet.
func Test_Service_packAsBundle_NilMainPacket_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: nil},
				},
			}, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &testutil.MockBundleHandler{}, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.ErrorContains(t, prog.packAsBundle(t.Context(), job, mf), "malformed file")
}

// Expectation: The function should return an error when the PAR2 file has multiple sets.
func Test_Service_packAsBundle_MultipleSets_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{}},
					{MainPacket: &par2.MainPacket{}},
				},
			}, nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &testutil.MockBundleHandler{}, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.ErrorContains(t, prog.packAsBundle(t.Context(), job, mf), "malformed file")
}

// Expectation: The function should return an error when the bundler Pack fails.
func Test_Service_packAsBundle_PackFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			return errors.New("pack failed")
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, bundler, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.ErrorContains(t, prog.packAsBundle(t.Context(), job, mf), "failed to pack bundle")
}

// Expectation: The manifest data passed to Pack should be valid JSON of the manifest.
func Test_Service_packAsBundle_ManifestContents_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	var capturedManifest bundle.ManifestInput
	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			capturedManifest = manifest

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, bundler, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()
	mf.Creation.Mode = schema.CreateFolderMode
	mf.Creation.Glob = "*.txt"

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.NoError(t, prog.packAsBundle(t.Context(), job, mf))

	require.Equal(t, "test"+schema.Par2Extension+schema.ManifestExtension, capturedManifest.Name)

	var unmarshaled schema.Manifest
	require.NoError(t, json.Unmarshal(capturedManifest.Bytes, &unmarshaled))
	require.Equal(t, schema.CreateFolderMode, unmarshaled.Creation.Mode)
	require.Equal(t, "*.txt", unmarshaled.Creation.Glob)
}

// Expectation: Hidden files should produce a bundle with the dot prefix in its name.
func Test_Service_packAsBundle_HideFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/.test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/.test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	var capturedBundlePath string
	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			capturedBundlePath = bundlePath

			require.NoError(t, afero.WriteFile(fsys, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, bundler, par2er)

	mf := schema.NewManifest(".test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     ".test" + schema.Par2Extension,
		par2Path:     "/data/folder/.test" + schema.Par2Extension,
		lockPath:     "/data/folder/.test" + schema.Par2Extension + schema.LockExtension,
		manifestName: ".test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/.test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
		hiddenFiles:  true,
	}

	require.NoError(t, prog.packAsBundle(t.Context(), job, mf))

	require.Equal(t, "/data/folder/.test"+schema.BundleExtension+schema.Par2Extension, capturedBundlePath)
	require.Equal(t, ".test"+schema.BundleExtension+schema.Par2Extension, job.par2Name)
}

// Expectation: The function should warn but not error when cleanup of a PAR2 file fails after bundling.
func Test_Service_packAsBundle_CleanupPar2FileFails_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))

	fs := &testutil.FailingRemoveFs{Fs: baseFs, FailSuffix: "test.vol00+01" + schema.Par2Extension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			require.NoError(t, afero.WriteFile(baseFs, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, bundler, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.NoError(t, prog.packAsBundle(t.Context(), job, mf))
	require.Contains(t, logBuf.String(), "Failed to cleanup a file after bundling")

	// The volume file that failed to remove should still exist.
	volExists, _ := afero.Exists(fs, "/data/folder/test.vol00+01"+schema.Par2Extension)
	require.True(t, volExists)

	// The index file that succeeded removal should be gone.
	indexExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, indexExists)

	// Bundle should still exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)
}

// Expectation: The function should warn but not error when cleanup of the manifest file fails after bundling.
func Test_Service_packAsBundle_CleanupManifestFails_Success(t *testing.T) {
	t.Parallel()

	baseFs := afero.NewMemMapFs()
	require.NoError(t, baseFs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension, []byte("par2index"), 0o644))
	require.NoError(t, afero.WriteFile(baseFs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("{}"), 0o644))

	fs := &testutil.FailingRemoveFs{Fs: baseFs, FailSuffix: schema.ManifestExtension}

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	setID := [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			return &par2.File{
				Sets: []par2.Set{
					{MainPacket: &par2.MainPacket{SetID: setID}},
				},
			}, nil
		},
	}

	bundler := &testutil.MockBundleHandler{
		PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
			require.NoError(t, afero.WriteFile(baseFs, bundlePath, []byte("bundledata"), 0o644))

			return nil
		},
	}

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, bundler, par2er)

	mf := schema.NewManifest("test" + schema.Par2Extension)
	mf.Creation = schema.NewCreationManifest()

	job := &Job{
		workingDir:   "/data/folder",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
		asBundle:     true,
	}

	require.NoError(t, prog.packAsBundle(t.Context(), job, mf))
	require.Contains(t, logBuf.String(), "Failed to cleanup a file after bundling")

	// The manifest file that failed to remove should still exist.
	manifestExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension)
	require.True(t, manifestExists)

	// The PAR2 index that succeeded removal should be gone.
	indexExists, _ := afero.Exists(fs, "/data/folder/test"+schema.Par2Extension)
	require.False(t, indexExists)

	// Bundle should still exist.
	bundleExists, _ := afero.Exists(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension)
	require.True(t, bundleExists)
}

// Expectation: The function should find only PAR2 index and volume files matching the base name.
func Test_Service_findBundleableFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		par2Name:   "test" + schema.Par2Extension,
		par2Path:   "/data/folder/test" + schema.Par2Extension,
	}

	files, err := prog.findBundleableFiles(job)
	require.NoError(t, err)
	require.Len(t, files, 3)

	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	require.Contains(t, names, "test"+schema.Par2Extension)
	require.Contains(t, names, "test.vol00+01"+schema.Par2Extension)
	require.Contains(t, names, "test.vol01+02"+schema.Par2Extension)
}

// Expectation: The function should not include files that do not match the base name prefix.
func Test_Service_findBundleableFiles_IgnoresUnrelatedFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/other"+schema.Par2Extension, []byte("other"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test1"+schema.Par2Extension, []byte("content"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("manifest"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("lock"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		par2Name:   "test" + schema.Par2Extension,
		par2Path:   "/data/folder/test" + schema.Par2Extension,
	}

	files, err := prog.findBundleableFiles(job)
	require.NoError(t, err)
	require.Len(t, files, 2)

	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	require.Contains(t, names, "test"+schema.Par2Extension)
	require.Contains(t, names, "test.vol00+01"+schema.Par2Extension)
	require.NotContains(t, names, "other"+schema.Par2Extension)
	require.NotContains(t, names, "test1"+schema.Par2Extension)
}

// Expectation: The function should not include directories.
func Test_Service_findBundleableFiles_IgnoresDirectories_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/test.subdir", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("index"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		par2Name:   "test" + schema.Par2Extension,
		par2Path:   "/data/folder/test" + schema.Par2Extension,
	}

	files, err := prog.findBundleableFiles(job)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "test"+schema.Par2Extension, files[0].Name)
}

// Expectation: The function should not include files that already have the bundle extension.
func Test_Service_findBundleableFiles_IgnoresBundleFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("bundle"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		par2Name:   "test" + schema.Par2Extension,
		par2Path:   "/data/folder/test" + schema.Par2Extension,
	}

	files, err := prog.findBundleableFiles(job)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "test"+schema.Par2Extension, files[0].Name)
}

// Expectation: The function should return an error when no PAR2 files match.
func Test_Service_findBundleableFiles_NoMatches_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/other.txt", []byte("content"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		par2Name:   "test" + schema.Par2Extension,
		par2Path:   "/data/folder/test" + schema.Par2Extension,
	}

	files, err := prog.findBundleableFiles(job)
	require.ErrorContains(t, err, "no files to bundle")
	require.Nil(t, files)
}

// Expectation: The function should return correct file paths.
func Test_Service_findBundleableFiles_CorrectPaths_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		par2Name:   "test" + schema.Par2Extension,
		par2Path:   "/data/folder/test" + schema.Par2Extension,
	}

	files, err := prog.findBundleableFiles(job)
	require.NoError(t, err)

	for _, f := range files {
		require.True(t, strings.HasPrefix(f.Path, "/data/folder/"))
		require.Equal(t, filepath.Join("/data/folder", f.Name), f.Path)
	}
}

// Expectation: The function should work with hidden file names (dot prefix).
func Test_Service_findBundleableFiles_HiddenFiles_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/.test"+schema.Par2Extension, []byte("index"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/.test.vol00+01"+schema.Par2Extension, []byte("vol1"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("other"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir: "/data/folder",
		par2Name:   ".test" + schema.Par2Extension,
		par2Path:   "/data/folder/.test" + schema.Par2Extension,
	}

	files, err := prog.findBundleableFiles(job)
	require.NoError(t, err)
	require.Len(t, files, 2)

	names := make([]string, 0, len(files))
	for _, f := range files {
		names = append(names, f.Name)
	}
	require.Contains(t, names, ".test"+schema.Par2Extension)
	require.Contains(t, names, ".test.vol00+01"+schema.Par2Extension)
	require.NotContains(t, names, "test"+schema.Par2Extension)
}
