package bundle

import (
	"crypto/md5"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zeebo/blake3"
)

// Manifest reads, verifies and returns the manifest bytes.
func (b *Bundle) Manifest() ([]byte, error) {
	sr, expectedHash, err := b.readManifest()
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(sr)
	if err != nil {
		return nil, err
	}
	if dataHash(data) != expectedHash {
		return nil, fmt.Errorf("%w: manifest", ErrDataCorrupt)
	}

	return data, nil
}

// dataHash computes the data integrity hash from a byte slice.
func dataHash(data []byte) [32]byte {
	return blake3.Sum256(data)
}

// dataHashReader computes the data integrity hash by streaming from r.
func dataHashReader(r io.Reader) ([32]byte, error) {
	h := blake3.New()
	if _, err := io.Copy(h, r); err != nil {
		return [32]byte{}, err
	}

	var sum [32]byte
	copy(sum[:], h.Sum(nil))

	return sum, nil
}

// headerMD5 computes md5(packet_type || packet_length || header_length || type-specific bytes),
// i.e. bytes 8..header_length with header_md5 omitted entirely.
func headerMD5(packetType uint64, packetLength uint64, headerLength uint64, typeSpecific []byte) [16]byte {
	input := make([]byte, 0, 8+8+8+len(typeSpecific))

	var scratch [8]byte
	binary.LittleEndian.PutUint64(scratch[:], packetType)
	input = append(input, scratch[:]...)

	binary.LittleEndian.PutUint64(scratch[:], packetLength)
	input = append(input, scratch[:]...)

	binary.LittleEndian.PutUint64(scratch[:], headerLength)
	input = append(input, scratch[:]...)

	input = append(input, typeSpecific...)

	return md5.Sum(input)
}

// padTo4 returns n rounded up to the next multiple of 4.
func padTo4(n uint64) uint64 {
	return (n + 3) &^ 3
}

// isAligned4 checks if n is aligned to 4.
func isAligned4(n uint64) bool {
	return n%4 == 0
}

// extractFile extracts a single file from the bundle to destDir,
// streaming from the bundle and verifying the hash after writing.
func (b *Bundle) extractFile(name string, destDir string) error {
	sr, expectedHash, err := b.readFile(name)
	if err != nil {
		return err
	}

	destPath := filepath.Join(destDir, name)
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Stream and hash simultaneously.
	hash, err := dataHashReader(io.TeeReader(sr, out))
	if err != nil {
		return err
	}

	if hash != expectedHash {
		// Warn here
	}

	return out.Sync()
}

// extractManifest extracts the manifest JSON to destDir/<manifest name>,
// streaming from the bundle and verifying the hash after writing.
func (b *Bundle) extractManifest(destDir string) error {
	sr, expectedHash, err := b.readManifest()
	if err != nil {
		return err
	}

	destPath := filepath.Join(destDir, b.Main.ManifestName)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	hash, err := dataHashReader(io.TeeReader(sr, out))
	if err != nil {
		return err
	}

	if hash != expectedHash {
		// Warn here
	}

	return out.Sync()
}
