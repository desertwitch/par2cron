package par2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Expectation: mergeFiles should handle empty file slice.
func Test_mergeFiles_EmptyFiles_Success(t *testing.T) {
	t.Parallel()

	result, err := mergeFiles([]File{})
	require.NoError(t, err)

	require.NotNil(t, result)
	require.Empty(t, result.Files)
	require.Empty(t, result.SetsMerged)
}

// Expectation: mergeFiles should handle single file with single set.
func Test_mergeFiles_SingleFileSingleSet_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Name: "test.par2",
			Sets: []Set{
				{
					SetID: Hash(sID),
					MainPacket: &MainPacket{
						SetID:       Hash(sID),
						SliceSize:   4096,
						RecoveryIDs: []Hash{Hash(idA)},
					},
					RecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idA), Name: "file.txt", Size: 100},
					},
				},
			},
		},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	require.Len(t, result.Files, 1)
	require.Len(t, result.SetsMerged, 1)
	require.Equal(t, files[0].Sets[0].RecoverySet, result.SetsMerged[0].RecoverySet)
}

// Expectation: mergeFiles should merge multiple files with same set ID.
func Test_mergeFiles_MultipleFilesSameSetID_Success(t *testing.T) {
	t.Parallel()

	mainPacket := &MainPacket{
		SetID:       Hash(sID),
		SliceSize:   4096,
		RecoveryIDs: []Hash{Hash(idA), Hash(idB)},
	}

	files := []File{
		{
			Name: "test.vol00+01.par2",
			Sets: []Set{
				{
					SetID:      Hash(sID),
					MainPacket: mainPacket,
					RecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idA), Name: "file1.txt", Size: 100},
					},
					MissingRecoveryPackets: []Hash{Hash(idB)},
				},
			},
		},
		{
			Name: "test.vol01+02.par2",
			Sets: []Set{
				{
					SetID:      Hash(sID),
					MainPacket: mainPacket,
					RecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idB), Name: "file2.txt", Size: 200},
					},
					MissingRecoveryPackets: []Hash{Hash(idA)},
				},
			},
		},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	require.Len(t, result.Files, 2)
	require.Len(t, result.SetsMerged, 1)
	require.Len(t, result.SetsMerged[0].RecoverySet, 2)
	require.Empty(t, result.SetsMerged[0].MissingRecoveryPackets)
}

// Expectation: mergeFiles should handle multiple files with different set IDs.
func Test_mergeFiles_MultipleFilesDifferentSetIDs_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Name: "archive1.par2",
			Sets: []Set{
				{
					SetID: Hash(idA),
					MainPacket: &MainPacket{
						SetID:       Hash(idA),
						SliceSize:   4096,
						RecoveryIDs: []Hash{Hash(idA)},
					},
					RecoverySet: []FilePacket{
						{SetID: Hash(idA), FileID: Hash(idA), Name: "a.txt", Size: 100},
					},
				},
			},
		},
		{
			Name: "archive2.par2",
			Sets: []Set{
				{
					SetID: Hash(idB),
					MainPacket: &MainPacket{
						SetID:       Hash(idB),
						SliceSize:   4096,
						RecoveryIDs: []Hash{Hash(idB)},
					},
					RecoverySet: []FilePacket{
						{SetID: Hash(idB), FileID: Hash(idB), Name: "b.txt", Size: 200},
					},
				},
			},
		},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	require.Len(t, result.Files, 2)
	require.Len(t, result.SetsMerged, 2)
	require.Len(t, result.SetsMerged[0].RecoverySet, 1)
	require.Len(t, result.SetsMerged[1].RecoverySet, 1)
}

