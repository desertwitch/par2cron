package create

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"

	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type MarkerConfig struct {
	Par2Name   *string           `yaml:"name"`
	Par2Args   *[]string         `yaml:"args"`
	Par2Glob   *string           `yaml:"glob"`
	Par2Mode   *flags.CreateMode `yaml:"mode"`
	Par2Verify *bool             `yaml:"verify"`
	HideFiles  *bool             `yaml:"hidden"`
}

func NewMarkerConfig(markerPath string, opts Options) *MarkerConfig {
	cfg := &MarkerConfig{}

	par2Name := filepath.Base(filepath.Dir(markerPath)) + schema.Par2Extension
	par2Args := slices.Clone(opts.Par2Args)
	par2Glob := opts.Par2Glob
	par2Mode := opts.Par2Mode
	par2Verify := opts.Par2Verify
	hideFiles := opts.HideFiles

	cfg.Par2Name = &par2Name
	cfg.Par2Args = &par2Args
	cfg.Par2Glob = &par2Glob
	cfg.Par2Mode = &par2Mode
	cfg.Par2Verify = &par2Verify
	cfg.HideFiles = &hideFiles

	return cfg
}

func (prog *Service) parseMarkerFile(markerPath string, opts Options) (*MarkerConfig, error) {
	logger := prog.markerLogger(markerPath, nil, nil)
	logger.Debug("Found marker file")

	cfg := NewMarkerConfig(markerPath, opts)

	prog.parseMarkerFilename(markerPath, cfg)

	if err := prog.parseMarkerContent(markerPath, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse marker file: %w", err)
	}

	prog.considerRecursiveMarker(markerPath, cfg)

	return cfg, nil
}

func (prog *Service) parseMarkerFilename(markerPath string, cfg *MarkerConfig) {
	_, suffix, _ := strings.Cut(filepath.Base(markerPath), createMarkerPathPrefix)

	seen := make(map[byte]bool)
	for t := range strings.SplitSeq(suffix, createMarkerPathSeparator) {
		if len(t) < 1 {
			continue
		}

		flag := t[0]
		if seen[flag] {
			continue
		}

		arg := "-" + string(flag)
		value := t[1:]

		logger := prog.markerLogger(markerPath, string(flag), value)
		logger.Debug("Parsed setting from marker filename")

		prog.modifyOrAddArgument(cfg, arg, value, markerPath)
		seen[flag] = true
	}
}

