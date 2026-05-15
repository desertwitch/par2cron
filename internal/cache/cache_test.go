package cache

import (
	"testing"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// Expectation: NewGobCache should create a cache with a hashed filename and .gob.zst extension.
func Test_NewGobCache_HashedFilename_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "my-cache")

	require.Contains(t, c.path, "/cache/")
	require.Contains(t, c.path, GobCacheExtension)
}

// Expectation: NewGobCache should produce different filenames for different cache names.
func Test_NewGobCache_DifferentNames_DifferentPaths_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c1 := NewGobCache(fsys, "/cache", "cache-a")
	c2 := NewGobCache(fsys, "/cache", "cache-b")

	require.NotEqual(t, c1.path, c2.path)
}

// Expectation: NewGobCache should produce the same filename for the same cache name.
func Test_NewGobCache_SameName_SamePath_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c1 := NewGobCache(fsys, "/cache", "cache-a")
	c2 := NewGobCache(fsys, "/cache", "cache-a")

	require.Equal(t, c1.path, c2.path)
}

// Expectation: NewGobCache should initialize an empty items map.
func Test_NewGobCache_EmptyItems_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	require.NotNil(t, c.items)
	require.Empty(t, c.items)
}

// Expectation: All should return all items in the cache.
func Test_GobCache_All_ReturnsAllItems_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	m1 := &schema.JobMeta{Par2Path: "/a.par2"}
	m2 := &schema.JobMeta{Par2Path: "/b.par2"}
	c.items[m1.Par2Path] = m1
	c.items[m2.Par2Path] = m2

	all := c.All()

	require.Len(t, all, 2)
	require.ElementsMatch(t, []*schema.JobMeta{m1, m2}, all)
}

// Expectation: All should return an empty slice when the cache is empty.
func Test_GobCache_All_Empty_ReturnsEmptySlice_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	all := c.All()

	require.NotNil(t, all)
	require.Empty(t, all)
}

// Expectation: Len should return the number of entries in the cache.
func Test_GobCache_Len_ReturnsCount_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	require.Equal(t, 0, c.Len())

	c.items["/a.par2"] = &schema.JobMeta{Par2Path: "/a.par2"}
	require.Equal(t, 1, c.Len())

	c.items["/b.par2"] = &schema.JobMeta{Par2Path: "/b.par2"}
	require.Equal(t, 2, c.Len())
}

// Expectation: Get should return the item and true when the key exists.
func Test_GobCache_Get_ExistingKey_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	m := &schema.JobMeta{Par2Path: "/a.par2"}
	c.items["/a.par2"] = m

	got, ok := c.Get("/a.par2")

	require.True(t, ok)
	require.Equal(t, m, got)
}

// Expectation: Get should return nil and false when the key does not exist.
func Test_GobCache_Get_MissingKey_ReturnsFalse_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	got, ok := c.Get("/nonexistent.par2")

	require.False(t, ok)
	require.Nil(t, got)
}

// Expectation: Get should set the walked state to true on a found item.
func Test_GobCache_Get_SetsWalkedTrue_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	m := &schema.JobMeta{Par2Path: "/a.par2", Walked: false}
	c.items["/a.par2"] = m

	got, _ := c.Get("/a.par2")

	require.True(t, got.Walked)
}

// Expectation: Set should add a new entry to the cache.
func Test_GobCache_Set_NewEntry_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	m := &schema.JobMeta{Par2Path: "/a.par2"}
	c.Set("/a.par2", m)

	require.Equal(t, 1, c.Len())
	require.Equal(t, m, c.items["/a.par2"])
}

// Expectation: Set should update an existing entry in the cache.
func Test_GobCache_Set_UpdateExisting_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	m1 := &schema.JobMeta{Par2Path: "/a.par2", CountCorrupted: 1}
	c.Set("/a.par2", m1)

	m2 := &schema.JobMeta{Par2Path: "/a.par2", CountCorrupted: 5}
	c.Set("/a.par2", m2)

	require.Equal(t, 1, c.Len())
	require.Equal(t, 5, c.items["/a.par2"].CountCorrupted)
}

// Expectation: Set should set the walked state to true.
func Test_GobCache_Set_SetsWalkedTrue_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	m := &schema.JobMeta{Par2Path: "/a.par2", Walked: false}
	c.Set("/a.par2", m)

	require.True(t, c.items["/a.par2"].Walked)
}

