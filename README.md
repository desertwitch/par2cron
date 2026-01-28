<div align="center">
    <img alt="Logo" src="assets/par2cron.png" width="150">
    <br>
    <h1>par2cron</h1>
    <p>PAR2 Integrity & Self-Repair Engine<br>Selective automated protection for directory trees</p>
</div>

<div align="center">
    <a href="https://github.com/desertwitch/par2cron/releases"><img alt="Release" src="https://img.shields.io/github/release/desertwitch/par2cron.svg"></a>
    <a href="https://go.dev/"><img alt="Go Version" src="https://img.shields.io/badge/Go-%3E%3D%201.25.5-%23007d9c"></a>
    <a href="https://pkg.go.dev/github.com/desertwitch/par2cron"><img alt="Go Reference" src="https://pkg.go.dev/badge/github.com/desertwitch/par2cron.svg"></a>
    <a href="https://goreportcard.com/report/github.com/desertwitch/par2cron"><img alt="Go Report" src="https://goreportcard.com/badge/github.com/desertwitch/par2cron"></a>
    <a href="./LICENSE"><img alt="License" src="https://img.shields.io/github/license/desertwitch/par2cron"></a>
    <br>
    <a href="https://app.codecov.io/gh/desertwitch/par2cron"><img alt="Codecov" src="https://codecov.io/github/desertwitch/par2cron/graph/badge.svg?token=SLUM5DRVHR"></a>
    <a href="https://github.com/desertwitch/par2cron/actions/workflows/golangci-lint.yml"><img alt="Lint" src="https://github.com/desertwitch/par2cron/actions/workflows/golangci-lint.yml/badge.svg"></a>
    <a href="https://github.com/desertwitch/par2cron/actions/workflows/golang-tests.yml"><img alt="Tests" src="https://github.com/desertwitch/par2cron/actions/workflows/golang-tests.yml/badge.svg"></a>
    <a href="https://github.com/desertwitch/par2cron/actions/workflows/golang-build.yml"><img alt="Build" src="https://github.com/desertwitch/par2cron/actions/workflows/golang-build.yml/badge.svg"></a>
</div>

<div align="center">
<sup>This software is in development, expect more frequent releases until a stable release.</sup>
</div>

## Overview

par2cron is a tool that wraps `par2cmdline` (a parity-based file recovery tool)
to achieve automated periodic integrity creation, verification and repair within
any given directory tree. It is designed for use with non-changing WORM-type of
files, perfect for adding a degree of protection to media libraries or backups.

The driving idea is that you do not need to invest in a filesystem (like ZFS)
that protects all your data, at the disadvantage of additional complexities,
when you really only care that important subsets of your data remain protected.

A given directory tree on any filesystem is scanned for marker files, and a
PAR2 set created for every directory containing such a `_par2cron` file. For
verification, the program loads the PAR2 sets and verifies that the data which
they are protecting is healthy, otherwise flagging the PAR2 set for repair.
Once repair runs, corrupted or missing files are recovered. Many command-line
tunables, as well as configuration directives, are offered for more granular
adjustment of how to create, when to verify and in what situation to repair.

A set-and-forget setup is as easy as adding three commands to `crontab`:
- `par2cron create`
- `par2cron verify`
- `par2cron repair`

That being set up, you can simply protect any valuable folder by just placing a
`_par2cron` file in it; the tool will create a PAR2 set and pick it up into the
periodic verification and repair cycle - now protected from corruption/bitrot.

## Quick Start

A default setup involves adding three simple `crontab` entries:
```bash
0 1 * * * par2cron create /mnt/storage
0 3 * * * par2cron verify /mnt/storage
0 5 * * * par2cron repair /mnt/storage
```

Once configured, protecting a new folder is as simple as:

- Navigating to any directory within `/mnt/storage`

- Creating an empty "marker" file named `_par2cron`

- Done - your files are protected after the next scheduled run!

PAR2 sets are then verified and repaired with by the set up periodic tasks.

**A condensed quick guide and cheatsheet can be found in the [QUICKGUIDE](QUICKGUIDE) file.**

> **One PAR2 per folder:** To keep your mental model simple, marker-based PAR2
> creation does not recurse into subfolders. The flat protection scope ensures
> that you know exactly which files a PAR2 covers, without needing to remember
> any directory tree complexities (avoiding surprises during data recovering).

## Installation

To build from source, a `Makefile` is included with the project's source code.
Running `make all` will compile the application and pull in any necessary
dependencies. `make check` runs the test suite and static analysis tools.

