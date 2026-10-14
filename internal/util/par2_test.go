package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"testing"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/desertwitch/par2cron/internal/testutil"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
)

// psSet returns a [par2.Set] with the given set ID byte, protecting the given names.
// The set is complete, unless main is false (then it lacks its main packet).
func psSet(id byte, main bool, names ...string) par2.Set {
	s := par2.Set{SetID: par2.Hash{id}}
	if main {
		s.MainPacket = &par2.MainPacket{SetID: par2.Hash{id}}
	}
	for _, name := range names {
		s.RecoverySet = append(s.RecoverySet, par2.FilePacket{SetID: par2.Hash{id}, Name: name})
	}

	return s
}

// psMissing returns a [par2.Set] with the given set ID byte that has its main packet,
// but misses a listed file (so it is incomplete, but more useful than one without main packet).
func psMissing(id byte, names ...string) par2.Set {
	s := psSet(id, true, names...)
	s.MissingRecoveryPackets = []par2.Hash{{0xff}}

	return s
}

// psIndexOnly returns a [testutil.MockPar2Handler] whose ParseFile returns the given sets,
// and which fails the test if the other set members are merged in.
func psIndexOnly(t *testing.T, sets ...par2.Set) *testutil.MockPar2Handler {
	t.Helper()

	return &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: sets}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			require.Fail(t, "should not merge the other set members")

			return nil, errors.New("unexpected call")
		},
	}
}

// Expectation: A complete index should be returned as is, without merging the other set members.
func Test_ParsePar2Set_CompleteIndex_NoMerge_Success(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, true, "a.txt")}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", psIndexOnly(t, sets...), &testutil.MockBundleHandler{})

	require.NoError(t, err)
	require.Equal(t, &sets[0], best)
	require.Equal(t, sets, all)
}

// Expectation: An incomplete index should be merged with the other set members, index first.
func Test_ParsePar2Set_IncompleteIndex_MergesMembers_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	for _, name := range []string{"file.par2", "file.vol1+2.par2", "file.vol0+1.par2", "other.par2", "file.txt"} {
		require.NoError(t, afero.WriteFile(fsys, "/data/"+name, nil, 0o644))
	}
	require.NoError(t, fsys.MkdirAll("/data/file.vol2+3.par2", 0o755))

	merged := []par2.Set{psSet(1, true, "a.txt", "b.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
			require.Equal(t, "/data/file.par2", path)
			require.True(t, panicAsErr)

			return &par2.File{Sets: []par2.Set{psSet(1, false, "a.txt")}}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, paths []string, panicAsErr bool) (*par2.FileSet, error) {
			require.Equal(t, []string{"/data/file.par2", "/data/file.vol0+1.par2", "/data/file.vol1+2.par2"}, paths)
			require.True(t, panicAsErr)

			return &par2.FileSet{SetsMerged: merged}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), fsys, "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.NoError(t, err)
	require.Equal(t, &merged[0], best)
	require.Equal(t, merged, all)
}

