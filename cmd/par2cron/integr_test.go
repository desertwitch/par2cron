package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/util"
	"github.com/stretchr/testify/require"
)

func setupTestDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "_par2cron"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "testfile.txt"), []byte("hello world\n"), 0o600))

	return dir
}

// Expectation: A command should write a CPU profile when --pprof is set.
//
//nolint:paralleltest
func Test_Integration_RootCmd_pprof_Success(t *testing.T) {
	dir := setupTestDir(t)
	profPath := filepath.Join(t.TempDir(), "cpu.prof")

	cmd := newRootCmd(t.Context())
	cmd.SetArgs([]string{"--pprof", profPath, "create", dir})
	require.NoError(t, cmd.Execute())

	_, err := os.Stat(profPath)
	require.NoError(t, err)
}

// Expectation: The "create" command should create PAR2 files in a directory with a marker file.
//
//nolint:paralleltest
func Test_Integration_CreateCmd_Success(t *testing.T) {
	dir := setupTestDir(t)

	cmd := newRootCmd(t.Context())
	cmd.SetArgs([]string{"create", dir})

	require.NoError(t, cmd.Execute())

	matches, err := filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	require.NotEmpty(t, matches, "expected par2 files to be created")
}

// Expectation: The "create" command should create PAR2 files in a directory with a marker file.
//
//nolint:paralleltest
func Test_Integration_CreateCmd_CustomArgs_Success(t *testing.T) {
	dir := setupTestDir(t)

	cmd := newRootCmd(t.Context())
	cmd.SetArgs([]string{"create", dir, "--", "-r15", "-n1"})

	require.NoError(t, cmd.Execute())

	matches, err := filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	require.NotEmpty(t, matches, "expected par2 files to be created")
}

// Expectation: The "verify" command should verify a previously created PAR2 set without error.
//
//nolint:paralleltest
func Test_Integration_VerifyCmd_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir})
	require.NoError(t, createCmd.Execute())

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.NoError(t, verifyCmd.Execute())
}

// Expectation: The "verify" command should verify a previously created PAR2 set without error.
//
//nolint:paralleltest
func Test_Integration_VerifyCmd_CustomArgs_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir, "--", "-v"})
	require.NoError(t, createCmd.Execute())

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.NoError(t, verifyCmd.Execute())
}

// Expectation: The "repair" command should restore a corrupted file to its original content.
//
//nolint:paralleltest
func Test_Integration_RepairCmd_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir})
	require.NoError(t, createCmd.Execute())

	dataFile := filepath.Join(dir, "testfile.txt")
	require.NoError(t, os.WriteFile(dataFile, []byte("yello world\n"), 0o600))

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.ErrorIs(t, verifyCmd.Execute(), schema.ErrExitRepairable)

	repairCmd := newRootCmd(t.Context())
	repairCmd.SetArgs([]string{"repair", dir})
	require.NoError(t, repairCmd.Execute())

	restored, err := os.ReadFile(dataFile)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(restored))
}

// Expectation: The "repair" command should restore a corrupted file to its original content.
//
//nolint:paralleltest
func Test_Integration_RepairCmd_CustomArgs_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir, "--", "-v"})
	require.NoError(t, createCmd.Execute())

	dataFile := filepath.Join(dir, "testfile.txt")
	require.NoError(t, os.WriteFile(dataFile, []byte("yello world\n"), 0o600))

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.ErrorIs(t, verifyCmd.Execute(), schema.ErrExitRepairable)

	repairCmd := newRootCmd(t.Context())
	repairCmd.SetArgs([]string{"repair", dir})
	require.NoError(t, repairCmd.Execute())

	restored, err := os.ReadFile(dataFile)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(restored))
}

// Expectation: The "info" command should report on a previously created PAR2 set without error.
//
//nolint:paralleltest
func Test_Integration_InfoCmd_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir})
	require.NoError(t, createCmd.Execute())

	infoCmd := newRootCmd(t.Context())
	infoCmd.SetArgs([]string{"info", dir})
	require.NoError(t, infoCmd.Execute())
}

