//go:generate go run ../../tool/generate-bundle -dir testdata -out generated/multipar.p2c.par2 -parse testdata/multipar/files.par2 testdata/multipar/files.par2 testdata/multipar/files.vol00+7.par2 testdata/multipar/files.vol07+6.par2 testdata/multipar/files.vol13+6.par2
//go:generate go run ../../tool/generate-bundle -dir testdata -out generated/par2cmdline.p2c.par2 -parse testdata/par2cmdline/files.par2 testdata/par2cmdline/files.par2 testdata/par2cmdline/files.vol0+1.par2 testdata/par2cmdline/files.vol1+1.par2 testdata/par2cmdline/files.vol2+1.par2
//go:generate go run ../../tool/generate-bundle -dir testdata -out generated/par2cmdline-turbo.p2c.par2 -parse testdata/par2cmdline-turbo/files.par2 testdata/par2cmdline-turbo/files.par2 testdata/par2cmdline-turbo/files.vol0+1.par2 testdata/par2cmdline-turbo/files.vol1+1.par2 testdata/par2cmdline-turbo/files.vol2+1.par2
//go:generate go run ../../tool/generate-bundle -dir testdata -out generated/parpar.p2c.par2 -parse testdata/parpar/files.par2 testdata/parpar/files.par2 testdata/parpar/files.vol00+05.par2 testdata/parpar/files.vol05+05.par2 testdata/parpar/files.vol10+03.par2
//go:generate go run ../../tool/generate-bundle -dir testdata -out generated/quickpar.p2c.par2 -parse testdata/quickpar/files.par2 testdata/quickpar/files.par2 testdata/quickpar/files.vol0+1.PAR2 testdata/quickpar/files.vol1+1.PAR2 testdata/quickpar/files.vol2+2.PAR2
//nolint:dupword
package bundle

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/afero"
)

var Magic = [8]byte{'P', 'A', 'R', '2', 0, 'P', 'K', 'T'}

var (
	PacketTypeIndex    = [16]byte{'P', '2', 'C', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'I', 'n', 'd', 'x', 0, 0}
	PacketTypeFile     = [16]byte{'P', '2', 'C', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'F', 'i', 'l', 'e', 0, 0}
	PacketTypeManifest = [16]byte{'P', '2', 'C', ' ', 'B', 'u', 'n', 'd', 'l', 'e', 'M', 'f', 's', 't', 0, 0}
)

const (
	// Version is the current format version.
	Version uint64 = 1

	// FlagIndexRebuilt signals that an index packet was re-built (corrupted).
	// At this point file entry table in index packet is no longer guaranteed.
	FlagIndexRebuilt uint64 = 1 << 0
)

var ErrDataCorrupt = errors.New("data corrupt")

// Bundle is an opened bundle file with a parsed index packet. If the index was
// corrupt on open, it is reconstructed from intact found packets and OpenError
// holds the original error. Calling the Update function restores bundle to its
// cleanest possible state, replacing both the manifest and the index, repairing
// any corruption in either. Corruption in file packets only reduces the chance
// of extracting a bundled PAR2 data stream later, while corruption in bundled
// PAR2 data streams is handled gracefully by downstream PAR2 parsing programs.
type Bundle struct {
	f    afero.File // os.O_RDWR
	size int64      // guaranteed > 0

	Index     IndexPacket
	OpenError error
}

// Open opens a bundle file and reads the index packet. If the index packet is
// corrupt, Open attempts to reconstruct it by scanning for intact file and
// manifest packets. Use Validate.. functions to check the bundle's integrity
// after opening (if that should be required). Update() restores working state.
func Open(fsys afero.Fs, bundlePath string) (*Bundle, error) {
	f, err := fsys.OpenFile(bundlePath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()

		return nil, fmt.Errorf("failed to stat: %w", err)
	}
	if fi.Size() < 0 {
		_ = f.Close()

		return nil, fmt.Errorf("file size < 0: %d", fi.Size())
	}

	b := &Bundle{f: f, size: fi.Size()}
	if err := b.readIndexPacket(); err != nil {
		files, manifest := Scan(f, fi.Size(), true)

		if manifest == nil {
			_ = f.Close()

			return nil, fmt.Errorf("%w: bundle too damaged", ErrDataCorrupt)
		}

		b.Index = reconstructIndex(manifest, files)
		b.Index.Flags |= FlagIndexRebuilt
		b.OpenError = fmt.Errorf("index reconstructed: %w", err)
	}

	return b, nil
}

// Close closes the bundle file.
func (b *Bundle) Close() error {
	return b.f.Close() //nolint:wrapcheck
}

