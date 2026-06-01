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
	if err == nil {
		return false
	}

	for {
		// If it's a joined error, every child must OnlyContain the sentinel.
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			errs := joined.Unwrap()

			if len(errs) == 0 {
				return false
			}
			for _, e := range errs {
				if !OnlyContains(e, sentinel) {
					return false
				}
			}

			return true
		}

		// If it's a single-wrap error, peel one layer and keep going.
		if single, ok := err.(interface{ Unwrap() error }); ok {
			inner := single.Unwrap()

			if inner == nil {
				break
			}
			err = inner

			continue
		}

		// Leaf node - does it match?
		break
	}

	return errors.Is(err, sentinel)
}