// Expectation: The "check-config" command should accept a valid configuration file.
//
//nolint:paralleltest
func Test_Integration_CheckConfigCmd_Success(t *testing.T) {
	yamlContent := `create:
  mode: "nested"
  glob: "**/*.mp4"`

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(yamlContent), 0o600))

	cmd := newRootCmd(t.Context())
	cmd.SetArgs([]string{"check-config", cfgFile})
	require.NoError(t, cmd.Execute())
}

// Expectation: The "check-config" command should reject an invalid configuration file.
//
//nolint:paralleltest
func Test_Integration_CheckConfigCmd_InvalidConfig_Error(t *testing.T) {
	yamlContent := `create:
  mode: "nested"
  glob: **/*.mp4"`

	cfgFile := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(cfgFile, []byte(yamlContent), 0o600))

	cmd := newRootCmd(t.Context())
	cmd.SetArgs([]string{"check-config", cfgFile})
	require.ErrorIs(t, cmd.Execute(), schema.ErrExitBadInvocation)
}

// Expectation: The usual cron layout (create, verify, repair) should work as expected.
//
//nolint:paralleltest
func Test_Integration_Cron_Complex_Success(t *testing.T) {
	dir := t.TempDir()
	dir1 := filepath.Join(dir, "data1")
	dir2 := filepath.Join(dir, "data2")

	require.NoError(t, os.MkdirAll(dir1, 0o750))
	require.NoError(t, os.MkdirAll(dir2, 0o750))

	require.NoError(t, os.WriteFile(filepath.Join(dir1, "_par2cron"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir1, "testfile1.txt"), []byte("hello world\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "_par2cron"), nil, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir2, "testfile2.txt"), []byte("hello world\n"), 0o600))

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir1, dir2, "--", "-r15", "-n1"})
	require.NoError(t, createCmd.Execute())

	matches, err := filepath.Glob(filepath.Join(dir1, "*.par2"))
	require.NoError(t, err)
	require.NotEmpty(t, matches, "expected par2 files to be created")

	matches, err = filepath.Glob(filepath.Join(dir2, "*.par2"))
	require.NoError(t, err)
	require.NotEmpty(t, matches, "expected par2 files to be created")

	dataFile1 := filepath.Join(dir1, "testfile1.txt")
	require.NoError(t, os.WriteFile(dataFile1, []byte("yello world\n"), 0o600))

	dataFile2 := filepath.Join(dir2, "testfile2.txt")
	require.NoError(t, os.WriteFile(dataFile2, []byte("UNREPAIRABLE\n"), 0o600))

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir1, dir2, "--", "-v"})
	require.ErrorIs(t, verifyCmd.Execute(), schema.ErrExitUnrepairable)

	repairCmd := newRootCmd(t.Context())
	repairCmd.SetArgs([]string{"repair", dir1, dir2, "--", "-v"})
	require.NoError(t, repairCmd.Execute())

	restored, err := os.ReadFile(dataFile1)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(restored))

	unrecoverable, err := os.ReadFile(dataFile2)
	require.NoError(t, err)
	require.Equal(t, "UNREPAIRABLE\n", string(unrecoverable))

	verifyCmd = newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir1, "--", "-v"})
	require.NoError(t, verifyCmd.Execute())
}

// Expectation: The "verify" command should verify a packed PAR2 set without error.
//
//nolint:paralleltest
func Test_Integration_BundlePack_Verify_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir})
	require.NoError(t, createCmd.Execute())

	bundlePackCmd := newRootCmd(t.Context())
	bundlePackCmd.SetArgs([]string{"bundle", "pack", dir})
	require.NoError(t, bundlePackCmd.Execute())

	par2Files, err := filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	for _, f := range par2Files {
		require.True(t, util.IsPar2Bundle(f), "expected only bundle files after pack, found: %s", f)
	}

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.NoError(t, verifyCmd.Execute())
}

