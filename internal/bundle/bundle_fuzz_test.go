package bundle

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/afero"
)

type fuzzSeedInput struct {
	Name string
	Data []byte
}

var (
	fuzzSeedOnce         sync.Once
	fuzzSeedErr          error //nolint:errname
	fuzzReferenceBundles []fuzzSeedInput
	fuzzPar2Inputs       []fuzzSeedInput
)

func loadFuzzSeedCorpus() error {
	// Collect generated bundles from testdata/generated/.
	genEntries, err := os.ReadDir(filepath.Join("testdata", "generated"))
	if err != nil {
		return fmt.Errorf("failed to read testdata/generated: %w", err)
	}

	for _, entry := range genEntries {
		if entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		if !strings.HasSuffix(name, ".par2") {
			continue
		}

		p := filepath.Join("testdata", "generated", entry.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("failed to read bundle %q: %w", p, err)
		}

		fuzzReferenceBundles = append(fuzzReferenceBundles, fuzzSeedInput{
			Name: entry.Name(),
			Data: data,
		})
	}

	if len(fuzzReferenceBundles) == 0 {
		return errors.New("no generated bundles found in testdata/generated")
	}

	// Collect par2 files from each tool subdirectory.
	topEntries, err := os.ReadDir("testdata")
	if err != nil {
		return fmt.Errorf("failed to read testdata: %w", err)
	}

	for _, entry := range topEntries {
		if !entry.IsDir() {
			continue
		}

		dirName := entry.Name()
		if dirName == "generated" || dirName == "sources" {
			continue
		}

		subEntries, err := os.ReadDir(filepath.Join("testdata", dirName))
		if err != nil {
			return fmt.Errorf("failed to read testdata/%s: %w", dirName, err)
		}

		for _, sub := range subEntries {
			if sub.IsDir() {
				continue
			}

			name := strings.ToLower(sub.Name())
			if !strings.HasSuffix(name, ".par2") {
				continue
			}

			p := filepath.Join("testdata", dirName, sub.Name())
			data, err := os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("failed to read seed file %q: %w", p, err)
			}

			fuzzPar2Inputs = append(fuzzPar2Inputs, fuzzSeedInput{
				Name: sub.Name(),
				Data: data,
			})
		}
	}

	if len(fuzzPar2Inputs) == 0 {
		return errors.New("no source .par2 files found in testdata subdirectories")
	}

	sort.Slice(fuzzReferenceBundles, func(i, j int) bool {
		return fuzzReferenceBundles[i].Name < fuzzReferenceBundles[j].Name
	})

	sort.Slice(fuzzPar2Inputs, func(i, j int) bool {
		return fuzzPar2Inputs[i].Name < fuzzPar2Inputs[j].Name
	})

	return nil
}

func mustFuzzSeed(tb testing.TB) ([]fuzzSeedInput, []fuzzSeedInput) {
	tb.Helper()

	fuzzSeedOnce.Do(func() {
		fuzzSeedErr = loadFuzzSeedCorpus()
	})
	if fuzzSeedErr != nil {
		tb.Fatalf("failed to initialize fuzz seed corpus: %v", fuzzSeedErr)
	}

	bundles := make([]fuzzSeedInput, len(fuzzReferenceBundles))
	for i, b := range fuzzReferenceBundles {
		bundles[i] = fuzzSeedInput{Name: b.Name, Data: append([]byte(nil), b.Data...)}
	}

	return bundles, fuzzPar2Inputs
}

func Fuzz_Bundle_Open(f *testing.F) {
	bundles, _ := mustFuzzSeed(f)
	for _, b := range bundles {
		f.Add(b.Data)
	}

	// We fuzz the content of the reference bundle.
	f.Fuzz(func(t *testing.T, data []byte) {
		fs := afero.NewMemMapFs()
		const bundlePath = "/fuzz.bundle"

		if err := afero.WriteFile(fs, bundlePath, data, 0o600); err != nil {
			return
		}

		b, err := Open(fs, bundlePath)
		if err != nil {
			return
		}
		defer func() { _ = b.Close() }()
	})
}

func Fuzz_Bundle_Scan(f *testing.F) {
	bundles, par2Inputs := mustFuzzSeed(f)
	for _, b := range bundles {
		f.Add(b.Data)
	}
	for _, in := range par2Inputs {
		f.Add(in.Data)
	}

	// We fuzz the content of the reference bundle.
	f.Fuzz(func(t *testing.T, data []byte) {
		r := bytes.NewReader(data)
		_, _ = Scan(r, int64(len(data)))
	})
}