// Expectation: Only sets with an ID of the (damaged) index should be selected as best after merging.
func Test_ParsePar2Set_MergedOtherSetID_NoCompleteSet_Error(t *testing.T) {
	t.Parallel()

	// The index still knows set 1, the volume files only complete an unrelated set 2.
	merged := []par2.Set{psSet(1, false, "a.txt"), psSet(2, true, "a.txt", "b.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: []par2.Set{psSet(1, false, "a.txt")}}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return &par2.FileSet{SetsMerged: merged}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.Equal(t, &merged[0], best) // The incomplete set of the index, not the complete other one.
	require.Equal(t, merged, all)
}

// Expectation: With an unreadable index, there should be no best set, even if a merged set is complete.
func Test_ParsePar2Set_UnreadableIndex_NoBest_Unconfirmed_Error(t *testing.T) {
	t.Parallel()

	merged := []par2.Set{psSet(1, false, "a.txt"), psSet(2, true, "a.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return nil, errors.New("simulated parse failure")
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return &par2.FileSet{SetsMerged: merged}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorIs(t, err, ErrNoConfirmedSetID)
	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.Nil(t, best)
	require.Equal(t, merged, all)
}

// Expectation: If no merged set has an ID of the index, there should be no best set.
func Test_ParsePar2Set_NoCandidate_NilBest_Error(t *testing.T) {
	t.Parallel()

	// The index knows set 1, but the merge only yields an unrelated (complete) set 2.
	merged := []par2.Set{psSet(2, true, "a.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: []par2.Set{psSet(1, false, "a.txt")}}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return &par2.FileSet{SetsMerged: merged}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.NotErrorIs(t, err, ErrNoConfirmedSetID)
	require.Nil(t, best)
	require.Equal(t, merged, all)
}

// Expectation: All sets should be returned in their parse order, regardless of which is the best set.
func Test_ParsePar2Set_AllKeepsParseOrder_Error(t *testing.T) {
	t.Parallel()

	merged := []par2.Set{psSet(3, false, "c.txt"), psSet(1, true, "a.txt"), psSet(2, true, "b.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return nil, errors.New("simulated parse failure")
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return &par2.FileSet{SetsMerged: slices.Clone(merged)}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorIs(t, err, ErrNoConfirmedSetID) // Unreadable index.
	require.Nil(t, best)
	require.Equal(t, merged, all)
}

// Expectation: If merging fails after a readable but incomplete index, its sets should be returned.
func Test_ParsePar2Set_MergeFailsAfterIndex_KeepsIndexSets_Error(t *testing.T) {
	t.Parallel()

	index := []par2.Set{psSet(1, false, "a.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: index}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return nil, errors.New("simulated merge failure")
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.NotErrorIs(t, err, ErrNoConfirmedSetID)
	require.Equal(t, &index[0], best)
	require.Equal(t, index, all)
}

// Expectation: If both the index and merging fail, the merge error should be returned without sets.
func Test_ParsePar2Set_IndexAndMergeFail_Error(t *testing.T) {
	t.Parallel()

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return nil, errors.New("simulated parse failure")
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return nil, errors.New("simulated merge failure")
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorContains(t, err, "simulated merge failure")
	require.NotErrorIs(t, err, ErrNoCompleteSet)
	require.Nil(t, best)
	require.Nil(t, all)
}

// Expectation: A missing index without other set members should fail, instead of returning no sets.
func Test_ParsePar2Set_MissingIndexNoMembers_Error(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	require.NoError(t, fsys.MkdirAll("/data", 0o755))

	best, all, err := ParsePar2Set(t.Context(), fsys, "file.par2", "/data", &Par2Handler{}, &testutil.MockBundleHandler{})

	require.ErrorContains(t, err, "all paths failed to parse")
	require.NotErrorIs(t, err, ErrNoCompleteSet)
	require.Nil(t, best)
	require.Nil(t, all)
}

// Expectation: The first set of the index should stay the best set, even if a later set is complete (as par2cmdline locks onto it).
func Test_ParsePar2Set_IndexFirstSetLocked_NoCompleteSet_Error(t *testing.T) {
	t.Parallel()

	index := []par2.Set{psMissing(1, "a.txt"), psSet(2, true, "b.txt")}
	merged := []par2.Set{psMissing(1, "a.txt"), psSet(2, true, "b.txt")}

	mergedCalled := false
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: index}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			mergedCalled = true

			return &par2.FileSet{SetsMerged: merged}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.True(t, mergedCalled) // The complete second set must not stop the merge.
	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.NotErrorIs(t, err, ErrNoConfirmedSetID)
	require.Equal(t, &index[0], best)
	require.Equal(t, merged, all)
}

// Expectation: The first set of the index should be the best set once merging completes it.
func Test_ParsePar2Set_IndexFirstSetCompletedByMerge_Success(t *testing.T) {
	t.Parallel()

	merged := []par2.Set{psSet(1, true, "a.txt"), psSet(2, true, "b.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{Sets: []par2.Set{psMissing(1, "a.txt"), psSet(2, true, "b.txt")}}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return &par2.FileSet{SetsMerged: merged}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.NoError(t, err)
	require.Equal(t, &merged[0], best)
	require.Equal(t, merged, all)
}

// Expectation: A readable index without any sets should not confirm a merged set.
func Test_ParsePar2Set_IndexWithoutSets_Unconfirmed_Error(t *testing.T) {
	t.Parallel()

	merged := []par2.Set{psSet(2, true, "a.txt")}
	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return &par2.FileSet{SetsMerged: merged}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorIs(t, err, ErrNoConfirmedSetID)
	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.Nil(t, best)
	require.Equal(t, merged, all)
}

// Expectation: Without a readable index or merge, both errors should be returned without any sets.
func Test_ParsePar2Set_IndexWithoutSetsMergeFails_Error(t *testing.T) {
	t.Parallel()

	par2er := &testutil.MockPar2Handler{
		ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
			return &par2.File{}, nil
		},
		ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
			return nil, errors.New("simulated merge failure")
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

	require.ErrorIs(t, err, ErrNoConfirmedSetID)
	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.Nil(t, best)
	require.Empty(t, all)
}

// Expectation: Cancellation and panics while parsing the index should be returned without falling back.
func Test_ParsePar2Set_IndexFatalError_NoFallback_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	tests := map[string]struct {
		ctx context.Context //nolint:containedctx
		err error
	}{
		"cancelled": {ctx: ctx, err: ctx.Err()},
		"panic":     {ctx: t.Context(), err: &par2.ParserPanicError{Value: "boom"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			par2er := &testutil.MockPar2Handler{
				ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
					return nil, fmt.Errorf("wrapped: %w", tc.err)
				},
				ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
					require.Fail(t, "should not merge the other set members")

					return nil, errors.New("unexpected call")
				},
			}

			best, all, err := ParsePar2Set(tc.ctx, afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

			require.ErrorIs(t, err, tc.err)
			require.Nil(t, best)
			require.Nil(t, all)
		})
	}
}

// Expectation: Cancellation and panics while merging should not fall back to the readable index.
//
//nolint:paralleltest,tparallel
func Test_ParsePar2Set_MergeFatalError_NoIndexFallback_Error(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	tests := map[string]struct {
		ctx    context.Context //nolint:containedctx
		cancel bool
		err    error
	}{
		"cancelled": {ctx: ctx, cancel: true, err: context.Canceled},
		"panic":     {ctx: t.Context(), err: &par2.ParserPanicError{Value: "boom"}},
	}

	for name, tc := range tests {
		// Shared context state, do not run in parallel.
		t.Run(name, func(t *testing.T) {
			par2er := &testutil.MockPar2Handler{
				ParseFileFunc: func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
					return &par2.File{Sets: []par2.Set{psSet(1, false, "a.txt")}}, nil
				},
				ParseFileSetFunc: func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
					if tc.cancel {
						cancel() // Cancelled while merging, after the index was read.
					}

					return nil, fmt.Errorf("wrapped: %w", tc.err)
				},
			}

			best, all, err := ParsePar2Set(tc.ctx, afero.NewMemMapFs(), "file.par2", "/data", par2er, &testutil.MockBundleHandler{})

			require.ErrorIs(t, err, tc.err)
			require.NotErrorIs(t, err, ErrNoCompleteSet)
			require.Nil(t, best)
			require.Nil(t, all)
		})
	}
}

// psBundleHandler returns a [testutil.MockBundleHandler] for a bundle with a
// single index entry, and the recovery set ID of set 1 (see [psSet]).
func psBundleHandler() *testutil.MockBundleHandler {
	return &testutil.MockBundleHandler{
		OpenFunc: func(_ afero.Fs, _ string) (schema.Bundle, error) {
			return &testutil.MockBundle{
				RecoverySetIDValue: par2.Hash{1},
				EntriesFunc: func() []bundle.IndexEntry {
					return []bundle.IndexEntry{{Name: "file.par2", DataLength: 10}}
				},
				ExtractEntryFunc: func(_ bundle.IndexEntry, w io.Writer) error {
					_, err := w.Write([]byte("index data"))

					return err
				},
			}, nil
		},
	}
}

// Expectation: A complete bundle index should be returned without parsing the bundle as a whole.
func Test_ParsePar2Set_BundleCompleteIndex_Success(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, true, "a.txt")}
	par2er := psIndexOnly(t)
	par2er.ParseFunc = func(_ io.ReadSeeker, _ bool) ([]par2.Set, error) {
		return sets, nil
	}
	par2er.ParseFileFunc = func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
		require.Fail(t, "should not parse the bundle as a whole")

		return nil, errors.New("unexpected call")
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, psBundleHandler())

	require.NoError(t, err)
	require.Equal(t, &sets[0], best)
	require.Equal(t, sets, all)
}

// Expectation: An incomplete bundle index should fall back to parsing the bundle as a whole.
func Test_ParsePar2Set_BundleIncompleteIndex_ParsesBundle_Success(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, true, "a.txt")}

	parsedBundle := false
	par2er := psIndexOnly(t)
	par2er.ParseFunc = func(_ io.ReadSeeker, _ bool) ([]par2.Set, error) {
		return []par2.Set{psSet(1, false, "a.txt")}, nil // Incomplete embedded index.
	}
	par2er.ParseFileFunc = func(_ afero.Fs, path string, panicAsErr bool) (*par2.File, error) {
		parsedBundle = true
		require.Equal(t, "/data/file.p2c.par2", path)
		require.True(t, panicAsErr)

		return &par2.File{Sets: sets}, nil
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, psBundleHandler())

	require.NoError(t, err)
	require.True(t, parsedBundle)
	require.Equal(t, &sets[0], best)
	require.Equal(t, sets, all)
}

// Expectation: An incomplete bundle should not be merged with other files, as it contains its whole set.
func Test_ParsePar2Set_BundleIncomplete_NoMerge_Error(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, false, "a.txt")}
	par2er := psIndexOnly(t, sets...) // Fails the test on a merge.
	par2er.ParseFunc = func(_ io.ReadSeeker, _ bool) ([]par2.Set, error) {
		return []par2.Set{psSet(1, false, "a.txt")}, nil // Incomplete embedded index.
	}
	par2er.ParseFileSetFunc = func(fsys afero.Fs, paths []string, panicAsErr bool) (*par2.FileSet, error) {
		require.Fail(t, "should not start merging files")

		return nil, errors.New("unexpected call")
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, psBundleHandler())

	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.Equal(t, &sets[0], best)
	require.Equal(t, sets, all)
}

