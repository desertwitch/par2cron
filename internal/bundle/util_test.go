package bundle

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const sha256Size = 32

// nopSeeker wraps a *bytes.Buffer to satisfy io.WriteSeeker with a no-op Seek.
type nopSeeker struct {
	*bytes.Buffer
}

func (nopSeeker) Seek(int64, int) (int64, error) { return 0, nil }

// memWriteSeeker is a minimal in-memory io.WriteSeeker for testing Seek behaviour.
type memWriteSeeker struct {
	pos int64
}

func (m *memWriteSeeker) Write(p []byte) (int, error) {
	m.pos += int64(len(p))

	return len(p), nil
}

func (m *memWriteSeeker) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		m.pos = offset
	case io.SeekCurrent:
		m.pos += offset
	case io.SeekEnd:
		m.pos += offset
	}

	return m.pos, nil
}

// Expectation: contextReader should pass through reads when the context is active.
func Test_contextReader_Read_Success(t *testing.T) {
	t.Parallel()

	cr := &contextReader{ctx: t.Context(), reader: strings.NewReader("hello")}

	buf := make([]byte, 5)
	n, err := cr.Read(buf)

	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "hello", string(buf))
}

// Expectation: contextReader should return a context error when the context is already cancelled.
func Test_contextReader_Read_Cancelled_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cr := &contextReader{ctx: ctx, reader: strings.NewReader("hello")}

	_, err := cr.Read(make([]byte, 5))

	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "context error")
}

// Expectation: contextReader should propagate EOF from the underlying reader.
func Test_contextReader_Read_EOF(t *testing.T) {
	t.Parallel()

	cr := &contextReader{ctx: t.Context(), reader: strings.NewReader("")}

	_, err := cr.Read(make([]byte, 1))

	require.ErrorIs(t, err, io.EOF)
}

// Expectation: contextReaderAt should pass through reads at a given offset when the context is active.
func Test_contextReaderAt_ReadAt_Success(t *testing.T) {
	t.Parallel()

	cr := &contextReaderAt{ctx: t.Context(), reader: bytes.NewReader([]byte("hello world"))}

	buf := make([]byte, 5)
	n, err := cr.ReadAt(buf, 6)

	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "world", string(buf))
}

// Expectation: contextReaderAt should return a context error when the context is already cancelled.
func Test_contextReaderAt_ReadAt_Cancelled_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cr := &contextReaderAt{ctx: ctx, reader: bytes.NewReader([]byte("hello"))}

	_, err := cr.ReadAt(make([]byte, 5), 0)

	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "context error")
}

// Expectation: contextReaderAt should propagate EOF when reading past the end of the underlying reader.
func Test_contextReaderAt_ReadAt_EOF(t *testing.T) {
	t.Parallel()

	cr := &contextReaderAt{ctx: t.Context(), reader: bytes.NewReader([]byte("hi"))}

	_, err := cr.ReadAt(make([]byte, 5), 0)

	require.ErrorIs(t, err, io.EOF)
}

// Expectation: contextWriteSeeker should pass through writes when the context is active.
func Test_contextWriteSeeker_Write_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ws := nopSeeker{&buf}
	cw := &contextWriteSeeker{ctx: t.Context(), writer: ws}

	n, err := cw.Write([]byte("hello"))

	require.NoError(t, err)
	require.Equal(t, 5, n)
	require.Equal(t, "hello", buf.String())
}

// Expectation: contextWriteSeeker should return a context error on Write when the context is already cancelled.
func Test_contextWriteSeeker_Write_Cancelled_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var buf bytes.Buffer
	cw := &contextWriteSeeker{ctx: ctx, writer: nopSeeker{&buf}}

	_, err := cw.Write([]byte("hello"))

	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "context error")
	require.Empty(t, buf.String())
}

// Expectation: contextWriteSeeker should pass through seeks when the context is active.
func Test_contextWriteSeeker_Seek_Success(t *testing.T) {
	t.Parallel()

	inner := &memWriteSeeker{}
	cw := &contextWriteSeeker{ctx: t.Context(), writer: inner}

	pos, err := cw.Seek(42, io.SeekStart)

	require.NoError(t, err)
	require.Equal(t, int64(42), pos)
}

// Expectation: contextWriteSeeker should return a context error on Seek when the context is already cancelled.
func Test_contextWriteSeeker_Seek_Cancelled_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	cw := &contextWriteSeeker{ctx: ctx, writer: &memWriteSeeker{}}

	_, err := cw.Seek(0, io.SeekStart)

	require.ErrorIs(t, err, context.Canceled)
	require.ErrorContains(t, err, "context error")
}

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
