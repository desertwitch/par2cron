package util

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: The function should return an error for a non-bundle file.
func Test_ParseBundlePar2Index_NotBundleFile_Error(t *testing.T) {
	t.Parallel()

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.txt", &testutil.MockPar2Handler{}, &testutil.MockBundleHandler{})

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a bundle file")
}

// Expectation: The function should return an error for a plain par2 file that is not a bundle.
func Test_ParseBundlePar2Index_PlainPar2NotBundle_Error(t *testing.T) {
	t.Parallel()

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.par2", &testutil.MockPar2Handler{}, &testutil.MockBundleHandler{})

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a bundle file")
}

// Expectation: The function should return an error for a par2 volume file.
func Test_ParseBundlePar2Index_Par2VolumeFile_Error(t *testing.T) {
	t.Parallel()

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.vol0+1.par2", &testutil.MockPar2Handler{}, &testutil.MockBundleHandler{})

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a bundle file")
}

// Expectation: The function should return the error from Open when the bundle fails to open.
func Test_ParseBundlePar2Index_OpenError_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("open failed")
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return nil, expectedErr
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	require.Contains(t, err.Error(), "failed to open bundle")
}

// Expectation: The function should return an error when the bundle has no par2 index entries.
func Test_ParseBundlePar2Index_NoIndexEntries_Error(t *testing.T) {
	t.Parallel()

	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.vol0+1.par2", DataLength: 100},
				{Name: "readme.txt", DataLength: 50},
			}
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no index file found in bundle")
}

// Expectation: The function should return an error when the bundle has no entries at all.
func Test_ParseBundlePar2Index_EmptyEntries_Error(t *testing.T) {
	t.Parallel()

	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{}
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no index file found in bundle")
}

// Expectation: The function should return an error when an index entry exceeds the maximum size.
func Test_ParseBundlePar2Index_IndexTooLarge_Error(t *testing.T) {
	t.Parallel()

	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.par2", DataLength: 100*1024*1024 + 1},
			}
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "index file too large")
}

// Expectation: The function should return an error when entry extraction fails.
func Test_ParseBundlePar2Index_ExtractEntryError_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("extract failed")
	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.par2", DataLength: 100},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			return expectedErr
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	require.Contains(t, err.Error(), "failed to extract index file")
}

// Expectation: The function should return an error when parsing the extracted index fails.
func Test_ParseBundlePar2Index_ParseError_Error(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("parse failed")
	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.par2", DataLength: 100},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			_, err := w.Write([]byte("fake par2 data"))

			return err
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}
	par2Handler := &testutil.MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			return nil, expectedErr
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
	require.Contains(t, err.Error(), "failed to parse index file")
}

// Expectation: The function should successfully parse a bundle with a single index entry.
func Test_ParseBundlePar2Index_SingleIndexEntry_Success(t *testing.T) {
	t.Parallel()

	expectedSets := []par2.Set{{}}
	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.par2", DataLength: 100},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			_, err := w.Write([]byte("fake par2 data"))

			return err
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}
	par2Handler := &testutil.MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			require.True(t, checkMD5)

			data, err := io.ReadAll(r)
			require.NoError(t, err)
			require.Equal(t, "fake par2 data", string(data))

			return expectedSets, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Equal(t, expectedSets, sets)
}

// Expectation: The function should aggregate sets from multiple index entries in a bundle.
func Test_ParseBundlePar2Index_MultipleIndexEntries_Success(t *testing.T) {
	t.Parallel()

	set1 := par2.Set{}
	set2 := par2.Set{}
	callCount := 0

	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "a.par2", DataLength: 50},
				{Name: "readme.txt", DataLength: 10},
				{Name: "b.par2", DataLength: 60},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			_, err := w.Write([]byte("data-" + e.Name))

			return err
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}
	par2Handler := &testutil.MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			callCount++

			data, err := io.ReadAll(r)
			require.NoError(t, err)

			if string(data) == "data-a.par2" {
				return []par2.Set{set1}, nil
			}

			return []par2.Set{set2}, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Len(t, sets, 2)
	require.Equal(t, 2, callCount)
}

// Expectation: The function should skip volume entries in a bundle and only parse index entries.
func Test_ParseBundlePar2Index_SkipsVolumeEntries_Success(t *testing.T) {
	t.Parallel()

	expectedSets := []par2.Set{{}}
	var extractedNames []string

	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.vol0+1.par2", DataLength: 100},
				{Name: "file.vol1+2.par2", DataLength: 200},
				{Name: "file.par2", DataLength: 50},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			extractedNames = append(extractedNames, e.Name)
			_, err := w.Write([]byte("data"))

			return err
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}
	par2Handler := &testutil.MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			return expectedSets, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Equal(t, expectedSets, sets)
	require.Equal(t, []string{"file.par2"}, extractedNames)
}

// Expectation: The function should not return an error when the index entry is exactly at the size limit.
func Test_ParseBundlePar2Index_IndexExactlyAtMaxSize_Success(t *testing.T) {
	t.Parallel()

	expectedSets := []par2.Set{{}}
	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.par2", DataLength: 100 * 1024 * 1024},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			_, err := w.Write([]byte("data"))

			return err
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}
	par2Handler := &testutil.MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			return expectedSets, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Equal(t, expectedSets, sets)
}

// Expectation: The function should pass the extracted data correctly to the Parse function.
func Test_ParseBundlePar2Index_ExtractedDataPassedToParser_Success(t *testing.T) {
	t.Parallel()

	expectedData := []byte{0x50, 0x41, 0x52, 0x32, 0x00, 0x50, 0x4B, 0x54}
	expectedSets := []par2.Set{{}}

	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.par2", DataLength: uint64(len(expectedData))},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			_, err := w.Write(expectedData)

			return err
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}
	par2Handler := &testutil.MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			var buf bytes.Buffer

			_, err := io.Copy(&buf, r)
			require.NoError(t, err)
			require.Equal(t, expectedData, buf.Bytes())

			return expectedSets, nil
		},
	}

	sets, err := ParseBundlePar2Index(afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Equal(t, expectedSets, sets)
}
