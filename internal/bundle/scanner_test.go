package bundle

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type failingReaderAt struct {
	err error
}

func (r failingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	return 0, r.err
}

// Expectation: Scan should find intact file and manifest packets in a valid bundle.
func Test_Scan_Success(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	raw := readBundleBytes(t, fixture.fs, fixture.bundlePath)

	files, manifest := Scan(bytes.NewReader(raw), int64(len(raw)), true)

	require.NotNil(t, manifest)
	require.Len(t, files, len(fixture.files))
	require.Equal(t, fixture.manifest.Name, manifest.Name)
	require.Equal(t, []string{"alpha.par2", "zeta.vol00+01.par2"}, []string{files[0].Name, files[1].Name})
}

// Expectation: Scan should skip corrupt leading bytes and resume from the next magic sequence.
func Test_Scan_SkipsCorruptPrefix_Success(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	raw := append([]byte("junk!"), readBundleBytes(t, fixture.fs, fixture.bundlePath)...)

	files, manifest := Scan(bytes.NewReader(raw), int64(len(raw)), true)

	require.NotNil(t, manifest)
	require.Len(t, files, len(fixture.files))
	require.Positive(t, manifest.packetOffset)
	require.Positive(t, files[0].packetOffset)
}

// Expectation: findNextMagic should return the offset of the first matching magic sequence.
func Test_findNextMagic_Success(t *testing.T) {
	t.Parallel()

	data := append([]byte("prefix"), Magic[:]...)

	offset, err := findNextMagic(bytes.NewReader(data), 0, int64(len(data)))

	require.NoError(t, err)
	require.Equal(t, int64(6), offset)
}

// Expectation: findNextMagic should detect magic sequences that cross the scan buffer boundary.
func Test_findNextMagic_CrossChunkBoundary_Success(t *testing.T) {
	t.Parallel()

	prefix := bytes.Repeat([]byte{'x'}, 16*1024-len(Magic)+1)
	data := append(prefix, Magic[:]...) //nolint:gocritic

	offset, err := findNextMagic(bytes.NewReader(data), 0, int64(len(data)))

	require.NoError(t, err)
	require.Equal(t, int64(len(prefix)), offset)
}

// Expectation: findNextMagic should surface underlying ReaderAt errors encountered during scanning.
func Test_findNextMagic_ReadError_Error(t *testing.T) {
	t.Parallel()

	_, err := findNextMagic(failingReaderAt{err: errors.New("read boom")}, 0, commonHeaderSize)

	require.ErrorContains(t, err, "failed to io")
	require.ErrorContains(t, err, "read boom")
}

// Expectation: findNextMagic should return io.EOF when no further magic sequence exists.
func Test_findNextMagic_NotFound_Error(t *testing.T) {
	t.Parallel()

	_, err := findNextMagic(bytes.NewReader([]byte("plain bytes")), 0, int64(len("plain bytes")))

	require.ErrorIs(t, err, io.EOF)
}

// Expectation: reconstructIndex should sort scanned files by name and copy manifest metadata into the index.
func Test_reconstructIndex_SortsEntries_Success(t *testing.T) {
	t.Parallel()

	manifest := &ManifestPacket{
		CommonHeader: CommonHeader{RecoverySetID: testRecoverySetID},
		DataLength:   10,
		DataSHA256:   dataHash([]byte("manifest")),
		NameLen:      uint64(len("manifest.json")),
		Name:         "manifest.json",
		packetOffset: 400,
		dataOffset:   480,
	}

	files := []FilePacket{
		{
			DataLength:   3,
			DataSHA256:   dataHash([]byte("bbb")),
			NameLen:      uint64(len("z.par2")),
			Name:         "z.par2",
			packetOffset: 200,
			dataOffset:   264,
		},
		{
			DataLength:   5,
			DataSHA256:   dataHash([]byte("aaaaa")),
			NameLen:      uint64(len("a.par2")),
			Name:         "a.par2",
			packetOffset: 100,
			dataOffset:   164,
		},
	}

	got := reconstructIndex(manifest, files)

	require.Equal(t, testRecoverySetID, got.RecoverySetID)
	require.Equal(t, FlagIndexRebuilt, got.Flags)
	require.Equal(t, manifest.packetOffset, got.ManifestPacketOffset)
	require.Equal(t, []string{"a.par2", "z.par2"}, []string{got.Entries[0].Name, got.Entries[1].Name})
}
