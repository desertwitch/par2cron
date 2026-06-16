## par2cron bundle unpack

Unpacks all existing bundles of a folder into PAR2 sets

### Synopsis

Unpacks all existing bundles of a folder into PAR2 sets

Walks the given directories and unpacks existing bundles into
their original files, meaning separation of PAR2 sets and the
par2cron application-specific files (manifest, lockfile). This
can be helpful for returning bundles to the default PAR2 format.

By default bundles will only unpack when it can be guaranteed
that all original files can be replicated and are not corrupted.
In some cases it may be needed to unpack corrupted bundles, this
is achievable (as a last resort) by using the --force argument.

To exclude directories from this operation, put ignore files:
  - ".par2cron-ignore" (ignore directory)
  - ".par2cron-ignore-all" (ignore directory and subdirectories)

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron bundle unpack [flags] <dir> [dir...]
```

### Options

```
      --force   proceed regardless of errors/corruption (use with care)
  -h, --help    help for unpack
```

### Options inherited from parent commands

```
      --json              output results/logs in JSON format (where applicable)
  -l, --log-level level   minimum level of emitted logs (debug|info|warn|error) (default info)
      --mprof string      write RAM allocation profile to file
      --pprof string      write CPU performance profile to file
      --seq-key string    API key for a (remote) Seq logging server
      --seq-url string    CLEF ingestion URL for a (remote) Seq logging server
```

### SEE ALSO

* [par2cron bundle](par2cron_bundle.md)	 - Commands for interacting with par2cron's bundle format

