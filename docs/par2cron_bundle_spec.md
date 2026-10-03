## par2cron bundle spec

Prints the par2cron bundle file specification

### Synopsis

Prints the par2cron bundle file specification
Writes the bundle format of this par2cron version to standard output

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron bundle spec [flags]
```

### Examples

```

Write the bundle file specification to a text file:
  par2cron bundle spec > bundle_specification.txt
```

### Options

```
  -h, --help   help for spec
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

