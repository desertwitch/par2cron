package par2

import (
	"fmt"
	"slices"
)

type mergedSet struct {
	setID              Hash
	mainPacket         *MainPacket
	recoveryFiles      map[Hash]FilePacket
	nonRecoveryFiles   map[Hash]FilePacket
	strayFiles         map[Hash]FilePacket
	missingRecovery    map[Hash]struct{}
	missingNonRecovery map[Hash]struct{}
}

// mergeFiles combines multiple PAR2 files into a unified FileSet.
// It merges sets with the same SetID, resolves missing files from stray
// packets across files, and validates that MainPackets are all consistent.
func mergeFiles(files []File) (*FileSet, error) {
	if len(files) == 0 {
		return &FileSet{}, nil
	}

	// Group and merge all sets by SetID
	setMap, err := groupSetsByID(files)
	if err != nil {
		return nil, err
	}

	// Preserve set order from first file
	order := buildSetOrder(files, setMap)

	// Build final merged sets
	results := buildMergedSets(order, setMap)

	return &FileSet{
		Files:      slices.Clone(files),
		SetsMerged: results,
		IsComplete: allSetsComplete(results),
	}, nil
}

// groupSetsByID groups sets from all files by their SetID and validates consistency.
func groupSetsByID(files []File) (map[Hash]*mergedSet, error) {
	setMap := make(map[Hash]*mergedSet)

	for _, file := range files {
		for _, set := range file.Sets {
			ms, exists := setMap[set.SetID]
			if !exists {
				ms = &mergedSet{
					setID:              set.SetID,
					recoveryFiles:      make(map[Hash]FilePacket),
					nonRecoveryFiles:   make(map[Hash]FilePacket),
					strayFiles:         make(map[Hash]FilePacket),
					missingRecovery:    make(map[Hash]struct{}),
					missingNonRecovery: make(map[Hash]struct{}),
				}
				setMap[set.SetID] = ms
			}

			// Validate MainPacket consistency
			if set.MainPacket != nil {
				if ms.mainPacket == nil {
					// The packet belongs to the set, so copy it.
					ms.mainPacket = set.MainPacket.Copy()
				} else if !ms.mainPacket.Equal(set.MainPacket) {
					// Two differing main packets with the same ID.
					return nil, fmt.Errorf("%w: conflicting main packets", errFileCorrupted)
				}
			}

			// Merge all files
			for _, fp := range set.RecoverySet {
				ms.recoveryFiles[fp.FileID] = fp
			}
			for _, fp := range set.NonRecoverySet {
				ms.nonRecoveryFiles[fp.FileID] = fp
			}
			for _, fp := range set.StrayPackets {
				ms.strayFiles[fp.FileID] = fp
			}

			// Collect missing IDs
			for _, id := range set.MissingRecoveryPackets {
				ms.missingRecovery[id] = struct{}{}
			}
			for _, id := range set.MissingNonRecoveryPackets {
				ms.missingNonRecovery[id] = struct{}{}
			}
		}
	}

	return setMap, nil
}

// buildSetOrder creates an ordered list of SetIDs, preserving file order.
func buildSetOrder(files []File, setMap map[Hash]*mergedSet) []Hash {
	var order []Hash
	seen := make(map[Hash]bool)

	// Add sets from each file in order, preserving their original sequence
	for _, file := range files {
		for _, set := range file.Sets {
			if _, exists := setMap[set.SetID]; exists && !seen[set.SetID] {
				order = append(order, set.SetID)
				seen[set.SetID] = true
			}
		}
	}

	return order
}

// buildMergedSets converts mergedSet map to final Set slice.
func buildMergedSets(order []Hash, setMap map[Hash]*mergedSet) []Set {
	results := make([]Set, 0, len(order))

	for _, id := range order {
		ms := setMap[id]

		// Resolve missing and stray packets (after merge)
		ms.resolveUnlisted()

		// Build final lists
		recoveryList := make([]FilePacket, 0, len(ms.recoveryFiles))
		for _, fp := range ms.recoveryFiles {
			recoveryList = append(recoveryList, fp)
		}
		sortFilePackets(recoveryList)

		nonRecoveryList := make([]FilePacket, 0, len(ms.nonRecoveryFiles))
		for _, fp := range ms.nonRecoveryFiles {
			nonRecoveryList = append(nonRecoveryList, fp)
		}
		sortFilePackets(nonRecoveryList)

		strayList := make([]FilePacket, 0, len(ms.strayFiles))
		for _, fp := range ms.strayFiles {
			strayList = append(strayList, fp)
		}
		sortFilePackets(strayList)

		recoveryMissing := make([]Hash, 0, len(ms.missingRecovery))
		for h := range ms.missingRecovery {
			recoveryMissing = append(recoveryMissing, h)
		}
		sortFileIDs(recoveryMissing)

		nonRecoveryMissing := make([]Hash, 0, len(ms.missingNonRecovery))
		for h := range ms.missingNonRecovery {
			nonRecoveryMissing = append(nonRecoveryMissing, h)
		}
		sortFileIDs(nonRecoveryMissing)

		results = append(results, Set{
			SetID:                     id,
			MainPacket:                ms.mainPacket,
			RecoverySet:               recoveryList,
			NonRecoverySet:            nonRecoveryList,
			StrayPackets:              strayList,
			MissingRecoveryPackets:    recoveryMissing,
			MissingNonRecoveryPackets: nonRecoveryMissing,
			IsComplete:                len(strayList) == 0 && len(recoveryMissing) == 0 && len(nonRecoveryMissing) == 0,
		})
	}

	return results
}

// resolveUnlisted attempts to resolve missing files from stray packets,
// and removes strays/missing that are already in recovery/non-recovery lists.
func (ms *mergedSet) resolveUnlisted() {
	// Remove strays/missing that are already in recovery or non-recovery lists
	for id := range ms.recoveryFiles {
		delete(ms.strayFiles, id)
		delete(ms.missingRecovery, id)
	}
	for id := range ms.nonRecoveryFiles {
		delete(ms.strayFiles, id)
		delete(ms.missingNonRecovery, id)
	}

	// Check if any missing recovery files are in strays
	for id := range ms.missingRecovery {
		if fp, found := ms.strayFiles[id]; found {
			ms.recoveryFiles[id] = fp
			delete(ms.strayFiles, id)
			delete(ms.missingRecovery, id)
		}
	}

	// Check if any missing non-recovery files are in strays
	for id := range ms.missingNonRecovery {
		if fp, found := ms.strayFiles[id]; found {
			ms.nonRecoveryFiles[id] = fp
			delete(ms.strayFiles, id)
			delete(ms.missingNonRecovery, id)
		}
	}
}

func allSetsComplete(sets []Set) bool {
	for _, set := range sets {
		if !set.IsComplete {
			return false
		}
	}

	return true
}
