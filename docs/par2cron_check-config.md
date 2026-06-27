## par2cron check-config

Validates a par2cron YAML configuration file

### Synopsis

Validates the syntax of a par2cron YAML configuration
Use the command to check configurations before deploying

Invalid configurations will prevent par2cron from starting;
this command will exit with non-zero if the validation fails.

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron check-config [flags] <file>
```

### Examples

```

Validate a par2cron YAML configuration file:
  par2cron check-config /tmp/par2cron.yaml
```

### Options

```
  -h, --help   help for check-config
```

### Options inherited from parent commands

```
      --cgroup string     cgroup v2 directory to constrain par2 processes
      --json              output results/logs in JSON format (where applicable)
  -l, --log-level level   minimum level of emitted logs (debug|info|warn|error) (default info)
      --mprof string      write RAM allocation profile to file
      --pprof string      write CPU performance profile to file
      --seq-key string    API key for a (remote) Seq logging server
      --seq-url string    CLEF ingestion URL for a (remote) Seq logging server
```

### SEE ALSO

* [par2cron](par2cron.md)	 - PAR2 Integrity & Self-Repair Engine