// Expectation: mergeFiles should preserve set order from first file.
func Test_mergeFiles_PreservesSetOrder_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Name: "multi.par2",
			Sets: []Set{
				{SetID: Hash(idC)},
				{SetID: Hash(idA)},
				{SetID: Hash(idB)},
			},
		},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	require.Len(t, result.SetsMerged, 3)
	require.Equal(t, Hash(idC), result.SetsMerged[0].SetID)
	require.Equal(t, Hash(idA), result.SetsMerged[1].SetID)
	require.Equal(t, Hash(idB), result.SetsMerged[2].SetID)
}

// Expectation: mergeFiles should return error on conflicting main packets.
func Test_mergeFiles_ConflictingMainPackets_Error(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Name: "file1.par2",
			Sets: []Set{
				{
					SetID: Hash(sID),
					MainPacket: &MainPacket{
						SetID:       Hash(sID),
						SliceSize:   4096,
						RecoveryIDs: []Hash{Hash(idA)},
					},
				},
			},
		},
		{
			Name: "file2.par2",
			Sets: []Set{
				{
					SetID: Hash(sID),
					MainPacket: &MainPacket{
						SetID:       Hash(sID),
						SliceSize:   8192, // Different slice size = conflict
						RecoveryIDs: []Hash{Hash(idA)},
					},
				},
			},
		},
	}

	_, err := mergeFiles(files)
	require.ErrorIs(t, err, errUnresolvableConflict)
}

// Expectation: mergeFiles should clone files slice.
func Test_mergeFiles_ClonesFilesSlice_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{Name: "test.par2", Sets: []Set{{SetID: Hash(sID)}}},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	// Modify original slice
	files[0].Name = "modified.par2"

	// Result should be unaffected
	require.Equal(t, "test.par2", result.Files[0].Name)
}

// Expectation: Full merge workflow should resolve strays across multiple files.
func Test_mergeFiles_ResolvesStraysAcrossFiles_Success(t *testing.T) {
	t.Parallel()

	// File 1 has main packet listing idA and idB, but only has idA
	// File 2 has idB as a stray (without main packet)

	mainPacket := &MainPacket{
		SetID:       Hash(sID),
		SliceSize:   4096,
		RecoveryIDs: []Hash{Hash(idA), Hash(idB)},
	}

	files := []File{
		{
			Name: "test.par2",
			Sets: []Set{
				{
					SetID:      Hash(sID),
					MainPacket: mainPacket,
					RecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idA), Name: "a.txt", Size: 100},
					},
					MissingRecoveryPackets: []Hash{Hash(idB)},
				},
			},
		},
		{
			Name: "test.vol00+01.par2",
			Sets: []Set{
				{
					SetID: Hash(sID),
					StrayPackets: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idB), Name: "b.txt", Size: 200},
					},
				},
			},
		},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	require.Len(t, result.SetsMerged, 1)
	require.Len(t, result.SetsMerged[0].RecoverySet, 2)
	require.Empty(t, result.SetsMerged[0].StrayPackets)
	require.Empty(t, result.SetsMerged[0].MissingRecoveryPackets)
}

// Expectation: Full merge workflow should handle overlapping file packets.
func Test_mergeFiles_OverlappingFilePackets_Success(t *testing.T) {
	t.Parallel()

	mainPacket := &MainPacket{
		SetID:       Hash(sID),
		SliceSize:   4096,
		RecoveryIDs: []Hash{Hash(idA)},
	}

	// Both files have the same recovery file
	files := []File{
		{
			Name: "test.par2",
			Sets: []Set{
				{
					SetID:      Hash(sID),
					MainPacket: mainPacket,
					RecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idA), Name: "a.txt", Size: 100},
					},
				},
			},
		},
		{
			Name: "test.vol00+01.par2",
			Sets: []Set{
				{
					SetID:      Hash(sID),
					MainPacket: mainPacket,
					RecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idA), Name: "a.txt", Size: 100},
					},
				},
			},
		},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	require.Len(t, result.SetsMerged, 1)
	// Should only have one entry (deduplicated by map key)
	require.Len(t, result.SetsMerged[0].RecoverySet, 1)
}

