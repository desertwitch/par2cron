## par2cron tool md5

Extracts and displays MD5 checksums from PAR2 files

```
par2cron tool md5 [flags] <par2file> [par2file...]
```

### Examples

```

Print MD5 hashes for all PAR2 files in a directory:
  par2cron tool md5 *.par2

Save MD5 hashes to a combined checksum file:
  par2cron tool md5 *.par2 > checksums.md5

Verify protected files against their PAR2 checksums:
  par2cron tool md5 *.par2 | md5sum -c

Print MD5 hashes for a bundle or specific PAR2 file:
  par2cron tool md5 bundle.p2c.par2
```

### Options

```
      --all    attempt to parse all provided files (and not just PAR2 index files)
  -h, --help   help for md5
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

* [par2cron tool](par2cron_tool.md)	 - Useful utility commands for interacting with PAR2 files