// Expectation: An unreadable bundle should return its parse error without merging other files.
func Test_ParsePar2Set_BundleUnreadable_NoMerge_Error(t *testing.T) {
	t.Parallel()

	par2er := psIndexOnly(t) // Fails the test on a merge.
	par2er.ParseFileFunc = func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
		return nil, errors.New("simulated parse failure")
	}
	par2er.ParseFileSetFunc = func(fsys afero.Fs, paths []string, panicAsErr bool) (*par2.FileSet, error) {
		require.Fail(t, "should not start merging files")

		return nil, errors.New("unexpected call")
	}
	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(_ afero.Fs, _ string) (schema.Bundle, error) {
			return nil, errors.New("simulated open failure")
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, bundler)

	require.ErrorContains(t, err, "simulated parse failure")
	require.NotErrorIs(t, err, ErrNoCompleteSet)
	require.Nil(t, best)
	require.Nil(t, all)
}

// Expectation: A bundle with more than one index file should be parsed as a whole instead.
func Test_ParsePar2Set_BundleMultipleIndexes_ParsesBundle_Success(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, true, "a.txt")}

	parsedBundle := false
	par2er := psIndexOnly(t) // Fails the test on a merge.
	par2er.ParseFunc = func(_ io.ReadSeeker, _ bool) ([]par2.Set, error) {
		require.Fail(t, "should not parse an embedded index")

		return nil, errors.New("unexpected call")
	}
	par2er.ParseFileFunc = func(_ afero.Fs, path string, _ bool) (*par2.File, error) {
		parsedBundle = true
		require.Equal(t, "/data/file.p2c.par2", path)

		return &par2.File{Sets: sets}, nil
	}
	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(_ afero.Fs, _ string) (schema.Bundle, error) {
			return &testutil.MockBundle{
				RecoverySetIDValue: par2.Hash{1},
				EntriesFunc: func() []bundle.IndexEntry {
					return []bundle.IndexEntry{{Name: "file.par2", DataLength: 10}, {Name: "FILE.par2", DataLength: 10}}
				},
			}, nil
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, bundler)

	require.NoError(t, err)
	require.True(t, parsedBundle)
	require.Equal(t, &sets[0], best)
	require.Equal(t, sets, all)
}