// Expectation: ResetWalked should set all items to not walked.
func Test_GobCache_ResetWalked_ClearsAllWalkedState_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2"})

	require.True(t, c.items["/a.par2"].Walked)
	require.True(t, c.items["/b.par2"].Walked)

	c.ResetWalked()

	require.False(t, c.items["/a.par2"].Walked)
	require.False(t, c.items["/b.par2"].Walked)
}

// Expectation: ResetWalked on an empty cache should not panic.
func Test_GobCache_ResetWalked_EmptyCache_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	require.NotPanics(t, func() { c.ResetWalked() })
}

// Expectation: PruneUnwalked should remove entries that were not walked.
func Test_GobCache_PruneUnwalked_RemovesUnwalkedEntries_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2"})
	c.Set("/c.par2", &schema.JobMeta{Par2Path: "/c.par2"})

	c.items["/b.par2"].Walked = false

	pruned := c.PruneUnwalked()

	require.Equal(t, 1, pruned)
	require.Equal(t, 2, c.Len())
	require.Contains(t, c.items, "/a.par2")
	require.Contains(t, c.items, "/c.par2")
	require.NotContains(t, c.items, "/b.par2")
}

// Expectation: PruneUnwalked should return zero when all entries are walked.
func Test_GobCache_PruneUnwalked_AllWalked_PrunesNone_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2"})

	pruned := c.PruneUnwalked()

	require.Equal(t, 0, pruned)
	require.Equal(t, 2, c.Len())
}

// Expectation: PruneUnwalked should remove all entries when none are walked.
func Test_GobCache_PruneUnwalked_NoneWalked_PrunesAll_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2"})
	c.ResetWalked()

	pruned := c.PruneUnwalked()

	require.Equal(t, 2, pruned)
	require.Equal(t, 0, c.Len())
}

// Expectation: PruneUnwalked on an empty cache should return zero.
func Test_GobCache_PruneUnwalked_EmptyCache_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	pruned := c.PruneUnwalked()

	require.Equal(t, 0, pruned)
}

// Expectation: Save and Load should round-trip cache entries without data loss.
func Test_GobCache_SaveLoad_RoundTrip_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2", CountCorrupted: 3})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2", CountCorrupted: 7})

	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())

	require.Equal(t, 2, c2.Len())

	a, ok := c2.items["/a.par2"]
	require.True(t, ok)
	require.Equal(t, 3, a.CountCorrupted)

	b, ok := c2.items["/b.par2"]
	require.True(t, ok)
	require.Equal(t, 7, b.CountCorrupted)
}

// Expectation: Save should reset the walked state of all entries to false.
func Test_GobCache_Save_ResetsWalkedState_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	require.True(t, c.items["/a.par2"].Walked)

	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())
	require.False(t, c2.items["/a.par2"].Walked)
}

// Expectation: Save and Load should work with an empty cache.
func Test_GobCache_SaveLoad_EmptyCache_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())

	require.Equal(t, 0, c2.Len())
}

// Expectation: Save should overwrite a previous cache file without corruption.
func Test_GobCache_Save_OverwritesPreviousFile_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2"})
	require.NoError(t, c.Save())

	// Overwrite with fewer entries.
	c.items = make(map[string]*schema.JobMeta)
	c.Set("/c.par2", &schema.JobMeta{Par2Path: "/c.par2"})
	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())

	require.Equal(t, 1, c2.Len())
	require.Contains(t, c2.items, "/c.par2")
	require.NotContains(t, c2.items, "/a.par2")
	require.NotContains(t, c2.items, "/b.par2")
}

// Expectation: Load should return an error when the cache file does not exist.
func Test_GobCache_Load_FileNotFound_Error(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "nonexistent")

	err := c.Load()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open")
}

// Expectation: Load should return an error when the file contains invalid zstd data.
func Test_GobCache_Load_CorruptZstd_Error(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")

	require.NoError(t, afero.WriteFile(fsys, c.path, []byte("not valid zstd data"), 0o666))

	err := c.Load()

	require.Error(t, err)
}

// Expectation: Save should return an error when the cache directory does not exist.
func Test_GobCache_Save_DirectoryNotFound_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewOsFs()
	c := NewGobCache(fsys, "/nonexistent/directory/path", "test")

	err := c.Save()

	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open")
}

// Expectation: Save should create the cache file with the correct extension.
func Test_GobCache_Save_CreatesFileWithCorrectExtension_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	require.NoError(t, c.Save())

	_, err := fsys.Stat(c.path)
	require.NoError(t, err)
	require.Contains(t, c.path, GobCacheExtension)
}

