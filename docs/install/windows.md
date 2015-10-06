### Install on Windows

Create a folder somewhere in your system, ex.: `C:\l0xyd`.

Download the binary for [x86][]  or [amd64][] and put it into the folder you
created.

Run an `Administrator` command prompt ([How to][prompt]). The simplest is to
write `Command Prompt` in Windows search field, right click and select
`Run as administrator`. You will be asked to confirm that you want to execute
the elevated command prompt.

Install L0xyd as a service and start it. You have to enter a valid password
for the current user account, because it's required to start the service by Windows:

```bash
l0xyd install --password ENTER-YOUR-PASSWORD
l0xyd start
```

Voila! L0xyd is installed and will be run after system reboot.

Logs are stored in Windows Event Log.

#### Update

Stop service (you need elevated command prompt as before):

```bash
cd C:\l0xyd
l0xyd stop
```

Download the binary for [x86][] or [amd64][] and replace L0xyd's executable.

Start service:

```bash
l0xyd start
```

[x86]: https://github.com/LPgenerator/L0xyd/releases/download/v1.0/l0xyd-windows-386.exe
[amd64]: https://github.com/LPgenerator/L0xyd/releases/download/v1.0/l0xyd-windows-amd64.exe
