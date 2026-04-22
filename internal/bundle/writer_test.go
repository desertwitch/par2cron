package bundle

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

type chunkWriter struct {
	buf bytes.Buffer

	maxChunk int
}

func (w *chunkWriter) Write(p []byte) (int, error) {
	n := min(len(p), w.maxChunk)
	if n == 0 {
		return 0, nil
	}

	return w.buf.Write(p[:n])
}

type zeroWriter struct{}

func (zeroWriter) Write(p []byte) (int, error) {
	return 0, nil
}

// Expectation: Pack should write deterministic entries and place file and manifest bytes at the indexed offsets.
func Test_Pack_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	raw := readBundleBytes(t, fixture.fs, fixture.bundlePath)

	require.Equal(t, []string{"alpha.par2", "zeta.vol00+01.par2"}, []string{b.Index.Entries[0].Name, b.Index.Entries[1].Name})

	for _, entry := range b.Index.Entries {
		verifyDataAtOffset(t, raw, entry.DataOffset, entry.DataLength, fixture.files[entry.Name])
		require.Equal(t, Magic[:], raw[entry.PacketOffset:entry.PacketOffset+uint64(len(Magic))])
	}

	verifyDataAtOffset(t, raw, b.Index.ManifestDataOffset, b.Index.ManifestDataLength, fixture.manifest.Bytes)
	require.Equal(t, Magic[:], raw[b.Index.ManifestPacketOffset:b.Index.ManifestPacketOffset+uint64(len(Magic))])
}

