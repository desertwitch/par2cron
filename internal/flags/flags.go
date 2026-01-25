package flags

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/pflag"
	"github.com/xhit/go-str2duration/v2"
	"gopkg.in/yaml.v3"
)

var (
	_ pflag.Value = (*Duration)(nil)
	_ pflag.Value = (*LogLevel)(nil)
	_ pflag.Value = (*CreateMode)(nil)

	_ yaml.Unmarshaler = (*Duration)(nil)
	_ yaml.Unmarshaler = (*LogLevel)(nil)
	_ yaml.Unmarshaler = (*CreateMode)(nil)

	errInvalidValue = errors.New("invalid value")
)

type Duration struct {
	Raw   string
	Value time.Duration
}

func (f *Duration) String() string {
	return f.Raw
}

func (f *Duration) Set(s string) error {
	s = strings.ToLower(strings.TrimSpace(s))

	if s == "" {
		f.Value = 0
	} else {
		conv, err := str2duration.ParseDuration(s)
		if err != nil {
			return fmt.Errorf("failed to str2duration: %w", err)
		}
		f.Value = conv
	}

	f.Raw = s

	return nil
}

func (f *Duration) Type() string {
	return "duration"
}

func (f Duration) MarshalJSON() ([]byte, error) {
	by, err := json.Marshal(f.Raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal: %w", err)
	}

	return by, nil
}

func (f *Duration) UnmarshalJSON(data []byte) error {
	var s string

	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("failed to unmarshal: %w", err)
	}

	return f.Set(s)
}

func (f *Duration) UnmarshalYAML(node *yaml.Node) error {
	return f.Set(node.Value)
}

type LogLevel struct {
	Raw   string
	Value slog.Level
}

func (f *LogLevel) String() string {
	return f.Raw
}

func (f *LogLevel) Set(s string) error {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case "debug":
		f.Value = slog.LevelDebug
	case "info":
		f.Value = slog.LevelInfo
	case "warn", "warning":
		f.Value = slog.LevelWarn
	case "error":
		f.Value = slog.LevelError
	default:
		return fmt.Errorf("%w: %q is not recognized", errInvalidValue, s)
	}

	f.Raw = s

	return nil
}

func (f *LogLevel) Type() string {
	return "level"
}

func (f *LogLevel) UnmarshalYAML(node *yaml.Node) error {
	return f.Set(node.Value)
}

type CreateMode struct {
	Raw   string
	Value string
}

func (f *CreateMode) String() string {
	return f.Raw
}

func (f *CreateMode) Set(s string) error {
	s = strings.ToLower(strings.TrimSpace(s))

	switch s {
	case schema.CreateFileMode:
		f.Value = schema.CreateFileMode
	case schema.CreateFolderMode:
		f.Value = schema.CreateFolderMode
	default:
		return fmt.Errorf("%w: %q is not recognized", errInvalidValue, s)
	}

	f.Raw = s

	return nil
}

func (f *CreateMode) Type() string {
	return "mode"
}

func (f *CreateMode) UnmarshalYAML(node *yaml.Node) error {
	return f.Set(node.Value)
}