// Expectation: Full merge should handle mixed recovery and non-recovery across files.
func Test_mergeFiles_MixedRecoveryAndNonRecoveryAcrossFiles_Success(t *testing.T) {
	t.Parallel()

	mainPacket := &MainPacket{
		SetID:          Hash(sID),
		SliceSize:      4096,
		RecoveryIDs:    []Hash{Hash(idA)},
		NonRecoveryIDs: []Hash{Hash(idB)},
	}

	files := []File{
		{
			Name: "test.par2",
			Sets: []Set{
				{
					SetID:      Hash(sID),
					MainPacket: mainPacket,
					RecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idA), Name: "recovery.txt", Size: 100},
					},
					MissingNonRecoveryPackets: []Hash{Hash(idB)},
				},
			},
		},
		{
			Name: "test.vol00+01.par2",
			Sets: []Set{
				{
					SetID:                  Hash(sID),
					MainPacket:             mainPacket,
					MissingRecoveryPackets: []Hash{Hash(idA)},
					NonRecoverySet: []FilePacket{
						{SetID: Hash(sID), FileID: Hash(idB), Name: "nonrecovery.txt", Size: 200},
					},
				},
			},
		},
	}

	result, err := mergeFiles(files)
	require.NoError(t, err)

	require.Len(t, result.SetsMerged, 1)
	require.Len(t, result.SetsMerged[0].RecoverySet, 1)
	require.Len(t, result.SetsMerged[0].NonRecoverySet, 1)
	require.Empty(t, result.SetsMerged[0].MissingRecoveryPackets)
	require.Empty(t, result.SetsMerged[0].MissingNonRecoveryPackets)
}

// Expectation: groupSetsByID should handle empty files slice.
func Test_groupSetsByID_EmptyFiles_Success(t *testing.T) {
	t.Parallel()

	result, err := groupSetsByID([]File{})
	require.NoError(t, err)

	require.Empty(t, result)
}

// Expectation: groupSetsByID should merge recovery files from multiple sources.
func Test_groupSetsByID_MergesRecoveryFiles_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Sets: []Set{
				{
					SetID: Hash(sID),
					RecoverySet: []FilePacket{
						{FileID: Hash(idA), Name: "a.txt"},
					},
				},
			},
		},
		{
			Sets: []Set{
				{
					SetID: Hash(sID),
					RecoverySet: []FilePacket{
						{FileID: Hash(idB), Name: "b.txt"},
					},
				},
			},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.Len(t, result, 1)
	require.Len(t, result[Hash(sID)].recoveryFiles, 2)
}

// Expectation: groupSetsByID should merge non-recovery files from multiple sources.
func Test_groupSetsByID_MergesNonRecoveryFiles_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Sets: []Set{
				{
					SetID: Hash(sID),
					NonRecoverySet: []FilePacket{
						{FileID: Hash(idA), Name: "a.txt"},
					},
				},
			},
		},
		{
			Sets: []Set{
				{
					SetID: Hash(sID),
					NonRecoverySet: []FilePacket{
						{FileID: Hash(idB), Name: "b.txt"},
					},
				},
			},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.Len(t, result, 1)
	require.Len(t, result[Hash(sID)].nonRecoveryFiles, 2)
}

// Expectation: groupSetsByID should merge stray packets from multiple sources.
func Test_groupSetsByID_MergesStrayPackets_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Sets: []Set{
				{
					SetID: Hash(sID),
					StrayPackets: []FilePacket{
						{FileID: Hash(idA), Name: "stray1.txt"},
					},
				},
			},
		},
		{
			Sets: []Set{
				{
					SetID: Hash(sID),
					StrayPackets: []FilePacket{
						{FileID: Hash(idB), Name: "stray2.txt"},
					},
				},
			},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.Len(t, result, 1)
	require.Len(t, result[Hash(sID)].strayFiles, 2)
}