// Expectation: Pack should remove the bundle path again when a later write step fails.
func Test_Pack_CleanupAfterFailure_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/bundles", 0o755))

	err := Pack(fs, "/bundles/out.par2", testRecoverySetID, ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"ok":true}`),
	}, []FileInput{
		{Name: "missing.par2", Path: "/src/missing.par2"},
	})

	require.ErrorContains(t, err, "failed to write file segment")

	exists, existsErr := afero.Exists(fs, "/bundles/out.par2")
	require.NoError(t, existsErr)
	require.False(t, exists)
}

// Expectation: Pack should return an error when the destination bundle file cannot be created.
func Test_Pack_CreateFails_Error(t *testing.T) {
	t.Parallel()

	fs := &testFs{
		Fs: afero.NewMemMapFs(),
		createFunc: func(name string) (afero.File, error) {
			return nil, errors.New("create boom")
		},
	}

	err := Pack(fs, "/bundle.par2", testRecoverySetID, ManifestInput{Name: "manifest.json"}, nil)

	require.ErrorContains(t, err, "failed to create")
	require.ErrorContains(t, err, "create boom")
}

// Expectation: Pack should return an error when the manifest packet position cannot be read from the writer.
func Test_Pack_GetManifestPacketPositionFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		createFunc: func(name string) (afero.File, error) {
			f, err := base.Create(name)
			require.NoError(t, err)

			return &callbackFile{
				File: f,
				seekFunc: func(offset int64, whence int) (int64, error) {
					if whence == io.SeekCurrent {
						return 0, errors.New("seek boom")
					}

					return f.Seek(offset, whence)
				},
			}, nil
		},
	}

	err := Pack(fs, "/bundle.par2", testRecoverySetID, ManifestInput{Name: "manifest.json"}, nil)

	require.ErrorContains(t, err, "failed to get manifest packet position")
	require.ErrorContains(t, err, "seek boom")
}

// Expectation: Pack should reject negative manifest packet offsets reported by the writer.
func Test_Pack_InvalidManifestPacketOffset_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		createFunc: func(name string) (afero.File, error) {
			f, err := base.Create(name)
			require.NoError(t, err)

			return &callbackFile{
				File: f,
				seekFunc: func(offset int64, whence int) (int64, error) {
					if whence == io.SeekCurrent {
						return -1, nil
					}

					return f.Seek(offset, whence)
				},
			}, nil
		},
	}

	err := Pack(fs, "/bundle.par2", testRecoverySetID, ManifestInput{Name: "manifest.json"}, nil)

	require.ErrorContains(t, err, "failed to get valid manifest packet offset")
}

// Expectation: Pack should return an error when seeking back to offset zero for the index packet fails.
func Test_Pack_SeekToStartFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		createFunc: func(name string) (afero.File, error) {
			f, err := base.Create(name)
			require.NoError(t, err)

			return &callbackFile{
				File: f,
				seekFunc: func(offset int64, whence int) (int64, error) {
					if whence == io.SeekStart && offset == 0 {
						return 0, errors.New("seek start boom")
					}

					return f.Seek(offset, whence)
				},
			}, nil
		},
	}

	err := Pack(fs, "/bundle.par2", testRecoverySetID, ManifestInput{Name: "manifest.json"}, nil)

	require.ErrorContains(t, err, "failed to seek to start")
	require.ErrorContains(t, err, "seek start boom")
}

// Expectation: Pack should return an error when writing the manifest packet fails.
func Test_Pack_WriteManifestPacketFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		createFunc: func(name string) (afero.File, error) {
			f, err := base.Create(name)
			require.NoError(t, err)

			manifestPhase := false

			return &callbackFile{
				File: f,
				seekFunc: func(offset int64, whence int) (int64, error) {
					if whence == io.SeekCurrent {
						manifestPhase = true
					}

					return f.Seek(offset, whence)
				},
				writeFunc: func(p []byte) (int, error) {
					if manifestPhase {
						return 0, errors.New("write boom")
					}

					return f.Write(p)
				},
			}, nil
		},
	}

	err := Pack(fs, "/bundle.par2", testRecoverySetID, ManifestInput{Name: "manifest.json"}, nil)

	require.ErrorContains(t, err, "failed to write manifest packet")
	require.ErrorContains(t, err, "write boom")
}

// Expectation: Pack should return an error when writing the final index packet fails.
func Test_Pack_WriteIndexPacketFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		createFunc: func(name string) (afero.File, error) {
			f, err := base.Create(name)
			require.NoError(t, err)

			indexPhase := false

			return &callbackFile{
				File: f,
				seekFunc: func(offset int64, whence int) (int64, error) {
					if whence == io.SeekStart && offset == 0 {
						indexPhase = true
					}

					return f.Seek(offset, whence)
				},
				writeFunc: func(p []byte) (int, error) {
					if indexPhase {
						return 0, errors.New("write boom")
					}

					return f.Write(p)
				},
			}, nil
		},
	}

	err := Pack(fs, "/bundle.par2", testRecoverySetID, ManifestInput{Name: "manifest.json"}, nil)

	require.ErrorContains(t, err, "failed to write index packet")
	require.ErrorContains(t, err, "write boom")
}

// Expectation: Pack should return an error when syncing the finished bundle fails.
func Test_Pack_SyncFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	fs := &testFs{
		Fs: base,
		createFunc: func(name string) (afero.File, error) {
			f, err := base.Create(name)
			require.NoError(t, err)

			return &callbackFile{
				File: f,
				syncFunc: func() error {
					return errors.New("sync boom")
				},
			}, nil
		},
	}

	err := Pack(fs, "/bundle.par2", testRecoverySetID, ManifestInput{Name: "manifest.json"}, nil)

	require.ErrorContains(t, err, "failed to sync")
	require.ErrorContains(t, err, "sync boom")
}

// Expectation: Update should rewrite the manifest and keep the indexed offsets aligned with the actual bytes on disk.
func Test_Bundle_Update_Success(t *testing.T) {
	t.Parallel()

	b, fixture := openTestBundle(t)
	newManifest := []byte(`{"version":2,"items":["rewritten"]}`)

	require.NoError(t, b.Update(newManifest))

	raw := readBundleBytes(t, fixture.fs, fixture.bundlePath)
	verifyDataAtOffset(t, raw, b.Index.ManifestDataOffset, b.Index.ManifestDataLength, newManifest)
	for _, entry := range b.Index.Entries {
		verifyDataAtOffset(t, raw, entry.DataOffset, entry.DataLength, fixture.files[entry.Name])
	}

	got, err := b.Manifest()
	require.NoError(t, err)
	require.Equal(t, newManifest, got)
	require.Equal(t, dataHash(newManifest), b.Index.ManifestDataSHA256)
}

// Expectation: Update should return an error when truncating away the old manifest fails.
func Test_Bundle_Update_TruncateFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	b.f = &testFile{
		File: orig,
		truncateFunc: func(size int64) error {
			return errors.New("truncate boom")
		},
	}

	require.ErrorContains(t, b.Update([]byte(`{}`)), "failed to truncate manifest packet")
}

// Expectation: Update should return an error when seeking to the manifest offset fails.
func Test_Bundle_Update_SeekManifestFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	manifestOffset := int64(b.Index.ManifestPacketOffset) //nolint:gosec
	b.f = &testFile{
		File: orig,
		seekFunc: func(offset int64, whence int) (int64, error) {
			if whence == io.SeekStart && offset == manifestOffset {
				return 0, errors.New("seek manifest boom")
			}

			return orig.Seek(offset, whence)
		},
	}

	require.ErrorContains(t, b.Update([]byte(`{}`)), "failed to seek to manifest offset")
}

// Expectation: Update should return an error when seeking back to the start for the rewritten index fails.
func Test_Bundle_Update_SeekStartFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	manifestOffset := int64(b.Index.ManifestPacketOffset) //nolint:gosec
	var manifestSeekSeen bool
	b.f = &testFile{
		File: orig,
		seekFunc: func(offset int64, whence int) (int64, error) {
			if whence == io.SeekStart && offset == manifestOffset {
				manifestSeekSeen = true
			}
			if manifestSeekSeen && whence == io.SeekStart && offset == 0 {
				return 0, errors.New("seek start boom")
			}

			return orig.Seek(offset, whence)
		},
	}

	require.ErrorContains(t, b.Update([]byte(`{}`)), "failed to seek to start")
}

// Expectation: Update should return an error when syncing the rewritten manifest packet fails.
func Test_Bundle_Update_FirstSyncFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	b.f = &testFile{
		File: orig,
		syncFunc: func() error {
			return errors.New("sync boom")
		},
	}

	require.ErrorContains(t, b.Update([]byte(`{}`)), "failed to sync")
}

// Expectation: Update should return an error when writing the replacement manifest packet fails.
func Test_Bundle_Update_WriteManifestPacketFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	manifestPhase := false
	manifestOffset := int64(b.Index.ManifestPacketOffset) //nolint:gosec
	b.f = &callbackFile{
		File: orig,
		seekFunc: func(offset int64, whence int) (int64, error) {
			if whence == io.SeekStart && offset == manifestOffset {
				manifestPhase = true
			}

			return orig.Seek(offset, whence)
		},
		writeFunc: func(p []byte) (int, error) {
			if manifestPhase {
				return 0, errors.New("write boom")
			}

			return orig.Write(p)
		},
	}

	err := b.Update([]byte(`{"updated":true}`))

	require.ErrorContains(t, err, "failed to write manifest packet")
	require.ErrorContains(t, err, "write boom")
}

// Expectation: Update should return an error when writing the rewritten index packet fails.
func Test_Bundle_Update_WriteIndexPacketFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	indexPhase := false
	syncCount := 0
	b.f = &callbackFile{
		File: orig,
		seekFunc: func(offset int64, whence int) (int64, error) {
			if whence == io.SeekStart && offset == 0 {
				indexPhase = true
			}

			return orig.Seek(offset, whence)
		},
		writeFunc: func(p []byte) (int, error) {
			if indexPhase {
				return 0, errors.New("write boom")
			}

			return orig.Write(p)
		},
		syncFunc: func() error {
			syncCount++
			if syncCount == 1 {
				return nil
			}

			return orig.Sync()
		},
	}

	err := b.Update([]byte(`{"updated":true}`))

	require.ErrorContains(t, err, "failed to write index packet")
	require.ErrorContains(t, err, "write boom")
}

// Expectation: Update should return an error when the second sync after rewriting the index fails.
func Test_Bundle_Update_SecondSyncFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	syncCount := 0
	b.f = &callbackFile{
		File: orig,
		syncFunc: func() error {
			syncCount++
			if syncCount == 2 {
				return errors.New("sync boom")
			}

			return orig.Sync()
		},
	}

	err := b.Update([]byte(`{"updated":true}`))

	require.ErrorContains(t, err, "failed to sync")
	require.ErrorContains(t, err, "sync boom")
}

// Expectation: Update should return an error when statting the updated file fails at the end.
func Test_Bundle_Update_StatFails_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	b.f = &testFile{
		File: orig,
		statFunc: func() (os.FileInfo, error) {
			return nil, errors.New("stat boom")
		},
	}

	require.ErrorContains(t, b.Update([]byte(`{}`)), "failed to stat")
}

// Expectation: Update should reject a negative file size reported after a successful rewrite.
func Test_Bundle_Update_NegativeFileSizeAfterUpdate_Error(t *testing.T) {
	t.Parallel()

	b, _ := openTestBundle(t)
	orig := b.f
	b.f = &callbackFile{
		File: orig,
		statFunc: func() (os.FileInfo, error) {
			info, err := orig.Stat()
			require.NoError(t, err)

			return fileInfoWithSize{FileInfo: info, size: -1}, nil
		},
	}

	err := b.Update([]byte(`{"updated":true}`))

	require.ErrorContains(t, err, "file size < 0 after update")
}

// Expectation: writeCommonPacket should serialize the common header followed by the exact body bytes.
func Test_writeCommonPacket_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	body := []byte("data")

	require.NoError(t, writeCommonPacket(&buf, testRecoverySetID, PacketTypeFile, body))

	ch, gotBody, err := readAndValidatePacket(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len()), true)
	require.NoError(t, err)
	require.Equal(t, testRecoverySetID, ch.RecoverySetID)
	require.Equal(t, PacketTypeFile, ch.PacketType)
	require.Equal(t, body, gotBody)
}

// Expectation: writeIndexPacket should serialize a parseable index packet with the provided manifest and entry metadata.
func Test_writeIndexPacket_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	entries := []IndexEntry{
		{
			PacketOffset: 100,
			DataOffset:   164,
			DataLength:   5,
			DataSHA256:   dataHash([]byte("alpha")),
			NameLen:      uint64(len("alpha.par2")),
			Name:         "alpha.par2",
		},
	}
	manifest := manifestWriteEntry{
		packetOffset: 200,
		dataOffset:   264,
		dataLength:   7,
		dataSHA256:   dataHash([]byte("payload")),
		nameLen:      uint64(len("manifest.json")),
		name:         "manifest.json",
	}

	require.NoError(t, writeIndexPacket(&buf, testRecoverySetID, FlagIndexRebuilt, entries, manifest))

	ch, body, err := readAndValidatePacket(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len()), true)
	require.NoError(t, err)

	got, err := parseIndexPacket(bytes.NewReader(body), ch)
	require.NoError(t, err)
	require.Equal(t, manifest.packetOffset, got.ManifestPacketOffset)
	require.Equal(t, []string{"alpha.par2"}, []string{got.Entries[0].Name})
}

// Expectation: writeFileSegment should write the file packet and raw file bytes at the returned offsets.
func Test_writeFileSegment_Success(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, fs.MkdirAll("/src", 0o755))
	require.NoError(t, afero.WriteFile(fs, "/src/file.par2", []byte("abc"), 0o644))

	out, err := fs.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	entry, err := writeFileSegment(fs, out, testRecoverySetID, FileInput{
		Name: "file.par2",
		Path: "/src/file.par2",
	})
	require.NoError(t, err)

	raw := readBundleBytes(t, fs, "/out.par2")
	verifyDataAtOffset(t, raw, entry.DataOffset, entry.DataLength, []byte("abc"))
	require.Equal(t, Magic[:], raw[entry.PacketOffset:entry.PacketOffset+uint64(len(Magic))])
}

// Expectation: writeFileSegment should return an error when the source file cannot be opened.
func Test_writeFileSegment_OpenFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	out, err := fs.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	_, err = writeFileSegment(fs, out, testRecoverySetID, FileInput{
		Name: "missing.par2",
		Path: "/missing.par2",
	})

	require.ErrorContains(t, err, "failed to open")
}

// Expectation: writeFileSegment should return an error when statting the source file fails.
func Test_writeFileSegment_StatFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/src/file.par2", []byte("abc"), 0o644))
	fs := &testFs{
		Fs: base,
		openFunc: func(name string) (afero.File, error) {
			f, err := base.Open(name)
			require.NoError(t, err)

			return &callbackFile{
				File: f,
				statFunc: func() (os.FileInfo, error) {
					return nil, errors.New("stat boom")
				},
			}, nil
		},
	}

	out, err := base.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	_, err = writeFileSegment(fs, out, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to stat")
	require.ErrorContains(t, err, "stat boom")
}

// Expectation: writeFileSegment should reject negative source file sizes.
func Test_writeFileSegment_InvalidFileSize_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/src/file.par2", []byte("abc"), 0o644))
	fs := &testFs{
		Fs: base,
		openFunc: func(name string) (afero.File, error) {
			f, err := base.Open(name)
			require.NoError(t, err)

			return &callbackFile{
				File: f,
				statFunc: func() (os.FileInfo, error) {
					info, statErr := f.Stat()
					require.NoError(t, statErr)

					return fileInfoWithSize{FileInfo: info, size: -1}, nil
				},
			}, nil
		},
	}

	out, err := base.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	_, err = writeFileSegment(fs, out, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to get valid file size")
}

// Expectation: writeFileSegment should return an error when the destination position cannot be queried.
func Test_writeFileSegment_GetFilePacketPositionFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/src/file.par2", []byte("abc"), 0o644))

	out, err := fs.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	w := &callbackFile{
		File: out,
		seekFunc: func(offset int64, whence int) (int64, error) {
			if whence == io.SeekCurrent {
				return 0, errors.New("seek boom")
			}

			return out.Seek(offset, whence)
		},
	}

	_, err = writeFileSegment(fs, w, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to get file packet position")
	require.ErrorContains(t, err, "seek boom")
}

// Expectation: writeFileSegment should reject negative destination positions reported for the file packet offset.
func Test_writeFileSegment_InvalidFilePacketOffset_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/src/file.par2", []byte("abc"), 0o644))

	out, err := fs.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	w := &callbackFile{
		File: out,
		seekFunc: func(offset int64, whence int) (int64, error) {
			if whence == io.SeekCurrent {
				return -1, nil
			}

			return out.Seek(offset, whence)
		},
	}

	_, err = writeFileSegment(fs, w, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to get valid file packet offset")
}

// Expectation: writeFileSegment should return an error when seeking the source file back to the start fails.
func Test_writeFileSegment_SeekSourceToStartFails_Error(t *testing.T) {
	t.Parallel()

	base := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(base, "/src/file.par2", []byte("abc"), 0o644))
	fs := &testFs{
		Fs: base,
		openFunc: func(name string) (afero.File, error) {
			f, err := base.Open(name)
			require.NoError(t, err)

			return &callbackFile{
				File: f,
				seekFunc: func(offset int64, whence int) (int64, error) {
					if whence == io.SeekStart && offset == 0 {
						return 0, errors.New("seek boom")
					}

					return f.Seek(offset, whence)
				},
			}, nil
		},
	}

	out, err := base.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	_, err = writeFileSegment(fs, out, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to seek to start")
	require.ErrorContains(t, err, "seek boom")
}

// Expectation: writeFileSegment should return an error when writing the file packet fails.
func Test_writeFileSegment_WriteFilePacketFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/src/file.par2", []byte("abc"), 0o644))

	out, err := fs.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	w := &callbackFile{
		File: out,
		writeFunc: func(p []byte) (int, error) {
			return 0, errors.New("write boom")
		},
	}

	_, err = writeFileSegment(fs, w, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to write file packet")
	require.ErrorContains(t, err, "write boom")
}

// Expectation: writeFileSegment should return an error when writing the raw file stream fails.
func Test_writeFileSegment_WriteFileStreamFails_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/src/file.par2", []byte("abc"), 0o644))

	out, err := fs.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	nameLen := uint64(len("file.par2"))
	filePacketLength := uint64(commonHeaderSize) + fileBodyPrefixSize + padTo4(nameLen)
	written := uint64(0)
	w := &callbackFile{
		File: out,
		writeFunc: func(p []byte) (int, error) {
			if written >= filePacketLength {
				return 0, errors.New("stream boom")
			}

			n, err := out.Write(p)
			written += uint64(n) //nolint:gosec

			return n, err
		},
	}

	_, err = writeFileSegment(fs, w, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to write file stream")
	require.ErrorContains(t, err, "stream boom")
}

// Expectation: writeFileSegment should surface io.ErrShortWrite when the data stream write makes only partial progress.
func Test_writeFileSegment_ShortWrite_Error(t *testing.T) {
	t.Parallel()

	fs := afero.NewMemMapFs()
	require.NoError(t, afero.WriteFile(fs, "/src/file.par2", []byte("abc"), 0o644))

	out, err := fs.Create("/out.par2")
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, out.Close())
	})

	nameLen := uint64(len("file.par2"))
	filePacketLength := uint64(commonHeaderSize) + fileBodyPrefixSize + padTo4(nameLen)
	written := uint64(0)
	shortWritten := false
	w := &callbackFile{
		File: out,
		writeFunc: func(p []byte) (int, error) {
			if written >= filePacketLength && !shortWritten {
				shortWritten = true
				n, writeErr := out.Write(p[:1])
				written += uint64(n) //nolint:gosec

				return n, writeErr
			}

			n, writeErr := out.Write(p)
			written += uint64(n) //nolint:gosec

			return n, writeErr
		},
	}

	_, err = writeFileSegment(fs, w, testRecoverySetID, FileInput{Name: "file.par2", Path: "/src/file.par2"})

	require.ErrorContains(t, err, "failed to write file stream")
	require.ErrorIs(t, err, io.ErrShortWrite)
}

// Expectation: writeFilePacket should serialize the file metadata into a parseable packet.
func Test_writeFilePacket_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	hash := dataHash([]byte("abc"))

	require.NoError(t, writeFilePacket(&buf, testRecoverySetID, "alpha.par2", 3, hash))

	ch, body, err := readAndValidatePacket(bytes.NewReader(buf.Bytes()), 0, int64(buf.Len()), true)
	require.NoError(t, err)

	got, err := parseFilePacket(bytes.NewReader(body), ch, 0)
	require.NoError(t, err)
	require.Equal(t, "alpha.par2", got.Name)
	require.Equal(t, uint64(3), got.DataLength)
	require.Equal(t, hash, got.DataSHA256)
}

// Expectation: writeManifestPacket should place the manifest bytes at the calculated data offset inside the packet.
func Test_writeManifestPacket_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	manifest := ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"ok":true}`),
	}

	entry, err := writeManifestPacket(&buf, testRecoverySetID, manifest, 128)
	require.NoError(t, err)

	raw := buf.Bytes()
	start := entry.dataOffset - entry.packetOffset
	verifyDataAtOffset(t, raw, start, entry.dataLength, manifest.Bytes)

	ch, body, err := readAndValidatePacket(bytes.NewReader(raw), 0, int64(len(raw)), true)
	require.NoError(t, err)

	got, err := parseManifestPacket(bytes.NewReader(body), ch, 0)
	require.NoError(t, err)
	require.Equal(t, manifest.Name, got.Name)
	require.Equal(t, uint64(len(manifest.Bytes)), got.DataLength)
}

