package logging

import (
	"io"
	"log/slog"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/lmittmann/tint"
)

type Options struct {
	LogLevel flags.LogLevel

	Logout io.Writer
	Stdout io.Writer
	Stderr io.Writer

	WantJSON bool
}

type Logger struct {
	*slog.Logger

	Options Options
}

func NewLogger(opts Options) *Logger {
	var logger *slog.Logger

	if opts.WantJSON {
		logger = slog.New(slog.NewJSONHandler(opts.Logout, &slog.HandlerOptions{
			Level: opts.LogLevel.Value,
		}))
	} else {
		logger = slog.New(tint.NewHandler(opts.Logout,
			&tint.Options{
				Level:      opts.LogLevel.Value,
				TimeFormat: time.TimeOnly,
			}))
	}

	return &Logger{
		Logger:  logger,
		Options: opts,
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger:  l.Logger.With(args...),
		Options: l.Options,
	}
}