// Expectation: An empty bundle should return no best set and no sets, without merging other files.
func Test_ParsePar2Set_BundleEmpty_NoMerge_Error(t *testing.T) {
	t.Parallel()

	tests := map[string][]bundle.IndexEntry{
		"no entries":         nil,
		"index without sets": {{Name: "file.par2", DataLength: 10}},
	}

	for name, entries := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			parsedBundle := false
			par2er := psIndexOnly(t) // Fails the test on a merge.
			par2er.ParseFunc = func(_ io.ReadSeeker, _ bool) ([]par2.Set, error) {
				return nil, nil // Embedded index without any sets.
			}
			par2er.ParseFileFunc = func(_ afero.Fs, path string, _ bool) (*par2.File, error) {
				parsedBundle = true
				require.Equal(t, "/data/file.p2c.par2", path)

				return &par2.File{}, nil // Bundle without any sets.
			}
			par2er.ParseFileSetFunc = func(_ afero.Fs, _ []string, _ bool) (*par2.FileSet, error) {
				require.Fail(t, "should not start merging files")

				return nil, errors.New("unexpected call")
			}
			bundler := &testutil.MockBundleHandler{
				OpenFunc: func(_ afero.Fs, _ string) (schema.Bundle, error) {
					return &testutil.MockBundle{
						EntriesFunc: func() []bundle.IndexEntry {
							return entries
						},
						ExtractEntryFunc: func(_ bundle.IndexEntry, w io.Writer) error {
							_, err := w.Write([]byte("index data"))

							return err
						},
						RecoverySetIDValue: par2.Hash{1},
					}, nil
				},
			}

			best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, bundler)

			require.ErrorIs(t, err, ErrNoCompleteSet)
			require.True(t, parsedBundle)
			require.Nil(t, best)
			require.Empty(t, all)
		})
	}
}

