package bundle

import (
	"fmt"
	"io"

	"github.com/spf13/afero"
	"github.com/zeebo/blake3"
)

// writeToFile creates path on fsys, runs extract to populate it, and syncs.
func writeToFile(fsys afero.Fs, path string, extract func(io.Writer) error) error {
	out, err := fsys.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create: %w", err)
	}
	defer out.Close()

	if err := extract(out); err != nil {
		return fmt.Errorf("failed to extract: %w", err)
	}

	if err := out.Sync(); err != nil {
		return fmt.Errorf("failed to sync: %w", err)
	}

	return nil
}

// dataHash computes the data integrity hash from a byte slice.
func dataHash(data []byte) [32]byte {
	return blake3.Sum256(data)
}

// dataHashReader computes the data integrity hash by streaming from r.
func dataHashReader(r io.Reader) ([32]byte, error) {
	h := blake3.New()
	if _, err := io.Copy(h, r); err != nil {
		return [32]byte{},
			fmt.Errorf("failed to io: %w", err)
	}

	var sum [32]byte
	copy(sum[:], h.Sum(nil))

	return sum, nil
}

// padTo4 returns n rounded up to the next multiple of 4.
//
//nolint:mnd
func padTo4(n uint64) uint64 {
	return (n + 3) &^ 3
}

// isAligned4 checks if n is aligned to 4.
func isAligned4(n uint64) bool {
	return n%4 == 0
}
