package create

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: All relevant files should be removed, but others not.
func Test_Service_cleanupAfterFailure_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.BundleExtension+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing"+schema.Par2Extension, []byte("par2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	prog.cleanupAfterFailure(t.Context(), job)

	for _, tt := range []struct {
		path   string
		exists bool
	}{
		{"/data/folder/test" + schema.Par2Extension, false},
		{"/data/folder/test" + schema.BundleExtension + schema.Par2Extension, false},
		{"/data/folder/test" + schema.Par2Extension + schema.ManifestExtension, false},
		{"/data/folder/test" + schema.Par2Extension + schema.LockExtension, false},
		{"/data/folder/test.vol01+02" + schema.Par2Extension, false},
		{"/data/folder/existing" + schema.Par2Extension, true},
	} {
		exists, _ := afero.Exists(fs, tt.path)
		require.Equal(t, tt.exists, exists, tt.path)
	}
}

// Expectation: Cleanup should not touch unrelated files or directories.
func Test_Service_cleanupAfterFailure_EdgeCases_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, fs.MkdirAll("/data/folder/test", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test2"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.txt", []byte("text"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/unrelated.vol01+02"+schema.Par2Extension, []byte("vol"), 0o644))

	var logBuf testutil.SafeBuffer

	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}

	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	prog.cleanupAfterFailure(t.Context(), job)

	for _, tt := range []struct {
		path   string
		exists bool
	}{
		{"/data/folder/test" + schema.Par2Extension, false},
		{"/data/folder/test" + schema.Par2Extension + schema.ManifestExtension, false},
		{"/data/folder/test" + schema.Par2Extension + schema.LockExtension, false},
		{"/data/folder/test2" + schema.Par2Extension, true},
		{"/data/folder/test.txt", true},
		{"/data/folder/unrelated.vol01+02" + schema.Par2Extension, true},
	} {
		exists, _ := afero.Exists(fs, tt.path)
		require.Equal(t, tt.exists, exists, tt.path)
	}

	// Subdirectory with a prefix-matching name should not be removed.
	info, err := fs.Stat("/data/folder/test")
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

// Expectation: Non-failing files should be removed regardless of failure.
func Test_Service_cleanupAfterFailure_OneFails_Error(t *testing.T) {
	t.Parallel()

	fs := &testutil.FailingRemoveFs{Fs: afero.NewMemMapFs(), FailSuffix: schema.LockExtension}

	require.NoError(t, fs.MkdirAll("/data/folder", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.ManifestExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test"+schema.Par2Extension+schema.LockExtension, []byte("par2"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/test.vol01+02"+schema.Par2Extension, []byte("vol"), 0o644))
	require.NoError(t, afero.WriteFile(fs, "/data/folder/existing"+schema.Par2Extension, []byte("par2"), 0o644))

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("info")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	job := &Job{
		workingDir:   "/data/folder",
		markerPath:   "/data/folder/_par2cron",
		par2Name:     "test" + schema.Par2Extension,
		par2Path:     "/data/folder/test" + schema.Par2Extension,
		par2Args:     []string{"-r10"},
		par2Glob:     "*",
		lockPath:     "/data/folder/test" + schema.Par2Extension + schema.LockExtension,
		manifestName: "test" + schema.Par2Extension + schema.ManifestExtension,
		manifestPath: "/data/folder/test" + schema.Par2Extension + schema.ManifestExtension,
	}

	prog.cleanupAfterFailure(t.Context(), job)

	for _, tt := range []struct {
		path   string
		exists bool
	}{
		{"/data/folder/test" + schema.Par2Extension, false},
		{"/data/folder/test" + schema.Par2Extension + schema.ManifestExtension, false},
		{"/data/folder/test" + schema.Par2Extension + schema.LockExtension, true},
		{"/data/folder/test.vol01+02" + schema.Par2Extension, false},
		{"/data/folder/existing" + schema.Par2Extension, true},
	} {
		exists, _ := afero.Exists(fs, tt.path)
		require.Equal(t, tt.exists, exists, tt.path)
	}

	require.Contains(t, logBuf.String(), "Failed to cleanup a file after failure")
}

// Expectation: An error should be returned when -R is in args but mode is not recursive.
func Test_Service_considerRecursive_HasRArgButNotRecursiveMode_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	opts := &Options{
		Par2Args: []string{"-r10", "-R"},
	}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateFileMode))

	err := prog.considerRecursive(opts)

	require.ErrorIs(t, err, errWrongModeArgument)
	require.Contains(t, logBuf.String(), "par2 default argument -R needs par2cron default --mode recursive")
}

// Expectation: The -R argument should be added when mode is recursive but -R is not in args.
func Test_Service_considerRecursive_RecursiveModeButNoRArg_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	opts := &Options{
		Par2Args: []string{"-r10", "-n3"},
	}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateRecursiveMode))

	err := prog.considerRecursive(opts)

	require.NoError(t, err)
	require.Contains(t, opts.Par2Args, "-R")
	require.Contains(t, logBuf.String(), "Adding -R to par2 default arguments (due to --mode recursive)")
}

// Expectation: No changes should be made when mode is recursive and -R is already present.
func Test_Service_considerRecursive_RecursiveModeWithRArg_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	opts := &Options{
		Par2Args: []string{"-r10", "-R"},
	}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateRecursiveMode))

	err := prog.considerRecursive(opts)

	require.NoError(t, err)
	require.Len(t, opts.Par2Args, 2)
	require.Equal(t, "-r10", opts.Par2Args[0])
	require.Equal(t, "-R", opts.Par2Args[1])
}