// psBundleWithID returns a [testutil.MockBundleHandler] for a bundle with the given recovery set ID,
// whose embedded index is unusable (so the bundle is parsed as a whole).
func psBundleWithID(id byte) *testutil.MockBundleHandler {
	return &testutil.MockBundleHandler{
		OpenFunc: func(_ afero.Fs, _ string) (schema.Bundle, error) {
			return &testutil.MockBundle{
				RecoverySetIDValue: par2.Hash{id},
				EntriesFunc: func() []bundle.IndexEntry {
					return nil // No index entry.
				},
			}, nil
		},
	}
}

// Expectation: A bundle's recovery set ID should decide the best set, not an earlier (stale) complete set.
func Test_ParsePar2Set_BundleRecoverySetIDLocked_Table(t *testing.T) {
	t.Parallel()

	var confirmedSet byte = 1

	tests := map[string]struct {
		sets       []par2.Set
		want       int // -1 for no best set.
		noComplete bool
	}{
		"confirmed set complete after stale set": {
			sets: []par2.Set{psSet(2, true, "a.txt"), psSet(1, true, "a.txt")},
			want: 1,
		},
		"confirmed set incomplete after stale set": {
			sets:       []par2.Set{psSet(2, true, "a.txt"), psMissing(1, "a.txt")},
			want:       1,
			noComplete: true,
		},
		"only stale set": {
			sets:       []par2.Set{psSet(2, true, "a.txt")},
			want:       -1,
			noComplete: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			par2er := psIndexOnly(t, tc.sets...) // Fails the test on a merge.

			best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, psBundleWithID(confirmedSet))

			require.NotErrorIs(t, err, ErrNoConfirmedSetID)
			require.Equal(t, tc.noComplete, errors.Is(err, ErrNoCompleteSet))
			if !tc.noComplete {
				require.NoError(t, err)
			}
			if tc.want < 0 {
				require.Nil(t, best)
			} else {
				require.Equal(t, &tc.sets[tc.want], best)
			}
			require.Equal(t, tc.sets, all)
		})
	}
}

