package tool

import (
	"errors"
	"strings"
	"testing"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: OutputMD5 should print hash and base filename for a single file in the recovery set.
func Test_OutputMD5_SingleFile_SingleSet_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/data/test.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{
							RecoverySet: []par2.FilePacket{
								{
									FileID: par2.Hash{0x01},
									Hash:   par2.Hash{0xaa, 0xbb, 0xcc},
									Name:   "hello.txt",
								},
							},
						},
					},
				}, nil
			},
		},
	)

	require.NoError(t, err)
	require.Contains(t, stdout.String(), "hello.txt")
	require.Contains(t, stdout.String(), "aabbcc")
	require.Empty(t, stderr.String())
}

// Expectation: OutputMD5 should print multiple distinct files from the recovery set.
func Test_OutputMD5_MultipleFiles_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/test.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{
							RecoverySet: []par2.FilePacket{
								{FileID: par2.Hash{0x01}, Hash: par2.Hash{0xaa}, Name: "one.txt"},
								{FileID: par2.Hash{0x02}, Hash: par2.Hash{0xbb}, Name: "two.txt"},
								{FileID: par2.Hash{0x03}, Hash: par2.Hash{0xcc}, Name: "three.txt"},
							},
						},
					},
				}, nil
			},
		},
	)

	require.NoError(t, err)

	output := stdout.String()
	require.Contains(t, output, "one.txt")
	require.Contains(t, output, "two.txt")
	require.Contains(t, output, "three.txt")
}

// Expectation: OutputMD5 should handle multiple sets within a single par2 file.
func Test_OutputMD5_MultipleSets_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/test.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{
							RecoverySet: []par2.FilePacket{
								{FileID: par2.Hash{0x01}, Hash: par2.Hash{0xaa}, Name: "set1.txt"},
							},
						},
						{
							RecoverySet: []par2.FilePacket{
								{FileID: par2.Hash{0x02}, Hash: par2.Hash{0xbb}, Name: "set2.txt"},
							},
						},
					},
				}, nil
			},
		},
	)

	require.NoError(t, err)

	output := stdout.String()
	require.Contains(t, output, "set1.txt")
	require.Contains(t, output, "set2.txt")
}

// Expectation: OutputMD5 should deduplicate files with the same FileID across sets.
func Test_OutputMD5_DuplicateFileID_SameFile_Deduplicated_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	fp := par2.FilePacket{
		FileID: par2.Hash{0x01},
		Hash:   par2.Hash{0xaa},
		Name:   "dup.txt",
	}

	err := OutputMD5(
		[]string{"/test.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{RecoverySet: []par2.FilePacket{fp}},
						{RecoverySet: []par2.FilePacket{fp}},
					},
				}, nil
			},
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(stdout.String(), "dup.txt"))
}

// Expectation: OutputMD5 should deduplicate files with the same FileID across multiple input paths.
func Test_OutputMD5_DuplicateFileID_AcrossPaths_Deduplicated_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	fp := par2.FilePacket{
		FileID: par2.Hash{0x01},
		Hash:   par2.Hash{0xaa},
		Name:   "shared.txt",
	}

	err := OutputMD5(
		[]string{"/a.par2", "/b.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{RecoverySet: []par2.FilePacket{fp}},
					},
				}, nil
			},
		},
	)

	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(stdout.String(), "shared.txt"))
}

// Expectation: OutputMD5 should return ErrExitPartialFailure when a file fails to parse.
func Test_OutputMD5_ParseError_SingleFile_Error(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/bad.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return nil, errors.New("corrupt par2")
			},
		},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.Contains(t, err.Error(), "1 files failed to parse")
}

// Expectation: OutputMD5 should write the error details to stderr when parsing fails.
func Test_OutputMD5_ParseError_WritesStderr_Error(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	_ = OutputMD5(
		[]string{"/bad.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return nil, errors.New("corrupt par2")
			},
		},
	)

	require.Contains(t, stderr.String(), "/bad.par2")
	require.Contains(t, stderr.String(), "corrupt par2")
	require.Empty(t, stdout.String())
}

// Expectation: OutputMD5 should count all failures and continue processing remaining paths.
func Test_OutputMD5_ParseError_MultiplePaths_ContinuesProcessing_Error(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/bad1.par2", "/good.par2", "/bad2.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				if path == "/good.par2" {
					return &par2.File{
						Sets: []par2.Set{
							{
								RecoverySet: []par2.FilePacket{
									{FileID: par2.Hash{0x01}, Hash: par2.Hash{0xaa}, Name: "ok.txt"},
								},
							},
						},
					}, nil
				}

				return nil, errors.New("parse failed")
			},
		},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.Contains(t, err.Error(), "2 files failed to parse")
	require.Contains(t, stdout.String(), "ok.txt")
	require.Contains(t, stderr.String(), "/bad1.par2")
	require.Contains(t, stderr.String(), "/bad2.par2")
}

// Expectation: OutputMD5 should pass panicAsErr=false to ParseFile.
func Test_OutputMD5_ParseFile_PanicAsErrFalse_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	var capturedPanicAsErr bool

	err := OutputMD5(
		[]string{"/test.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				capturedPanicAsErr = panicAsErr

				return &par2.File{}, nil
			},
		},
	)

	require.NoError(t, err)
	require.False(t, capturedPanicAsErr)
}

// Expectation: OutputMD5 should return nil for an empty paths slice.
func Test_OutputMD5_EmptyPaths_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{},
	)

	require.NoError(t, err)
	require.Empty(t, stdout.String())
	require.Empty(t, stderr.String())
}

// Expectation: OutputMD5 should return nil when par2 files have no sets.
func Test_OutputMD5_NoSets_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/empty.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{}, nil
			},
		},
	)

	require.NoError(t, err)
	require.Empty(t, stdout.String())
}

// Expectation: OutputMD5 should return nil when the recovery set is empty.
func Test_OutputMD5_EmptyRecoverySet_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/test.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{RecoverySet: []par2.FilePacket{}},
					},
				}, nil
			},
		},
	)

	require.NoError(t, err)
	require.Empty(t, stdout.String())
}

// Expectation: OutputMD5 should format output as "<hex_hash>  <basename>\n".
func Test_OutputMD5_OutputFormat_Success(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/test.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{
							RecoverySet: []par2.FilePacket{
								{
									FileID: par2.Hash{0x01},
									Hash:   par2.Hash{0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0xcd, 0xef, 0x01, 0x23, 0x45, 0x67, 0x89},
									Name:   "test.bin",
								},
							},
						},
					},
				}, nil
			},
		},
	)

	require.NoError(t, err)
	require.Equal(t, "abcdef0123456789abcdef0123456789  test.bin\n", stdout.String())
}

// Expectation: OutputMD5 should return error with correct count when all files fail to parse.
func Test_OutputMD5_AllFilesFail_Error(t *testing.T) {
	t.Parallel()

	stdout := &testutil.SafeBuffer{}
	stderr := &testutil.SafeBuffer{}

	err := OutputMD5(
		[]string{"/a.par2", "/b.par2", "/c.par2"},
		afero.NewMemMapFs(),
		stdout,
		stderr,
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return nil, errors.New("failed")
			},
		},
	)

	require.Error(t, err)
	require.ErrorIs(t, err, schema.ErrExitPartialFailure)
	require.Contains(t, err.Error(), "3 files failed to parse")
	require.Empty(t, stdout.String())
}
