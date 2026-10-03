## par2cron list

Lists all par2cron-managed PAR2 sets and their status

### Synopsis

Lists all par2cron-managed PAR2 sets and their status

Scans a directory tree for PAR2 sets with a par2cron manifest;
external PAR2 sets without a manifest are not listed. Results
are sorted by severity (most urgent first), then by the path:

  - "unrepairable" (corruption found, cannot be repaired)
  - "repairable" (corruption found, can be repaired)
  - "unverified" (not verified yet)
  - "healthy" (verified, no corruption found)

Columns are separated by whitespace with the path always last,
so the output can be filtered with standard tools (grep, awk).
For scripting, do prefer --json for a machine-readable format.

The column "FAILURES" shows how many times a set that has been
marked as corrupted has consecutively been tested as corrupted.
It visualizes the barrier for the --min-tested repair argument.

The column "EDITED" visualizes if a set that has been marked as
corrupted may have had the protected files intentionally edited
by the user, as newer modification times (mtimes) were detected.
It visualizes the barrier for the --skip-maybe-edited argument.

If a --cache is provided, data from the cache is shown for any
elements that are found in the cache. If none is provided, all
data is loaded from disk instead (as shown by cache indicator).

To exclude directories from this operation, put ignore files:
  - ".par2cron-ignore" (ignore directory)
  - ".par2cron-ignore-all" (ignore directory and subdirectories)

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron list [flags] <dir> [dir...]
```

### Examples

```

List all PAR2 sets with their current status:
  par2cron list /mnt/storage

Show only PAR2 sets with corruption found:
  par2cron list /mnt/storage | grep -E '^(unrepairable|repairable) '

Output results as JSON (stdout/standard output):
  par2cron list --json /mnt/storage
```

### Options

```
      --cache string       directory for optional manifest cache (use same for all commands)
  -h, --help               help for list
      --skip-not-created   skip PAR2 sets without a par2cron manifest containing a creation record
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

