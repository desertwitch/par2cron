package main

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/desertwitch/par2cron/internal/create"
	"github.com/desertwitch/par2cron/internal/flags"
	"github.com/desertwitch/par2cron/internal/info"
	"github.com/desertwitch/par2cron/internal/logging"
	"github.com/desertwitch/par2cron/internal/repair"
	"github.com/desertwitch/par2cron/internal/verify"
	"github.com/spf13/afero"
	"gopkg.in/yaml.v3"
)

type configFile struct {
	Create *configFileCreate `yaml:"create"`
	Verify *configFileVerify `yaml:"verify"`
	Repair *configFileRepair `yaml:"repair"`
	Info   *configFileInfo   `yaml:"info"`
}

func parseConfigFile(fsys afero.Fs, path string) (*configFile, error) {
	data, err := afero.ReadFile(fsys, path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	yamlConfig := &configFile{}
	if err := decoder.Decode(&yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to decode yaml: %w", err)
	}

	return yamlConfig, nil
}

type configFileCreate struct {
	Par2Args *[]string `yaml:"args"`

	Par2Glob    *string           `yaml:"glob"`
	Par2Verify  *bool             `yaml:"verify"`
	Par2Mode    *flags.CreateMode `yaml:"mode"`
	MaxDuration *flags.Duration   `yaml:"duration"`
	HideFiles   *bool             `yaml:"hidden"`

	LogLevel *flags.LogLevel `yaml:"log-level"`
	WantJSON *bool           `yaml:"json"`
}

func (yamlCfg *configFileCreate) Merge(cfg *create.Options, logs *logging.Options, hasExternalArgs bool, setFlags map[string]bool) {
	if yamlCfg.Par2Args != nil && !hasExternalArgs {
		cfg.Par2Args = slices.Clone(*yamlCfg.Par2Args)
	}
	if yamlCfg.Par2Glob != nil && !setFlags["glob"] {
		cfg.Par2Glob = *yamlCfg.Par2Glob
	}
	if yamlCfg.Par2Verify != nil && !setFlags["verify"] {
		cfg.Par2Verify = *yamlCfg.Par2Verify
	}
	if yamlCfg.Par2Mode != nil && !setFlags["mode"] {
		cfg.Par2Mode = *yamlCfg.Par2Mode
	}
	if yamlCfg.MaxDuration != nil && !setFlags["duration"] {
		cfg.MaxDuration = *yamlCfg.MaxDuration
	}
	if yamlCfg.HideFiles != nil && !setFlags["hidden"] {
		cfg.HideFiles = *yamlCfg.HideFiles
	}
	if yamlCfg.LogLevel != nil && !setFlags["log-level"] {
		logs.LogLevel = *yamlCfg.LogLevel
	}
	if yamlCfg.WantJSON != nil && !setFlags["json"] {
		logs.WantJSON = *yamlCfg.WantJSON
	}
}

type configFileVerify struct {
	Par2Args *[]string `yaml:"args"`

	MaxDuration     *flags.Duration `yaml:"duration"`
	MinAge          *flags.Duration `yaml:"age"`
	RunInterval     *flags.Duration `yaml:"calc-run-interval"`
	IncludeExternal *bool           `yaml:"include-external"`
	SkipNotCreated  *bool           `yaml:"skip-not-created"`

	LogLevel *flags.LogLevel `yaml:"log-level"`
	WantJSON *bool           `yaml:"json"`
}

func (yamlCfg *configFileVerify) Merge(cfg *verify.Options, logs *logging.Options, hasExternalArgs bool, setFlags map[string]bool) {
	if yamlCfg.Par2Args != nil && !hasExternalArgs {
		cfg.Par2Args = slices.Clone(*yamlCfg.Par2Args)
	}
	if yamlCfg.MaxDuration != nil && !setFlags["duration"] {
		cfg.MaxDuration = *yamlCfg.MaxDuration
	}
	if yamlCfg.MinAge != nil && !setFlags["age"] {
		cfg.MinAge = *yamlCfg.MinAge
	}
	if yamlCfg.RunInterval != nil && !setFlags["calc-run-interval"] {
		cfg.RunInterval = *yamlCfg.RunInterval
	}
	if yamlCfg.IncludeExternal != nil && !setFlags["include-external"] {
		cfg.IncludeExternal = *yamlCfg.IncludeExternal
	}
	if yamlCfg.SkipNotCreated != nil && !setFlags["skip-not-created"] {
		cfg.SkipNotCreated = *yamlCfg.SkipNotCreated
	}
	if yamlCfg.LogLevel != nil && !setFlags["log-level"] {
		logs.LogLevel = *yamlCfg.LogLevel
	}
	if yamlCfg.WantJSON != nil && !setFlags["json"] {
		logs.WantJSON = *yamlCfg.WantJSON
	}
}

type configFileRepair struct {
	Par2Args   *[]string `yaml:"args"`
	Par2Verify *bool     `yaml:"verify"`

	MaxDuration          *flags.Duration `yaml:"duration"`
	MinTestedCount       *int            `yaml:"min-tested"`
	SkipNotCreated       *bool           `yaml:"skip-not-created"`
	AttemptUnrepairables *bool           `yaml:"attempt-unrepairables"`
	PurgeBackups         *bool           `yaml:"purge-backups"`
	RestoreBackups       *bool           `yaml:"restore-backups"`

	LogLevel *flags.LogLevel `yaml:"log-level"`
	WantJSON *bool           `yaml:"json"`
}

func (yamlCfg *configFileRepair) Merge(cfg *repair.Options, logs *logging.Options, hasExternalArgs bool, setFlags map[string]bool) {
	if yamlCfg.Par2Args != nil && !hasExternalArgs {
		cfg.Par2Args = slices.Clone(*yamlCfg.Par2Args)
	}
	if yamlCfg.Par2Verify != nil && !setFlags["verify"] {
		cfg.Par2Verify = *yamlCfg.Par2Verify
	}
	if yamlCfg.MaxDuration != nil && !setFlags["duration"] {
		cfg.MaxDuration = *yamlCfg.MaxDuration
	}
	if yamlCfg.MinTestedCount != nil && !setFlags["min-tested"] {
		cfg.MinTestedCount = *yamlCfg.MinTestedCount
	}
	if yamlCfg.SkipNotCreated != nil && !setFlags["skip-not-created"] {
		cfg.SkipNotCreated = *yamlCfg.SkipNotCreated
	}
	if yamlCfg.AttemptUnrepairables != nil && !setFlags["attempt-unrepairables"] {
		cfg.AttemptUnrepairables = *yamlCfg.AttemptUnrepairables
	}
	if yamlCfg.PurgeBackups != nil && !setFlags["purge-backups"] {
		cfg.PurgeBackups = *yamlCfg.PurgeBackups
	}
	if yamlCfg.RestoreBackups != nil && !setFlags["restore-backups"] {
		cfg.RestoreBackups = *yamlCfg.RestoreBackups
	}
	if yamlCfg.LogLevel != nil && !setFlags["log-level"] {
		logs.LogLevel = *yamlCfg.LogLevel
	}
	if yamlCfg.WantJSON != nil && !setFlags["json"] {
		logs.WantJSON = *yamlCfg.WantJSON
	}
}

type configFileInfo struct {
	MaxDuration     *flags.Duration `yaml:"duration"`
	MinAge          *flags.Duration `yaml:"age"`
	RunInterval     *flags.Duration `yaml:"calc-run-interval"`
	IncludeExternal *bool           `yaml:"include-external"`
	SkipNotCreated  *bool           `yaml:"skip-not-created"`

	LogLevel *flags.LogLevel `yaml:"log-level"`
	WantJSON *bool           `yaml:"json"`
}

func (yamlCfg *configFileInfo) Merge(cfg *info.Options, logs *logging.Options, _ bool, setFlags map[string]bool) {
	if yamlCfg.MaxDuration != nil && !setFlags["duration"] {
		cfg.MaxDuration = *yamlCfg.MaxDuration
	}
	if yamlCfg.MinAge != nil && !setFlags["age"] {
		cfg.MinAge = *yamlCfg.MinAge
	}
	if yamlCfg.RunInterval != nil && !setFlags["calc-run-interval"] {
		cfg.RunInterval = *yamlCfg.RunInterval
	}
	if yamlCfg.IncludeExternal != nil && !setFlags["include-external"] {
		cfg.IncludeExternal = *yamlCfg.IncludeExternal
	}
	if yamlCfg.SkipNotCreated != nil && !setFlags["skip-not-created"] {
		cfg.SkipNotCreated = *yamlCfg.SkipNotCreated
	}
	if yamlCfg.LogLevel != nil && !setFlags["log-level"] {
		logs.LogLevel = *yamlCfg.LogLevel
	}
	if yamlCfg.WantJSON != nil && !setFlags["json"] {
		logs.WantJSON = *yamlCfg.WantJSON
	}
}
