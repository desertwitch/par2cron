package logging

import (
	"io"
	"log/slog"
	"os"
	"time"

	"github.com/desertwitch/par2cron/internal/flags"
	slogseq "github.com/desertwitch/slog-seq"
	"github.com/lmittmann/tint"
)

type Options struct {
	LogLevel flags.LogLevel

	Logout io.Writer
	Stdout io.Writer
	Stderr io.Writer

	SeqURL string
	SeqKey string

	WantJSON bool
}

type Logger struct {
	*slog.Logger

	Options    Options
	seqHandler *slogseq.SeqHandler
}

func NewLogger(opts Options) *Logger {
	var consoleHandler slog.Handler
	if opts.WantJSON {
		consoleHandler = slog.NewJSONHandler(opts.Logout, &slog.HandlerOptions{
			Level: opts.LogLevel.Value,
		})
	} else {
		consoleHandler = tint.NewHandler(opts.Logout, &tint.Options{
			Level:      opts.LogLevel.Value,
			TimeFormat: time.TimeOnly,
		})
	}

	var logger *slog.Logger
	var seqHandler *slogseq.SeqHandler
	if opts.SeqURL != "" {
		debugLogger := slog.New(consoleHandler)

		attrs := []slog.Attr{
			slog.String("app", "par2cron"),
		}
		if hostname, err := os.Hostname(); err == nil {
			attrs = append(attrs, slog.String("hostname", hostname))
		}

		seqOpts := []slogseq.SeqOption{
			slogseq.WithGlobalAttrs(attrs...),
			slogseq.WithBatchSize(10), //nolint:mnd
			slogseq.WithInsecure(),
			slogseq.WithErrorHandlerFunc(func(err error) {
				debugLogger.Debug("Failed to log to Seq", "error", err)
			}),
		}
		if opts.SeqKey != "" {
			seqOpts = append(seqOpts, slogseq.WithAPIKey(opts.SeqKey))
		}

		seqHandler = slogseq.NewSeqHandler(opts.SeqURL, seqOpts...)
		logger = slog.New(&fanoutHandler{
			handlers: []slog.Handler{consoleHandler, seqHandler},
		})
	} else {
		logger = slog.New(consoleHandler)
	}

	return &Logger{
		Logger:     logger,
		Options:    opts,
		seqHandler: seqHandler,
	}
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger:  l.Logger.With(args...),
		Options: l.Options,
	}
}

func (l *Logger) Close() {
	if l.seqHandler != nil {
		_ = l.seqHandler.Close()
	}
}
