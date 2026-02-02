package schema

// ProgramVersion is the program version as filled in by the Makefile.
var ProgramVersion = "devel"

const (
	ExitCodeSuccess        int = 0
	ExitCodePartialFailure int = 1 // ErrExitPartialFailure
	ExitCodeBadInvocation  int = 2 // ErrExitBadInvocation
	ExitCodeRepairable     int = 3 // ErrExitRepairable
	ExitCodeUnrepairable   int = 4 // ErrExitUnrepairable
	ExitCodeUnclassified   int = 5 // ErrExitUnclassified

	// https://github.com/Parchive/par2cmdline/blob/master/src/libpar2.h

	Par2ExitCodeSuccess          int = 0
	Par2ExitCodeRepairPossible   int = 1
	Par2ExitCodeRepairImpossible int = 2

	Par2Extension     string = ".par2" // used as par2Extension
	LockExtension     string = ".lock" // used as par2Extension+lockExtension
	ManifestExtension string = ".json" // used as par2Extension+manifestExtension

	IgnoreFile    string = ".par2cron-ignore"
	IgnoreAllFile string = ".par2cron-ignore-all"

	CreateFileMode      string = "file"
	CreateFolderMode    string = "folder"
	CreateRecursiveMode string = "recursive"
)

type ctxKey int

const (
	PosKey  ctxKey = iota
	MposKey ctxKey = iota
	PrioKey ctxKey = iota
)