// Expectation: No changes should be made when mode is file and -R is not present.
func Test_Service_considerRecursive_FileModeWithoutRArg_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	opts := &Options{
		Par2Args: []string{"-r10", "-n3"},
	}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateFileMode))

	err := prog.considerRecursive(opts)

	require.NoError(t, err)
	require.Len(t, opts.Par2Args, 2)
	require.NotContains(t, opts.Par2Args, "-R")
}

// Expectation: No changes should be made when mode is folder and -R is not present.
func Test_Service_considerRecursive_FolderModeWithoutRArg_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()

	var logBuf testutil.SafeBuffer
	ls := logging.Options{
		Logout: &logBuf,
		Stdout: io.Discard,
		Stderr: io.Discard,
	}
	_ = ls.LogLevel.Set("debug")

	prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

	opts := &Options{
		Par2Args: []string{"-r10", "-n3"},
	}
	require.NoError(t, opts.Par2Mode.Set(schema.CreateFolderMode))

	err := prog.considerRecursive(opts)

	require.NoError(t, err)
	require.Len(t, opts.Par2Args, 2)
	require.NotContains(t, opts.Par2Args, "-R")
}

// Expectation: The function should detect existing PAR2 files in all naming variants.
func Test_Service_par2AlreadyExists_Table(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		existingFile  string
		par2Name      string
		markerPersist bool
		expected      bool
	}{
		{
			name:         "plain par2 name detects plain existing",
			existingFile: "/data/folder/test" + schema.Par2Extension,
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "plain par2 name detects hidden existing",
			existingFile: "/data/folder/.test" + schema.Par2Extension,
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "plain par2 name detects bundle existing",
			existingFile: "/data/folder/test" + schema.BundleExtension + schema.Par2Extension,
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "plain par2 name detects hidden bundle existing",
			existingFile: "/data/folder/.test" + schema.BundleExtension + schema.Par2Extension,
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "hidden par2 name detects plain existing",
			existingFile: "/data/folder/test" + schema.Par2Extension,
			par2Name:     ".test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "hidden par2 name detects hidden existing",
			existingFile: "/data/folder/.test" + schema.Par2Extension,
			par2Name:     ".test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "hidden par2 name detects bundle existing",
			existingFile: "/data/folder/test" + schema.BundleExtension + schema.Par2Extension,
			par2Name:     ".test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:          "hidden par2 name detects hidden bundle existing",
			existingFile:  "/data/folder/.test" + schema.BundleExtension + schema.Par2Extension,
			par2Name:      ".test" + schema.Par2Extension,
			markerPersist: true,
			expected:      true,
		},
		{
			name:         "uppercase par2 name detects plain existing",
			existingFile: "/data/folder/test" + schema.Par2Extension,
			par2Name:     "test" + strings.ToUpper(schema.Par2Extension),
			expected:     true,
		},
		{
			name:         "uppercase par2 name detects hidden existing",
			existingFile: "/data/folder/.test" + schema.Par2Extension,
			par2Name:     "test" + strings.ToUpper(schema.Par2Extension),
			expected:     true,
		},
		{
			name:         "uppercase par2 name detects bundle existing",
			existingFile: "/data/folder/test" + schema.BundleExtension + schema.Par2Extension,
			par2Name:     "test" + strings.ToUpper(schema.Par2Extension),
			expected:     true,
		},
		{
			name:         "uppercase par2 name detects hidden bundle existing",
			existingFile: "/data/folder/.test" + schema.BundleExtension + schema.Par2Extension,
			par2Name:     "test" + strings.ToUpper(schema.Par2Extension),
			expected:     true,
		},
		{
			name:         "plain par2 name detects uppercase existing",
			existingFile: "/data/folder/test" + strings.ToUpper(schema.Par2Extension),
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "plain par2 name detects hidden uppercase existing",
			existingFile: "/data/folder/.test" + strings.ToUpper(schema.Par2Extension),
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "plain par2 name detects uppercase bundle existing",
			existingFile: "/data/folder/test" + schema.BundleExtension + strings.ToUpper(schema.Par2Extension),
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "plain par2 name detects hidden uppercase bundle existing",
			existingFile: "/data/folder/.test" + schema.BundleExtension + strings.ToUpper(schema.Par2Extension),
			par2Name:     "test" + schema.Par2Extension,
			expected:     true,
		},
		{
			name:         "no par2 exists",
			existingFile: "",
			par2Name:     "test" + schema.Par2Extension,
			expected:     false,
		},
		{
			name:         "unrelated file exists",
			existingFile: "/data/folder/test.txt",
			par2Name:     "test" + schema.Par2Extension,
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fs := afero.NewMemMapFs()
			require.NoError(t, fs.MkdirAll("/data/folder", 0o755))

			if tt.existingFile != "" {
				require.NoError(t, afero.WriteFile(fs, tt.existingFile, []byte("existing"), 0o644))
			}

			var logBuf testutil.SafeBuffer
			ls := logging.Options{
				Logout: &logBuf,
				Stdout: io.Discard,
				Stderr: io.Discard,
			}
			_ = ls.LogLevel.Set("debug")

			prog := NewService(fs, logging.NewLogger(ls), &testutil.MockRunner{}, &util.BundleHandler{}, &util.Par2Handler{})

			job := &Job{
				workingDir:    "/data/folder",
				par2Name:      tt.par2Name,
				par2Path:      filepath.Join("/data/folder", tt.par2Name),
				markerPersist: tt.markerPersist,
			}

			result := prog.par2AlreadyExists(t.Context(), job)

			require.Equal(t, tt.expected, result)

			if tt.expected {
				require.Contains(t, logBuf.String(), "Same-named PAR2 already exists in folder")
			} else {
				require.NotContains(t, logBuf.String(), "Same-named PAR2 already exists")
			}
		})
	}
}
