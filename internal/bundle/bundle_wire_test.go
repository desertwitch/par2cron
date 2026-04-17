//nolint:gosec
package bundle

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type goldenSet struct {
	Name          string
	Dir           string
	Bundle        string
	IndexFile     string
	Par2Files     []string
	RecoverySetID [16]byte
}

// The recipe below MUST match tool/generate-bundle/main.go exactly. If the
// generator's inputs change, update both. The wire-format smoke test relies
// on feeding identical inputs to Pack and getting byte-identical output back.
var (
	goldenSets []goldenSet

	goldenManifest = ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"version":1,"description":"reference bundle"}`),
	}

	goldenSourceFiles = []string{
		"file1.bin", "file2.bin", "file3.bin",
	}
)

func TestMain(m *testing.M) {
	fs := afero.NewOsFs()

	for _, base := range []struct{ name, dir, bundle string }{
		{"multipar", "multipar", "generated/multipar.p2c.par2"},
		{"par2cmdline", "par2cmdline", "generated/par2cmdline.p2c.par2"},
		{"par2cmdline-turbo", "par2cmdline-turbo", "generated/par2cmdline-turbo.p2c.par2"},
		{"parpar", "parpar", "generated/parpar.p2c.par2"},
		{"quickpar", "quickpar", "generated/quickpar.p2c.par2"},
	} {
		par2Files := findPar2Files(base.dir)
		indexFile := findIndexPar2(par2Files)

		pf, err := par2.ParseFile(fs, indexFile, true)
		if err != nil {
			log.Fatalf("TestMain: parse %s: %v", indexFile, err)
		}

		if len(pf.Sets) < 1 || pf.Sets[0].MainPacket == nil {
			log.Fatalf("TestMain: %s has no sets or main packet", indexFile)
		}

		goldenSets = append(goldenSets, goldenSet{
			Name:          base.name,
			Dir:           base.dir,
			Bundle:        base.bundle,
			IndexFile:     indexFile,
			Par2Files:     par2Files,
			RecoverySetID: pf.Sets[0].MainPacket.SetID,
		})
	}

	os.Exit(m.Run())
}

// findPar2Files globs all par2/PAR2 files in the set's directory.
func findPar2Files(dir string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, ext := range []string{"*.par2", "*.PAR2"} {
		matches, err := filepath.Glob(filepath.Join("testdata", dir, ext))
		if err != nil {
			log.Fatalf("TestMain: glob %s: %v", dir, err)
		}

		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				result = append(result, m)
			}
		}
	}

	if len(result) == 0 {
		log.Fatalf("TestMain: no par2 files found in testdata/%s", dir)
	}

	// We specifically don't sort here to test deterministic results.

	return result
}

// findIndexPar2 returns the index .par2 (the one without .vol in the name).
func findIndexPar2(par2Files []string) string {
	for _, m := range par2Files {
		if !strings.Contains(strings.ToLower(filepath.Base(m)), ".vol") {
			return m
		}
	}

	log.Fatal("TestMain: no index par2 found")

	return ""
}

func (gs goldenSet) Inputs() []FileInput {
	inputs := make([]FileInput, len(gs.Par2Files))

	for i, p := range gs.Par2Files {
		inputs[i] = FileInput{Name: filepath.Base(p), Path: p}
	}

	return inputs
}

func ensureTestdataBundle(t *testing.T, gs goldenSet, dir string, corrupt bool) {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("testdata", gs.Bundle))
	require.NoError(t, err)

	dst := filepath.Join(dir, filepath.Base(gs.Bundle))

	if corrupt {
		data = data[:len(data)-1]
	}

	require.NoError(t, os.WriteFile(dst, data, 0o600))
}

func ensureTestdataSourceFiles(t *testing.T, dir string, corrupt bool) {
	t.Helper()

	for _, name := range goldenSourceFiles {
		data, err := os.ReadFile(filepath.Join("testdata", "sources", name))
		require.NoError(t, err)

		if corrupt {
			data = data[:len(data)-1]
		}

		require.NoError(t, os.WriteFile(filepath.Join(dir, name), data, 0o600))
	}
}

func ensureTestdataPar2Files(t *testing.T, gs goldenSet, dir string, corrupt bool) {
	t.Helper()

	for _, p := range gs.Par2Files {
		data, err := os.ReadFile(p)
		require.NoError(t, err)

		if corrupt {
			data = data[:len(data)-1]
		}

		require.NoError(t, os.WriteFile(filepath.Join(dir, filepath.Base(p)), data, 0o600))
	}
}

func requireBundleMatchesTestdata(t *testing.T, gs goldenSet, dir string) {
	t.Helper()

	want, err := os.ReadFile(filepath.Join("testdata", gs.Bundle))
	require.NoError(t, err)

	got, err := os.ReadFile(filepath.Join(dir, filepath.Base(gs.Bundle)))
	require.NoError(t, err)

	require.Equal(t, want, got)
}

func requireSourcesMatchTestdata(t *testing.T, dir string) {
	t.Helper()

	for _, name := range goldenSourceFiles {
		want, err := os.ReadFile(filepath.Join("testdata", "sources", name))
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)

		require.Equal(t, want, got, "%q differs from testdata", name)
	}
}

func requirePar2MatchTestdata(t *testing.T, gs goldenSet, dir string) {
	t.Helper()

	for _, p := range gs.Par2Files {
		want, err := os.ReadFile(p)
		require.NoError(t, err)

		got, err := os.ReadFile(filepath.Join(dir, filepath.Base(p)))
		require.NoError(t, err)

		require.Equal(t, want, got, "%q differs from testdata", filepath.Base(p))
	}
}

// Expectation: Verification must pass on the testdata par2s using par2cmdline.
func Test_WireFormat_TestdataPar2s_Success(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			ensureTestdataSourceFiles(t, tmpDir, false)
			ensureTestdataPar2Files(t, gs, tmpDir, false)

			indexFile := filepath.Base(gs.IndexFile)
			cmd := exec.CommandContext(t.Context(), "par2", "verify", indexFile)
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "par2 verify failed: %s", out)
		})
	}
}

// Expectation: Verification must pass on the testdata bundle using par2cmdline.
func Test_WireFormat_TestdataBundle_Success(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			ensureTestdataSourceFiles(t, tmpDir, false)
			ensureTestdataBundle(t, gs, tmpDir, false)

			cmd := exec.CommandContext(t.Context(), "par2", "verify", filepath.Base(gs.Bundle))
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "par2 verify failed: %s", out)
		})
	}
}

// Expectation: Extracted par2s must be byte-equal to the testdata par2s after unpack.
func Test_WireFormat_TestdataBundle_Unpack_Success(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			fs := afero.NewOsFs()
			ensureTestdataBundle(t, gs, tmpDir, false)

			b, err := Open(fs, filepath.Join(tmpDir, filepath.Base(gs.Bundle)))
			require.NoError(t, err)
			require.NoError(t, b.Validate(true))
			t.Cleanup(func() { _ = b.Close() })

			require.NoError(t, b.Unpack(fs, tmpDir, true))
			requirePar2MatchTestdata(t, gs, tmpDir)
		})
	}
}

// Expectation: A freshly packed reference bundle must be byte-equal to the testdata bundle.
func Test_WireFormat_ReferenceBundle_Pack_EqualsTestdata_Success(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			fs := afero.NewOsFs()

			bundlePath := filepath.Join(tmpDir, filepath.Base(gs.Bundle))
			require.NoError(t, Pack(fs, bundlePath, gs.RecoverySetID, goldenManifest, gs.Inputs()))

			requireBundleMatchesTestdata(t, gs, tmpDir)

			b, err := Open(fs, bundlePath)
			require.NoError(t, err)
			t.Cleanup(func() { _ = b.Close() })

			require.NoError(t, b.Validate(true))
		})
	}
}

// Expectation: A freshly packed reference bundle must pass verification using par2cmdline.
func Test_WireFormat_ReferenceBundle_Pack_Verify(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			fs := afero.NewOsFs()
			ensureTestdataSourceFiles(t, tmpDir, false)

			bundlePath := filepath.Join(tmpDir, filepath.Base(gs.Bundle))
			require.NoError(t, Pack(fs, bundlePath, gs.RecoverySetID, goldenManifest, gs.Inputs()))

			cmd := exec.CommandContext(t.Context(), "par2", "verify", filepath.Base(gs.Bundle))
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "par2 verify failed: %s", out)
		})
	}
}

// Expectation: A freshly packed reference must be able to repair using par2cmdline.
func Test_WireFormat_ReferenceBundle_Pack_Repair(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			fs := afero.NewOsFs()
			ensureTestdataSourceFiles(t, tmpDir, true) // corrupt

			bundlePath := filepath.Join(tmpDir, filepath.Base(gs.Bundle))
			require.NoError(t, Pack(fs, bundlePath, gs.RecoverySetID, goldenManifest, gs.Inputs()))

			cmd := exec.CommandContext(t.Context(), "par2", "verify", filepath.Base(gs.Bundle))
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			require.Error(t, err)
			require.Contains(t, string(out), "Repair is possible")

			cmd = exec.CommandContext(t.Context(), "par2", "repair", filepath.Base(gs.Bundle))
			cmd.Dir = tmpDir
			out, err = cmd.CombinedOutput()
			require.NoError(t, err, "par2 repair failed: %s", out)

			requireSourcesMatchTestdata(t, tmpDir)
			requireBundleMatchesTestdata(t, gs, tmpDir) // Bundle did not change after repair.
		})
	}
}

// Expectation: An unpacked reference must pass verification using par2cmdline.
func Test_WireFormat_ReferenceBundle_Unpack_Verify(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			fs := afero.NewOsFs()
			ensureTestdataSourceFiles(t, tmpDir, false)

			bundlePath := filepath.Join(tmpDir, filepath.Base(gs.Bundle))
			require.NoError(t, Pack(fs, bundlePath, gs.RecoverySetID, goldenManifest, gs.Inputs()))

			b, err := Open(fs, bundlePath)
			require.NoError(t, err)
			require.NoError(t, b.Validate(true))
			require.NoError(t, b.Unpack(fs, tmpDir, true))
			require.NoError(t, b.Close())

			requirePar2MatchTestdata(t, gs, tmpDir)
			require.NoError(t, os.Remove(bundlePath))

			indexFile := filepath.Base(gs.IndexFile)
			cmd := exec.CommandContext(t.Context(), "par2", "verify", indexFile)
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			require.NoError(t, err, "par2 verify failed: %s", out)
		})
	}
}

// Expectation: An unpacked reference must be able to repair using par2cmdline.
func Test_WireFormat_ReferenceBundle_Unpack_Repair(t *testing.T) {
	t.Parallel()

	for _, gs := range goldenSets {
		t.Run(gs.Name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			fs := afero.NewOsFs()
			ensureTestdataSourceFiles(t, tmpDir, true) // corrupt

			bundlePath := filepath.Join(tmpDir, filepath.Base(gs.Bundle))
			require.NoError(t, Pack(fs, bundlePath, gs.RecoverySetID, goldenManifest, gs.Inputs()))

			b, err := Open(fs, bundlePath)
			require.NoError(t, err)
			require.NoError(t, b.Validate(true))
			require.NoError(t, b.Unpack(fs, tmpDir, true))
			require.NoError(t, b.Close())

			requirePar2MatchTestdata(t, gs, tmpDir)
			require.NoError(t, os.Remove(bundlePath))

			indexFile := filepath.Base(gs.IndexFile)
			cmd := exec.CommandContext(t.Context(), "par2", "verify", indexFile)
			cmd.Dir = tmpDir
			out, err := cmd.CombinedOutput()
			require.Error(t, err)
			require.Contains(t, string(out), "Repair is possible")

			cmd = exec.CommandContext(t.Context(), "par2", "repair", indexFile)
			cmd.Dir = tmpDir
			out, err = cmd.CombinedOutput()
			require.NoError(t, err, "par2 repair failed: %s", out)

			requireSourcesMatchTestdata(t, tmpDir)
		})
	}
}
