package bundle

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: Unpack should extract every bundled file and the manifest with exact bytes.
func Test_Bundle_Unpack_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	_, err := b.Unpack(fixture.fs, "/out", true)
	require.NoError(t, err)

	for name, want := range fixture.files {
		got, err := afero.ReadFile(fixture.fs, "/out/"+name)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}

	gotManifest, err := afero.ReadFile(fixture.fs, "/out/"+fixture.manifest.Name)
	require.NoError(t, err)
	require.Equal(t, fixture.manifest.Bytes, gotManifest)
}

// Expectation: Unpack should continue past extraction failures and join the resulting errors.
func Test_Bundle_Unpack_JoinedErrors_Error(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))
	require.NoError(t, afero.WriteFile(fixture.fs, "/out/"+b.Index.Entries[0].Name, []byte("existing"), 0o644))
	require.NoError(t, afero.WriteFile(fixture.fs, "/out/"+fixture.manifest.Name, []byte("existing"), 0o644))

	_, err := b.Unpack(fixture.fs, "/out", true)

	require.Error(t, err)
	require.ErrorContains(t, err, b.Index.Entries[0].Name)
	require.ErrorContains(t, err, "manifest")
}

// Expectation: Unpack should reject entry names that traverse outside the destination directory.
func Test_Bundle_Unpack_PathTraversal_Error(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	b.Index.Entries[0].Name = "../../etc/passwd"

	paths, err := b.Unpack(fixture.fs, "/out", true)

	require.Error(t, err)
	require.ErrorContains(t, err, "escapes destination directory")
	require.NotContains(t, paths, "/etc/passwd")
}

// Expectation: Unpack should reject manifest names that traverse outside the destination directory.
func Test_Bundle_Unpack_ManifestPathTraversal_Error(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	b.Index.ManifestName = "../manifest.json"

	paths, err := b.Unpack(fixture.fs, "/out", true)

	require.Error(t, err)
	require.ErrorContains(t, err, "escapes destination directory")
	for _, p := range paths {
		require.NotContains(t, p, "manifest.json")
	}
}

// Expectation: Unpack should continue extracting remaining entries after a path traversal rejection.
func Test_Bundle_Unpack_PathTraversalContinuesOthers(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))
	require.Greater(t, len(b.Index.Entries), 1, "fixture must have at least two entries")

	b.Index.Entries[0].Name = "../../escape"

	paths, err := b.Unpack(fixture.fs, "/out", true)

	require.Error(t, err)
	require.NotEmpty(t, paths, "valid entries should still be extracted")
}

// Expectation: Unpack should return all extracted paths on success.
func Test_Bundle_Unpack_ReturnsPaths(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	paths, err := b.Unpack(fixture.fs, "/out", true)

	require.NoError(t, err)
	require.Len(t, paths, len(b.Index.Entries)+1) // entries + manifest
	for _, e := range b.Index.Entries {
		require.Contains(t, paths, filepath.Join("/out", e.Name))
	}
	require.Contains(t, paths, filepath.Join("/out", b.Index.ManifestName))
}

// Expectation: Unpack strict should exclude failed entries from the returned paths.
func Test_Bundle_Unpack_Strict_ExcludesFailedPaths(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	// Block the first entry by pre-creating it (O_EXCL will fail).
	require.NoError(t, afero.WriteFile(fixture.fs, "/out/"+b.Index.Entries[0].Name, []byte("existing"), 0o644))

	paths, err := b.Unpack(fixture.fs, "/out", true)

	require.Error(t, err)
	require.NotContains(t, paths, filepath.Join("/out", b.Index.Entries[0].Name))
}

// Expectation: Unpack non-strict should include corrupt entries in the returned paths.
func Test_Bundle_Unpack_NonStrict_IncludesCorruptPaths(t *testing.T) {
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

	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	paths, err := b.Unpack(fixture.fs, "/out", false)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrDataCorrupt)
	require.Contains(t, paths, filepath.Join("/out", b.Index.Entries[0].Name))
}

// Expectation: Unpack strict should exclude corrupt entries from the returned paths.
func Test_Bundle_Unpack_Strict_ExcludesCorruptPaths(t *testing.T) {
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

	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	paths, err := b.Unpack(fixture.fs, "/out", true)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrDataCorrupt)
	require.NotContains(t, paths, filepath.Join("/out", b.Index.Entries[0].Name))
}

