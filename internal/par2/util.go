package par2

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"

	"github.com/spf13/afero"
)

var errUnexpectedLength = errors.New("unexpected length")

// Copy creates a deep copy of the MainPacket (returning nil on nil reciver).
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
func ParseFile(fsys afero.Fs, path string, panicAsErr bool) (p *File, e error) {
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

	sets, err := Parse(f, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAR2: %w", err)
	}

	return &File{
		Name: filepath.Base(path),
		Sets: sets,
	}, nil
}

// ParseFileSet parses an index PAR2 file and all related volume files.
// It ignores files which cannot be parsed, unless no files can be parsed.
// In case no files can be parsed, [errFileCorrupted] is returned instead.
//
// panicAsErr controls if a panic should be returned as [ParserPanicError].
// Beware the PAR2-specific packet and error handling as described in [Parse].
func ParseFileSet(fsys afero.Fs, indexFile string, panicAsErr bool) (*FileSet, error) {
	files := []File{}

	indexData, err := ParseFile(fsys, indexFile, panicAsErr)
	if err != nil {
		var pe *ParserPanicError

		if errors.As(err, &pe) {
			return nil, pe // Do not swallow panics.
		}
	} else {
		files = append(files, *indexData)
	}

	basePath := strings.TrimSuffix(indexFile, ".par2")
	pattern := basePath + "*.par2"

	matches, err := afero.Glob(fsys, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to glob: %w", err)
	}

	for _, match := range matches {
		if match == indexFile {
			continue
		}

		parsed, err := ParseFile(fsys, match, panicAsErr)
		if err != nil {
			var pe *ParserPanicError

			if errors.As(err, &pe) {
				return nil, pe // Do not swallow panics.
			}

			continue
		}

		files = append(files, *parsed)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("%w: no parseable files", errFileCorrupted)
	}

	merged, err := mergeFiles(files)
	if err != nil {
		return nil, fmt.Errorf("failed to merge files: %w", err)
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