// Expectation: A complete embedded index of another set than the bundle's should not be used.
func Test_ParsePar2Set_BundleIndexOtherSetID_ParsesBundle_Success(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, true, "a.txt")}

	parsedBundle := false
	par2er := psIndexOnly(t) // Fails the test on a merge.
	par2er.ParseFunc = func(_ io.ReadSeeker, _ bool) ([]par2.Set, error) {
		return []par2.Set{psSet(2, true, "a.txt")}, nil // Complete, but not the confirmed ID.
	}
	par2er.ParseFileFunc = func(_ afero.Fs, _ string, _ bool) (*par2.File, error) {
		parsedBundle = true

		return &par2.File{Sets: sets}, nil // The confirmed ID.
	}

	// psBundleHandler sets confirmed ID as 1.
	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, psBundleHandler())

	require.NoError(t, err)
	require.True(t, parsedBundle)
	require.Equal(t, &sets[0], best)
	require.Equal(t, sets, all)
}

// Expectation: A bundle that cannot be opened should not confirm the set of its parsed contents.
func Test_ParsePar2Set_BundleOpenFails_Unconfirmed_Error(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, true, "a.txt")}
	par2er := psIndexOnly(t, sets...) // Fails the test on a merge.
	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(_ afero.Fs, _ string) (schema.Bundle, error) {
			return nil, errors.New("simulated open failure")
		},
	}

	best, all, err := ParsePar2Set(t.Context(), afero.NewMemMapFs(), "file.p2c.par2", "/data", par2er, bundler)

	require.ErrorIs(t, err, ErrNoConfirmedSetID)
	require.ErrorIs(t, err, ErrNoCompleteSet)
	require.Nil(t, best)
	require.Equal(t, sets, all)
}

// Expectation: isFatalParseErr should only report cancellations of the given context and parser panics.
func Test_isFatalParseErr_Success(t *testing.T) {
	t.Parallel()

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	require.True(t, isFatalParseErr(cancelled, fmt.Errorf("wrapped: %w", context.Canceled)))
	require.True(t, isFatalParseErr(t.Context(), fmt.Errorf("wrapped: %w", &par2.ParserPanicError{})))
	require.False(t, isFatalParseErr(t.Context(), fmt.Errorf("wrapped: %w", context.Canceled))) // Not our context.
	require.False(t, isFatalParseErr(cancelled, errors.New("other error")))
}

// Expectation: isCompleteSet should require a main packet and no missing files.
func Test_isCompleteSet_Success(t *testing.T) {
	t.Parallel()

	require.True(t, isCompleteSet(psSet(1, true, "a.txt")))
	require.False(t, isCompleteSet(psSet(1, false, "a.txt")))

	missingRec := psSet(1, true, "a.txt")
	missingRec.MissingRecoveryPackets = []par2.Hash{{2}}
	require.False(t, isCompleteSet(missingRec))

	missingNonRec := psSet(1, true, "a.txt")
	missingNonRec.MissingNonRecoveryPackets = []par2.Hash{{2}}
	require.False(t, isCompleteSet(missingNonRec))
}

