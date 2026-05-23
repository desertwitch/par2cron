## par2cron completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	par2cron completion fish | source

To load completions for every new session, execute once:

	par2cron completion fish > ~/.config/fish/completions/par2cron.fish

You will need to start a new shell for this setup to take effect.


```
par2cron completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --mprof string   write RAM allocation profile to file
      --pprof string   write CPU performance profile to file
```

### SEE ALSO

* [par2cron completion](par2cron_completion.md)	 - Generate the autocompletion script for the specified shell

