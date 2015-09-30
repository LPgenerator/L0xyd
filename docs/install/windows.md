### Install on Windows

Create a folder somewhere in your system, ex.: `C:\lpg-load-balancer`.

Download the binary for [x86][]  or [amd64][] and put it into the folder you
created.

Run an `Administrator` command prompt ([How to][prompt]). The simplest is to
write `Command Prompt` in Windows search field, right click and select
`Run as administrator`. You will be asked to confirm that you want to execute
the elevated command prompt.

Install lpg-load-balancer as a service and start it. You have to enter a valid password
for the current user account, because it's required to start the service by Windows:

```bash
lpg-load-balancer install --password ENTER-YOUR-PASSWORD
lpg-load-balancer start
```

Voila! lpg-load-balancer is installed and will be run after system reboot.

Logs are stored in Windows Event Log.

#### Update

Stop service (you need elevated command prompt as before):

```bash
cd C:\lpg-load-balancer
lpg-load-balancer stop
```

Download the binary for [x86][] or [amd64][] and replace runner's executable.

Start service:

```bash
lpg-load-balancer start
```

[x86]: https://github.com/gotlium/lpg-load-balancer/releases/download/lpg-load-balancer-windows-386.exe
[amd64]: https://github.com/gotlium/lpg-load-balancer/releases/download/lpg-load-balancer-windows-amd64.exe