// Expectation: pickSet should pick the set with the confirmed set ID (even if a
// complete set comes earlier), and no set if there is no confirmed set ID.
func Test_pickSet_Table(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		sets         []par2.Set
		setID        *par2.Hash
		want         int // Index of the expected best set, -1 for none.
		noComplete   bool
		notConfirmed bool
	}{
		"confirmed complete": {
			sets:  []par2.Set{psSet(2, true, "a.txt"), psSet(1, true, "a.txt")},
			setID: &par2.Hash{1},
			want:  1,
		},
		"confirmed incomplete over earlier complete": {
			sets:       []par2.Set{psSet(2, true, "a.txt"), psMissing(1, "a.txt")},
			setID:      &par2.Hash{1},
			want:       1,
			noComplete: true,
		},
		"confirmed not found": {
			sets:       []par2.Set{psSet(2, true, "a.txt")},
			setID:      &par2.Hash{1},
			want:       -1,
			noComplete: true,
		},
		"confirmed no sets": {
			setID:      &par2.Hash{1},
			want:       -1,
			noComplete: true,
		},
		"unconfirmed with complete set": {
			sets:         []par2.Set{psMissing(1, "a.txt"), psSet(2, true, "a.txt")},
			want:         -1,
			noComplete:   true,
			notConfirmed: true,
		},
		"unconfirmed no sets": {
			want:         -1,
			noComplete:   true,
			notConfirmed: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			best, all, err := pickSet(tc.sets, tc.setID)

			require.Equal(t, tc.sets, all)
			if tc.want < 0 {
				require.Nil(t, best)
			} else {
				require.Equal(t, &tc.sets[tc.want], best)
			}
			if !tc.noComplete && !tc.notConfirmed {
				require.NoError(t, err)
			}

			require.Equal(t, tc.noComplete, errors.Is(err, ErrNoCompleteSet))
			require.Equal(t, tc.notConfirmed, errors.Is(err, ErrNoConfirmedSetID))
		})
	}
}

// Expectation: pickSet should return a copy, not a pointer into the given slice.
func Test_pickSet_ReturnsCopy_Success(t *testing.T) {
	t.Parallel()

	sets := []par2.Set{psSet(1, true, "a.txt")}

	best, _, err := pickSet(sets, &par2.Hash{1})
	require.NoError(t, err)
	best.SetID = par2.Hash{9}

	require.Equal(t, par2.Hash{1}, sets[0].SetID)
}

// Expectation: par2SetMemberPaths should return par2Path even if the directory cannot be read.
func Test_par2SetMemberPaths_ReadDirError_Success(t *testing.T) {
	t.Parallel()

	paths := par2SetMemberPaths(afero.NewMemMapFs(), "/missing", "file.par2", "/missing/file.par2")

	require.Equal(t, []string{"/missing/file.par2"}, paths)
}

// Expectation: par2SetMemberPaths should match the root case-sensitively, the
// extensions case-insensitively, and skip non-members.
func Test_par2SetMemberPaths_RootCaseSensitive_Success(t *testing.T) {
	t.Parallel()

	fsys := afero.NewMemMapFs()
	for _, name := range []string{
		"file.par2", "file.VOL0+1.PAR2", "FILE.par2", "FILE.vol0+1.par2",
		"File.vol1+2.par2", "file.vol0+1.par2.bak", "other.par2",
	} {
		require.NoError(t, afero.WriteFile(fsys, "/data/"+name, nil, 0o644))
	}

	paths := par2SetMemberPaths(fsys, "/data", "file.par2", "/data/file.par2")

	require.Equal(t, []string{"/data/file.par2", "/data/file.VOL0+1.PAR2"}, paths)
}

