package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/desertwitch/par2cron/internal/bundle"
	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

const maxIndexSize = 100 * 1024 * 1024 // 100MiB

var (
	ErrNoCompleteSet      = errors.New("no complete par2 set found")
	errMultipleIndexFiles = errors.New("multiple index files in bundle")
)

// ParsePar2Set parses the PAR2 set of a job, given by the name of its index
// (or bundle) file in workingDir.
//
// The job's own file is tried first: for a bundle its embedded index, then
// the file itself. If it yields a complete set (main packet present and no
// missing file packets), that is used and no other files are read. Otherwise
// the index is assumed damaged (PAR2 parsing skips unreadable packets, so a
// damaged index parses with less rather than failing), and it is merged with
// the other set members found in workingDir (volume files), the index first.
// Bundles are never merged, as they already contain all files of their set.
//
// Since members are matched by file name, they may contain other sets (e.g.
// stale volume files from an earlier version of the set). If the index still
// shows its set ID, only a set with that ID is a candidate for the best set,
// as that is the set par2cmdline locks onto when given the index. Only if the
// index yields no set ID at all are all merged sets candidates for best set.
//
// It returns the best set and all sets found (in parse order, i.e. the job's
// own file first, then the volume files by name, including sets with other set
// IDs). The best set is the most useful candidate: the first complete one, else
// the first with a main packet, else the first one at all.
//
// [ErrNoCompleteSet] is returned together with all sets found and either an
// incomplete best set, or a nil best set if there was no candidate: either no
// sets were found at all (the set slice is then empty), or the index showed
// its set ID but only sets with other set IDs were found. Other errors (e.g.
// cancellation, a parser panic, or no file being parseable) are returned
// without any sets. A nil error guarantees a non-nil and complete best set.
func ParsePar2Set(ctx context.Context, fsys afero.Fs, indexName string, workingDir string, p schema.Par2Handler, b schema.BundleHandler) (*par2.Set, []par2.Set, error) {
	par2Path := filepath.Join(workingDir, indexName)

	if IsPar2Bundle(indexName) {
		if sets, err := ParseBundlePar2Index(ctx, fsys, par2Path, p, b); err == nil {
			if best, complete := selectSet(sets, nil); complete {
				return best, sets, nil
			}
		}
	}

	ids := map[par2.Hash]struct{}{}

	f, err := p.ParseFile(ctx, fsys, par2Path, true)
	if err == nil {
		if best, complete := selectSet(f.Sets, nil); complete {
			// A complete set was found in the index.
			return best, f.Sets, nil
		}
		for _, s := range f.Sets {
			// Damaged index, but its set IDs can be trusted.
			ids[s.SetID] = struct{}{}
		}
	} else if isFatalParseErr(ctx, err) {
		return nil, nil, fmt.Errorf("failed to parse file: %w", err)
	}

	// A bundle contains all files of its set, there's nothing to merge.
	if IsPar2Bundle(indexName) {
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse file: %w", err)
		}

		best, _ := selectSet(f.Sets, nil)

		return best, f.Sets, ErrNoCompleteSet
	}

	// The index is unreadable or incomplete, so merge in the other set members.
	fs, ferr := p.ParseFileSet(ctx, fsys, par2SetMemberPaths(fsys, workingDir, indexName, par2Path), true)
	if ferr != nil {
		if err == nil && !isFatalParseErr(ctx, ferr) {
			// Merging did not help, keep what the index had.
			best, _ := selectSet(f.Sets, nil)

			return best, f.Sets, ErrNoCompleteSet
		}

		return nil, nil, fmt.Errorf("failed to parse file set: %w", ferr)
	}

	best, complete := selectSet(fs.SetsMerged, ids)
	if !complete {
		return best, fs.SetsMerged, ErrNoCompleteSet
	}

	return best, fs.SetsMerged, nil
}