func (prog *Service) parseMarkerContent(markerPath string, cfg *MarkerConfig) error {
	data, err := afero.ReadFile(prog.fsys, markerPath)
	if err != nil {
		return fmt.Errorf("failed to read marker file: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	yamlConfig := &MarkerConfig{}
	if err := decoder.Decode(&yamlConfig); err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("failed to decode marker file: %w", err)
	}

	if yamlConfig.Par2Name != nil {
		name := *yamlConfig.Par2Name
		if !strings.HasSuffix(strings.ToLower(name), schema.Par2Extension) {
			name += schema.Par2Extension
		}

		logger := prog.markerLogger(markerPath, "name", name)
		logger.Debug("Parsed setting from marker file contents")

		cfg.Par2Name = &name
	}

	if yamlConfig.Par2Args != nil {
		logger := prog.markerLogger(markerPath, "args", *yamlConfig.Par2Args)
		logger.Debug("Parsed setting from marker file contents")

		cfg.Par2Args = yamlConfig.Par2Args
	}

	if yamlConfig.Par2Glob != nil {
		logger := prog.markerLogger(markerPath, "files", *yamlConfig.Par2Glob)
		logger.Debug("Parsed setting from marker file contents")

		cfg.Par2Glob = yamlConfig.Par2Glob
	}

	if yamlConfig.Par2Mode != nil {
		logger := prog.markerLogger(markerPath, "mode", yamlConfig.Par2Mode.Value)
		logger.Debug("Parsed setting from marker file contents")

		cfg.Par2Mode = yamlConfig.Par2Mode
	}

	if yamlConfig.Par2Verify != nil {
		logger := prog.markerLogger(markerPath, "verify", *yamlConfig.Par2Verify)
		logger.Debug("Parsed setting from marker file contents")

		cfg.Par2Verify = yamlConfig.Par2Verify
	}

	if yamlConfig.HideFiles != nil {
		logger := prog.markerLogger(markerPath, "hidden", *yamlConfig.HideFiles)
		logger.Debug("Parsed setting from marker file contents")

		cfg.HideFiles = yamlConfig.HideFiles
	}

	return nil
}

func (prog *Service) considerRecursiveMarker(markerPath string, cfg *MarkerConfig) {
	if cfg.Par2Mode.Value != schema.CreateRecursiveMode && slices.Contains(*cfg.Par2Args, "-R") {
		logger := prog.markerLogger(markerPath, nil, nil)

		before := cfg.Par2Mode.Value
		_ = cfg.Par2Mode.Set(schema.CreateRecursiveMode)

		logger.Info(
			"Assuming recursive mode due to set par2 argument -R (recursive)",
			"mode-before", before,
			"mode-after", cfg.Par2Mode.Value,
			"args", *cfg.Par2Args,
		)
	}

	if cfg.Par2Mode.Value == schema.CreateRecursiveMode && !slices.Contains(*cfg.Par2Args, "-R") {
		logger := prog.markerLogger(markerPath, nil, nil)

		before := slices.Clone(*cfg.Par2Args)
		*cfg.Par2Args = append(*cfg.Par2Args, "-R")

		logger.Debug(
			"Adding -R to par2 argument slice (due to set recursive mode)",
			"mode", cfg.Par2Mode.Value,
			"args-before", before,
			"args-after", *cfg.Par2Args,
		)
	}
}

func (prog *Service) modifyOrAddArgument(cfg *MarkerConfig, arg string, value string, markerPath string) {
	for i := range *cfg.Par2Args {
		a := strings.TrimSpace((*cfg.Par2Args)[i])

		// "-r val"
		if strings.HasPrefix(a, arg+" ") {
			before := (*cfg.Par2Args)[i]
			(*cfg.Par2Args)[i] = arg + " " + value
			prog.debugArgsModified(arg, value, before, (*cfg.Par2Args)[i], true, markerPath)

			return
		}

		// "-r=val"
		if strings.HasPrefix(a, arg+"=") {
			before := (*cfg.Par2Args)[i]
			(*cfg.Par2Args)[i] = arg + "=" + value
			prog.debugArgsModified(arg, value, before, (*cfg.Par2Args)[i], true, markerPath)

			return
		}

		// "-rval"
		if strings.HasPrefix(a, arg) && len(a) > len(arg) &&
			strings.ContainsAny(string(a[len(arg)]), "0123456789gmk") {
			before := (*cfg.Par2Args)[i]
			(*cfg.Par2Args)[i] = arg + value
			prog.debugArgsModified(arg, value, before, (*cfg.Par2Args)[i], true, markerPath)

			return
		}

		// "-r" followed by val in next element
		if a == arg && i+1 < len(*cfg.Par2Args) &&
			!strings.HasPrefix(strings.TrimSpace((*cfg.Par2Args)[i+1]), "-") {
			before := (*cfg.Par2Args)[i : i+2]
			(*cfg.Par2Args)[i+1] = value
			prog.debugArgsModified(arg, value, before, (*cfg.Par2Args)[i:i+2], true, markerPath)

			return
		}

		// "-r" not followed by val in next element
		if a == arg {
			before := (*cfg.Par2Args)[i]
			(*cfg.Par2Args)[i] = arg + value
			prog.debugArgsModified(arg, value, before, (*cfg.Par2Args)[i], true, markerPath)

			return
		}
	}

	before := slices.Clone(*cfg.Par2Args)
	*cfg.Par2Args = append(*cfg.Par2Args, arg+value)
	prog.debugArgsModified(arg, value, before, *cfg.Par2Args, false, markerPath)
}
