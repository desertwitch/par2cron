package bundle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

const sha256Size = 32

// Expectation: dataHash should return the SHA256 hash of the provided byte slice.
func Test_dataHash_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, dataHash([]byte("payload")), dataHash([]byte("payload"))) //nolint:testifylint
	require.NotEqual(t, dataHash([]byte("payload")), dataHash([]byte("other")))
}

// Expectation: dataHashReader should hash streamed data the same way as dataHash.
func Test_dataHashReader_Success(t *testing.T) {
	t.Parallel()

	want := dataHash([]byte("payload"))

	got, err := dataHashReader(bytes.NewReader([]byte("payload")))

	require.NoError(t, err)
	require.Equal(t, want, got)
}

// Expectation: dataHashReader should surface reader I/O errors.
func Test_dataHashReader_IO_Error(t *testing.T) {
	t.Parallel()

	_, err := dataHashReader(errReader{err: errors.New("read boom")})

	require.ErrorContains(t, err, "failed to io")
	require.ErrorContains(t, err, "read boom")
}

// Expectation: computeIndexSize should include the padded manifest name and all padded entry names.
func Test_computeIndexSize_Success(t *testing.T) {
	t.Parallel()

	got := computeIndexSize(ManifestInput{Name: "manifest.json"}, []FileInput{
		{Name: "alpha.par2"},
		{Name: "z.par2"},
	})

	want := uint64(commonHeaderSize) + indexFixedSize +
		padTo4(uint64(len("manifest.json"))) +
		indexEntryFixedSize + padTo4(uint64(len("alpha.par2"))) +
		indexEntryFixedSize + padTo4(uint64(len("z.par2")))

	require.Equal(t, want, got)
}

// Expectation: computeIndexSizeFromEntries should size the index from manifest and entry name metadata.
func Test_computeIndexSizeFromEntries_Success(t *testing.T) {
	t.Parallel()

	got := computeIndexSizeFromEntries("manifest.json", []IndexEntry{
		{NameLen: uint64(len("alpha.par2"))},
		{NameLen: uint64(len("beta.par2"))},
	})

	want := uint64(commonHeaderSize) + indexFixedSize +
		padTo4(uint64(len("manifest.json"))) +
		indexEntryFixedSize + padTo4(uint64(len("alpha.par2"))) +
		indexEntryFixedSize + padTo4(uint64(len("beta.par2")))

	require.Equal(t, want, got)
}

// Expectation: padTo4 should round values up to the next 4-byte boundary.
func Test_padTo4_Success(t *testing.T) {
	t.Parallel()

	require.Equal(t, uint64(0), padTo4(0))
	require.Equal(t, uint64(4), padTo4(1))
	require.Equal(t, uint64(4), padTo4(4))
	require.Equal(t, uint64(8), padTo4(5))
}

// Expectation: isAligned4 should report whether a value is already 4-byte aligned.
func Test_isAligned4_Success(t *testing.T) {
	t.Parallel()

	require.True(t, isAligned4(0))
	require.True(t, isAligned4(8))
	require.False(t, isAligned4(6))
}

// Expectation: safeReadU64 should decode a little-endian uint64 into the provided destination.
func Test_safeReadU64_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, binary.Write(&buf, binary.LittleEndian, uint64(42)))

	var got uint64
	require.NoError(t, safeReadU64(bytes.NewReader(buf.Bytes()), &got, 99))
	require.Equal(t, uint64(42), got)
}

// Expectation: safeReadU64 should reject a nil destination pointer.
func Test_safeReadU64_Nil_Error(t *testing.T) {
	t.Parallel()

	require.ErrorContains(t, safeReadU64(bytes.NewReader(nil), nil, 1), "value is nil")
}

// Expectation: safeReadU64 should reject values above the configured maximum.
func Test_safeReadU64_TooLarge_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, binary.Write(&buf, binary.LittleEndian, uint64(100)))

	var got uint64
	require.ErrorContains(t, safeReadU64(bytes.NewReader(buf.Bytes()), &got, 99), "value is too large")
}

// Expectation: safeReadU64 should surface binary read failures from the underlying reader.
func Test_safeReadU64_ReadError_Error(t *testing.T) {
	t.Parallel()

	var got uint64
	err := safeReadU64(bytes.NewReader([]byte{0x01}), &got, math.MaxUint64)

	require.Error(t, err)
}