// Expectation: groupSetsByID should collect missing recovery IDs.
func Test_groupSetsByID_CollectsMissingRecoveryIDs_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Sets: []Set{
				{
					SetID:                  Hash(sID),
					MissingRecoveryPackets: []Hash{Hash(idA), Hash(idB)},
				},
			},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.Len(t, result[Hash(sID)].missingRecovery, 2)
}

// Expectation: groupSetsByID should collect missing non-recovery IDs.
func Test_groupSetsByID_CollectsMissingNonRecoveryIDs_Success(t *testing.T) {
	t.Parallel()

	files := []File{
		{
			Sets: []Set{
				{
					SetID:                     Hash(sID),
					MissingNonRecoveryPackets: []Hash{Hash(idA), Hash(idB)},
				},
			},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.Len(t, result[Hash(sID)].missingNonRecovery, 2)
}

// Expectation: groupSetsByID should copy main packet when first encountered.
func Test_groupSetsByID_CopiesMainPacket_Success(t *testing.T) {
	t.Parallel()

	mainPacket := &MainPacket{
		SetID:       Hash(sID),
		SliceSize:   4096,
		RecoveryIDs: []Hash{Hash(idA)},
	}

	files := []File{
		{
			Sets: []Set{
				{
					SetID:      Hash(sID),
					MainPacket: mainPacket,
				},
			},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.NotNil(t, result[Hash(sID)].mainPacket)
	require.Equal(t, mainPacket, result[Hash(sID)].mainPacket)   // content
	require.NotSame(t, mainPacket, result[Hash(sID)].mainPacket) // pointer
}

// Expectation: groupSetsByID should accept identical main packets.
func Test_groupSetsByID_IdenticalMainPackets_Success(t *testing.T) {
	t.Parallel()

	mainPacket := &MainPacket{
		SetID:       Hash(sID),
		SliceSize:   4096,
		RecoveryIDs: []Hash{Hash(idA)},
	}

	files := []File{
		{
			Sets: []Set{{SetID: Hash(sID), MainPacket: mainPacket}},
		},
		{
			Sets: []Set{{SetID: Hash(sID), MainPacket: mainPacket}},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.NotNil(t, result[Hash(sID)].mainPacket)
}

// Expectation: groupSetsByID should handle nil main packet in subsequent sets.
func Test_groupSetsByID_NilMainPacketSubsequent_Success(t *testing.T) {
	t.Parallel()

	mainPacket := &MainPacket{
		SetID:     Hash(sID),
		SliceSize: 4096,
	}

	files := []File{
		{
			Sets: []Set{{SetID: Hash(sID), MainPacket: mainPacket}},
		},
		{
			Sets: []Set{{SetID: Hash(sID), MainPacket: nil}},
		},
	}

	result, err := groupSetsByID(files)
	require.NoError(t, err)

	require.NotNil(t, result[Hash(sID)].mainPacket)
}

// Expectation: buildSetOrder should return empty for empty files.
func Test_buildSetOrder_EmptyFiles_Success(t *testing.T) {
	t.Parallel()

	mergedSets := make(map[Hash]*mergedSet)
	order := buildSetOrder([]File{}, mergedSets)
	require.Empty(t, order)
}

// Expectation: buildSetOrder should preserve order from first file.
func Test_buildSetOrder_PreservesFirstFileOrder_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(idA): {},
		Hash(idB): {},
		Hash(idC): {},
	}

	files := []File{
		{
			Sets: []Set{
				{SetID: Hash(idC)},
				{SetID: Hash(idA)},
				{SetID: Hash(idB)},
			},
		},
	}

	order := buildSetOrder(files, mergedSets)
	require.Len(t, order, 3)

	require.Equal(t, Hash(idC), order[0])
	require.Equal(t, Hash(idA), order[1])
	require.Equal(t, Hash(idB), order[2])
}

// Expectation: buildSetOrder should not duplicate set IDs.
func Test_buildSetOrder_NoDuplicates_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(idA): {},
	}

	files := []File{
		{
			Sets: []Set{{SetID: Hash(idA)}},
		},
		{
			Sets: []Set{{SetID: Hash(idA)}},
		},
	}

	order := buildSetOrder(files, mergedSets)
	require.Len(t, order, 1)

	require.Equal(t, Hash(idA), order[0])
}

// Expectation: buildSetOrder should skip sets not in mergedSets.
func Test_buildSetOrder_SkipsUnknownSets_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(idA): {},
	}

	files := []File{
		{
			Sets: []Set{
				{SetID: Hash(idA)},
				{SetID: Hash(idB)}, // Not in mergedSets
			},
		},
	}

	order := buildSetOrder(files, mergedSets)
	require.Len(t, order, 1)

	require.Equal(t, Hash(idA), order[0])
}