func Fuzz_Bundle_Pack(f *testing.F) {
	_, par2Entries := mustFuzzSeed(f)
	for i, a := range par2Entries {
		b := par2Entries[(i+1)%len(par2Entries)]
		f.Add(
			"manifest.json",
			[]byte(`{"seed":true}`),
			[]byte("fuzz-pack-rsid-01"),
			a.Name, a.Data,
			b.Name, b.Data,
			uint8(3), // both files
		)
	}
	f.Add("manifest.json", []byte("{}"), []byte{}, "a.par2", []byte{}, "b.par2", []byte{}, uint8(0))

	// Fuzz manifest name/data, recovery set ID, file names/data, and file-count/layout.
	f.Fuzz(func(t *testing.T,
		manifestName string,
		manifestData []byte,
		recoverySetID []byte,
		file1Name string, file1Data []byte,
		file2Name string, file2Data []byte,
		layout uint8,
	) {
		fsys := afero.NewMemMapFs()

		if err := fsys.MkdirAll("/in", 0o755); err != nil {
			t.Fatalf("failed to create input dir: %v", err)
		}
		if err := afero.WriteFile(fsys, "/in/file1.par2", file1Data, 0o600); err != nil {
			return
		}
		if err := afero.WriteFile(fsys, "/in/file2.par2", file2Data, 0o600); err != nil {
			return
		}

		if manifestName == "" {
			manifestName = "manifest.json"
		}

		var rsid [16]byte
		copy(rsid[:], recoverySetID)

		inputs := make([]FileInput, 0, 2)
		if layout&1 != 0 {
			inputs = append(inputs, FileInput{Name: file1Name, Path: "/in/file1.par2"})
		}
		if layout&2 != 0 {
			inputs = append(inputs, FileInput{Name: file2Name, Path: "/in/file2.par2"})
		}
		if len(inputs) == 0 {
			inputs = append(inputs, FileInput{Name: file1Name, Path: "/in/file1.par2"})
		}

		_ = Pack(fsys, "/bundle.out", rsid, ManifestInput{
			Name:  manifestName,
			Bytes: manifestData,
		}, inputs)
	})
}

func Fuzz_Bundle_Manifest(f *testing.F) {
	bundles, _ := mustFuzzSeed(f)
	for _, b := range bundles {
		f.Add(b.Data)
	}

	// We fuzz the content of the reference bundle.
	f.Fuzz(func(t *testing.T, bundleData []byte) {
		fs := afero.NewMemMapFs()
		const bundlePath = "/bundle.manifest"
		if err := afero.WriteFile(fs, bundlePath, bundleData, 0o600); err != nil {
			return
		}

		b, err := Open(fs, bundlePath)
		if err != nil {
			return
		}
		defer func() { _ = b.Close() }()

		_, _ = b.Manifest()
	})
}

func Fuzz_Bundle_Unpack(f *testing.F) {
	bundles, _ := mustFuzzSeed(f)
	for _, b := range bundles {
		f.Add(b.Data)
	}

	// We fuzz the content of the reference bundle.
	f.Fuzz(func(t *testing.T, bundleData []byte) {
		fs := afero.NewMemMapFs()
		const destDir = "/out"
		if err := fs.MkdirAll(destDir, 0o755); err != nil {
			t.Fatalf("failed to create output dir: %v", err)
		}

		const bundlePath = "/bundle.unpack"
		if err := afero.WriteFile(fs, bundlePath, bundleData, 0o600); err != nil {
			return
		}

		b, err := Open(fs, bundlePath)
		if err != nil {
			return
		}
		defer func() { _ = b.Close() }()

		_ = b.Unpack(fs, destDir, false)
	})
}

func Fuzz_Bundle_UpdateManifest(f *testing.F) {
	bundles, _ := mustFuzzSeed(f)
	for _, b := range bundles {
		f.Add(b.Data, []byte(`{"updated":true}`))
	}

	// We fuzz the content of the reference bundle and manifest bytes.
	f.Fuzz(func(t *testing.T, bundleData []byte, updatedManifest []byte) {
		fs := afero.NewMemMapFs()
		const bundlePath = "/bundle.update"
		if err := afero.WriteFile(fs, bundlePath, bundleData, 0o600); err != nil {
			return
		}

		b, err := Open(fs, bundlePath)
		if err != nil {
			return
		}
		defer func() { _ = b.Close() }()

		_ = b.UpdateManifest(updatedManifest)
	})
}

func Fuzz_Bundle_Validate(f *testing.F) {
	bundles, _ := mustFuzzSeed(f)
	for _, b := range bundles {
		f.Add(b.Data)
	}

	// We fuzz the content of the reference bundle.
	f.Fuzz(func(t *testing.T, bundleData []byte) {
		fs := afero.NewMemMapFs()
		const bundlePath = "/bundle.validate"
		if err := afero.WriteFile(fs, bundlePath, bundleData, 0o600); err != nil {
			return
		}

		b, err := Open(fs, bundlePath)
		if err != nil {
			return
		}
		defer func() { _ = b.Close() }()

		_ = b.ValidateContents(false)
		_ = b.ValidateContents(true)
	})
}
