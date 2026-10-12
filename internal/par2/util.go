package par2

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/spf13/afero"
)

var errUnexpectedLength = errors.New("unexpected length")

var _ io.Reader = (*contextReader)(nil)

// contextReader is an implementation of [io.Reader] that is Context-aware for
// receiving mid-transfer cancellation.
type contextReader struct {
	ctx    context.Context //nolint:containedctx
	reader io.Reader
}

// Read wraps the [io.Reader] reading function while being aware of and handling
// any mid-transfer Context cancellations.
func (cr *contextReader) Read(p []byte) (int, error) {
	select {
	case <-cr.ctx.Done():
		return 0, fmt.Errorf("context error: %w", cr.ctx.Err())
	default:
		return cr.reader.Read(p) //nolint:wrapcheck
	}
}

// Copy creates a deep copy of the MainPacket (returning nil on nil receiver).
func (m *MainPacket) Copy() *MainPacket {
	if m == nil {
		return nil
	}

	return &MainPacket{
		SetID:          m.SetID,
		SliceSize:      m.SliceSize,
		RecoveryIDs:    slices.Clone(m.RecoveryIDs),
		NonRecoveryIDs: slices.Clone(m.NonRecoveryIDs),
	}
}

// Equal checks if two MainPackets are equal.
// Returns true if both are nil, false if only one is nil.
func (m *MainPacket) Equal(other *MainPacket) bool {
	if m == nil && other == nil {
		return true
	}
	if m == nil || other == nil {
		return false
	}

	return m.SetID == other.SetID &&
		m.SliceSize == other.SliceSize &&
		slices.Equal(m.RecoveryIDs, other.RecoveryIDs) &&
		slices.Equal(m.NonRecoveryIDs, other.NonRecoveryIDs)
}

// ParserPanicError is an error that is returned on a recovered parsing panic.
// It contains the value of the panic and a byte slice containing a stack trace.
type ParserPanicError struct {
	Value any
	Stack []byte
}

// Error returns as string the error message.
func (e *ParserPanicError) Error() string {
	return fmt.Sprintf("parser panic: %v", e.Value)
}

// MarshalJSON marshals into a JSON byte slice.
func (h *Hash) MarshalJSON() ([]byte, error) {
	by, err := json.Marshal(hex.EncodeToString(h[:]))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return by, nil
}

// UnmarshalJSON unmarshals from a JSON byte slice.
func (h *Hash) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	decoded, err := hex.DecodeString(s)
	if err != nil {
		return fmt.Errorf("failed to decode to hex: %w", err)
	}

	if len(decoded) != HashSize {
		return fmt.Errorf("%w: expected %d bytes, got %d", errUnexpectedLength, HashSize, len(decoded))
	}

	copy(h[:], decoded)

	return nil
}

// ParseFile parses a PAR2 file into a [File] structure.
// panicAsErr controls if a panic should be returned as [ParserPanicError].
// Beware the PAR2-specific packet and error handling as described in [Parse].
//
//nolint:nonamedreturns
func ParseFile(ctx context.Context, fsys afero.Fs, path string, panicAsErr bool) (p *File, e error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open PAR2 file: %w", err)
	}
	defer f.Close()

	if panicAsErr {
		defer func() {
			if r := recover(); r != nil {
				p = nil
				e = &ParserPanicError{
					Value: r,
					Stack: debug.Stack(),
				}
			}
		}()
	}

	sets, err := Parse(ctx, f, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAR2 file: %w", err)
	}

	return &File{
		Name: filepath.Base(path),
		Sets: sets,
	}, nil
}

// ParseFileSet parses a slice of given non-duplicate PAR2 paths, typically
// containing an index file and several volume files belonging to the same set.
//
// Paths that fail to parse are skipped, and their errors are recorded in
// [FileSet.Errors] (nil if all paths parsed). An error is returned only if
// all paths fail, on context cancellation, on a panic (with panicAsErr), or
// when the parsed files conflict (e.g. differing main packets of one set).
// An empty paths slice returns an empty [FileSet] without error.
//
// panicAsErr controls if a panic should be returned as [ParserPanicError].
// Beware the PAR2-specific packet and error handling as described in [Parse].
func ParseFileSet(ctx context.Context, fsys afero.Fs, paths []string, panicAsErr bool) (*FileSet, error) {
	files := []File{}
	errs := []error{}
	errsByPath := make(map[string]error)

	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context error: %w", err)
		}

		parsed, err := ParseFile(ctx, fsys, path, panicAsErr)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
				return nil, fmt.Errorf("context error: %w", err)
			}
			if pe, ok := errors.AsType[*ParserPanicError](err); ok {
				return nil, pe // Do not swallow panics.
			}

			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			errsByPath[path] = err

			continue
		}

		files = append(files, *parsed)
	}

	if len(files) == 0 && len(errs) > 0 {
		return nil, fmt.Errorf("all paths failed to parse: %w",
			errors.Join(errs...))
	}

	merged, err := mergeFiles(files)
	if err != nil {
		return nil, fmt.Errorf("failed to merge files: %w", err)
	}

	merged.Paths = slices.Clone(paths)
	if len(errs) > 0 {
		merged.Errors = errsByPath
	}

	return merged, nil
}

// sortFilePackets sorts a slice of [FilePacket] by filename, ties by ID.
func sortFilePackets(list []FilePacket) {
	slices.SortFunc(list, func(a, b FilePacket) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}

		return bytes.Compare(a.FileID[:], b.FileID[:])
	})
}

// sortFileIDs sorts a slice of [Hash] by the contained hash.
func sortFileIDs(list []Hash) {
	slices.SortFunc(list, func(a, b Hash) int {
		return bytes.Compare(a[:], b[:])
	})
}