For convenience, precompiled static binaries for common architectures are
released through GitHub. These can be installed into `/usr/bin/` or respective
system locations; ensure they are executable by running `chmod +x` before use.

> All builds from source are designed to generate [reproducible builds](https://reproducible-builds.org/),
> meaning that they should compile as byte-identical to the respective released binaries and also have the
> exact same checksums upon integrity verification.

### Dependencies

- `par2` (the binary of the *par2cmdline* tool):
    - Debian/Ubuntu: `apt install par2`
    - macOS: `brew install par2`
    - Fedora: `dnf install par2cmdline`

### Building from source

```bash
git clone https://github.com/desertwitch/par2cron.git
cd par2cron
make all
```

### Running a built executable

```bash
./par2cron --help
```

## Usage

The program is divided into separate commands to achieve its tasks:

| Command                 | Purpose                                                |
| :---------------------- | :----------------------------------------------------- |
| `par2cron create`       | Creates PAR2 sets for directories with marker files    |
| `par2cron verify`       | Verifies existing PAR2 sets in a directory tree        |
| `par2cron repair`       | Repairs corrupted files using PAR2 recovery data       |
| `par2cron info`         | Shows verification cycle and configuration statistics  |
| `par2cron check-config` | Validates a par2cron YAML configuration file           |

### `par2cron create`

```
Scans a directory tree for "_par2cron" marker files
Creates PAR2 sets for directories containing a marker file

Usage:
  par2cron create [flags] <dir> [-- par2-args...]

Examples:

Use configuration file instead of CLI arguments:
  par2cron create -c /tmp/par2cron.yaml /mnt/storage

Pass "-r15 -n1" (15% redundancy, 1 recovery file) to par2:
  par2cron create /mnt/storage -- -r15 -n1

Run for around 1 hour (as soft limit), hide created files:
  par2cron create -d 1h --hidden /mnt/storage

Flags:
  -c, --config string       path to a par2cron YAML configuration file
  -d, --duration duration   time budget per run (best effort/soft limit)
  -g, --glob string         PAR2 set default glob (files to include) (default "*")
  -h, --help                help for create
      --hidden              create PAR2 sets and related files as hidden (dotfiles)
      --json                output structured logs in JSON format
  -l, --log-level level     minimum level of emitted logs (debug|info|warn|error) (default info)
  -m, --mode mode           PAR2 set default mode; per-file or per-folder (file|folder) (default folder)
  -v, --verify              PAR2 sets must pass verification as part of creation
```

### `par2cron verify`
```
Verifies all protected data using the existing PAR2 sets
Corrupted/missing files are flagged for the repair operation

Usage:
  par2cron verify [flags] <dir> [-- par2-args...]

Examples:

Use configuration file instead of CLI arguments:
  par2cron verify -c /tmp/par2cron.yaml /mnt/storage

Verify all sets, argument "-q" (quiet mode) for par2:
  par2cron verify /mnt/storage -- -q

Verify sets not verified < 7 days, run around 2 hours:
  par2cron verify -a 7d -d 2h /mnt/storage

Flags:
  -a, --age duration                 minimum time between re-verifications (skip if verified within this period)
  -i, --calc-run-interval duration   how often you run par2cron verify (for backlog calculations) (default 24h)
  -c, --config string                path to a par2cron YAML configuration file
  -d, --duration duration            time budget per run (best effort/soft limit)
  -h, --help                         help for verify
  -e, --include-external             include PAR2 sets without a par2cron manifest (and create one)
      --json                         output structured logs in JSON format
  -l, --log-level level              minimum level of emitted logs (debug|info|warn|error) (default info)
      --skip-not-created             skip PAR2 sets without a par2cron manifest containing a creation record
```

> **External PAR2**: While par2cron creates flat sets, it can verify existing
> sets (even recursive ones) created by other tools. Use the `--include-external`
> flag to pull these into par2cron's verification cycle (manifests will be created).

### `par2cron repair`
```
Repair all data flagged as repairable during verification
Uses existing PAR2 sets to recover corrupted/missing files

Usage:
  par2cron repair [flags] <dir> [-- par2-args...]

Examples:

Use configuration file instead of CLI arguments:
  par2cron repair -c /tmp/par2cron.yaml /mnt/storage

Repair all sets, argument "-q" (quiet mode) for par2:
  par2cron repair -u /mnt/storage -- -q

Repair repairable, verify after, run for around 1 hour:
  par2cron repair -d 1h -v /mnt/storage

Flags:
  -u, --attempt-unrepairables   attempt to repair PAR2 sets marked as unrepairable
  -c, --config string           path to a par2cron YAML configuration file
  -d, --duration duration       time budget per run (best effort/soft limit)
  -h, --help                    help for repair
      --json                    output structured logs in JSON format
  -l, --log-level level         minimum level of emitted logs (debug|info|warn|error) (default info)
  -t, --min-tested int          repair only when verified as corrupted at least X times
  -p, --purge-backups           remove obsolete backup files (.1, .2, ...) after successful repair
  -r, --restore-backups         roll back protected files to pre-repair state after unsuccessful repair
      --skip-not-created        skip PAR2 sets without a par2cron manifest containing a creation record
  -v, --verify                  PAR2 sets must pass verification as part of repair
```

### `par2cron info`
```
Analyzes the directory tree for statistics about PAR2 sets
Shows verification statistics and configuration information

Usage:
  par2cron info [flags] <dir>

Examples:

Analyze a 7-day cycle with 2-hour daily runs:
  par2cron info -a 7d -d 2h /mnt/storage

Analyze a 14-day cycle with 4-hour weekly runs:
  par2cron info -a 14d -d 4h -i 1w /mnt/storage

Output results as JSON (stdout/standard output):
  par2cron info --json /mnt/storage

Flags:
  -a, --age duration                 target cycle length (time between re-verifications)
  -i, --calc-run-interval duration   how often you run par2cron verify (default 24h)
  -c, --config string                path to a par2cron YAML configuration file
  -d, --duration duration            target time budget for each verify run (soft limit)
  -h, --help                         help for info
  -e, --include-external             include external PAR2 sets without a par2cron manifest
      --json                         output in JSON format (result to stdout, logs to stderr)
  -l, --log-level level              minimum level of emitted logs (debug|info|warn|error) (default info)
      --skip-not-created             skip PAR2 sets without a par2cron manifest containing a creation record
```

### `par2cron check-config`
```
Validates the syntax of a par2cron YAML configuration
Use the command to check configurations before deploying

Usage:
  par2cron check-config <file> [flags]

Examples:

Validate a par2cron YAML configuration file:
  par2cron check-config /tmp/par2cron.yaml

Flags:
  -h, --help   help for check-config
```

## Output Streams

As par2cron needs to coordinate between itself and the `par2` program, their
output is clearly and cleanly separated. All par2cron logs, using structured
logging (either text-/JSON-based), are written to standard error (`stderr`).
Unstructured `par2` program output is written to standard output (`stdout`).

The only anomaly to the above is the `info` command, which does not use the
`par2` program. In non-JSON mode, again structured *logging* is written to
standard error (`stderr`), and unstructured information to standard output
(`stdout`). In JSON mode, all structured *logging* is written to standard
error (`stderr`), and the JSON-encoded result to standard output (`stdout`).

As a general rule of thumb this can be condensed into:
- Structured *logging* goes to standard error (`stderr`)
- Command-related *output* goes to standard output (`stdout`)

## Exit Codes

Granular codes allow for integration with scripts and notification services:

| Code | Name            | Description                                                   |
| :--- | :-------------- | :------------------------------------------------------------ |
| 0    | Success         | All operations completed successfully.                        |
| 1    | Partial Failure | One or more tasks failed, but the process continued.          |
| 2    | Bad Invocation  | Invalid command-line arguments or configuration error.        |
| 3    | Repairable      | Corruption detected, but parity data is sufficient to repair. |
| 4    | Unrepairable    | Corruption detected that exceeds available redundancy.        |
| 5    | Unclassified    | An unexpected or unknown error occurred.                      |

In general the program is able to recover from most problematic situations
without user interaction, either retrying failures at a later time or with
rebuilding corrupted or missing manifests (read more about manifests below)
wherever possible. Failure-related exit codes usually directly relate to
encountered errors requiring some degree of manual inspection by the user.

## Creation Arguments

By default, no additional arguments are given to the `par2` program for the
three calling par2cron operations. However, it is strongly recommended to
set default `par2` arguments for the `create` command, to be reflecting your
personal needs and situation. You can decide the default set of arguments to
give to `par2` for any of the par2cron commands using either the configuration
file or appending them as `[-- par2-args...]`. An example of the latter below:

```bash
par2cron create /mnt/storage -- -r15 -n1
par2cron verify /mnt/storage -- -q
par2cron repair /mnt/storage -- -m512
```

As you can see, anything following `--` are treated as default arguments to
pass to the `par2` program for that par2cron operation. For the `create`
operation, this can then be influenced for individual creation jobs by use
of the marker filename or marker configuration (read more about this below).

A list for all the possible `par2` arguments can be found here:

https://github.com/Parchive/par2cmdline#using-par2cmdline

With the exception of the `-R` argument (as par2cron allows no recursion).

## State Management

The program aims to off-load all state directly next to the protected files.
As a result, par2cron creates a *manifest* and *lock* file next to each PAR2
set. While this may seem as clutter at first, it is a conscious design choice
eliminating the need for a central database and allowing these files to travel
alongside backups. Verification and repair heavily utilize the manifest file
to transition a verification result into an eventual repair or re-verification
where needed. The lock file is used to ensure that multiple par2cron instances
can run concurrently on the same directory tree without any cross-interference.

```
/mnt/storage/Pictures/
├── beach.jpg
├── flowers.jpg
├── Pictures.par2          <-- par2 index file
├── Pictures.vol00+01.par2 <-- par2 recovery data
├── Pictures.par2.json     <-- par2cron manifest
└── Pictures.par2.lock     <-- par2cron lockfile
```

Because all state is stored locally within the directory tree, you can move your
protected folders between different drives or servers. As long as par2cron is
running on the new host, it will pick up existing manifests and continue the
verification cycle. While the lockfile ensures multiple par2cron instances on
the same computer do not collide, you need to ensure that shared locations are
only ever accessed by one par2cron instance at a time (network/cloud drives).

The `--hidden` argument of `create` can be useful to hide the PAR2 sets, if
the amount of files is something that can be a bother for file organization.
If opting for this, it should be noted that some backup programs will not
transfer hidden files (dotfiles) without being configured to do so, so you
should consider this when moving around par2cron-protected directory trees.

## Marker Files

The core of the par2cron `create` operation are the marker files. A found marker
file denotes that **files** in the containing directory need protecting. In most
basic form it is just an empty `_par2cron` file, with the defaults from the set
command-line arguments or configuration file being applied, but more control for
individual creation jobs is possible through the marker files (read more below).

Upon successful creation of the PAR2 set, the marker file is deleted. In case
of failure, the creation is retried with the next run. If a same-named PAR2 set
is already present in the directory, the marker file is skipped and a warning
presented to the user (not resulting in a non-zero exit code).

Subfolders are not considered for the created PAR2 set, as par2cron promotes a
clear mental model of "One PAR2 per folder". This helps to reduce cognitive load
and wondering "Which files did this PAR2 protect again?". If you wish to create
recursive PAR2 sets (which we do not recommend), you can use other software and
then import these into your par2cron verification cycle (refer to `verify` usage).

```
/mnt/storage/Pictures/
├── Nature/
├── _par2cron  <-- par2cron marker file
├── beach.jpg  <-- will be protected using PAR2
└── sunset.jpg <-- will be protected using PAR2
```

Users wanting to re-create any of their PAR2 sets (having added or updated files)
simply need to delete that PAR2 set, placing a new marker file into the directory.

### Marker Filename

You can *modify* the default arguments that are given to `par2`, settable
through the command-line arguments or configuration file (see *Usage* and
*Configuration* sections), for individual creation jobs. An example would be
that your defaults are `-r15 -n1`, for 15% redundancy and 1 recovery file. Now
you have an especially important dataset that you would like to have 30% of
redundancy for, without wanting to change your configuration or affecting other
creations. This can simply be realized by creating a marker file with the name
`_par2cron_r30`, which will then create the PAR2 set using `-r30 -n1`, so
leaving in place the other default arguments (in this case `-n1`).

If an argument provided as part of a marker filename is not among the default
arguments given to `par2`, it is simply added for that creation job. An example
would be wanting to add `-q` to your `-r15 -n1` default, in which case you would
simply create a marker file named `_par2cron_q`. This also applies if no default
arguments to be given to `par2` were set, effectively adding any arguments that
are part of the marker filename, again - only for that individual creation job.

| Marker filename   | Default arguments | Resulting arguments |
| :---------------- | :---------------- | :------------------ |
| `_par2cron`       | `-r15 -n1`        | `-r15 -n1`          |
| `_par2cron_q`     | `-r15 -n1`        | `-r15 -n1 -q`       |
| `_par2cron_r30`   | `-r15 -n1`        | `-r30 -n1`          |
| `_par2cron_r30_q` | `-r15 -n1`        | `-r30 -n1 -q`       |

The use case for this is being able to fine-tune individual creation jobs by
just memorizing the often-used, important arguments for e.g. redundancy without
having to remember the entire (set as default) collection of `par2` arguments.

Above examples assumed `-r15 -n1` were set as the `par2` default arguments for
the creation task (using `par2cron create /mnt/storage -- -r15 -n1`). However,
it applies for any combination of default creation arguments passed to `par2`.

For more control, or *replacing* the entire default arguments that are given
to `par2` (again only for the individual creation job), read below about marker
configuration (the optional content that can be placed within a marker file).

### Marker Configuration

In most cases a marker file will have no content, but for maximum control over
individual creation jobs it is possible to place YAML directives inside. These
directives allow overriding the defaults set via command-line arguments or
configuration file, but only for the individual creation job. All settings are
optional, so you can just pick the ones needed for the individual creation job.

Below is an example of a configuration using all directives:

```yaml
# Override name of the PAR2 set to be created
# Beware that this setting does not apply in file mode
name: "Ubuntu"

# Override the arguments passed to par2
# Replaces the default arguments set in CLI/configuration
args: ["-r30", "-n1"]

# Override the glob pattern for which files in the directory to protect
glob: "*.iso"

# Override the creation mode (per-file or per-folder) [file|folder]
mode: "folder"

# Override whether to verify the PAR2 set after creation
verify: true

# Override whether to create the PAR2 set and related files as hidden
hidden: true
```

The directives are designed to be easy to remember, although for the rare case
that you should need such a marker configuration a little cheat-sheet is to be
recommended, because YAML directive errors will result in a non-zero exit code.

### Folder Mode vs. File Mode

The `create` mode of par2cron offers two distinct operation modes. The default,
`folder` mode, creates one PAR2 set for the entire folder the marker file was
found in. This is useful for medium-sized sets of data where multiple PAR2 sets
would unnecessarily pollute the folder.

In folder mode, unless changed through below means, PAR2 sets created for found
marker files will assume the name of the directory they reside in, so the PAR2
set for files within `/mnt/storage/Pictures` will be named `Pictures.par2`.

The `file` mode creates one PAR2 set for each file of the folder instead, which
can be useful for large sets of data where verification time may be a concern.
The disadvantage is that more files are produced, cluttering directories more.

In file mode, any PAR2 sets that are created will always be named after the file
they are meant to protect, beware that this is not changeable (at this time).

## Ignore Files

A situation may arise where you want to exclude a folder (or directory tree)
from all par2cron operations either temporarily or permanently. You can do so
by placing an ignore file in that directory, so that it is excluded from the
job enumeration of par2cron. This allows to e.g. exclude directories with PAR2
sets that you do not want verified or otherwise interacted with by the program.

- `.par2cron-ignore` (ignore this folder)
- `.par2cron-ignore-all` (ignore this folder and subfolders)

## Configuration

A configuration file can be given to par2cron, which is reusable and replaces
the need to achieve complex setups through the command-line arguments entirely.

You should verify the configuration using `par2cron check-config`, as malformed
configuration will prevent the program from starting (bad invocation exit code).

**For a full configuration example, refer to the [par2cron.yaml](par2cron.yaml) file.**

## Limitations

par2cron, and PAR2 in general, is mostly designed to operate on non-changing
data. It simply has no concept of data being updated, instead flagging such
updates as possible corruption. If you need to update any protected files,
you will need to manually delete the PAR2 set and then have it recreated using
the marker file approach (equals the process for new sets of protectable data).

A marker file only triggers PAR2 creation for files in its immediate directory.
It will not recurse/traverse into subdirectories. To protect a directory tree,
you must place a marker file in each specific folder you wish to secure. As a
result, the `par2` argument `-R` has no effect with the `create` command.

par2cron-generated PAR2 set will consist of at least 4 files and possibly more
depending on your `par2` arguments. This can cause significant file clutter in
directories, which can be mitigated by using the `--hidden` argument with
`create` (read more about this in the above section *State Management*).

While the lockfile ensures multiple par2cron instances on the same computer
do not collide, you need to ensure that shared (network) locations are only
ever accessed by one par2cron computer at a time (e.g. different weekdays).

## License

All code is licensed under the MIT License.