// Expectation: buildSetOrder should add new sets from subsequent files.
func Test_buildSetOrder_AddsNewSetsFromSubsequentFiles_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(idA): {},
		Hash(idB): {},
	}

	files := []File{
		{
			Sets: []Set{{SetID: Hash(idA)}},
		},
		{
			Sets: []Set{{SetID: Hash(idB)}},
		},
	}

	order := buildSetOrder(files, mergedSets)
	require.Len(t, order, 2)

	require.Equal(t, Hash(idA), order[0])
	require.Equal(t, Hash(idB), order[1])
}

// Expectation: buildMergedSets should handle empty order.
func Test_buildMergedSets_EmptyOrder_Success(t *testing.T) {
	t.Parallel()

	result := buildMergedSets([]Hash{}, map[Hash]*mergedSet{})
	require.Empty(t, result)
}

// Expectation: buildMergedSets should build sets in order.
func Test_buildMergedSets_PreservesOrder_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(idA): {
			setID:              Hash(idA),
			recoveryFiles:      make(map[Hash]FilePacket),
			nonRecoveryFiles:   make(map[Hash]FilePacket),
			strayFiles:         make(map[Hash]FilePacket),
			missingRecovery:    make(map[Hash]struct{}),
			missingNonRecovery: make(map[Hash]struct{}),
		},
		Hash(idB): {
			setID:              Hash(idB),
			recoveryFiles:      make(map[Hash]FilePacket),
			nonRecoveryFiles:   make(map[Hash]FilePacket),
			strayFiles:         make(map[Hash]FilePacket),
			missingRecovery:    make(map[Hash]struct{}),
			missingNonRecovery: make(map[Hash]struct{}),
		},
	}
	order := []Hash{Hash(idB), Hash(idA)}

	result := buildMergedSets(order, mergedSets)
	require.Len(t, result, 2)

	require.Equal(t, Hash(idB), result[0].SetID)
	require.Equal(t, Hash(idA), result[1].SetID)
}

// Expectation: buildMergedSets should copy main packet.
func Test_buildMergedSets_CopiesMainPacket_Success(t *testing.T) {
	t.Parallel()

	mainPacket := &MainPacket{
		SetID:     Hash(sID),
		SliceSize: 4096,
	}

	mergedSets := map[Hash]*mergedSet{
		Hash(sID): {
			setID:              Hash(sID),
			mainPacket:         mainPacket,
			recoveryFiles:      make(map[Hash]FilePacket),
			nonRecoveryFiles:   make(map[Hash]FilePacket),
			strayFiles:         make(map[Hash]FilePacket),
			missingRecovery:    make(map[Hash]struct{}),
			missingNonRecovery: make(map[Hash]struct{}),
		},
	}
	order := []Hash{Hash(sID)}

	result := buildMergedSets(order, mergedSets)
	require.NotNil(t, result[0].MainPacket)
	require.Equal(t, result[0].MainPacket, mergedSets[sID].mainPacket)   // content
	require.NotSame(t, result[0].MainPacket, mergedSets[sID].mainPacket) // pointer
}

