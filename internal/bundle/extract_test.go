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

	_, err := b.Unpack(t.Context(), fixture.fs, "/out", true)
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

	_, err := b.Unpack(t.Context(), fixture.fs, "/out", true)

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

	paths, err := b.Unpack(t.Context(), fixture.fs, "/out", true)

	require.Error(t, err)
	require.ErrorContains(t, err, "invalid path")
	require.NotContains(t, paths, "/etc/passwd")
	require.NotErrorIs(t, err, ErrDataCorrupt)
}

// Expectation: Unpack should reject manifest names that traverse outside the destination directory.
func Test_Bundle_Unpack_ManifestPathTraversal_Error(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	b.Index.ManifestName = "../manifest.json"

	paths, err := b.Unpack(t.Context(), fixture.fs, "/out", true)

	require.Error(t, err)
	require.ErrorContains(t, err, "invalid path")
	for _, p := range paths {
		require.NotContains(t, p, "manifest.json")
	}
	require.NotErrorIs(t, err, ErrDataCorrupt)
}

// Expectation: Unpack should continue extracting remaining entries after a path traversal rejection.
func Test_Bundle_Unpack_PathTraversalContinuesOthers_Error(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))
	require.Greater(t, len(b.Index.Entries), 1, "fixture must have at least two entries")

	b.Index.Entries[0].Name = "../../escape"

	paths, err := b.Unpack(t.Context(), fixture.fs, "/out", true)

	require.ErrorContains(t, err, "invalid path")
	require.NotErrorIs(t, err, ErrDataCorrupt)

	// All other entries plus the manifest, but nothing for the rejected one.
	require.Len(t, paths, len(b.Index.Entries))
	require.NotContains(t, paths, "escape")
	for _, e := range b.Index.Entries[1:] {
		require.Contains(t, paths, filepath.Join("/out", e.Name))
	}

	exists, err := afero.Exists(fixture.fs, "/escape")
	require.NoError(t, err)
	require.False(t, exists)
}

// Expectation: Unpack should return all extracted paths on success.
func Test_Bundle_Unpack_ReturnsPaths(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	paths, err := b.Unpack(t.Context(), fixture.fs, "/out", true)

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

	paths, err := b.Unpack(t.Context(), fixture.fs, "/out", true)

	require.Error(t, err)
	require.NotContains(t, paths, filepath.Join("/out", b.Index.Entries[0].Name))
}

// Expectation: Unpack non-strict should include corrupt entries in the returned paths.
func Test_Bundle_Unpack_NonStrict_IncludesCorruptPaths(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[b.Index.Entries[0].DataOffset] ^= 0xFF
	})

	b, err = Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	paths, err := b.Unpack(t.Context(), fixture.fs, "/out", false)

	require.Error(t, err)
	require.ErrorIs(t, err, ErrDataCorrupt)
	require.Contains(t, paths, filepath.Join("/out", b.Index.Entries[0].Name))
}

// Expectation: Unpack strict should exclude corrupt entries from the returned paths.
func Test_Bundle_Unpack_Strict_ExcludesCorruptPaths(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[b.Index.Entries[0].DataOffset] ^= 0xFF
	})

	b, err = Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	require.NoError(t, fixture.fs.MkdirAll("/out", 0o755))

	paths, err := b.Unpack(t.Context(), fixture.fs, "/out", true)

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
	err := b.ExtractEntry(t.Context(), entry, &buf)

	require.NoError(t, err)
	require.Equal(t, fixture.files[entry.Name], buf.Bytes())
}

// Expectation: ExtractEntry should return ErrDataCorrupt after copying bytes when the data hash mismatches.
func Test_Bundle_ExtractEntry_HashMismatch_Error(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[b.Index.Entries[0].DataOffset] ^= 0xFF
	})

	b, err = Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	var buf bytes.Buffer
	err = b.ExtractEntry(t.Context(), b.Index.Entries[0], &buf)

	require.ErrorIs(t, err, ErrDataCorrupt)
	require.NotErrorIs(t, err, errIO)
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

	err := b.ExtractEntry(t.Context(), entry, w)

	require.ErrorIs(t, err, errIO)
	require.NotErrorIs(t, err, ErrDataCorrupt)
	require.ErrorContains(t, err, "writer boom")
	require.NotEmpty(t, w.buf.Bytes())
}

