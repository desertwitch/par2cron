package tool

import (
	"context"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
)

func (prog *Service) toolLogger(ctx context.Context, path any) *logging.Logger {
	logElems := []any{}

	if ctx.Value(schema.ModeKey) != nil {
		logElems = append(logElems, "mode", ctx.Value(schema.ModeKey))
	}

	if path != nil {
		logElems = append(logElems, "path", path)
	}

	return prog.log.With(logElems...)
}
