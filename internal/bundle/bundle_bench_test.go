package bundle

import (
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	"github.com/spf13/afero"
)

func seedFiles(fsys afero.Fs, n, size int) []FileInput {
	files := make([]FileInput, n)
	buf := make([]byte, size)

	for i := range n {
		_, _ = rand.Read(buf)

		path := fmt.Sprintf("/src/file_%d.par2", i)
		_ = fsys.MkdirAll("/src", 0o755)
		_ = afero.WriteFile(fsys, path, buf, 0o644)

		files[i] = FileInput{
			Name: fmt.Sprintf("file_%d.par2", i),
			Path: path,
		}
	}

	return files
}

func packTestBundle(fsys afero.Fs, path string, numFiles, fileSize int) []byte {
	files := seedFiles(fsys, numFiles, fileSize)
	manifest := []byte(`{"files":["a.txt","b.txt"],"created":"2025-01-01T00:00:00Z"}`)
	mi := ManifestInput{Name: "test.manifest", Bytes: manifest}

	var rid [16]byte
	_, _ = rand.Read(rid[:])

	if err := Pack(fsys, path, rid, mi, files); err != nil {
		panic(err)
	}

	return manifest
}

func Benchmark_Pack(b *testing.B) {
	for _, tc := range []struct {
		name     string
		numFiles int
		fileSize int
	}{
		{"1x1KB", 1, 1 << 10},
		{"5x64KB", 5, 64 << 10},
		{"10x1MB", 10, 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			fsys := afero.NewMemMapFs()
			files := seedFiles(fsys, tc.numFiles, tc.fileSize)
			mi := ManifestInput{Name: "test.manifest", Bytes: []byte("manifest")}
			var rid [16]byte
			_, _ = rand.Read(rid[:])

			b.ResetTimer()
			for i := range b.N {
				path := fmt.Sprintf("/bench_%d.p2c.par2", i)
				if err := Pack(fsys, path, rid, mi, files); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func Benchmark_Open(b *testing.B) {
	for _, tc := range []struct {
		name     string
		numFiles int
		fileSize int
	}{
		{"1x1KB", 1, 1 << 10},
		{"5x64KB", 5, 64 << 10},
		{"10x1MB", 10, 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			fsys := afero.NewMemMapFs()
			packTestBundle(fsys, "/bench.p2c.par2", tc.numFiles, tc.fileSize)

			b.ResetTimer()
			for range b.N {
				bnd, err := Open(fsys, "/bench.p2c.par2")
				if err != nil {
					b.Fatal(err)
				}
				_ = bnd.Close()
			}
		})
	}
}

func Benchmark_Update(b *testing.B) {
	for _, tc := range []struct {
		name     string
		numFiles int
		fileSize int
	}{
		{"1x1KB", 1, 1 << 10},
		{"5x64KB", 5, 64 << 10},
		{"10x1MB", 10, 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			fsys := afero.NewMemMapFs()
			packTestBundle(fsys, "/bench.p2c.par2", tc.numFiles, tc.fileSize)
			newManifest := []byte(`{"files":["a.txt","b.txt","c.txt"],"updated":"2025-06-01T00:00:00Z"}`)

			bnd, err := Open(fsys, "/bench.p2c.par2")
			if err != nil {
				b.Fatal(err)
			}
			defer bnd.Close()

			b.ResetTimer()
			for range b.N {
				if err := bnd.Update(newManifest); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func Benchmark_Validate(b *testing.B) {
	for _, strict := range []bool{false, true} {
		label := "Lenient"
		if strict {
			label = "Strict"
		}
		b.Run(label, func(b *testing.B) {
			fsys := afero.NewMemMapFs()
			packTestBundle(fsys, "/bench.p2c.par2", 5, 64<<10)

			bnd, err := Open(fsys, "/bench.p2c.par2")
			if err != nil {
				b.Fatal(err)
			}
			defer bnd.Close()

			b.ResetTimer()
			for range b.N {
				if err := bnd.Validate(strict); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func Benchmark_Scan(b *testing.B) {
	for _, tc := range []struct {
		name     string
		numFiles int
		fileSize int
	}{
		{"5x64KB", 5, 64 << 10},
		{"10x1MB", 10, 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			fsys := afero.NewMemMapFs()
			packTestBundle(fsys, "/bench.p2c.par2", tc.numFiles, tc.fileSize)

			f, err := fsys.Open("/bench.p2c.par2")
			if err != nil {
				b.Fatal(err)
			}
			defer f.Close()

			fi, err := f.Stat()
			if err != nil {
				b.Fatal(err)
			}

			ra, ok := f.(io.ReaderAt)
			if !ok {
				b.Skip("MemMapFs file does not implement io.ReaderAt")
			}

			b.ResetTimer()
			for range b.N {
				files, manifest := Scan(ra, fi.Size(), true)
				if manifest == nil || len(files) == 0 {
					b.Fatal("scan returned no results")
				}
			}
		})
	}
}

func Benchmark_Unpack(b *testing.B) {
	for _, tc := range []struct {
		name     string
		numFiles int
		fileSize int
	}{
		{"5x64KB", 5, 64 << 10},
		{"10x1MB", 10, 1 << 20},
	} {
		b.Run(tc.name, func(b *testing.B) {
			fsys := afero.NewMemMapFs()
			packTestBundle(fsys, "/bench.p2c.par2", tc.numFiles, tc.fileSize)

			bnd, err := Open(fsys, "/bench.p2c.par2")
			if err != nil {
				b.Fatal(err)
			}
			defer bnd.Close()

			b.ResetTimer()
			for i := range b.N {
				destDir := fmt.Sprintf("/out/%d", i)
				_ = fsys.MkdirAll(destDir, 0o755)

				paths, err := bnd.Unpack(fsys, destDir, true)
				if err != nil {
					b.Fatal(err)
				}
				if len(paths) == 0 {
					b.Fatal("unpack returned no paths")
				}
			}
		})
	}
}
