package par2

// HashSize is the size of the PAR2-contained hashes.
// These are MD5 hashes used for various purposes within packets.
const HashSize = 16

// Hash is the type used for all PAR2-contained hashes.
// According to the specification, it is a [16]byte array.
type Hash [HashSize]byte

// FileSet represents a set of multiple PAR2 [File]
// It includes a merged [Set] slice of their combined information.
type FileSet struct {
	Files      []File `json:"files"`       // Unmerged PAR2 files and their sets
	SetsMerged []Set  `json:"sets_merged"` // Merged information of all PAR2 files
}

// File represents a single parsed PAR2 file.
type File struct {
	Name string `json:"name"` // Name of the PAR2 file

	// Sets represents the datasets of different set IDs,
	// though commonly there is only one dataset in a PAR2 file.
	Sets []Set `json:"sets"`
}

// Set represents a dataset with unique set ID.
type Set struct {
	SetID          Hash         `json:"set_id"`                // Dataset ID
	MainPacket     *MainPacket  `json:"main_packet,omitempty"` // Main packet (can be nil)
	RecoverySet    []FilePacket `json:"recovery_set"`          // Protected (recovery) files
	NonRecoverySet []FilePacket `json:"non_recovery_set"`      // Auxiliary (non-recovery) files

	// StrayPackets are packets that have the right set ID,
	// but whose file ID is not listed within the PAR2 [MainPacket].
	StrayPackets []FilePacket `json:"stray_packets"`

	// MissingRecoveryPackets are recovery files that have their
	// file ID in the PAR2 [MainPacket], but have not been found.
	MissingRecoveryPackets []Hash `json:"missing_recovery_packets"`

	// MissingNonRecoveryPackets are non-recovery files that have
	// their file ID in the PAR2 [MainPacket], but have not been found.
	MissingNonRecoveryPackets []Hash `json:"missing_non_recovery_packets"`
}

// MainPacket represents a PAR2 main packet.
type MainPacket struct {
	SetID          Hash   `json:"set_id"`           // [Set] the packet belongs to
	SliceSize      uint64 `json:"slice_size"`       // Recovery slice size
	RecoveryIDs    []Hash `json:"recovery_ids"`     // Protected (recovery) IDs
	NonRecoveryIDs []Hash `json:"non_recovery_ids"` // Auxiliary (non-recovery) IDs
}

// FilePacket represents a PAR2 file description packet.
type FilePacket struct {
	SetID       Hash   `json:"set_id"`       // [Set] the packet belongs to
	FileID      Hash   `json:"file_id"`      // ID of the file (MD5)
	Name        string `json:"name"`         // Filename of the file
	Size        int64  `json:"size"`         // Size of the file
	Hash        Hash   `json:"hash"`         // MD5 hash of entire file
	Hash16k     Hash   `json:"hash_16k"`     // MD5 hash of first 16KB
	FromUnicode bool   `json:"from_unicode"` // Name came from a Unicode packet
}

// UnicodePacket represents a PAR2 unicode file description packet.
type UnicodePacket struct {
	SetID  Hash   `json:"set_id"`  // [Set] the packet belongs to
	FileID Hash   `json:"file_id"` // ID of the file (MD5)
	Name   string `json:"name"`    // Unicode name of the file
}
