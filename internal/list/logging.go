package list

import (
	"context"

	"github.com/desertwitch/par2cron/internal/logging"
)

func (prog *Service) listLogger(_ context.Context, path any) *logging.Logger {
	logElems := []any{}

	if path != nil {
		logElems = append(logElems, "path", path)
	}

	return prog.log.With(logElems...)
}