// Expectation: Load should use Par2Path as the map key for each entry.
func Test_GobCache_Load_KeyedByPar2Path_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/x.par2", &schema.JobMeta{Par2Path: "/x.par2", IsBundle: true})
	c.Set("/y.par2", &schema.JobMeta{Par2Path: "/y.par2", IsBundle: false})
	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())

	require.Contains(t, c2.items, "/x.par2")
	require.Contains(t, c2.items, "/y.par2")
	require.True(t, c2.items["/x.par2"].IsBundle)
	require.False(t, c2.items["/y.par2"].IsBundle)
}

// Expectation: Load should replace existing in-memory items with the file contents.
func Test_GobCache_Load_ReplacesExistingItems_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	c2.Set("/stale.par2", &schema.JobMeta{Par2Path: "/stale.par2"})
	require.NoError(t, c2.Load())

	require.Equal(t, 1, c2.Len())
	require.Contains(t, c2.items, "/a.par2")
	require.NotContains(t, c2.items, "/stale.par2")
}

// Expectation: Save and Load should preserve all JobMeta fields across a round-trip.
func Test_GobCache_SaveLoad_PreservesAllFields_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")

	verifyTime := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	verifyDuration := 42 * time.Second

	original := &schema.JobMeta{
		Par2Path:        "/data/test.par2",
		VerifyTime:      verifyTime,
		VerifyDuration:  verifyDuration,
		CountCorrupted:  3,
		MetaVersion:     schema.MetaVersion,
		IsBundle:        true,
		HasManifest:     true,
		HasCreation:     true,
		HasVerification: true,
		RepairNeeded:    true,
		RepairPossible:  false,
	}
	c.Set(original.Par2Path, original)

	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())

	loaded := c2.items[original.Par2Path]
	require.NotNil(t, loaded)
	require.Equal(t, original, loaded)
}

// Expectation: Multiple save and load cycles should not corrupt data.
func Test_GobCache_SaveLoad_MultipleCycles_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2", CountCorrupted: 1})
	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())
	c2.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2", CountCorrupted: 2})
	require.NoError(t, c2.Save())

	c3 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c3.Load())

	require.Equal(t, 2, c3.Len())
	require.Equal(t, 1, c3.items["/a.par2"].CountCorrupted)
	require.Equal(t, 2, c3.items["/b.par2"].CountCorrupted)
}

// Expectation: A full walk cycle of ResetWalked, Get/Set, PruneUnwalked should retain only walked entries.
func Test_GobCache_WalkCycle_RetainsOnlyWalked_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2"})
	c.Set("/c.par2", &schema.JobMeta{Par2Path: "/c.par2"})

	// Simulate a new walk cycle.
	c.ResetWalked()

	// Only visit /a and /c during this walk.
	c.Get("/a.par2")
	c.Get("/c.par2")

	pruned := c.PruneUnwalked()

	require.Equal(t, 1, pruned)
	require.Equal(t, 2, c.Len())
	require.Contains(t, c.items, "/a.par2")
	require.Contains(t, c.items, "/c.par2")
	require.NotContains(t, c.items, "/b.par2")
}

// Expectation: A walk cycle with Set adding new entries should retain both visited and new entries.
func Test_GobCache_WalkCycle_SetAddsNewEntry_RetainsBoth_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	c := NewGobCache(fsys, "/cache", "test")

	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})

	c.ResetWalked()

	c.Get("/a.par2")
	c.Set("/new.par2", &schema.JobMeta{Par2Path: "/new.par2"})

	pruned := c.PruneUnwalked()

	require.Equal(t, 0, pruned)
	require.Equal(t, 2, c.Len())
}

// Expectation: A walk cycle followed by Save and Load should persist only walked entries.
func Test_GobCache_WalkCycle_SaveLoad_PersistsCorrectly_Success(t *testing.T) {
	t.Parallel()

	dir := "/"
	fsys := afero.NewMemMapFs()

	c := NewGobCache(fsys, dir, "test")
	c.Set("/a.par2", &schema.JobMeta{Par2Path: "/a.par2"})
	c.Set("/b.par2", &schema.JobMeta{Par2Path: "/b.par2"})
	c.Set("/c.par2", &schema.JobMeta{Par2Path: "/c.par2"})

	c.ResetWalked()
	c.Get("/a.par2")
	c.PruneUnwalked()

	require.NoError(t, c.Save())

	c2 := NewGobCache(fsys, dir, "test")
	require.NoError(t, c2.Load())

	require.Equal(t, 1, c2.Len())
	require.Contains(t, c2.items, "/a.par2")
}