// Expectation: buildMergedSets should handle nil main packet.
func Test_buildMergedSets_NilMainPacket_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(sID): {
			setID:              Hash(sID),
			mainPacket:         nil,
			recoveryFiles:      make(map[Hash]FilePacket),
			nonRecoveryFiles:   make(map[Hash]FilePacket),
			strayFiles:         make(map[Hash]FilePacket),
			missingRecovery:    make(map[Hash]struct{}),
			missingNonRecovery: make(map[Hash]struct{}),
		},
	}
	order := []Hash{Hash(sID)}

	result := buildMergedSets(order, mergedSets)
	require.Nil(t, result[0].MainPacket)
}

// Expectation: buildMergedSets should convert recovery files to sorted slice.
func Test_buildMergedSets_SortsRecoveryFiles_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(sID): {
			setID: Hash(sID),
			recoveryFiles: map[Hash]FilePacket{
				Hash(idB): {FileID: Hash(idB), Name: "b.txt"},
				Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
			},
			nonRecoveryFiles:   make(map[Hash]FilePacket),
			strayFiles:         make(map[Hash]FilePacket),
			missingRecovery:    make(map[Hash]struct{}),
			missingNonRecovery: make(map[Hash]struct{}),
		},
	}
	order := []Hash{Hash(sID)}

	result := buildMergedSets(order, mergedSets)
	require.Len(t, result[0].RecoverySet, 2)

	require.Equal(t, "a.txt", result[0].RecoverySet[0].Name)
	require.Equal(t, "b.txt", result[0].RecoverySet[1].Name)
}

// Expectation: buildMergedSets should convert non-recovery files to sorted slice.
func Test_buildMergedSets_SortsNonRecoveryFiles_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(sID): {
			setID:         Hash(sID),
			recoveryFiles: make(map[Hash]FilePacket),
			nonRecoveryFiles: map[Hash]FilePacket{
				Hash(idB): {FileID: Hash(idB), Name: "b.txt"},
				Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
			},
			strayFiles:         make(map[Hash]FilePacket),
			missingRecovery:    make(map[Hash]struct{}),
			missingNonRecovery: make(map[Hash]struct{}),
		},
	}
	order := []Hash{Hash(sID)}

	result := buildMergedSets(order, mergedSets)
	require.Len(t, result[0].NonRecoverySet, 2)

	require.Equal(t, "a.txt", result[0].NonRecoverySet[0].Name)
	require.Equal(t, "b.txt", result[0].NonRecoverySet[1].Name)
}

// Expectation: buildMergedSets should convert stray files to sorted slice.
func Test_buildMergedSets_SortsStrayFiles_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(sID): {
			setID:            Hash(sID),
			recoveryFiles:    make(map[Hash]FilePacket),
			nonRecoveryFiles: make(map[Hash]FilePacket),
			strayFiles: map[Hash]FilePacket{
				Hash(idB): {FileID: Hash(idB), Name: "b.txt"},
				Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
			},
			missingRecovery:    make(map[Hash]struct{}),
			missingNonRecovery: make(map[Hash]struct{}),
		},
	}
	order := []Hash{Hash(sID)}

	result := buildMergedSets(order, mergedSets)
	require.Len(t, result[0].StrayPackets, 2)

	require.Equal(t, "a.txt", result[0].StrayPackets[0].Name)
	require.Equal(t, "b.txt", result[0].StrayPackets[1].Name)
}

// Expectation: buildMergedSets should convert missing recovery to sorted slice.
func Test_buildMergedSets_SortsMissingRecovery_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(sID): {
			setID:            Hash(sID),
			recoveryFiles:    make(map[Hash]FilePacket),
			nonRecoveryFiles: make(map[Hash]FilePacket),
			strayFiles:       make(map[Hash]FilePacket),
			missingRecovery: map[Hash]struct{}{
				Hash(idB): {},
				Hash(idA): {},
			},
			missingNonRecovery: make(map[Hash]struct{}),
		},
	}
	order := []Hash{Hash(sID)}

	result := buildMergedSets(order, mergedSets)
	require.Len(t, result[0].MissingRecoveryPackets, 2)

	require.Equal(t, Hash(idA), result[0].MissingRecoveryPackets[0])
	require.Equal(t, Hash(idB), result[0].MissingRecoveryPackets[1])
}