// Expectation: ExtractEntry should write the full file payload from the referenced offset.
func Test_Bundle_ExtractEntry_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	entry := b.Index.Entries[0]

	var buf bytes.Buffer
	err := b.ExtractEntry(entry, &buf)

	require.NoError(t, err)
	require.Equal(t, fixture.files[entry.Name], buf.Bytes())
}

// Expectation: ExtractEntry should return ErrDataCorrupt after copying bytes when the data hash mismatches.
func Test_Bundle_ExtractEntry_HashMismatch_Error(t *testing.T) {
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

	var buf bytes.Buffer
	err = b.ExtractEntry(b.Index.Entries[0], &buf)

	require.ErrorIs(t, err, ErrDataCorrupt)
	require.NotEmpty(t, buf.Bytes())
}

// Expectation: ExtractEntry should surface writer I/O failures while streaming data.
func Test_Bundle_ExtractEntry_WriterError_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	entry := b.Index.Entries[0]
	w := &limitedWriter{
		remaining: int(entry.DataLength / 2), //nolint:gosec
		err:       errors.New("writer boom"),
	}

	err := b.ExtractEntry(entry, w)

	require.ErrorContains(t, err, "failed to io")
	require.ErrorContains(t, err, "writer boom")
	require.NotEmpty(t, w.buf.Bytes())
}

// Expectation: ExtractManifest should write the full manifest bytes on success.
func Test_Bundle_ExtractManifest_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)

	var buf bytes.Buffer
	err := b.ExtractManifest(&buf)

	require.NoError(t, err)
	require.Equal(t, fixture.manifest.Bytes, buf.Bytes())
}

// Expectation: ExtractManifest should return ErrDataCorrupt after copying bytes when the manifest hash mismatches.
func Test_Bundle_ExtractManifest_HashMismatch_Error(t *testing.T) {
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

	var buf bytes.Buffer
	err = b.ExtractManifest(&buf)

	require.ErrorIs(t, err, ErrDataCorrupt)
	require.ErrorContains(t, err, "failed to validate")
	require.NotEmpty(t, buf.Bytes())
	require.NotEqual(t, fixture.manifest.Bytes, buf.Bytes())
}

// Expectation: ExtractManifest should surface writer I/O failures.
func Test_Bundle_ExtractManifest_WriterError_Error(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	w := &limitedWriter{
		remaining: len(fixture.manifest.Bytes) / 2,
		err:       errors.New("writer boom"),
	}

	err := b.ExtractManifest(w)

	require.ErrorContains(t, err, "failed to io")
	require.ErrorContains(t, err, "writer boom")
	require.NotEmpty(t, w.buf.Bytes())
}

// Expectation: extractToFile should persist the extracted bytes on success.
func Test_extractToFile_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	err := extractToFile(fs, "/out.bin", func(w io.Writer) error {
		_, writeErr := w.Write([]byte("payload"))

		return writeErr
	}, true)

	require.NoError(t, err)

	got, readErr := afero.ReadFile(fs, "/out.bin")
	require.NoError(t, readErr)
	require.Equal(t, []byte("payload"), got)
}

// Expectation: extractToFile should return an error when the output file cannot be created.
func Test_extractToFile_CreateFails_Error(t *testing.T) {
	t.Parallel()

	fs := &testFs{
		Fs: afero.NewMemMapFs(),
		openFileFunc: func(name string, flag int, perm os.FileMode) (afero.File, error) {
			return nil, errors.New("create boom")
		},
	}

	err := extractToFile(fs, "/out.bin", func(w io.Writer) error {
		return nil
	}, true)

	require.ErrorContains(t, err, "failed to create")
	require.ErrorContains(t, err, "create boom")
}

// Expectation: extractToFile should remove the output file for non-corruption extraction failures even when strict is disabled.
func Test_extractToFile_GenericExtractErrorRemovesFile_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	err := extractToFile(fs, "/out.bin", func(w io.Writer) error {
		_, writeErr := w.Write([]byte("payload"))
		require.NoError(t, writeErr)

		return errors.New("extract boom")
	}, false)

	require.ErrorContains(t, err, "extract boom")

	exists, existsErr := afero.Exists(fs, "/out.bin")
	require.NoError(t, existsErr)
	require.False(t, exists)
}

