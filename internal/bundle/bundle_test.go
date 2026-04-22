package bundle

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

var testRecoverySetID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

type testBundleFixture struct {
	fs         afero.Fs
	bundlePath string
	manifest   ManifestInput
	files      map[string][]byte
}

type testFs struct {
	afero.Fs

	openFunc     func(name string) (afero.File, error)
	openFileFunc func(name string, flag int, perm os.FileMode) (afero.File, error)
	createFunc   func(name string) (afero.File, error)
}

func (f *testFs) Open(name string) (afero.File, error) {
	if f.openFunc != nil {
		return f.openFunc(name)
	}

	return f.Fs.Open(name)
}

func (f *testFs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if f.openFileFunc != nil {
		return f.openFileFunc(name, flag, perm)
	}

	return f.Fs.OpenFile(name, flag, perm)
}

func (f *testFs) Create(name string) (afero.File, error) {
	if f.createFunc != nil {
		return f.createFunc(name)
	}

	return f.Fs.Create(name)
}

type testFile struct {
	afero.File

	statFunc     func() (os.FileInfo, error)
	syncFunc     func() error
	closeFunc    func() error
	seekFunc     func(offset int64, whence int) (int64, error)
	truncateFunc func(size int64) error
}

func (f *testFile) Stat() (os.FileInfo, error) {
	if f.statFunc != nil {
		return f.statFunc()
	}

	return f.File.Stat()
}

func (f *testFile) Sync() error {
	if f.syncFunc != nil {
		return f.syncFunc()
	}

	return f.File.Sync()
}

func (f *testFile) Close() error {
	if f.closeFunc != nil {
		return f.closeFunc()
	}

	return f.File.Close()
}

func (f *testFile) Seek(offset int64, whence int) (int64, error) {
	if f.seekFunc != nil {
		return f.seekFunc(offset, whence)
	}

	return f.File.Seek(offset, whence)
}

func (f *testFile) Truncate(size int64) error {
	if f.truncateFunc != nil {
		return f.truncateFunc(size)
	}

	return f.File.Truncate(size)
}

type callbackFile struct {
	afero.File

	readAtFunc func(p []byte, off int64) (int, error)
	writeFunc  func(p []byte) (int, error)
	seekFunc   func(offset int64, whence int) (int64, error)
	syncFunc   func() error
	statFunc   func() (os.FileInfo, error)
}

func (f *callbackFile) ReadAt(p []byte, off int64) (int, error) {
	if f.readAtFunc != nil {
		return f.readAtFunc(p, off)
	}

	return f.File.ReadAt(p, off)
}

func (f *callbackFile) Write(p []byte) (int, error) {
	if f.writeFunc != nil {
		return f.writeFunc(p)
	}

	return f.File.Write(p)
}

func (f *callbackFile) Seek(offset int64, whence int) (int64, error) {
	if f.seekFunc != nil {
		return f.seekFunc(offset, whence)
	}

	return f.File.Seek(offset, whence)
}

func (f *callbackFile) Sync() error {
	if f.syncFunc != nil {
		return f.syncFunc()
	}

	return f.File.Sync()
}

func (f *callbackFile) Stat() (os.FileInfo, error) {
	if f.statFunc != nil {
		return f.statFunc()
	}

	return f.File.Stat()
}

type fileInfoWithSize struct {
	os.FileInfo

	size int64
}

func (fi fileInfoWithSize) Size() int64 {
	return fi.size
}

type errReader struct {
	err error
}

func (r errReader) Read(p []byte) (int, error) {
	return 0, r.err
}

type limitedWriter struct {
	buf bytes.Buffer

	remaining int
	err       error
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	if w.remaining <= 0 {
		return 0, w.err
	}

	n := min(w.remaining, len(p))
	if n > 0 {
		_, _ = w.buf.Write(p[:n])
		w.remaining -= n
	}

	if n < len(p) {
		return n, w.err
	}

	return n, nil
}