// Expectation: ExtractManifest should write the full manifest bytes on success.
func Test_Bundle_ExtractManifest_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)

	var buf bytes.Buffer
	err := b.ExtractManifest(t.Context(), &buf)

	require.NoError(t, err)
	require.Equal(t, fixture.manifest.Bytes, buf.Bytes())
}

// Expectation: ExtractManifest should return ErrDataCorrupt after copying bytes when the manifest hash mismatches.
func Test_Bundle_ExtractManifest_HashMismatch_Error(t *testing.T) {
	t.Parallel()

	fixture := newTestBundleFixture(t)
	b, err := Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	require.NoError(t, b.Close())

	overwriteBundleBytes(t, fixture.fs, fixture.bundlePath, func(raw []byte) {
		raw[b.Index.ManifestDataOffset] ^= 0xFF
	})

	b, err = Open(t.Context(), fixture.fs, fixture.bundlePath)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, b.Close())
	})

	var buf bytes.Buffer
	err = b.ExtractManifest(t.Context(), &buf)

	require.ErrorIs(t, err, ErrDataCorrupt)
	require.NotErrorIs(t, err, errIO)
	require.ErrorContains(t, err, "checksum mismatch")
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

	err := b.ExtractManifest(t.Context(), w)

	require.ErrorIs(t, err, errIO)
	require.NotErrorIs(t, err, ErrDataCorrupt)
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

// Expectation: jailedJoinPath should accept names that stay inside the
// directory, reject names that escape it or resolve to it.
func Test_jailedJoinPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		dir     string
		input   string
		want    string // Expected path, if no error.
		wantErr string // Expected error substring, empty if none.
	}{
		// Valid names.
		{name: "simple file", dir: "/dest", input: "file.txt", want: "/dest/file.txt"},
		{name: "subdirectory", dir: "/dest", input: "sub/file.txt", want: "/dest/sub/file.txt"},
		{name: "nested subdirectory", dir: "/dest", input: "a/b/c/file.txt", want: "/dest/a/b/c/file.txt"},
		{name: "leading dots in name", dir: "/dest", input: "..foo", want: "/dest/..foo"},
		{name: "hidden file", dir: "/dest", input: ".hidden", want: "/dest/.hidden"},
		{name: "traversal staying inside", dir: "/dest", input: "a/../file.txt", want: "/dest/file.txt"},
		{name: "leading dot segment", dir: "/dest", input: "./a", want: "/dest/a"},
		{name: "duplicate separators", dir: "/dest", input: "a//b", want: "/dest/a/b"},
		{name: "dir with trailing separator", dir: "/dest/", input: "file.txt", want: "/dest/file.txt"},

		// Names that are not local.
		{name: "empty", dir: "/dest", input: "", wantErr: "not a local path"},
		{name: "absolute", dir: "/dest", input: "/etc/passwd", wantErr: "not a local path"},
		{name: "exact parent", dir: "/dest", input: "..", wantErr: "not a local path"},
		{name: "parent file", dir: "/dest", input: "../x", wantErr: "not a local path"},
		{name: "nested escape", dir: "/dest", input: "a/../../x", wantErr: "not a local path"},
		{name: "escape and return", dir: "/dest", input: "../dest/file.txt", wantErr: "not a local path"},

		// Names resolving to the directory itself.
		{name: "dot", dir: "/dest", input: ".", wantErr: "resolves to destination directory"},
		{name: "dot with separator", dir: "/dest", input: "./", wantErr: "resolves to destination directory"},
		{name: "subdir and back", dir: "/dest", input: "a/..", wantErr: "resolves to destination directory"},
		{name: "dot with dir trailing separator", dir: "/dest/", input: ".", wantErr: "resolves to destination directory"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := jailedJoinPath(tt.dir, tt.input)

			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				require.Empty(t, got)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