// Expectation: The "verify" command should verify a packed then unpacked PAR2 set without error.
//
//nolint:paralleltest
func Test_Integration_BundlePackUnpack_Verify_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir})
	require.NoError(t, createCmd.Execute())

	bundlePackCmd := newRootCmd(t.Context())
	bundlePackCmd.SetArgs([]string{"bundle", "pack", dir})
	require.NoError(t, bundlePackCmd.Execute())

	par2Files, err := filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	for _, f := range par2Files {
		require.True(t, util.IsPar2Bundle(f), "expected only bundle files after pack, found: %s", f)
	}

	bundleUnpackCmd := newRootCmd(t.Context())
	bundleUnpackCmd.SetArgs([]string{"bundle", "unpack", dir})
	require.NoError(t, bundleUnpackCmd.Execute())

	matches, err := filepath.Glob(filepath.Join(dir, "*.p2c.par2"))
	require.NoError(t, err)
	require.Empty(t, matches, "expected bundle files to be gone after unpack")

	par2Files, err = filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	require.NotEmpty(t, par2Files, "expected original par2 files to be restored after unpack")

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.NoError(t, verifyCmd.Execute())
}

// Expectation: The "repair" command should restore a corrupted file after a pack cycle.
//
//nolint:paralleltest
func Test_Integration_BundlePack_Repair_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir})
	require.NoError(t, createCmd.Execute())

	bundlePackCmd := newRootCmd(t.Context())
	bundlePackCmd.SetArgs([]string{"bundle", "pack", dir})
	require.NoError(t, bundlePackCmd.Execute())

	par2Files, err := filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	for _, f := range par2Files {
		require.True(t, util.IsPar2Bundle(f), "expected only bundle files after pack, found: %s", f)
	}

	dataFile := filepath.Join(dir, "testfile.txt")
	require.NoError(t, os.WriteFile(dataFile, []byte("yello world\n"), 0o600))

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.ErrorIs(t, verifyCmd.Execute(), schema.ErrExitRepairable)

	repairCmd := newRootCmd(t.Context())
	repairCmd.SetArgs([]string{"repair", dir})
	require.NoError(t, repairCmd.Execute())

	restored, err := os.ReadFile(dataFile)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(restored))
}

// Expectation: The "repair" command should restore a corrupted file after a pack/unpack cycle.
//
//nolint:paralleltest
func Test_Integration_BundlePackUnpack_Repair_Success(t *testing.T) {
	dir := setupTestDir(t)

	createCmd := newRootCmd(t.Context())
	createCmd.SetArgs([]string{"create", dir})
	require.NoError(t, createCmd.Execute())

	bundlePackCmd := newRootCmd(t.Context())
	bundlePackCmd.SetArgs([]string{"bundle", "pack", dir})
	require.NoError(t, bundlePackCmd.Execute())

	par2Files, err := filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	for _, f := range par2Files {
		require.True(t, util.IsPar2Bundle(f), "expected only bundle files after pack, found: %s", f)
	}

	bundleUnpackCmd := newRootCmd(t.Context())
	bundleUnpackCmd.SetArgs([]string{"bundle", "unpack", dir})
	require.NoError(t, bundleUnpackCmd.Execute())

	matches, err := filepath.Glob(filepath.Join(dir, "*.p2c.par2"))
	require.NoError(t, err)
	require.Empty(t, matches, "expected bundle files to be gone after unpack")

	par2Files, err = filepath.Glob(filepath.Join(dir, "*.par2"))
	require.NoError(t, err)
	require.NotEmpty(t, par2Files, "expected original par2 files to be restored after unpack")

	dataFile := filepath.Join(dir, "testfile.txt")
	require.NoError(t, os.WriteFile(dataFile, []byte("yello world\n"), 0o600))

	verifyCmd := newRootCmd(t.Context())
	verifyCmd.SetArgs([]string{"verify", dir})
	require.ErrorIs(t, verifyCmd.Execute(), schema.ErrExitRepairable)

	repairCmd := newRootCmd(t.Context())
	repairCmd.SetArgs([]string{"repair", dir})
	require.NoError(t, repairCmd.Execute())

	restored, err := os.ReadFile(dataFile)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", string(restored))
}
