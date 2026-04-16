package bundle

import (
	"crypto/md5"
	"fmt"
	"io"
	"path/filepath"

	"github.com/zeebo/blake3"
)

// Manifest reads, verifies and returns the manifest bytes.
func (b *Bundle) Manifest() ([]byte, error) {
	sr, expectedHash := b.readManifest()

	data, err := io.ReadAll(sr)
	if err != nil {
		return nil,
			fmt.Errorf("failed to read: %w", err)
	}

	if dataHash(data) != expectedHash {
		return nil,
			fmt.Errorf("failed to validate: %w", ErrDataCorrupt)
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
		return [32]byte{},
			fmt.Errorf("failed to io: %w", err)
	}

	var sum [32]byte
	copy(sum[:], h.Sum(nil))

	return sum, nil
}

// packetMD5 computes md5(recovery_set_id || packet_type || body).
func packetMD5(recoverySetID [16]byte, packetType [16]byte, body []byte) [16]byte {
	input := make([]byte, 0, len(recoverySetID)+len(packetType)+len(body))
	input = append(input, recoverySetID[:]...)
	input = append(input, packetType[:]...)
	input = append(input, body...)

	return md5.Sum(input)
}

// isKnownPacketType returns if the packet is of a par2cron-specific type.
func isKnownPacketType(t [16]byte) bool {
	switch t {
	case PacketTypeIndex, PacketTypeFile, PacketTypeManifest:
		return true
	default:
		return false
	}
}

// padTo4 returns n rounded up to the next multiple of 4.
//
//nolint:mnd
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
		return fmt.Errorf("failed to read: %w", err)
	}

	destPath := filepath.Join(destDir, name)
	out, err := b.fsys.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create: %w", err)
	}
	defer out.Close()

	hash, err := dataHashReader(io.TeeReader(sr, out))
	if err != nil {
		return fmt.Errorf("failed to io: %w", err)
	}

	if hash != expectedHash {
		return fmt.Errorf("failed to validate: %w", ErrDataCorrupt)
	}

	if err := out.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	return nil
}

// extractManifest extracts the manifest JSON to destDir/<manifest name>,
// streaming from the bundle and verifying the hash after writing.
func (b *Bundle) extractManifest(destDir string) error {
	sr, expectedHash := b.readManifest()

	destPath := filepath.Join(destDir, b.Index.ManifestName)

	out, err := b.fsys.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create: %w", err)
	}
	defer out.Close()

	hash, err := dataHashReader(io.TeeReader(sr, out))
	if err != nil {
		return fmt.Errorf("failed to io: %w", err)
	}

	if hash != expectedHash {
		return fmt.Errorf("failed to validate: %w", ErrDataCorrupt)
	}

	if err := out.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	return nil
}
