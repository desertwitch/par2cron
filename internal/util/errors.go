package util

import (
	"errors"
	"os/exec"

	"github.com/desertwitch/par2cron/internal/schema"
)

func HighestError(errs []error) error {
	var highest error
	highestPriority := -1

	for _, e := range errs {
		if e == nil {
			continue
		}

		priority := schema.ExitCodeFor(e)
		if priority > highestPriority {
			highestPriority = priority
			highest = e
		}
	}

	return highest
}

func AsExitCode(err error) *int {
	var exitCode int

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		exitCode = exitErr.ExitCode()

		return &exitCode
	}

	return nil
}

func OnlyContains(err, sentinel error) bool {
	joined, ok := err.(interface{ Unwrap() []error })
	if !ok {
		return errors.Is(err, sentinel)
	}

	for _, e := range joined.Unwrap() {
		if !errors.Is(e, sentinel) {
			return false
		}
	}

	return true
}
