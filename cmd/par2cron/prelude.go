package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/schema"
	"github.com/spf13/afero"
	"github.com/spf13/pflag"
)

type configMergeable[A any] interface {
	*configFileCreate | *configFileVerify | *configFileRepair | *configFileInfo
	Merge(opts A, logs *logging.Options, hasExternalArgs bool, setFlags map[string]bool)
}

type preludeInput[A any, C configMergeable[A]] struct {
	FSys           afero.Fs
	Args           []string
	DashAt         int
	ConfigPath     string
	CommandOptions A
	LogSettings    *logging.Options
	ExtractSection func(cfg *configFile) C
	VisitFlags     func(fn func(*pflag.Flag))
}

type preludeResult struct {
	ResolvedPaths []string
}

func runPrelude[A any, C configMergeable[A]](in *preludeInput[A, C]) (*preludeResult, error) {
	hasExternalArgs := (in.DashAt != -1)

	if hasExternalArgs && in.DashAt < 1 {
		return nil, errors.New("need at least one <dir> path before --")
	}

	var externalArgs []string
	pathArgs := slices.Clone(in.Args)

	if hasExternalArgs {
		pathArgs = append([]string{}, in.Args[:in.DashAt]...)
		externalArgs = append([]string{}, in.Args[in.DashAt:]...)
	}

	if hasExternalArgs && len(externalArgs) > 0 {
		if !strings.HasPrefix(externalArgs[0], "-") {
			return nil, errors.New("first argument after -- does not " +
				"start with -, provide valid par2cmdline arguments after --")
		}
	}

	if in.ConfigPath != "" {
		cfg, err := parseConfigFile(in.FSys, in.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse --config file: %w", err)
		}

		section := in.ExtractSection(cfg)
		if section != nil {
			setFlags := make(map[string]bool)
			in.VisitFlags(func(f *pflag.Flag) {
				setFlags[f.Name] = true
			})
			section.Merge(in.CommandOptions, in.LogSettings, hasExternalArgs, setFlags)
		}
	}

	if hasExternalArgs {
		if setter, ok := any(in.CommandOptions).(schema.OptionsPar2ArgsSettable); ok {
			setter.SetPar2Args(externalArgs)
		}
	}

	resolved, err := resolvePathArgs(in.FSys, pathArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve paths: %w", err)
	}

	if validator, ok := any(in.CommandOptions).(schema.OptionsValidatable); ok {
		if err := validator.Validate(); err != nil {
			if errors.Is(err, schema.ErrUnsupportedGlob) {
				return nil, fmt.Errorf("%w: cannot use deep globs (/, **) in recursive mode, "+
					"use non-recursive modes with deep globs instead (see documentation)", err)
			}

			return nil, fmt.Errorf("failed to validate options: %w", err)
		}
	}

	return &preludeResult{ResolvedPaths: resolved}, nil
}

func resolvePathArgs(fsys afero.Fs, pathArgs []string) ([]string, error) {
	resolved := make([]string, len(pathArgs))

	for i, p := range pathArgs {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("failed to convert path to absolute: %w", err)
		}

		resolved[i] = abs
		if fi, err := fsys.Stat(abs); err != nil {
			return nil, fmt.Errorf("failed to access root directory: %w", err)
		} else if !fi.IsDir() {
			return nil, fmt.Errorf("root directory is not a directory: %s", abs)
		}
	}

	return resolved, nil
}
