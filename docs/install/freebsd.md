### Install on FreeBSD

Download the binary for your system:

```bash
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/v1.0/l0xyd-freebsd-amd64
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/v1.0/l0xyd-freebsd-386
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/l0xyd
```

Run GitLab-Runner:

```bash
cd ~
l0xyd run
```

Voila! l0xyd is currently running, but it will not start automatically after system reboot.
