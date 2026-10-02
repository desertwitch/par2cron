## par2cron bundle debug

Prints bundle debug information to standard output

### Synopsis

Prints bundle debug information to standard output

Parses bundles located at the provided file paths and outputs
the bundle internal metadata and manifest. Returns an exit code
zero in case of all bundles passing internal structural validation,
otherwise a non-zero exit code (depending on failures encountered).

The output of this command should not be used in scripting, as it
may change between versions of par2cron. The bundle specification
can be used to implement custom parsers to retrieve required data.

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron bundle debug [flags] <file> [file...]
```

### Examples

```

Print debug information about a single bundle file:
  par2cron bundle debug /mnt/storage/bundle.p2c.par2

Print debug information about multiple bundle files:
  par2cron bundle debug /mnt/storage/a.p2c.par2 /mnt/storage/b.p2c.par2

Print debug information about bundle files in working directory:
  par2cron bundle debug *.p2c.par2
```

### Options

```
  -h, --help   help for debug
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

* [par2cron bundle](par2cron_bundle.md)	 - Commands for interacting with par2cron's bundle format