// isFatalParseErr reports if an error must not be handled by falling back.
func isFatalParseErr(ctx context.Context, err error) bool {
	if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
		return true
	}
	if _, isPanic := errors.AsType[*par2.ParserPanicError](err); isPanic {
		return true
	}

	return false
}

// isCompleteSet reports if a set has its main packet and no missing files.
func isCompleteSet(s par2.Set) bool {
	return s.MainPacket != nil &&
		len(s.MissingRecoveryPackets) == 0 &&
		len(s.MissingNonRecoveryPackets) == 0
}

// Ranks of sets for [selectSet], from least to most useful.
const (
	setRankAny      = iota + 1 // Neither main packet nor complete
	setRankMain                // Main packet, but with missing files
	setRankComplete            // Main packet and no missing files
)

// selectSet returns the most useful set, restricted to the given set IDs if
// any: the first complete set, else the first with a main packet, else the
// first set at all. Sets are in parse order, so the job's own file comes
// first. It also reports if the returned set is complete, the set is nil
// if there are no candidates.
func selectSet(sets []par2.Set, ids map[par2.Hash]struct{}) (*par2.Set, bool) {
	best, bestRank := -1, 0
	for i, s := range sets {
		if _, ok := ids[s.SetID]; len(ids) > 0 && !ok {
			continue
		}

		rank := setRankAny
		if s.MainPacket != nil {
			rank = setRankMain
		}
		if isCompleteSet(s) {
			rank = setRankComplete
		}

		// Strictly greater, so that the earlier set wins on equal ranks.
		if rank > bestRank {
			best, bestRank = i, rank
		}
		if bestRank == setRankComplete {
			break
		}
	}
	if best < 0 {
		return nil, false
	}

	s := sets[best] // Copy, so the caller doesn't hold a pointer into the slice.

	return &s, bestRank == setRankComplete
}

// par2SetMemberPaths returns the set member paths, with par2Path first.
// par2Path is always included, so that a missing index without any other
// members results in a "all paths failed" error rather than an empty result.
func par2SetMemberPaths(fsys afero.Fs, workingDir, par2Name, par2Path string) []string {
	infos, err := afero.ReadDir(fsys, workingDir)
	if err != nil {
		return []string{par2Path}
	}

	paths := make([]string, 1, len(infos)+1)
	paths[0] = par2Path

	for _, info := range infos {
		path := filepath.Join(workingDir, info.Name())
		if info.IsDir() || path == par2Path || !IsPar2SetMember(par2Name, info.Name()) {
			continue
		}
		paths = append(paths, path)
	}
	slices.Sort(paths[1:]) // Keep par2Path first.

	return paths
}

// ParseBundlePar2Index tries to parse only the index file from a bundle.
// Bundles with more than one index file are rejected, those are better
// handled by parsing the bundle as a whole (as par2cmdline would see it).
func ParseBundlePar2Index(ctx context.Context, fsys afero.Fs, path string, p schema.Par2Handler, b schema.BundleHandler) ([]par2.Set, error) {
	if !IsPar2Bundle(path) {
		return nil, errors.New("not a bundle file")
	}

	bun, err := b.Open(ctx, fsys, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer bun.Close()

	var index *bundle.IndexEntry
	for _, e := range bun.Entries() {
		if !IsPar2Index(e.Name) {
			continue
		}
		if index != nil {
			return nil, errMultipleIndexFiles
		}
		index = &e
	}
	if index == nil {
		return nil, errors.New("no index file in bundle")
	}

	if index.DataLength > maxIndexSize {
		return nil, errors.New("index file too large")
	}

	var buf bytes.Buffer
	if err := bun.ExtractEntry(ctx, *index, &buf); err != nil {
		return nil, fmt.Errorf("failed to extract index file: %w", err)
	}

	sets, err := p.Parse(ctx, bytes.NewReader(buf.Bytes()), true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse index file: %w", err)
	}
	if len(sets) == 0 {
		return nil, errors.New("no sets in index file")
	}

	return sets, nil
}
