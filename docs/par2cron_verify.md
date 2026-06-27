## par2cron verify

Verifies the existing PAR2 sets found in a directory tree

### Synopsis

Verifies all protected data using the existing PAR2 sets
Corrupted/missing files are flagged for the repair operation

Scans a directory tree for PAR2 sets and par2cron manifests.
If --include-external is enabled and no manifest is found, one
is generated and the set included in future verification cycles.
Otherwise, only PAR2 sets with an existing par2cron manifest are
verified and all external PAR2 sets will be skipped over instead.

To exclude directories from this operation, put ignore files:
  - ".par2cron-ignore" (ignore directory)
  - ".par2cron-ignore-all" (ignore directory and subdirectories)

Full documentation at: https://github.com/desertwitch/par2cron

```
par2cron verify [flags] <dir> [dir...] [-- par2-arg...]
```

### Examples

```

Use configuration file instead of CLI arguments:
  par2cron verify -c /tmp/par2cron.yaml /mnt/storage

Verify all sets, argument "-q" (quiet mode) for par2:
  par2cron verify /mnt/storage -- -q

Verify sets not verified < 7 days, run around 2 hours:
  par2cron verify -a 7d -d 2h /mnt/storage
```

### Options

```
  -a, --age duration                 minimum time between re-verifications (skip if verified within this period)
      --cache string                 directory for optional manifest cache (use same for all commands)
  -i, --calc-run-interval duration   how often you run par2cron verify (for backlog calculations) (default 24h)
  -c, --config string                path to a par2cron YAML configuration file
  -d, --duration duration            time budget per run (best effort/soft limit)
  -h, --help                         help for verify
  -e, --include-external             include PAR2 sets without a par2cron manifest (and create one)
      --skip-not-created             skip PAR2 sets without a par2cron manifest containing a creation record
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