// Expectation: ParseBundlePar2Index should return the recovery set ID once the bundle is opened, even on later errors.
func Test_ParseBundlePar2Index_RecoverySetID_Success(t *testing.T) {
	t.Parallel()

	_, setID, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, psBundleWithID(7))
	require.ErrorContains(t, err, "no index file in bundle")
	require.Equal(t, &par2.Hash{7}, setID)

	bundler := &testutil.MockBundleHandler{
		OpenFunc: func(_ afero.Fs, _ string) (schema.Bundle, error) {
			return nil, errors.New("simulated open failure")
		},
	}
	_, setID, err = ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundler)
	require.ErrorContains(t, err, "simulated open failure")
	require.Nil(t, setID)
}

// Expectation: The function should return an error for a non-bundle file.
func Test_ParseBundlePar2Index_NotBundleFile_Error(t *testing.T) {
	t.Parallel()

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.txt", &testutil.MockPar2Handler{}, &testutil.MockBundleHandler{})

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a bundle file")
}

// Expectation: The function should return an error for a plain par2 file that is not a bundle.
func Test_ParseBundlePar2Index_PlainPar2NotBundle_Error(t *testing.T) {
	t.Parallel()

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.par2", &testutil.MockPar2Handler{}, &testutil.MockBundleHandler{})

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a bundle file")
}

// Expectation: The function should return an error for a par2 volume file.
func Test_ParseBundlePar2Index_Par2VolumeFile_Error(t *testing.T) {
	t.Parallel()

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.vol0+1.par2", &testutil.MockPar2Handler{}, &testutil.MockBundleHandler{})

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no index file in bundle")
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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no index file in bundle")
}

// Expectation: The function should return an error when the index entries contain no sets.
func Test_ParseBundlePar2Index_IndexWithoutSets_Error(t *testing.T) {
	t.Parallel()

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
			return []par2.Set{}, nil
		},
	}

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.Nil(t, sets)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no sets in index file")
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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", &testutil.MockPar2Handler{}, bundleHandler)

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Equal(t, expectedSets, sets)
}

// Expectation: The function should reject a bundle with multiple index entries, without extracting any of them.
func Test_ParseBundlePar2Index_MultipleIndexEntries_Error(t *testing.T) {
	t.Parallel()

	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "a.par2", DataLength: 50},
				{Name: "readme.txt", DataLength: 10},
				{Name: "b.par2", DataLength: 60},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			require.Fail(t, "should not extract any entry")

			return errors.New("unexpected call")
		},
	}
	bundleHandler := &testutil.MockBundleHandler{
		OpenFunc: func(fsys afero.Fs, bundlePath string) (schema.Bundle, error) {
			return mockBundle, nil
		},
	}
	par2Handler := &testutil.MockPar2Handler{
		ParseFunc: func(r io.ReadSeeker, checkMD5 bool) ([]par2.Set, error) {
			require.Fail(t, "should not parse any entry")

			return nil, errors.New("unexpected call")
		},
	}

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.ErrorIs(t, err, errMultipleIndexFiles)
	require.Nil(t, sets)
}

// Expectation: The function should parse the single index entry, wherever it is among the entries.
func Test_ParseBundlePar2Index_IndexAfterVolumes_Success(t *testing.T) {
	t.Parallel()

	expectedSets := []par2.Set{{}}
	mockBundle := &testutil.MockBundle{
		EntriesFunc: func() []bundle.IndexEntry {
			return []bundle.IndexEntry{
				{Name: "file.vol0+1.par2", DataLength: 50},
				{Name: "file.par2", DataLength: 10},
			}
		},
		ExtractEntryFunc: func(e bundle.IndexEntry, w io.Writer) error {
			require.Equal(t, "file.par2", e.Name)
			_, err := w.Write([]byte("index"))

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Equal(t, expectedSets, sets)
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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

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

	sets, _, err := ParseBundlePar2Index(t.Context(), afero.NewMemMapFs(), "/data/file.p2c.par2", par2Handler, bundleHandler)

	require.NoError(t, err)
	require.Equal(t, expectedSets, sets)
}