// Expectation: writeManifestPacket should surface writeCommonPacket failures with the packet-specific prefix.
func Test_writeManifestPacket_WritePacketFails_Error(t *testing.T) {
	t.Parallel()

	_, err := writeManifestPacket(zeroWriter{}, testRecoverySetID, ManifestInput{
		Name:  "manifest.json",
		Bytes: []byte(`{"ok":true}`),
	}, 0)

	require.ErrorContains(t, err, "failed to write packet")
	require.ErrorIs(t, err, io.ErrShortWrite)
}

// Expectation: writeDataPadding should add only the bytes needed to reach the next 4-byte boundary.
func Test_writeDataPadding_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeDataPadding(&buf, 3))
	require.Equal(t, []byte{0}, buf.Bytes())

	buf.Reset()
	require.NoError(t, writeDataPadding(&buf, 4))
	require.Empty(t, buf.Bytes())
}

// Expectation: writeUint64LE should encode uint64 values in little-endian byte order.
func Test_writeUint64LE_Success(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	require.NoError(t, writeUint64LE(&buf, 0x0102030405060708))

	require.Equal(t, []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}, buf.Bytes())
}

// Expectation: writeAll should continue writing until the full payload has been flushed.
func Test_writeAll_ChunkedWriter_Success(t *testing.T) {
	t.Parallel()

	w := &chunkWriter{maxChunk: 2}

	require.NoError(t, writeAll(w, []byte("payload")))
	require.Equal(t, []byte("payload"), w.buf.Bytes())
}

// Expectation: writeAll should fail with io.ErrShortWrite when a writer reports zero progress without an error.
func Test_writeAll_ZeroWriter_Error(t *testing.T) {
	t.Parallel()

	require.ErrorIs(t, writeAll(zeroWriter{}, []byte("payload")), io.ErrShortWrite)
}
