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
		return nil, fmt.Errorf("%w: need at least one <dir> path before --",
			schema.ErrExitBadInvocation)
	}

	var externalArgs []string
	pathArgs := slices.Clone(in.Args)

	if hasExternalArgs {
		pathArgs = append([]string{}, in.Args[:in.DashAt]...)
		externalArgs = append([]string{}, in.Args[in.DashAt:]...)
	}

	if hasExternalArgs && len(externalArgs) > 0 {
		if !strings.HasPrefix(externalArgs[0], "-") {
			return nil, fmt.Errorf("%w: first argument after -- does not "+
				"start with -, provide valid par2cmdline arguments after --",
				schema.ErrExitBadInvocation)
		}
	}

	if in.ConfigPath != "" {
		cfg, err := parseConfigFile(in.FSys, in.ConfigPath)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to parse --config file: %w",
				schema.ErrExitBadInvocation, err)
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

	resolved := make([]string, len(pathArgs))
	for i, p := range pathArgs {
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, fmt.Errorf("%w: failed to convert relative path to absolute: %w",
				schema.ErrExitBadInvocation, err)
		}

		resolved[i] = abs
		if _, err := in.FSys.Stat(abs); err != nil {
			return nil, fmt.Errorf("%w: failed to access root directory: %w",
				schema.ErrExitBadInvocation, err)
		}
	}

	if validator, ok := any(in.CommandOptions).(schema.OptionsValidatable); ok {
		if err := validator.Validate(); err != nil {
			if errors.Is(err, schema.ErrUnsupportedGlob) {
				return nil, fmt.Errorf("%w: %w: cannot use deep globs (/, **) in recursive mode, "+
					"use non-recursive modes with deep globs instead (see documentation)",
					schema.ErrExitBadInvocation, err)
			}

			return nil, fmt.Errorf("%w: %w", schema.ErrExitBadInvocation, err)
		}
	}

	return &preludeResult{ResolvedPaths: resolved}, nil
}
