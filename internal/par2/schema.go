package par2

const HashSize = 16 // Size of the MD5 hashes (File ID, Set ID, ...)

type Hash [HashSize]byte

// File represents a single parsed PAR2 file.
type File struct {
	// Sets represents the datasets of different set IDs,
	// though commonly there is only one dataset in a PAR2 file.
	Sets []Set `json:"sets,omitempty"`
}

// Set represents a dataset with unique set ID.
type Set struct {
	SetID          Hash         `json:"set_id"`                     // Dataset ID
	MainPacket     *MainPacket  `json:"main_packet,omitempty"`      // Main packet (can be nil)
	RecoverySet    []FilePacket `json:"recovery_set,omitempty"`     // Protected (recovery) files
	NonRecoverySet []FilePacket `json:"non_recovery_set,omitempty"` // Auxiliary (non-recovery) files

	// StrayPackets are packets that have the right set ID,
	// but whose file ID is not listed within the PAR2 [MainPacket].
	StrayPackets []FilePacket `json:"stray_packets,omitempty"`

	// MissingRecoveryPackets are recovery files that have their
	// file ID in the PAR2 [MainPacket], but have not been found.
	MissingRecoveryPackets []Hash `json:"missing_recovery_packets,omitempty"`

	// MissingNonRecoveryPackets are non-recovery files that have
	// their file ID in the PAR2 [MainPacket], but have not been found.
	MissingNonRecoveryPackets []Hash `json:"missing_non_recovery_packets,omitempty"`
}

// MainPacket represents a main packet.
type MainPacket struct {
	SetID          Hash   `json:"set_id"`                     // [Set] the packet belongs to
	SliceSize      uint64 `json:"slice_size"`                 // Recovery slice size
	RecoveryIDs    []Hash `json:"recovery_ids,omitempty"`     // Protected (recovery) IDs
	NonRecoveryIDs []Hash `json:"non_recovery_ids,omitempty"` // Auxiliary (non-recovery) IDs
}

// FilePacket represents a file description packet.
type FilePacket struct {
	SetID       Hash   `json:"set_id"`       // [Set] the packet belongs to
	FileID      Hash   `json:"file_id"`      // ID of the file (MD5)
	Name        string `json:"name"`         // Filename of the file
	Size        int64  `json:"size"`         // Size of the file
	Hash        Hash   `json:"hash"`         // MD5 hash of entire file
	Hash16k     Hash   `json:"hash_16k"`     // MD5 hash of first 16KB
	FromUnicode bool   `json:"from_unicode"` // Name came from a Unicode packet
}

// UnicodePacket represents a unicode file description packet.
type UnicodePacket struct {
	SetID  Hash   `json:"set_id"`  // [Set] the packet belongs to
	FileID Hash   `json:"file_id"` // ID of the file (MD5)
	Name   string `json:"name"`    // Unicode name of the file
}
