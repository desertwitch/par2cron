package main

const rootUsage = "par2cron"

const rootHelpShort = "PAR2 Integrity & Self-Repair Engine"

const rootHelpLong = `par2cron - PAR2 Integrity & Self-Repair Engine
Selective automated protection for directory trees

par2cron wraps par2cmdline to provide cron-friendly, automated
integrity verification and repair for directory trees. Designed
for WORM (non-changing) files like media libraries and backups.

Quick Start:
  1. Add crontab entries for create, verify, and repair
  2. Place "_par2cron" marker files in directories to protect
  3. PAR2 sets are created, verified and repaired automatically

See 'par2cron <command> --help' for command-specific information.
Documentation: https://github.com/desertwitch/par2cron`

const checkConfigUsage = "check-config <file>"

const checkConfigHelpShort = "Validates a par2cron YAML configuration file"

const checkConfigHelpLong = `Validates the syntax of a par2cron YAML configuration
Use the command to check configurations before deploying

Invalid configurations will prevent par2cron from starting;
this command will exit with non-zero if the validation fails.

Full documentation at: https://github.com/desertwitch/par2cron`

const checkConfigHelpExample = `
Validate a par2cron YAML configuration file:
  par2cron check-config /tmp/par2cron.yaml`

const createUsage = "create [flags] <dir>... [-- par2-args...]"

const createHelpShort = "Creates PAR2 sets for directories with marker files"

const createHelpLong = `Scans a directory tree for "_par2cron" marker files
Creates PAR2 sets for directories containing a marker file

Marker file name can be used to change default [-- par2-args]
for individual jobs. Example: "_par2cron_r30" changes default
arguments of "-r15 -n1" to "-r30 -n1". "_par2cron_q" changes
default arguments "-r15 -n1" to "-r15 -n1 -q". If no default
arguments are given, filename arguments get used for that job.

Marker file content can be a YAML configuration to override
most settings (set through CLI or a --config configuration)
for the individual job, refer to documentation for examples.

One PAR2 per folder: By default a marker file only triggers
PAR2 creation for files in its immediate directory, it does
not recurse into subdirectories. Recursion is possible using
deep globs (**) or in recursive mode, refer to documentation.

To exclude directories from this operation, put ignore files:
  ".par2cron-ignore" - ignore directory
  ".par2cron-ignore-all" - ignore directory and subdirectories

Documentation: https://github.com/desertwitch/par2cron`

const createHelpExample = `
Use configuration file instead of CLI arguments:
  par2cron create -c /tmp/par2cron.yaml /mnt/storage

Pass "-r15 -n1" (15% redundancy, 1 recovery file) to par2:
  par2cron create /mnt/storage -- -r15 -n1

Run for around 1 hour (as soft limit), hide created files:
  par2cron create -d 1h --hidden /mnt/storage`

const verifyUsage = "verify [flags] <dir>... [-- par2-args...]"

const verifyHelpShort = "Verifies the existing PAR2 sets found in a directory tree"

const verifyHelpLong = `Verifies all protected data using the existing PAR2 sets
Corrupted/missing files are flagged for the repair operation

Scans a directory tree for PAR2 sets and par2cron manifests.
If --include-external is enabled and no manifest is found, one
is generated and the set included in future verification cycles.
Otherwise, only PAR2 sets with an existing par2cron manifest are
verified and all external PAR2 sets will be skipped over instead.

To exclude directories from this operation, put ignore files:
  ".par2cron-ignore" - ignore directory
  ".par2cron-ignore-all" - ignore directory and subdirectories

Full documentation at: https://github.com/desertwitch/par2cron`

const verifyHelpExample = `
Use configuration file instead of CLI arguments:
  par2cron verify -c /tmp/par2cron.yaml /mnt/storage

Verify all sets, argument "-q" (quiet mode) for par2:
  par2cron verify /mnt/storage -- -q

Verify sets not verified < 7 days, run around 2 hours:
  par2cron verify -a 7d -d 2h /mnt/storage`

const repairUsage = "repair [flags] <dir>... [-- par2-args...]"

const repairHelpShort = "Repairs any corrupted files using the PAR2 recovery data"

const repairHelpLong = `Repair all data flagged as repairable during verification
Uses existing PAR2 sets to recover corrupted/missing files

Scan the directory tree for manifests marked with corruption
by an earlier verification run. With --attempt-unrepairables
not set, only manifests marked repairable will be attempted
for repair. Otherwise, manifests with any kind of corruption
will be attempted, but beware this may lead to non-zero exit
codes if the underlying data should really not be repairable.

To exclude directories from this operation, put ignore files:
  ".par2cron-ignore" - ignore directory
  ".par2cron-ignore-all" - ignore directory and subdirectories

Full documentation at: https://github.com/desertwitch/par2cron`

const repairHelpExample = `
Use configuration file instead of CLI arguments:
  par2cron repair -c /tmp/par2cron.yaml /mnt/storage

Repair all sets, argument "-q" (quiet mode) for par2:
  par2cron repair -u /mnt/storage -- -q

Repair repairable, verify after, run for around 1 hour:
  par2cron repair -d 1h -v /mnt/storage`

const infoUsage = "info [flags] <dir>..."

const infoHelpShort = "Shows verification cycle and configuration statistics"

const infoHelpLong = `Analyzes the directory tree for statistics about PAR2 sets
Shows verification statistics and configuration information

Use the same arguments/configuration as for "verify" command.

To exclude directories from this operation, put ignore files:
  ".par2cron-ignore" - ignore directory
  ".par2cron-ignore-all" - ignore directory and subdirectories

Full documentation at: https://github.com/desertwitch/par2cron`

const infoHelpExample = `
Analyze a 7-day cycle with 2-hour daily runs:
  par2cron info -a 7d -d 2h /mnt/storage

Analyze a 14-day cycle with 4-hour weekly runs:
  par2cron info -a 14d -d 4h -i 1w /mnt/storage

Output results as JSON (stdout/standard output):
  par2cron info --json /mnt/storage`

const bundleUsage = "bundle"

const bundleHelpShort = "Commands for interacting with par2cron's bundle format"

const bundleHelpLong = `Commands for interacting with par2cron's bundle format

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
existing par2cron bundles, which these commands offer to do.`

const bundlePackUsage = "pack [flags] <dir>... "

const bundlePackHelpShort = "Packs all existing PAR2 sets of a folder into bundles"

const bundlePackHelpLong = `Packs all existing PAR2 sets of a folder into bundles

Walks the given directories and replaces existing PAR2 sets
with a single bundle file. This can be helpful for switching
already created PAR2 sets to the bundle format introduced in
later releases of par2cron. For new sets, the create command
should be used with --bundle or respective marker directives.`

const bundleUnpackUsage = "unpack [flags] <dir>... "

const bundleUnpackHelpShort = "Unpacks all existing bundles of a folder into PAR2 sets"

const bundleUnpackHelpLong = `Unpacks all existing bundles of a folder into PAR2 sets

Walks the given directories and unpacks existing bundles into
their original files, meaning separation of PAR2 sets and the
par2cron application-specific files (manifest, lockfile). This
can be helpful for returning bundles to the default PAR2 format.

By default bundles will only unpack when it can be guaranteed
that all original files can be replicated and are not corrupted.
In some cases it may be needed to unpack corrupted bundles, this
is achievable (as a last resort) by using the --force argument.`
