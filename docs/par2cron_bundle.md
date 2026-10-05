## par2cron bundle

Commands for interacting with par2cron's bundle format

### Synopsis

Commands for interacting with par2cron's bundle format

By default par2cron creates its application-specific files
next to each respective PAR2 set. This results in at least
4 files per PAR2 set, which some people may find unwieldy.

Bundle files offer an alternative, a format where par2cron
files and PAR2 set are bundled into one single file, which
in turn is compatible with both par2cron and PAR2 software.

The bundle format itself acts as a mere container for PAR2
data and was designed compliant to the PAR2 specification.
It is resilient against most corruption, able to self-heal
without user interaction upon detecting corrupted metadata.

Normally bundle files are packed during "create" operations
where --bundle was given or set as a marker directive. Some
situations may require to pack existing PAR2 sets or unpack
existing par2cron bundles, which these commands offer to do.

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron bundle [flags]
```

### Options

```
  -h, --help   help for bundle
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
* [par2cron bundle debug](par2cron_bundle_debug.md)	 - Prints bundle debug information to standard output
* [par2cron bundle pack](par2cron_bundle_pack.md)	 - Packs all existing PAR2 sets of a folder into bundles
* [par2cron bundle spec](par2cron_bundle_spec.md)	 - Prints the par2cron bundle file specification
* [par2cron bundle unpack](par2cron_bundle_unpack.md)	 - Unpacks all existing bundles of a folder into PAR2 sets

