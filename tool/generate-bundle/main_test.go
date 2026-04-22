package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type failingMkdirFs struct {
	afero.Fs
}

func (f *failingMkdirFs) MkdirAll(path string, perm os.FileMode) error {
	return errors.New("mkdir boom")
}

type errWriter struct{}

func (w errWriter) Write(p []byte) (int, error) {
	return 0, errors.New("writer boom")
}

// Expectation: parseArgs should fail when -parse is missing.
func Test_parseArgs_RequiresParseFlag_Error(t *testing.T) {
	t.Parallel()

	_, err := parseArgs([]string{"file.par2"})

	require.ErrorContains(t, err, "args error")
	require.ErrorContains(t, err, "-parse flag is required")
}

// Expectation: parseArgs should fail when no input files are provided.
func Test_parseArgs_RequiresInputFiles_Error(t *testing.T) {
	t.Parallel()

	_, err := parseArgs([]string{"-parse", "index.par2"})

	require.ErrorContains(t, err, "args error")
	require.ErrorContains(t, err, "at least one input file must be given")
}

// Expectation: parseArgs should return an args-prefixed error when flag parsing fails.
func Test_parseArgs_FlagParseError_Error(t *testing.T) {
	t.Parallel()

	_, err := parseArgs([]string{"-does-not-exist"})

	require.ErrorContains(t, err, "args error")
	require.ErrorContains(t, err, "flag provided but not defined")
}

// Expectation: Service.Run should return a parse-prefixed error when ParseFile fails.
func Test_Service_Run_ParseFails_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return nil, errors.New("parse boom")
			},
		},
		&testutil.MockBundleHandler{},
	)

	_, err := service.Run(Options{
		Dir:   "/data",
		Out:   "out.par2",
		Parse: "index.par2",
		Files: []string{"files.par2"},
	})

	require.ErrorContains(t, err, "parse error")
	require.ErrorContains(t, err, "parse boom")
}

// Expectation: Service.Run should fail when parsed PAR2 has no usable main packet.
func Test_Service_Run_NoMainPacket_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{{MainPacket: nil}},
				}, nil
			},
		},
		&testutil.MockBundleHandler{},
	)

	_, err := service.Run(Options{
		Dir:   "/data",
		Out:   "out.par2",
		Parse: "index.par2",
		Files: []string{"files.par2"},
	})

	require.ErrorContains(t, err, "parsed file has no sets or main packet")
}

// Expectation: Service.Run should return an fs-prefixed error when output directory creation fails.
func Test_Service_Run_MkdirFails_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		&failingMkdirFs{Fs: afero.NewMemMapFs()},
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{MainPacket: &par2.MainPacket{SetID: [16]byte{1}}},
					},
				}, nil
			},
		},
		&testutil.MockBundleHandler{},
	)

	_, err := service.Run(Options{
		Dir:   "/data",
		Out:   "generated/out.par2",
		Parse: "index.par2",
		Files: []string{"files.par2"},
	})

	require.ErrorContains(t, err, "fs error")
	require.ErrorContains(t, err, "mkdir boom")
}

// Expectation: Service.Run should return a pack-prefixed error when bundle pack fails.
func Test_Service_Run_PackFails_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{MainPacket: &par2.MainPacket{SetID: [16]byte{1}}},
					},
				}, nil
			},
		},
		&testutil.MockBundleHandler{
			PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
				return errors.New("pack boom")
			},
		},
	)

	_, err := service.Run(Options{
		Dir:   "/data",
		Out:   "out.par2",
		Parse: "index.par2",
		Files: []string{"files.par2"},
	})

	require.ErrorContains(t, err, "pack error")
	require.ErrorContains(t, err, "pack boom")
}

// Expectation: Service.Run should return an open-prefixed error when bundle open fails.
func Test_Service_Run_BundleOpenFails_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{MainPacket: &par2.MainPacket{SetID: [16]byte{1}}},
					},
				}, nil
			},
		},
		&testutil.MockBundleHandler{
			PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, mf bundle.ManifestInput, files []bundle.FileInput) error {
				return nil
			},
			OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
				return nil, errors.New("open boom")
			},
		},
	)

	_, err := service.Run(Options{
		Dir:   "/data",
		Out:   "out.par2",
		Parse: "index.par2",
		Files: []string{"files.par2"},
	})

	require.ErrorContains(t, err, "bundle open error")
	require.ErrorContains(t, err, "open boom")
}

// Expectation: Service.Run should return an args-prefixed error when Options validation fails.
func Test_Service_Run_OptionsValidateFails_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{},
		&testutil.MockBundleHandler{},
	)

	_, err := service.Run(Options{
		Dir:   "/data",
		Out:   "out.par2",
		Parse: "", // invalid: required
		Files: []string{"files.par2"},
	})

	require.ErrorContains(t, err, "args error")
	require.ErrorContains(t, err, "-parse flag is required")
}

