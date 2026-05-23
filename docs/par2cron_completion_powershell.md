## par2cron completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	par2cron completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
par2cron completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
      --mprof string   write RAM allocation profile to file
      --pprof string   write CPU performance profile to file
```

### SEE ALSO

* [par2cron completion](par2cron_completion.md)	 - Generate the autocompletion script for the specified shell

