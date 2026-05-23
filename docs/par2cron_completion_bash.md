## par2cron completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(par2cron completion bash)

To load completions for every new session, execute once:

#### Linux:

	par2cron completion bash > /etc/bash_completion.d/par2cron

#### macOS:

	par2cron completion bash > $(brew --prefix)/etc/bash_completion.d/par2cron

You will need to start a new shell for this setup to take effect.


```
par2cron completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --mprof string   write RAM allocation profile to file
      --pprof string   write CPU performance profile to file
```

### SEE ALSO

* [par2cron completion](par2cron_completion.md)	 - Generate the autocompletion script for the specified shell