// Expectation: Service.Run should return a validate-prefixed error and close the bundle on validate failure.
func Test_Service_Run_ValidateFails_Error(t *testing.T) {
	t.Parallel()

	var closeCalled bool
	mockBundle := &testutil.MockBundle{
		ValidateFunc: func(strict bool) error {
			require.True(t, strict)

			return errors.New("validate boom")
		},
		CloseFunc: func() error {
			closeCalled = true

			return nil
		},
	}

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{MainPacket: &par2.MainPacket{SetID: [16]byte{1}}},
					},
				}, nil
			},
		},
		&testutil.MockBundleHandler{
			PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, manifest bundle.ManifestInput, files []bundle.FileInput) error {
				return nil
			},
			OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
				return mockBundle, nil
			},
		},
	)

	_, err := service.Run(Options{
		Dir:   "/data",
		Out:   "out.par2",
		Parse: "index.par2",
		Files: []string{"files.par2"},
	})

	require.ErrorContains(t, err, "bundle validate error")
	require.ErrorContains(t, err, "validate boom")
	require.True(t, closeCalled)
}

// Expectation: run should print a success line when bundle generation succeeds.
func Test_run_Success(t *testing.T) {
	t.Parallel()

	var parseCalled bool
	var packCalled bool
	var openCalled bool
	var closeCalled bool

	mockBundle := &testutil.MockBundle{
		ValidateFunc: func(strict bool) error {
			require.True(t, strict)

			return nil
		},
		CloseFunc: func() error {
			closeCalled = true

			return nil
		},
	}

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				parseCalled = true
				require.Equal(t, filepath.Join("/data", "index.par2"), path)

				return &par2.File{
					Sets: []par2.Set{
						{MainPacket: &par2.MainPacket{SetID: [16]byte{1, 2, 3}}},
					},
				}, nil
			},
		},
		&testutil.MockBundleHandler{
			PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, mf bundle.ManifestInput, files []bundle.FileInput) error {
				packCalled = true
				require.Equal(t, filepath.Join("/data", "generated", "out.par2"), bundlePath)
				require.Equal(t, []bundle.FileInput{
					{Name: "files.par2", Path: filepath.Join("/data", "files.par2")},
					{Name: "files.vol00+01.par2", Path: filepath.Join("/data", "files.vol00+01.par2")},
				}, files)

				return nil
			},
			OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
				openCalled = true
				require.Equal(t, filepath.Join("/data", "generated", "out.par2"), bundlePath)

				return mockBundle, nil
			},
		},
	)

	var out bytes.Buffer
	err := run([]string{
		"-dir", "/data",
		"-out", "generated/out.par2",
		"-parse", "index.par2",
		"files.par2",
		"files.vol00+01.par2",
	}, &out, service)
	require.NoError(t, err)

	require.True(t, parseCalled)
	require.True(t, packCalled)
	require.True(t, openCalled)
	require.True(t, closeCalled)
	require.Contains(t, out.String(), programName+": success: "+filepath.Join("/data", "generated", "out.par2"))
}

// Expectation: run should return an args-prefixed error when parseArgs fails.
func Test_run_ParseArgsError_Error(t *testing.T) {
	t.Parallel()

	var parseCalled bool
	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				parseCalled = true

				return nil, nil //nolint:nilnil
			},
		},
		&testutil.MockBundleHandler{},
	)

	err := run([]string{"files.par2"}, &bytes.Buffer{}, service)
	require.ErrorContains(t, err, "args error")
	require.ErrorContains(t, err, "-parse flag is required")
	require.False(t, parseCalled)
}

// Expectation: run should return service errors as-is when Service.Run fails.
func Test_run_ServiceRunError_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return nil, errors.New("parse boom")
			},
		},
		&testutil.MockBundleHandler{},
	)

	err := run([]string{"-dir", "/data", "-parse", "index.par2", "files.par2"}, &bytes.Buffer{}, service)
	require.ErrorContains(t, err, "parse error")
	require.ErrorContains(t, err, "parse boom")
}

// Expectation: run should return an output-prefixed error when writing success output fails.
func Test_run_FprintfError_Error(t *testing.T) {
	t.Parallel()

	service := NewService(
		afero.NewMemMapFs(),
		&testutil.MockPar2Handler{
			ParseFileFunc: func(fsys afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
				return &par2.File{
					Sets: []par2.Set{
						{MainPacket: &par2.MainPacket{SetID: [16]byte{1}}},
					},
				}, nil
			},
		},
		&testutil.MockBundleHandler{
			PackFunc: func(fsys afero.Fs, bundlePath string, recoverySetID [16]byte, mf bundle.ManifestInput, files []bundle.FileInput) error {
				return nil
			},
			OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
				return &testutil.MockBundle{}, nil
			},
		},
	)

	err := run([]string{"-dir", "/data", "-parse", "index.par2", "files.par2"}, errWriter{}, service)
	require.ErrorContains(t, err, "failed to write output")
	require.ErrorContains(t, err, "writer boom")
}
