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
	ErrNoCompleteSet      = errors.New("no both confirmed and complete par2 set")
	ErrNoConfirmedSetID   = errors.New("index or bundle yielded no par2 set ID")
	errMultipleIndexFiles = errors.New("multiple index files in bundle")
)

// ParsePar2Set parses the PAR2 set of indexName (an index or bundle file) in
// workingDir, returning the "best" (most trustworthy) set and all sets found.
//
// The best set is what par2cmdline locks onto when given indexName, which has
// the set ID of the first packet it reads: for an index file, the index's first
// set; for a bundle, the bundle's recovery set ID (its own packets come first
// and carry that set ID). As indexName itself names that set, it is "confirmed".
//
// indexName is parsed first (for a bundle: index first, on error the bundle
// as a whole). If the best set is complete there (main packet present and no
// missing file packets), no other files are read. Otherwise the index is
// assumed damaged (PAR2 parsing skips unreadable packets, so a damaged index
// parses with less rather than failing), and it is merged with the other set
// members in workingDir (volume files), the index first. Bundles are never
// merged, as they are self-contained (already contain all files of their set).
//
// Since members are matched by file name, they may contain other sets (e.g.
// stale volume files from an earlier version of the set). These never become
// the best set, even if they are more complete. If indexName yields no set ID
// at all (an unreadable index or one without any set, or a bundle that cannot
// be opened), there is no best set: par2cmdline then locks onto the first
// readable packet of the other files (read in directory order), but we make
// no recommendation or assumption about which one that would be.
//
// The returned pointer is a copy of the best set, it does not point into the
// returned slice. The slice holds all sets found in parse order (indexName
// first, then the volume files by name), including sets with other set IDs.
// It may or may not include the sets only found in the volume files, depending
// on whether the other set member files (volumes) were merged in (see above).
//
// [ErrNoCompleteSet] is returned together with all sets found and either the
// incomplete best set, or a nil best set if there is none (the confirmed set ID
// was not among the sets found, or no set ID could be confirmed). If indexName
// was too damaged to yield a set ID to confirm, error [ErrNoConfirmedSetID] is
// wrapped together with [ErrNoCompleteSet]. Other errors (e.g. cancellation, a
// parser panic, or no file being parseable) are returned without any sets.
//
// A nil error guarantees a non-nil and complete best set, confirmed by
// indexName as the set par2cmdline locks onto (and typically the only set).
func ParsePar2Set(ctx context.Context, fsys afero.Fs, indexName string, workingDir string, p schema.Par2Handler, b schema.BundleHandler) (*par2.Set, []par2.Set, error) {
	par2Path := filepath.Join(workingDir, indexName)

	if IsPar2Bundle(indexName) {
		sets, setID, err := ParseBundlePar2Index(ctx, fsys, par2Path, p, b)
		if err == nil && setID != nil {
			if best, all, perr := pickSet(sets, setID); perr == nil {
				// A complete set was found in the embedded index.
				return best, all, nil
			}
		}

		// The embedded index is unusable, so parse the bundle as a whole.
		f, err := p.ParseFile(ctx, fsys, par2Path, true)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse file: %w", err)
		}

		return pickSet(f.Sets, setID)
	}

	var setID *par2.Hash

	f, err := p.ParseFile(ctx, fsys, par2Path, true)
	if err != nil && isFatalParseErr(ctx, err) {
		return nil, nil, fmt.Errorf("failed to parse file: %w", err)
	}
	if err == nil && len(f.Sets) > 0 {
		// Even a damaged index can be trusted with the set IDs it still shows.
		setID = &f.Sets[0].SetID
		if isCompleteSet(f.Sets[0]) {
			return pickSet(f.Sets, setID)
		}
	}

	// The index is unreadable or incomplete, so merge in the other set members.
	fs, ferr := p.ParseFileSet(ctx, fsys, par2SetMemberPaths(fsys, workingDir, indexName, par2Path), true)
	if ferr != nil {
		if err == nil && !isFatalParseErr(ctx, ferr) {
			// Merging did not help, keep what the index had.
			return pickSet(f.Sets, setID)
		}

		return nil, nil, fmt.Errorf("failed to parse file set: %w", ferr)
	}

	return pickSet(fs.SetsMerged, setID)
}

// pickSet returns the set with the confirmed setID as best set, along with all
// sets and an error as described for [ParsePar2Set]. If setID is nil (no set ID
// was confirmed), there is no best set.
func pickSet(sets []par2.Set, setID *par2.Hash) (*par2.Set, []par2.Set, error) {
	if setID == nil {
		return nil, sets, fmt.Errorf("%w: %w", ErrNoCompleteSet, ErrNoConfirmedSetID)
	}

	i := slices.IndexFunc(sets, func(s par2.Set) bool {
		return s.SetID == *setID
	})
	if i < 0 {
		return nil, sets, ErrNoCompleteSet
	}

	best := sets[i] // Copy, so the caller doesn't hold a pointer into the slice.
	if !isCompleteSet(best) {
		return &best, sets, ErrNoCompleteSet
	}

	return &best, sets, nil
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

// ParseBundlePar2Index is a fast-path function that tries to parse only the
// index file from a bundle. Bundles with more than one index file are rejected,
// those are better handled by parsing the bundle as a whole (as par2cmdline
// would see it). It also returns the bundle's recovery set ID once the bundle
// could be opened (even on later errors), else it is nil.
func ParseBundlePar2Index(ctx context.Context, fsys afero.Fs, path string, p schema.Par2Handler, b schema.BundleHandler) ([]par2.Set, *par2.Hash, error) {
	if !IsPar2Bundle(path) {
		return nil, nil, errors.New("not a bundle file")
	}

	bun, err := b.Open(ctx, fsys, path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open bundle: %w", err)
	}
	defer bun.Close()

	setID := par2.Hash(bun.RecoverySetID())

	var index *bundle.IndexEntry
	for _, e := range bun.Entries() {
		if !IsPar2Index(e.Name) {
			continue
		}
		if index != nil {
			return nil, &setID, errMultipleIndexFiles
		}
		index = &e
	}
	if index == nil {
		return nil, &setID, errors.New("no index file in bundle")
	}

	if index.DataLength > maxIndexSize {
		return nil, &setID, errors.New("index file too large")
	}

	var buf bytes.Buffer
	if err := bun.ExtractEntry(ctx, *index, &buf); err != nil {
		return nil, &setID, fmt.Errorf("failed to extract index file: %w", err)
	}

	sets, err := p.Parse(ctx, bytes.NewReader(buf.Bytes()), true)
	if err != nil {
		return nil, &setID, fmt.Errorf("failed to parse index file: %w", err)
	}
	if len(sets) == 0 {
		return nil, &setID, errors.New("no sets in index file")
	}

	return sets, &setID, nil
}
