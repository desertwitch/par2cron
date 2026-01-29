package par2

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/spf13/afero"
)

var errUnexpectedLength = errors.New("unexpected length")

func (h *Hash) MarshalJSON() ([]byte, error) {
	by, err := json.Marshal(hex.EncodeToString(h[:]))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return by, nil
}

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

// ParseFile parses a PAR2 file into an [Archive].
func ParseFile(fsys afero.Fs, filename string) (*Archive, error) {
	f, err := fsys.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open PAR2 file: %w", err)
	}
	defer f.Close()

	sets, err := Parse(f, true)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PAR2: %w", err)
	}

	return &Archive{Time: time.Now(), Sets: sets}, nil
}

func sortFilePackets(list []FilePacket) {
	slices.SortFunc(list, func(a, b FilePacket) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}

		return bytes.Compare(a.FileID[:], b.FileID[:])
	})
}

func sortFileIDs(list []Hash) {
	slices.SortFunc(list, func(a, b Hash) int {
		return bytes.Compare(a[:], b[:])
	})
}
