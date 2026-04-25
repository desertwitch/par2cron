package par2

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func loadPar2ToMemFs(b *testing.B, path string) (afero.Fs, string) {
	b.Helper()

	fsys := afero.NewMemMapFs()
	data, err := afero.ReadFile(afero.NewOsFs(), path)
	if err != nil {
		b.Fatal(err)
	}

	name := filepath.Base(path)
	if err := afero.WriteFile(fsys, name, data, 0o644); err != nil {
		b.Fatal(err)
	}

	return fsys, name
}

func globTestdata(b *testing.B) []string {
	b.Helper()

	entries, err := filepath.Glob("testdata/*.par2")
	if err != nil {
		b.Fatal(err)
	}
	if len(entries) == 0 {
		b.Skip("no testdata found")
	}

	return entries
}

func Benchmark_ParseFile(b *testing.B) {
	for _, path := range globTestdata(b) {
		name := strings.TrimSuffix(filepath.Base(path), ".par2")
		b.Run(name, func(b *testing.B) {
			fsys, fname := loadPar2ToMemFs(b, path)

			b.ResetTimer()
			for range b.N {
				if _, err := ParseFile(fsys, fname, true); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