// Manifest reads and returns the manifest bytes, verified against the BLAKE3
// hash. On errors the bytes are still returned for inspection but should be
// treated as suspect. ErrDataCorrupt is returned on hash mismatch (corruption).
func (b *Bundle) Manifest() ([]byte, error) {
	var buf bytes.Buffer

	if err := b.ExtractManifest(&buf); err != nil {
		return buf.Bytes(), fmt.Errorf("failed to extract: %w", err)
	}

	return buf.Bytes(), nil
}

// Validate checks the bundle's integrity by validating the index, all file
// packets, and the manifest in sequence. If strict is true, manifest and file
// data is additionally verified against their BLAKE3 hashes, which requires
// reading the full data streams and may be slower for large bundles.
func (b *Bundle) Validate(strict bool) error {
	if err := b.ValidateIndex(); err != nil {
		return fmt.Errorf("index: %w", err)
	}

	if err := b.ValidateFiles(strict); err != nil {
		return fmt.Errorf("files: %w", err)
	}

	if err := b.ValidateManifest(strict); err != nil {
		return fmt.Errorf("manifest: %w", err)
	}

	return nil
}

// ValidateIndex reports whether the index packet was read cleanly from disk. It
// returns nil if the index was parsed and validated normally, or an error
// describing why reconstruction from a scan was necessary. Beware it only
// returns this as long as Update() has not written that reconstructed index
// back to the bundle file, restoring it to one functionally correct state.
func (b *Bundle) ValidateIndex() error {
	return b.OpenError
}

// ValidateFiles verifies that every file entry in the index points to a valid
// file packet at the expected offset. If strict is true, it additionally checks
// each file stream's data against its BLAKE3 hash to detect corruption, which
// requires reading the full data stream and may be slower for large bundles.
func (b *Bundle) ValidateFiles(strict bool) error {
	for i, entry := range b.Index.Entries {
		ch, _, err := readAndValidatePacket(b.f, int64(entry.PacketOffset), b.size, true) //nolint:gosec
		if err != nil {
			return fmt.Errorf("file packet %d (%q) at offset %d: %w", i, entry.Name, entry.PacketOffset, err)
		}
		if ch.PacketType != PacketTypeFile {
			return fmt.Errorf("file packet %d (%q) at offset %d: expected file type, got %q", i, entry.Name, entry.PacketOffset, ch.PacketType)
		}

		if strict {
			sr := io.NewSectionReader(b.f, int64(entry.DataOffset), int64(entry.DataLength)) //nolint:gosec

			hash, err := dataHashReader(sr)
			if err != nil {
				return fmt.Errorf("file data %d (%q) at offset %d: hash error: %w", i, entry.Name, entry.DataOffset, err)
			}
			if hash != entry.DataB3 {
				return fmt.Errorf("file data %d (%q) at offset %d: %w: hash mismatch", i, entry.Name, entry.DataOffset, ErrDataCorrupt)
			}
		}
	}

	return nil
}

// ValidateManifest verifies that the manifest packet is present and well-formed
// at the expected offset. If strict is true, it additionally checks the
// manifest data against its BLAKE3 hash to detect corruption, which requires
// reading the full data stream and may be slower for large bundles.
func (b *Bundle) ValidateManifest(strict bool) error {
	ch, _, err := readAndValidatePacket(b.f, int64(b.Index.ManifestPacketOffset), b.size, true) //nolint:gosec
	if err != nil {
		return fmt.Errorf("manifest packet at offset %d: %w", b.Index.ManifestPacketOffset, err)
	}
	if ch.PacketType != PacketTypeManifest {
		return fmt.Errorf("manifest packet at offset %d: expected manifest type, got %q", b.Index.ManifestPacketOffset, ch.PacketType)
	}

	if strict {
		sr := io.NewSectionReader(b.f, int64(b.Index.ManifestDataOffset), int64(b.Index.ManifestDataLength)) //nolint:gosec

		hash, err := dataHashReader(sr)
		if err != nil {
			return fmt.Errorf("manifest data at offset %d: hash error: %w", b.Index.ManifestDataOffset, err)
		}
		if hash != b.Index.ManifestDataB3 {
			return fmt.Errorf("manifest data at offset %d: %w: hash mismatch", b.Index.ManifestDataOffset, ErrDataCorrupt)
		}
	}

	return nil
}

// readIndexPacket reads and validates the index packet at offset 0.
func (b *Bundle) readIndexPacket() error {
	ch, body, err := readAndValidatePacket(b.f, 0, b.size, true)
	if err != nil {
		return fmt.Errorf("failed to read packet: %w", err)
	}

	if ch.PacketType != PacketTypeIndex {
		return fmt.Errorf("expected index packet at offset 0, got %q", ch.PacketType)
	}

	mp, err := parseIndexPacket(bytes.NewReader(body), ch)
	if err != nil {
		return fmt.Errorf("failed to parse packet: %w", err)
	}

	b.Index = mp

	return nil
}