// Expectation: extractToFile should keep corrupt-but-complete output when strict mode is disabled.
func Test_extractToFile_StrictFalseKeepsCorruptData_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	err := extractToFile(fs, "/out.bin", func(w io.Writer) error {
		_, writeErr := w.Write([]byte("payload"))
		require.NoError(t, writeErr)

		return ErrDataCorrupt
	}, false)

	require.ErrorIs(t, err, ErrDataCorrupt)

	got, readErr := afero.ReadFile(fs, "/out.bin")
	require.NoError(t, readErr)
	require.Equal(t, []byte("payload"), got)
}

// Expectation: extractToFile should remove corrupt output when strict mode is enabled.
func Test_extractToFile_StrictTrueRemovesCorruptData_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	err := extractToFile(fs, "/out.bin", func(w io.Writer) error {
		_, writeErr := w.Write([]byte("payload"))
		require.NoError(t, writeErr)

		return ErrDataCorrupt
	}, true)

	require.ErrorIs(t, err, ErrDataCorrupt)

	exists, existsErr := afero.Exists(fs, "/out.bin")
	require.NoError(t, existsErr)
	require.False(t, exists)
}

// Expectation: extractToFile should remove the output file when Sync fails.
func Test_extractToFile_SyncFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		openFileFunc: func(name string, flag int, perm os.FileMode) (afero.File, error) {
			f, err := base.OpenFile(name, flag, perm)
			require.NoError(t, err)

			return &testFile{
				File: f,
				syncFunc: func() error {
					return errors.New("sync boom")
				},
			}, nil
		},
	}

	err := extractToFile(fs, "/out.bin", func(w io.Writer) error {
		_, writeErr := w.Write([]byte("payload"))

		return writeErr
	}, true)

	require.ErrorContains(t, err, "failed to sync")

	exists, existsErr := afero.Exists(base, "/out.bin")
	require.NoError(t, existsErr)
	require.False(t, exists)
}

// Expectation: extractToFile should remove the output file when Close fails.
func Test_extractToFile_CloseFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		openFileFunc: func(name string, flag int, perm os.FileMode) (afero.File, error) {
			f, err := base.OpenFile(name, flag, perm)
			require.NoError(t, err)

			return &testFile{
				File: f,
				closeFunc: func() error {
					_ = f.Close()

					return errors.New("close boom")
				},
			}, nil
		},
	}

	err := extractToFile(fs, "/out.bin", func(w io.Writer) error {
		_, writeErr := w.Write([]byte("payload"))

		return writeErr
	}, true)

	require.ErrorContains(t, err, "failed to close")

	exists, existsErr := afero.Exists(base, "/out.bin")
	require.NoError(t, existsErr)
	require.False(t, exists)
}

// Expectation: safePath should accept simple relative names.
func Test_safePath_Valid(t *testing.T) {
	t.Parallel()

	path, err := safePath("/dest", "subdir/file.txt")

	require.NoError(t, err)
	require.Equal(t, filepath.Join("/dest", "subdir/file.txt"), path)
}

// Expectation: safePath should accept misleading relative names.
func Test_safePath_MisleadingName_Valid(t *testing.T) {
	t.Parallel()

	path, err := safePath("/dest", "subdir/..file.txt")

	require.NoError(t, err)
	require.Equal(t, filepath.Join("/dest", "subdir/..file.txt"), path)
}

// Expectation: safePath should reject parent traversal.
func Test_safePath_ParentTraversal_Error(t *testing.T) {
	t.Parallel()

	_, err := safePath("/dest", "../outside.txt")

	require.Error(t, err)
	require.ErrorContains(t, err, "escapes destination directory")
}

// Expectation: safePath should reject parent-directory traversal.
func Test_safePath_DotDot_Error(t *testing.T) {
	t.Parallel()

	_, err := safePath("/dest", "../outside")

	require.Error(t, err)
	require.ErrorContains(t, err, "escapes destination directory")
}

// Expectation: safePath should reject an exact parent-directory relative path ("..").
func Test_safePath_ExactDotDot_Error(t *testing.T) {
	t.Parallel()

	_, err := safePath("/dest", "..")

	require.Error(t, err)
	require.ErrorContains(t, err, "escapes destination directory")
}
