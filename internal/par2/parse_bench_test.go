package par2

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

// Recovery slice packet type: a real packet type that Parse does not handle.
var recvSliceType = []byte("PAR 2.0\x00RecvSlic")

// loadPar2ToMemFs copies a PAR2 file from disk into an in-memory filesystem,
// so that benchmarks measure parsing rather than disk I/O.
func loadPar2ToMemFs(b *testing.B, path string) (afero.Fs, string, int64) {
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

	return fsys, name, int64(len(data))
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

// benchRandom returns n random bytes with any accidental magic removed,
// so the benchmark measures one clean linear scan to EOF.
func benchRandom(n int) []byte {
	buf := make([]byte, n)
	rng := rand.NewChaCha8([32]byte{})
	_, _ = rng.Read(buf)

	return bytes.ReplaceAll(buf, packetMagic, make([]byte, len(packetMagic)))
}

// benchDenseMagic returns back-to-back magic sequences: every 8 bytes the
// scanner finds a hit, the header fails, and the scan restarts.
func benchDenseMagic(n int) []byte {
	return bytes.Repeat(packetMagic, n/len(packetMagic))
}

// benchHeaders returns back-to-back 64-byte headers of the given type
// and claimed length, with no bodies behind them.
func benchHeaders(n int, typ []byte, length uint64) []byte {
	hdr := make([]byte, packetHeaderSize)
	copy(hdr[0:8], packetMagic)
	binary.LittleEndian.PutUint64(hdr[8:16], length)
	copy(hdr[48:64], typ)

	return bytes.Repeat(hdr, n/packetHeaderSize)
}

// benchVolume returns a valid PAR2 volume of roughly n bytes: one main and file
// packet, followed by recovery slice packets with random bodies and valid MD5s.
func benchVolume(n, sliceSize int) []byte {
	main := buildMainPacket(uint64(sliceSize), [][16]byte{idA}, nil) //nolint:gosec
	set := setIDOf(main)
	out := slices.Concat(main, buildFileDescPacket("a.bin", uint64(n), idA, set)) //nolint:gosec

	body := benchRandom(4 + sliceSize) // Exponent (4 bytes) + slice data
	for len(out) < n {
		out = append(out, buildPacket(recvSliceType, body, set)...)
	}

	return out
}

// Benchmark_ParseFile measures parsing of real PAR2 files from testdata.
// These are small sets, so this mostly tracks fixed per-file overhead
// (open, seeks, grouping, sorting) and catches allocation regressions.
func Benchmark_ParseFile(b *testing.B) {
	for _, path := range globTestdata(b) {
		name := strings.TrimSuffix(filepath.Base(path), ".par2")
		fsys, fname, size := loadPar2ToMemFs(b, path)

		for _, checkMD5 := range []bool{true, false} {
			b.Run(fmt.Sprintf("%s/md5=%t", name, checkMD5), func(b *testing.B) {
				b.SetBytes(size)
				b.ReportAllocs()

				for b.Loop() {
					if _, err := ParseFile(b.Context(), fsys, fname, checkMD5); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}

// Benchmark_Parse_Volume measures the throughput users see on real volume files.
// With MD5 checking, recovery slices are hashed and skipped by length (MD5-bound).
// Without it, their lengths can't be trusted, so the scanner walks through them.
func Benchmark_Parse_Volume(b *testing.B) {
	data := benchVolume(64<<20, 1<<20) // 64 MiB volume, 1 MiB slices

	for _, checkMD5 := range []bool{true, false} {
		b.Run(fmt.Sprintf("md5=%t", checkMD5), func(b *testing.B) {
			b.SetBytes(int64(len(data)))
			b.ReportAllocs()

			for b.Loop() {
				sets, err := Parse(b.Context(), bytes.NewReader(data), checkMD5)
				if err != nil {
					b.Fatal(err)
				}
				if len(sets) != 1 || len(sets[0].RecoverySet) != 1 {
					b.Fatal("unexpected parse result")
				}
			}
		})
	}
}

// Benchmark_Parse_Fallback measures the recovery path on corrupt and hostile input.
// All cases should scale linearly: MB/s stays roughly flat as the size grows
// (hostile cases level off once each recovery scan reads a full buffer).
func Benchmark_Parse_Fallback(b *testing.B) {
	linear := []int{64 << 10, 1 << 20, 16 << 20}
	hostile := []int{4 << 10, 16 << 10, 64 << 10, 256 << 10}

	cases := []struct {
		name  string
		gen   func(int) []byte
		md5   bool
		sizes []int
	}{
		// Baseline: no magic anywhere, a single scan from start to EOF.
		{"random", benchRandom, false, linear},

		// Magic every 8 bytes: each hit fails header validation and triggers a
		// recovery scan. This is the worst case for the scanner.
		{"dense-magic", benchDenseMagic, false, hostile},

		// Unknown type claiming ~MaxInt64 bytes: must be rejected by the
		// remaining-size check, not streamed to EOF per header (was quadratic).
		{"unknown-hugelen-md5", func(n int) []byte {
			return benchHeaders(n, recvSliceType, math.MaxInt64&^3)
		}, true, hostile},

		// Main type claiming the max body: must be rejected by the
		// remaining-size check, not allocate 10 MiB per header.
		{"main-maxbody", func(n int) []byte {
			return benchHeaders(n, mainType, packetHeaderSize+maxPacketSize)
		}, false, hostile},
	}

	for _, tc := range cases {
		for _, size := range tc.sizes {
			data := tc.gen(size)

			b.Run(fmt.Sprintf("%s/%dKiB", tc.name, size>>10), func(b *testing.B) {
				b.SetBytes(int64(len(data)))
				b.ReportAllocs()

				for b.Loop() {
					if _, err := Parse(b.Context(), bytes.NewReader(data), tc.md5); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
