package util

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/desertwitch/par2cron/internal/par2"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
)

const maxIndexSize = 100 * 1024 * 1024 // 100MiB

var ErrNoCompleteSet = errors.New("no complete par2 set found")

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
// shows its set ID, only a set with that ID is selected as the best set, as
// that is the set par2cmdline locks onto when given the index. Only if the
// index yields no set ID at all are all merged sets candidates for best set.
//
// It returns the best set, being the first complete candidate in parse order
// (i.e. the job's own file first, then the volume files by name), and all sets
// found, including those with other set IDs. If none of the candidates for the
// best set is complete (according to above selection logic), [ErrNoCompleteSet]
// is returned together with all sets found, which may then be empty, incomplete,
// or even complete sets with other set IDs. Any other error (e.g. cancellation,
// a parser panic, or no file being parseable) is returned without sets.
//
// A nil error guarantees a non-nil best set.
func ParsePar2Set(ctx context.Context, fsys afero.Fs, indexName string, workingDir string, p schema.Par2Handler, b schema.BundleHandler) (*par2.Set, []par2.Set, error) {
	par2Path := filepath.Join(workingDir, indexName)

	if IsPar2Bundle(indexName) {
		if sets, err := ParseBundlePar2Index(ctx, fsys, par2Path, p, b); err == nil {
			if best := selectSet(sets, nil); best != nil {
				return best, sets, nil
			}
		}
	}

	ids := map[par2.Hash]struct{}{}

	f, err := p.ParseFile(ctx, fsys, par2Path, true)
	if err == nil {
		if best := selectSet(f.Sets, nil); best != nil {
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

		return nil, f.Sets, ErrNoCompleteSet
	}

	// The index is unreadable or incomplete, so merge in the other set members.
	fs, ferr := p.ParseFileSet(ctx, fsys, par2SetMemberPaths(fsys, workingDir, indexName, par2Path), true)
	if ferr != nil {
		if err == nil && !isFatalParseErr(ctx, ferr) {
			// Merging did not help, keep what the index had.
			return nil, f.Sets, ErrNoCompleteSet
		}

		return nil, nil, fmt.Errorf("failed to parse file set: %w", ferr)
	}

	best := selectSet(fs.SetsMerged, ids)
	if best == nil {
		return nil, fs.SetsMerged, ErrNoCompleteSet
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

// selectSet returns the first complete set, restricted to the given set IDs
// if any, or nil. Sets are in parse order, so the job's own file comes first.
func selectSet(sets []par2.Set, ids map[par2.Hash]struct{}) *par2.Set {
	for _, s := range sets {
		if _, ok := ids[s.SetID]; len(ids) > 0 && !ok {
			continue
		}
		if isCompleteSet(s) {
			return &s
		}
	}

	return nil
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
func ParseBundlePar2Index(ctx context.Context, fsys afero.Fs, path string, p schema.Par2Handler, b schema.BundleHandler) ([]par2.Set, error) {
	if !IsPar2Bundle(path) {
		return nil, errors.New("not a bundle file")
	}

	bun, err := b.Open(ctx, fsys, path)
	if err != nil {
		return nil, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer bun.Close()

	var found bool
	var sets []par2.Set
	for _, e := range bun.Entries() {
		if IsPar2Index(e.Name) {
			var buf bytes.Buffer

			if e.DataLength > maxIndexSize {
				return nil, errors.New("index file too large")
			}

			if err := bun.ExtractEntry(ctx, e, &buf); err != nil {
				return nil, fmt.Errorf("failed to extract index file: %w", err)
			}

			r := bytes.NewReader(buf.Bytes())

			s, err := p.Parse(ctx, r, true)
			if err != nil {
				return nil, fmt.Errorf("failed to parse index file: %w", err)
			}

			found = true
			sets = append(sets, s...)
		}
	}

	if len(sets) > 0 {
		return sets, nil
	}

	if found {
		return nil, errors.New("no sets in index file")
	}

	return nil, errors.New("no index file in bundle")
}
