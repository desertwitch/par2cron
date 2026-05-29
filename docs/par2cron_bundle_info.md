## par2cron bundle info

Prints bundle information to standard output

### Synopsis

Prints bundle information to standard output

Parses bundles located at the provided file paths and outputs
the bundle internal metadata and manifest. Returns an exit code
zero in case that all bundles pass strict validation, otherwise
a non zero exit code (depending on the failures encountered).

The output of this command should not be used in scripting, as it
may change between versions of par2cron. The bundle specification
can be used to implement custom parsers to retrieve required data.

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron bundle info [flags] <file> [file...]
```

### Examples

```

Print information about a single bundle file:
  par2cron bundle info /mnt/storage/bundle.p2c.par2

Print information about multiple bundle files:
  par2cron bundle info /mnt/storage/a.p2c.par2 /mnt/storage/b.p2c.par2

Print information about bundle files in working directory:
  par2cron bundle info *.p2c.par2
```

### Options

```
  -h, --help              help for info
  -l, --log-level level   minimum level of emitted logs (debug|info|warn|error) (default info)
```

### Options inherited from parent commands

```
      --mprof string   write RAM allocation profile to file
      --pprof string   write CPU performance profile to file
```

### SEE ALSO

* [par2cron bundle](par2cron_bundle.md)	 - Commands for interacting with par2cron's bundle format