// Expectation: buildMergedSets should convert missing non-recovery to sorted slice.
func Test_buildMergedSets_SortsMissingNonRecovery_Success(t *testing.T) {
	t.Parallel()

	mergedSets := map[Hash]*mergedSet{
		Hash(sID): {
			setID:            Hash(sID),
			recoveryFiles:    make(map[Hash]FilePacket),
			nonRecoveryFiles: make(map[Hash]FilePacket),
			strayFiles:       make(map[Hash]FilePacket),
			missingRecovery:  make(map[Hash]struct{}),
			missingNonRecovery: map[Hash]struct{}{
				Hash(idB): {},
				Hash(idA): {},
			},
		},
	}
	order := []Hash{Hash(sID)}

	result := buildMergedSets(order, mergedSets)
	require.Len(t, result[0].MissingNonRecoveryPackets, 2)

	require.Equal(t, Hash(idA), result[0].MissingNonRecoveryPackets[0])
	require.Equal(t, Hash(idB), result[0].MissingNonRecoveryPackets[1])
}

// Expectation: resolveUnlisted should remove strays that are in recovery files.
func Test_resolveUnlisted_RemovesStraysInRecovery_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		nonRecoveryFiles: make(map[Hash]FilePacket),
		strayFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		missingRecovery:    make(map[Hash]struct{}),
		missingNonRecovery: make(map[Hash]struct{}),
	}

	ms.resolveUnlisted()

	require.Empty(t, ms.strayFiles)
	require.Len(t, ms.recoveryFiles, 1)
}

// Expectation: resolveUnlisted should remove strays that are in non-recovery files.
func Test_resolveUnlisted_RemovesStraysInNonRecovery_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles: make(map[Hash]FilePacket),
		nonRecoveryFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		strayFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		missingRecovery:    make(map[Hash]struct{}),
		missingNonRecovery: make(map[Hash]struct{}),
	}

	ms.resolveUnlisted()

	require.Empty(t, ms.strayFiles)
	require.Len(t, ms.nonRecoveryFiles, 1)
}

// Expectation: resolveUnlisted should remove missing recovery that is already found.
func Test_resolveUnlisted_RemovesMissingRecoveryAlreadyFound_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		nonRecoveryFiles: make(map[Hash]FilePacket),
		strayFiles:       make(map[Hash]FilePacket),
		missingRecovery: map[Hash]struct{}{
			Hash(idA): {},
		},
		missingNonRecovery: make(map[Hash]struct{}),
	}

	ms.resolveUnlisted()

	require.Empty(t, ms.missingRecovery)
}

// Expectation: resolveUnlisted should remove missing non-recovery that is already found.
func Test_resolveUnlisted_RemovesMissingNonRecoveryAlreadyFound_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles: make(map[Hash]FilePacket),
		nonRecoveryFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		strayFiles:      make(map[Hash]FilePacket),
		missingRecovery: make(map[Hash]struct{}),
		missingNonRecovery: map[Hash]struct{}{
			Hash(idA): {},
		},
	}

	ms.resolveUnlisted()

	require.Empty(t, ms.missingNonRecovery)
}

// Expectation: resolveUnlisted should move stray to recovery when missing recovery found.
func Test_resolveUnlisted_MovesStrayToRecoveryWhenMissing_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles:    make(map[Hash]FilePacket),
		nonRecoveryFiles: make(map[Hash]FilePacket),
		strayFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		missingRecovery: map[Hash]struct{}{
			Hash(idA): {},
		},
		missingNonRecovery: make(map[Hash]struct{}),
	}

	ms.resolveUnlisted()

	require.Empty(t, ms.strayFiles)
	require.Empty(t, ms.missingRecovery)
	require.Len(t, ms.recoveryFiles, 1)
	require.Equal(t, "a.txt", ms.recoveryFiles[Hash(idA)].Name)
}

