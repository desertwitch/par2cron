package bundle

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

var (
	_ io.Reader      = (*contextReader)(nil)
	_ io.ReaderAt    = (*contextReaderAt)(nil)
	_ io.WriteSeeker = (*contextWriteSeeker)(nil)
)

// contextReader is an implementation of [io.Reader] that is Context-aware for
// receiving mid-transfer cancellation.
type contextReader struct {
	ctx    context.Context //nolint:containedctx
	reader io.Reader
}

// Read wraps the [io.Reader] reading function while being aware of and handling
// any mid-transfer Context cancellations.
func (cr *contextReader) Read(p []byte) (int, error) {
	select {
	case <-cr.ctx.Done():
		return 0, fmt.Errorf("context error: %w", cr.ctx.Err())
	default:
		return cr.reader.Read(p) //nolint:wrapcheck
	}
}

// contextReaderAt is an implementation of [io.ReaderAt] that is Context-aware for
// receiving mid-transfer cancellation.
type contextReaderAt struct {
	ctx    context.Context //nolint:containedctx
	reader io.ReaderAt
}

// ReadAt wraps the [io.ReaderAt] reading function while being aware of and handling
// any mid-transfer Context cancellations.
func (cr *contextReaderAt) ReadAt(p []byte, off int64) (int, error) {
	select {
	case <-cr.ctx.Done():
		return 0, fmt.Errorf("context error: %w", cr.ctx.Err())
	default:
		return cr.reader.ReadAt(p, off) //nolint:wrapcheck
	}
}

// contextWriteSeeker is an implementation of [io.WriteSeeker] that is Context-aware
// for receiving mid-transfer cancellation.
type contextWriteSeeker struct {
	ctx    context.Context //nolint:containedctx
	writer io.WriteSeeker
}

// Write wraps the [io.WriteSeeker] writing function while being aware of and handling
// any mid-transfer Context cancellations.
func (cw *contextWriteSeeker) Write(p []byte) (int, error) {
	select {
	case <-cw.ctx.Done():
		return 0, fmt.Errorf("context error: %w", cw.ctx.Err())
	default:
		return cw.writer.Write(p) //nolint:wrapcheck
	}
}

// Seek wraps the [io.WriteSeeker] seeking function while being aware of and handling
// any mid-transfer Context cancellations.
func (cw *contextWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	select {
	case <-cw.ctx.Done():
		return 0, fmt.Errorf("context error: %w", cw.ctx.Err())
	default:
		return cw.writer.Seek(offset, whence) //nolint:wrapcheck
	}
}

// dataHash computes the SHA256 data integrity hash from a byte slice.
func dataHash(data []byte) [32]byte {
	return sha256.Sum256(data)
}

// dataHashReader computes the SHA256 data integrity hash streaming from r.
func dataHashReader(r io.Reader) ([32]byte, error) {
	h := sha256.New()
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
