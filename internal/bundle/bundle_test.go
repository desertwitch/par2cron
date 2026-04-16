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

	goldenManifest = ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"version":1,"description":"reference bundle"}`),
	}

	goldenFiles = []string{
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

// Expectation: Verification must pass on the testdata originals.
func Test_WireFormat_ExistingOriginals_Success(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "par2", "verify", "test.par2")
	cmd.Dir = "testdata"
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)
}

// Expectation: Verification must pass on the testdata reference.
func Test_WireFormat_ExistingReference_Success(t *testing.T) {
	t.Parallel()

	cmd := exec.CommandContext(t.Context(), "par2", "verify", "reference.bundle.par2")
	cmd.Dir = "testdata"
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)
}

// Expectation: Files are byte-equal to the testdata originals after unpack.
func Test_WireFormat_ExistingReference_Unpack_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewOsFs()
	unpackDir := filepath.Join(t.TempDir(), "unpacked")
	require.NoError(t, fs.MkdirAll(unpackDir, 0o755))

	bundlePath := filepath.Join("testdata", "reference.bundle.par2")

	b, err := Open(fs, bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.Unpack(fs, unpackDir))

	for _, name := range goldenFiles {
		want, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(unpackDir, name))
		require.NoError(t, err)

		require.Equal(t, want, got, "unpacked %q differs from source", name)
	}
}

// Expectation: A freshly packed bundle must be byte-equal to the reference.
func Test_WireFormat_ReferenceFile_Pack_Success(t *testing.T) {
	t.Parallel()

	goldenBytes, err := os.ReadFile(filepath.Join("testdata", "reference.bundle.par2"))
	require.NoError(t, err)

	fs := afero.NewMemMapFs()

	inputs := make([]FileInput, len(goldenFiles))
	for i, name := range goldenFiles {
		// Read .par2 set files to pack into bundle.
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		// Write .par2 set files to memory filesystem.
		require.NoError(t, afero.WriteFile(fs, name, data, 0o600))

		inputs[i] = FileInput{Name: name, Path: name}
	}

	require.NoError(t,
		Pack(fs, "out.bundle.par2", goldenRecoverySetID, goldenManifest, inputs))

	packedBytes, err := afero.ReadFile(fs, "out.bundle.par2")
	require.NoError(t, err)
	require.Equal(t, goldenBytes, packedBytes)
}

// Expectation: A freshly packed and unpacked bundle must both pass verification.
func Test_WireFormat_ReferenceFile_PackUnpack_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	fs := afero.NewOsFs()

	inputs := make([]FileInput, len(goldenFiles))
	for i, name := range goldenFiles {
		inputs[i] = FileInput{Name: name, Path: filepath.Join("testdata", name)}
	}

	bundlePath := filepath.Join(tmpDir, "bundle.par2")
	require.NoError(t, Pack(fs, bundlePath, goldenRecoverySetID, goldenManifest, inputs))

	unpackDir := filepath.Join(tmpDir, "unpacked")
	require.NoError(t, fs.MkdirAll(unpackDir, 0o750))

	for _, name := range goldenSourceFiles {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		//nolint:gosec
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, name), data, 0o600))
	}

	// The freshly packed bundle must pass verification.
	cmd := exec.CommandContext(t.Context(), "par2", "verify", "bundle.par2")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)

	b, err := Open(fs, bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = b.Close() })

	require.NoError(t, b.Unpack(fs, unpackDir))

	for _, name := range goldenSourceFiles {
		data, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		//nolint:gosec
		require.NoError(t, os.WriteFile(filepath.Join(unpackDir, name), data, 0o600))
	}

	// The extracted .par2 files must pass verification.
	cmd = exec.CommandContext(t.Context(), "par2", "verify", "test.par2")
	cmd.Dir = unpackDir
	out, err = cmd.CombinedOutput()
	require.NoError(t, err, "par2 verify failed: %s", out)

	// The extracted .par2 files must be byte-equal to the testdata originals.
	for _, name := range goldenFiles {
		want, err := os.ReadFile(filepath.Join("testdata", name))
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(unpackDir, name))
		require.NoError(t, err)

		require.Equal(t, want, got, "unpacked %q differs from source", name)
	}
}
