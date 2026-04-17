package bundle

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// The recipe below MUST match tool/generate-bundle/main.go exactly. If the
// generator's inputs change, update both. The wire-format smoke test relies
// on feeding identical inputs to Pack and getting byte-identical output back.
var (
	goldenRecoverySetID = [16]byte{
		0xf3, 0x5c, 0x82, 0x41,
		0xc2, 0xfa, 0x13, 0x01,
		0x83, 0xc9, 0xdf, 0x6e,
		0xf3, 0x04, 0x62, 0x4b,
	}

	goldenBundle = "reference.bundle.par2"

	goldenManifest = ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"version":1,"description":"reference bundle"}`),
	}

	goldenPar2Files = []string{
		"test.par2",
		"test.vol000+34.par2",
		"test.vol034+33.par2",
		"test.vol067+33.par2",
	}

	goldenSourceFiles = []string{
		"file1.bin", "file2.bin", "file3.bin",
		"test1.txt", "test2.txt", "test3.txt",
	}
)

func goldenInputs() []FileInput {
	inputs := make([]FileInput, len(goldenPar2Files))

	for i, name := range goldenPar2Files {
		inputs[i] = FileInput{Name: name, Path: filepath.Join("testdata", name)}
	}

	return inputs
}

func ensureTestdataBundle(t *testing.T, dir string, corrupt bool) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", goldenBundle))
	require.NoError(t, err)

	if !corrupt {
		//nolint:gosec
		require.NoError(t, os.WriteFile(filepath.Join(dir, goldenBundle), data, 0o600))
		requireBundleMatchesTestdata(t, dir)
	} else {
		//nolint:gosec
		require.NoError(t, os.WriteFile(filepath.Join(dir, goldenBundle), data[:len(data)-1], 0o600))
	}
}

func ensureTestdataSourceFiles(t *testing.T, dir string, corrupt bool) {
	t.Helper()

	for _, name := range goldenSourceFiles {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		if !corrupt {
			//nolint:gosec
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
		} else {
			//nolint:gosec
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), data[:len(data)-1], 0o600))
		}
	}

	if !corrupt {
		requireSourcesMatchTestdata(t, dir)
	}
}

func ensureTestdataPar2Files(t *testing.T, dir string, corrupt bool) {
	t.Helper()

	for _, name := range goldenPar2Files {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		if !corrupt {
			//nolint:gosec
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
		} else {
			//nolint:gosec
			require.NoError(t, os.WriteFile(filepath.Join(dir, name), data[:len(data)-1], 0o600))
		}
	}

	if !corrupt {
		requirePar2MatchTestdata(t, dir)
	}
}

func requireBundleMatchesTestdata(t *testing.T, dir string) {
	t.Helper()

	want, err := os.ReadFile(filepath.Join("testdata", goldenBundle))
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dir, goldenBundle))
	require.NoError(t, err)

	require.Equal(t, want, got)
}

func requireSourcesMatchTestdata(t *testing.T, dir string) {
	t.Helper()

	for _, name := range goldenSourceFiles {
		want, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)

		require.Equal(t, want, got, "%q differs from testdata", name)
	}
}

func requirePar2MatchTestdata(t *testing.T, dir string) {
	t.Helper()

	for _, name := range goldenPar2Files {
		want, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)

		require.Equal(t, want, got, "%q differs from testdata", name)
	}
}

// Expectation: Verification must pass on the testdata par2s using par2cmdline.
func Test_WireFormat_TestdataPar2s_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ensureTestdataSourceFiles(t, tmpDir, false)
	ensureTestdataPar2Files(t, tmpDir, false)

	cmd := exec.CommandContext(t.Context(), "par2", "verify", goldenPar2Files[0])
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)
}

// Expectation: Verification must pass on the testdata bundle using par2cmdline.
func Test_WireFormat_TestdataBundle_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ensureTestdataSourceFiles(t, tmpDir, false)
	ensureTestdataBundle(t, tmpDir, false)

	cmd := exec.CommandContext(t.Context(), "par2", "verify", goldenBundle)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)
}

// Expectation: Extracted par2s must be byte-equal to the testdata par2s after unpack.
func Test_WireFormat_TestdataBundle_Unpack_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fs := afero.NewOsFs()
	ensureTestdataBundle(t, tmpDir, false)

	b, err := Open(fs, filepath.Join(tmpDir, goldenBundle))
	require.NoError(t, err)
	require.NoError(t, b.Validate(true))
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.Unpack(fs, tmpDir, true))
	requirePar2MatchTestdata(t, tmpDir)
}

// Expectation: A freshly packed reference bundle must be byte-equal to the testdata bundle.
func Test_WireFormat_ReferenceBundle_Pack_EqualsTestdata_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fs := afero.NewOsFs()

	bundlePath := filepath.Join(tmpDir, goldenBundle)
	require.NoError(t, Pack(fs, bundlePath, goldenRecoverySetID, goldenManifest, goldenInputs()))

	requireBundleMatchesTestdata(t, tmpDir)

	b, err := Open(fs, bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.Validate(true))
}

// Expectation: A freshly packed reference bundle must pass verification using par2cmdline.
func Test_WireFormat_ReferenceBundle_Pack_Verify(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fs := afero.NewOsFs()
	ensureTestdataSourceFiles(t, tmpDir, false)

	bundlePath := filepath.Join(tmpDir, goldenBundle)
	require.NoError(t, Pack(fs, bundlePath, goldenRecoverySetID, goldenManifest, goldenInputs()))

	cmd := exec.CommandContext(t.Context(), "par2", "verify", goldenBundle)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)
}

// Expectation: A freshly packed reference must be able to repair using par2cmdline.
func Test_WireFormat_ReferenceBundle_Pack_Repair(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fs := afero.NewOsFs()
	ensureTestdataSourceFiles(t, tmpDir, true) // corrupt

	bundlePath := filepath.Join(tmpDir, goldenBundle)
	require.NoError(t, Pack(fs, bundlePath, goldenRecoverySetID, goldenManifest, goldenInputs()))

	cmd := exec.CommandContext(t.Context(), "par2", "verify", goldenBundle)
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(out), "Repair is possible")

	cmd = exec.CommandContext(t.Context(), "par2", "repair", goldenBundle)
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "par2 repair failed: %s", out)

	requireSourcesMatchTestdata(t, tmpDir)  // Sources identical after repair.
	requireBundleMatchesTestdata(t, tmpDir) // Bundle unchanged after repair.
}

// Expectation: An unpacked reference must pass verification using par2cmdline.
func Test_WireFormat_ReferenceBundle_Unpack_Verify(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fs := afero.NewOsFs()
	ensureTestdataSourceFiles(t, tmpDir, false)

	bundlePath := filepath.Join(tmpDir, goldenBundle)
	require.NoError(t, Pack(fs, bundlePath, goldenRecoverySetID, goldenManifest, goldenInputs()))

	b, err := Open(fs, bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Validate(true))
	require.NoError(t, b.Unpack(fs, tmpDir, true))
	require.NoError(t, b.Close())

	requirePar2MatchTestdata(t, tmpDir)
	require.NoError(t, os.Remove(filepath.Join(tmpDir, goldenBundle)))

	cmd := exec.CommandContext(t.Context(), "par2", "verify", goldenPar2Files[0])
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)
}

// Expectation: An unpacked reference must be able to repair using par2cmdline.
func Test_WireFormat_ReferenceBundle_Unpack_Repair(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fs := afero.NewOsFs()
	ensureTestdataSourceFiles(t, tmpDir, true) // corrupt

	bundlePath := filepath.Join(tmpDir, goldenBundle)
	require.NoError(t, Pack(fs, bundlePath, goldenRecoverySetID, goldenManifest, goldenInputs()))

	b, err := Open(fs, bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Validate(true))
	require.NoError(t, b.Unpack(fs, tmpDir, true))
	require.NoError(t, b.Close())

	requirePar2MatchTestdata(t, tmpDir)
	require.NoError(t, os.Remove(filepath.Join(tmpDir, goldenBundle)))

	cmd := exec.CommandContext(t.Context(), "par2", "verify", goldenPar2Files[0])
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	require.Contains(t, string(out), "Repair is possible")

	cmd = exec.CommandContext(t.Context(), "par2", "repair", goldenPar2Files[0])
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "par2 repair failed: %s", out)

	requireSourcesMatchTestdata(t, tmpDir)
}
