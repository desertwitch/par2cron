package schema

// ProgramVersion is the program version as filled in by the Makefile.
var ProgramVersion = "devel"

// Par2Version is the program version of "par2" as filled in at runtime.
var Par2Version = ""

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

	BundleExtension   string = ".p2c"  // used as bundleExtension+par2Extension
	Par2VolPrefix     string = ".vol"  // used as Par2VolPrefix+par2Extension
	Par2Extension     string = ".par2" // used as par2Extension
	LockExtension     string = ".lock" // used as par2Extension+lockExtension
	ManifestExtension string = ".json" // used as par2Extension+manifestExtension

	IgnoreFile    string = ".par2cron-ignore"
	IgnoreAllFile string = ".par2cron-ignore-all"

	CreateFolderMode    string = "folder"
	CreateNestedMode    string = "nested"
	CreateFileMode      string = "file"
	CreateRecursiveMode string = "recursive"
)

type ctxKey int

const (
	PosKey    ctxKey = iota
	MposKey   ctxKey = iota
	PrioKey   ctxKey = iota
	OpModeKey ctxKey = iota
)