// Expectation: resolveUnlisted should move stray to non-recovery when missing non-recovery found.
func Test_resolveUnlisted_MovesStrayToNonRecoveryWhenMissing_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles:    make(map[Hash]FilePacket),
		nonRecoveryFiles: make(map[Hash]FilePacket),
		strayFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		missingRecovery: make(map[Hash]struct{}),
		missingNonRecovery: map[Hash]struct{}{
			Hash(idA): {},
		},
	}

	ms.resolveUnlisted()

	require.Empty(t, ms.strayFiles)
	require.Empty(t, ms.missingNonRecovery)
	require.Len(t, ms.nonRecoveryFiles, 1)
	require.Equal(t, "a.txt", ms.nonRecoveryFiles[Hash(idA)].Name)
}

// Expectation: resolveUnlisted should handle complex scenario with multiple resolutions.
func Test_resolveUnlisted_ComplexScenario_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "existing.txt"},
		},
		nonRecoveryFiles: make(map[Hash]FilePacket),
		strayFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "duplicate.txt"}, // Should be removed (already in recovery)
			Hash(idB): {FileID: Hash(idB), Name: "missing.txt"},   // Should be moved to recovery
			Hash(idC): {FileID: Hash(idC), Name: "stray.txt"},     // Should stay stray
		},
		missingRecovery: map[Hash]struct{}{
			Hash(idA): {}, // Should be removed (already in recovery)
			Hash(idB): {}, // Should be resolved from stray
		},
		missingNonRecovery: make(map[Hash]struct{}),
	}

	ms.resolveUnlisted()

	require.Len(t, ms.recoveryFiles, 2) // idA and idB
	require.Len(t, ms.strayFiles, 1)    // Only idC remains
	require.Empty(t, ms.missingRecovery)
	require.Contains(t, ms.strayFiles, Hash(idC))
}

// Expectation: resolveUnlisted should handle empty mergedSet.
func Test_resolveUnlisted_EmptyMergedSet_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles:      make(map[Hash]FilePacket),
		nonRecoveryFiles:   make(map[Hash]FilePacket),
		strayFiles:         make(map[Hash]FilePacket),
		missingRecovery:    make(map[Hash]struct{}),
		missingNonRecovery: make(map[Hash]struct{}),
	}

	ms.resolveUnlisted()

	require.Empty(t, ms.recoveryFiles)
	require.Empty(t, ms.nonRecoveryFiles)
	require.Empty(t, ms.strayFiles)
	require.Empty(t, ms.missingRecovery)
	require.Empty(t, ms.missingNonRecovery)
}

// Expectation: resolveUnlisted should prioritize recovery over non-recovery for strays.
func Test_resolveUnlisted_PrioritizesRecoveryOverNonRecovery_Success(t *testing.T) {
	t.Parallel()

	ms := &mergedSet{
		recoveryFiles:    make(map[Hash]FilePacket),
		nonRecoveryFiles: make(map[Hash]FilePacket),
		strayFiles: map[Hash]FilePacket{
			Hash(idA): {FileID: Hash(idA), Name: "a.txt"},
		},
		missingRecovery: map[Hash]struct{}{
			Hash(idA): {},
		},
		missingNonRecovery: map[Hash]struct{}{
			Hash(idA): {}, // Same ID in both missing lists
		},
	}

	ms.resolveUnlisted()

	// Should be moved to recovery (processed first)
	require.Len(t, ms.recoveryFiles, 1)
	require.Empty(t, ms.nonRecoveryFiles)
	require.Empty(t, ms.strayFiles)
	require.Empty(t, ms.missingRecovery)
	// missingNonRecovery for idA should remain since it was resolved as recovery
	require.Len(t, ms.missingNonRecovery, 1)
}
