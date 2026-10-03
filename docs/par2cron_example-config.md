## par2cron example-config

Prints a fully commented par2cron example configuration

### Synopsis

Prints a fully commented par2cron example configuration
Writes all options supported by this par2cron version to standard output

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron example-config [flags]
```

### Examples

```

Write the example configuration to a YAML file:
  par2cron example-config > par2cron.yaml

Validate the configuration file after editing it:
  par2cron check-config par2cron.yaml
```

### Options

```
  -h, --help   help for example-config
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

