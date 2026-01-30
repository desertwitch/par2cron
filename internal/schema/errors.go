package schema

import "errors"

var (
	ErrExitPartialFailure = errors.New("partial failure")                       // [ExitCodePartialFailure]
	ErrExitBadInvocation  = errors.New("bad invocation of the program")         // [ExitCodeBadInvocation]
	ErrExitRepairable     = errors.New("files are corrupted, but repairable")   // [ExitCodeRepairable]
	ErrExitUnrepairable   = errors.New("files are corrupted, but unrepairable") // [ExitCodeUnrepairable]
	ErrExitUnclassified   = errors.New("unclassified error")                    // [ExitCodeUnclassified]

	ErrAlreadyExists    = errors.New("file exists")
	ErrFileIsLocked     = errors.New("file is locked")
	ErrNonFatal         = errors.New("non-fatal error")
	ErrSilentSkip       = errors.New("skip without error")
	ErrManifestMismatch = errors.New("manifest mismatch")
)

var exitErrorsByPriority = []struct {
	err  error
	code int
}{
	{ErrExitUnclassified, ExitCodeUnclassified},     // 5
	{ErrExitUnrepairable, ExitCodeUnrepairable},     // 4
	{ErrExitRepairable, ExitCodeRepairable},         // 3
	{ErrExitBadInvocation, ExitCodeBadInvocation},   // 2
	{ErrExitPartialFailure, ExitCodePartialFailure}, // 1
}

func ExitCodeFor(err error) int {
	if err == nil {
		return ExitCodeSuccess
	}

	for _, entry := range exitErrorsByPriority {
		if errors.Is(err, entry.err) {
			return entry.code
		}
	}

	return ExitCodeUnclassified
}