func newTestBundleFixture(t *testing.T) testBundleFixture {
	t.Helper()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/src", 0o755))
	require.NoError(t, fs.MkdirAll("/bundles", 0o755))

	files := map[string][]byte{
		"zeta.vol00+01.par2": {1, 2, 3},
		"alpha.par2":         []byte("hello"),
	}

	inputs := make([]FileInput, 0, len(files))
	for name, data := range files {
		path := "/src/" + name
		require.NoError(t, afero.WriteFile(fs, path, data, 0o644))
		inputs = append(inputs, FileInput{Name: name, Path: path})
	}

	manifest := ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"version":1,"items":["alpha","zeta"]}`),
	}

	bundlePath := "/bundles/sample.p2c.par2"
	require.NoError(t, Pack(fs, bundlePath, testRecoverySetID, manifest, inputs))

	return testBundleFixture{
		fs:         fs,
		bundlePath: bundlePath,
		manifest:   manifest,
		files:      files,
	}
}

func openTestBundle(t *testing.T) (*Bundle, testBundleFixture) {
	t.Helper()

	fixture := newTestBundleFixture(t)
	b, err := Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	return b, fixture
}

func readBundleBytes(t *testing.T, fs afero.Fs, path string) []byte {
	t.Helper()

	raw, err := afero.ReadFile(fs, path)
	require.NoError(t, err)

	return raw
}

func overwriteBundleBytes(t *testing.T, fs afero.Fs, path string, mutate func([]byte)) {
	t.Helper()

	raw := readBundleBytes(t, fs, path)
	mutate(raw)
	require.NoError(t, afero.WriteFile(fs, path, raw, 0o644))
}

func verifyDataAtOffset(t *testing.T, raw []byte, offset, length uint64, want []byte) {
	t.Helper()

	require.Equal(t, uint64(len(want)), length)
	require.Equal(t, want, raw[offset:offset+length])
}

// Expectation: Opening a valid bundle should parse the index and expose the manifest and entries.
func Test_Open_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)

	require.Equal(t, testRecoverySetID, b.Index.RecoverySetID)
	require.Equal(t, fixture.manifest.Name, b.Index.ManifestName)
	require.Len(t, b.Index.Entries, len(fixture.files))
	require.NoError(t, b.Validate(true))
	require.NoError(t, b.ValidateIndex())
}

// Expectation: Open should return an error when the bundle cannot be opened.
func Test_Open_OpenFails_Error(t *testing.T) {
	t.Parallel()

	_, err := Open(afero.NewMemMapFs(), "/missing.par2")

	require.ErrorContains(t, err, "failed to open")
}

// Expectation: Open should return an error when the file stat call fails after opening.
func Test_Open_StatFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/bundle.par2", []byte("data"), 0o644))

	fs := &testFs{
		Fs: base,
		openFileFunc: func(name string, flag int, perm os.FileMode) (afero.File, error) {
			f, err := base.OpenFile(name, flag, perm)
			require.NoError(t, err)

			return &testFile{
				File: f,
				statFunc: func() (os.FileInfo, error) {
					return nil, errors.New("stat boom")
				},
			}, nil
		},
	}

	_, err := Open(fs, "/bundle.par2")

	require.ErrorContains(t, err, "failed to stat")
}

// Expectation: Open should reject a file with a negative reported size.
func Test_Open_NegativeFileSize_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/bundle.par2", []byte("data"), 0o644))

	fs := &testFs{
		Fs: base,
		openFileFunc: func(name string, flag int, perm os.FileMode) (afero.File, error) {
			f, err := base.OpenFile(name, flag, perm)
			require.NoError(t, err)

			return &testFile{
				File: f,
				statFunc: func() (os.FileInfo, error) {
					info, err := f.Stat()
					require.NoError(t, err)

					return fileInfoWithSize{FileInfo: info, size: -1}, nil
				},
			}, nil
		},
	}

	_, err := Open(fs, "/bundle.par2")

	require.ErrorContains(t, err, "file size < 0")
}

// Expectation: Open should reconstruct the index from intact packets when the on-disk index is corrupt.
func Test_Open_ReconstructsIndex_Success(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[0] ^= 0xFF
	})

	b, err := Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	require.ErrorContains(t, b.OpenError, "index reconstructed")
	require.NotZero(t, b.Index.Flags&FlagIndexRebuilt)
	require.Len(t, b.Index.Entries, len(fixture.files))
	require.Equal(t, fixture.manifest.Name, b.Index.ManifestName)
}

// Expectation: Open should report irrecoverable corruption when no intact manifest can be found.
func Test_Open_TooDamaged_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/bundle.par2", []byte("broken"), 0o644))

	_, err := Open(fs, "/bundle.par2")

	require.ErrorIs(t, err, ErrDataCorrupt)
	require.ErrorContains(t, err, "bundle too damaged")
}

// Expectation: Close should forward file close errors.
func Test_Bundle_Close_Error(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	orig := b.f
	b.f = &testFile{
		File: orig,
		closeFunc: func() error {
			_ = orig.Close()

			return errors.New("close boom")
		},
	}

	require.ErrorContains(t, b.Close(), "close boom")
}

// Expectation: Manifest should return the manifest bytes from a valid bundle.
func Test_Bundle_Manifest_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)

	got, err := b.Manifest()

	require.NoError(t, err)
	require.Equal(t, fixture.manifest.Bytes, got)
}

// Expectation: Manifest should still return the extracted bytes when hash validation fails.
func Test_Bundle_Manifest_ExtractFails_Error(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[b.Index.ManifestDataOffset] ^= 0xFF
	})

	b, err = Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	got, err := b.Manifest()

	require.ErrorContains(t, err, "failed to extract")
	require.ErrorIs(t, err, ErrDataCorrupt)
	require.NotEmpty(t, got)
	require.NotEqual(t, fixture.manifest.Bytes, got)
}

// Expectation: Validate should succeed on a clean bundle.
func Test_Bundle_Validate_Success(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)

	require.NoError(t, b.Validate(true))
}

// Expectation: Validate should stop at index validation failures before checking files or manifest.
func Test_Bundle_Validate_IndexError_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	b.OpenError = errors.New("rebuilt index")

	err := b.Validate(false)

	require.ErrorContains(t, err, "index: rebuilt index")
}

// Expectation: Validate should return a files-prefixed error when file validation fails.
func Test_Bundle_Validate_FilesError_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	b.Index.Entries[0].PacketOffset = b.Index.ManifestPacketOffset

	err := b.Validate(false)

	require.ErrorContains(t, err, "files:")
	require.ErrorContains(t, err, "expected file type")
}

// Expectation: Validate should return a manifest-prefixed error when manifest validation fails.
func Test_Bundle_Validate_ManifestError_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	b.Index.ManifestPacketOffset = b.Index.Entries[0].PacketOffset

	err := b.Validate(false)

	require.ErrorContains(t, err, "manifest:")
	require.ErrorContains(t, err, "expected manifest type")
}

// Expectation: ValidateFiles should surface packet read failures from the indexed packet offset.
func Test_Bundle_ValidateFiles_ReadPacketFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	b.Index.Entries[0].PacketOffset = uint64(b.size) //nolint:gosec

	err := b.ValidateFiles(false)

	require.ErrorContains(t, err, "file packet 0")
	require.ErrorContains(t, err, "unexpected EOF")
}

// Expectation: ValidateFiles should surface hash-read I/O failures in strict mode.
func Test_Bundle_ValidateFiles_DataHashReadFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	entry := b.Index.Entries[0]
	orig := b.f
	b.f = &callbackFile{
		File: orig,
		readAtFunc: func(p []byte, off int64) (int, error) {
			if off >= int64(entry.DataOffset) { //nolint:gosec
				return 0, errors.New("read boom")
			}

			return orig.ReadAt(p, off)
		},
	}

	err := b.ValidateFiles(true)

	require.ErrorContains(t, err, "hash error")
	require.ErrorContains(t, err, "read boom")
}

// Expectation: ValidateFiles should detect strict data hash mismatches.
func Test_Bundle_ValidateFiles_StrictHashMismatch_Error(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[b.Index.Entries[0].DataOffset] ^= 0xFF
	})

	b, err = Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	require.NoError(t, b.ValidateFiles(false))
	require.ErrorContains(t, b.ValidateFiles(true), "hash mismatch")
}

// Expectation: ValidateFiles should reject entries that point at a non-file packet.
func Test_Bundle_ValidateFiles_WrongPacketType_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	b.Index.Entries[0].PacketOffset = b.Index.ManifestPacketOffset

	err := b.ValidateFiles(false)

	require.ErrorContains(t, err, "expected file type")
}

// Expectation: ValidateManifest should detect strict manifest hash mismatches.
func Test_Bundle_ValidateManifest_StrictHashMismatch_Error(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[b.Index.ManifestDataOffset] ^= 0xFF
	})

	b, err = Open(fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	require.Error(t, b.Validate(true))
}

// Expectation: ValidateManifest should reject a manifest offset that points at a file packet.
func Test_Bundle_ValidateManifest_WrongPacketType_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	b.Index.ManifestPacketOffset = b.Index.Entries[0].PacketOffset

	err := b.ValidateManifest(false)

	require.ErrorContains(t, err, "expected manifest type")
}

// Expectation: ValidateManifest should surface hash-read I/O failures in strict mode.
func Test_Bundle_ValidateManifest_DataHashReadFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	b.f = &callbackFile{
		File: orig,
		readAtFunc: func(p []byte, off int64) (int, error) {
			if off >= int64(b.Index.ManifestDataOffset) { //nolint:gosec
				return 0, errors.New("read boom")
			}

			return orig.ReadAt(p, off)
		},
	}

	err := b.ValidateManifest(true)

	require.ErrorContains(t, err, "hash error")
	require.ErrorContains(t, err, "read boom")
}

// Expectation: ValidateManifest should detect strict manifest hash mismatches.
func Test_Bundle_ValidateManifest_HashMismatch_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)

	badHash := b.Index.ManifestDataSHA256
	badHash[0] ^= 0xFF
	b.Index.ManifestDataSHA256 = badHash

	err := b.ValidateManifest(true)

	require.ErrorContains(t, err, "manifest data at offset")
	require.ErrorContains(t, err, "hash mismatch")
	require.ErrorIs(t, err, ErrDataCorrupt)
}

// Expectation: ValidateManifest should surface packet-read failures from the indexed manifest packet offset.
func Test_Bundle_ValidateManifest_ReadPacketFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	b.Index.ManifestPacketOffset = uint64(b.size) //nolint:gosec

	err := b.ValidateManifest(false)

	require.ErrorContains(t, err, "manifest packet at offset")
	require.ErrorContains(t, err, "unexpected EOF")
}

// Expectation: readIndexPacket should parse a valid on-disk index packet at offset zero.
func Test_Bundle_readIndexPacket_Success(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)

	f, err := fixture.fs.OpenFile(fixture.bundlePath, os.O_RDWR, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	fi, err := f.Stat()
	require.NoError(t, err)

	b := &Bundle{
		f:    f,
		size: fi.Size(),
	}

	require.NoError(t, b.readIndexPacket())

	require.Equal(t, testRecoverySetID, b.Index.RecoverySetID)
	require.Equal(t, fixture.manifest.Name, b.Index.ManifestName)
	require.Len(t, b.Index.Entries, len(fixture.files))
}

// Expectation: readIndexPacket should reject a packet of the wrong type at offset zero.
func Test_Bundle_readIndexPacket_WrongType_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	entry, err := writeManifestPacket(&buf, testRecoverySetID, ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"ok":true}`),
	}, 0)
	require.NoError(t, err)
	require.NotZero(t, entry.dataOffset)

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/bundle.par2", buf.Bytes(), 0o644))

	f, err := fs.OpenFile("/bundle.par2", os.O_RDWR, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	b := &Bundle{f: f, size: int64(buf.Len())}

	require.ErrorContains(t, b.readIndexPacket(), fmt.Sprintf("expected index packet at offset 0, got %q", PacketTypeManifest))
}

// Expectation: readIndexPacket should report parse errors for malformed index packet bodies.
func Test_Bundle_readIndexPacket_ParseFails_Error(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeCommonPacket(&buf, testRecoverySetID, PacketTypeIndex, nil))

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/bundle.par2", buf.Bytes(), 0o644))

	f, err := fs.OpenFile("/bundle.par2", os.O_RDWR, 0)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, f.Close())
	})

	b := &Bundle{f: f, size: int64(buf.Len())}

	require.ErrorContains(t, b.readIndexPacket(), "failed to parse packet")
}
