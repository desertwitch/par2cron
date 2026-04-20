package bundle

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/zeebo/blake3"
)

// dataHash computes the BLAKE3 data integrity hash from a byte slice.
func dataHash(data []byte) [32]byte {
	return blake3.Sum256(data)
}

// dataHashReader computes the BLAKE3 data integrity hash streaming from r.
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

// computeIndexSize returns the total index packet size.
func computeIndexSize(manifest ManifestInput, files []FileInput) uint64 {
	entries := make([]IndexEntry, len(files))
	for i, fi := range files {
		entries[i] = IndexEntry{NameLen: uint64(len(fi.Name))}
	}

	return computeIndexSizeFromEntries(manifest.Name, entries)
}

// computeIndexSizeFromEntries returns the total index packet size.
func computeIndexSizeFromEntries(manifestName string, entries []IndexEntry) uint64 {
	size := uint64(commonHeaderSize) + indexFixedSize + padTo4(uint64(len(manifestName)))
	for _, e := range entries {
		size += indexEntryFixedSize + padTo4(e.NameLen)
	}

	return size
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

// safeReadU64 reads a little-endian uint64 from r into data, returning an
// error if the value exceeds maxVal. The caller must ensure data is non-nil.
func safeReadU64(r io.Reader, data *uint64, maxVal uint64) error {
	if data == nil {
		return errors.New("value is nil")
	}
	if err := binary.Read(r, binary.LittleEndian, data); err != nil {
		return err //nolint:wrapcheck
	}
	if *data > maxVal {
		return errors.New("value is too large")
	}

	return nil
}
