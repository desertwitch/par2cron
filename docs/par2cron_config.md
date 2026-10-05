## par2cron config

Commands for interacting with par2cron configuration files

### Synopsis

Commands for interacting with par2cron configuration files

par2cron can be configured entirely through command-line arguments,
but given the amount of customization available, some users may
prefer a configuration file instead. A single configuration file
can be shared across all par2cron commands. When CLI arguments and
a configuration file are combined, CLI arguments take precedence
over the same options set in the configuration file.

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron config [flags]
```

### Options

```
  -h, --help   help for config
```

### Options inherited from parent commands

```
      --cgroup string     cgroup v2 directory to constrain par2 processes
      --json              output results/logs in JSON format (where applicable)
  -l, --log-level level   minimum level of emitted logs (debug|info|warn|error) (default info)
      --log-plain         emit uncolored plain-text logs with full timestamps
      --mprof string      write RAM allocation profile to file
      --pprof string      write CPU performance profile to file
      --seq-key string    API key for a (remote) Seq logging server
      --seq-url string    CLEF ingestion URL for a (remote) Seq logging server
```

### SEE ALSO

* [par2cron](par2cron.md)	 - PAR2 Integrity & Self-Repair Engine
* [par2cron config check](par2cron_config_check.md)	 - Validates a par2cron YAML configuration file
* [par2cron config example](par2cron_config_example.md)	 - Prints a commented par2cron example configuration

