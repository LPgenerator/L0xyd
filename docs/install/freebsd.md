### Install on FreeBSD

Download the binary for your system:

```bash
wget -O /usr/local/bin/lpg-load-balancer https://git.lpgenerator.ru/sys/lpg-load-balancer/releases/download/lpg-load-balancer-freebsd-amd64
wget -O /usr/local/bin/lpg-load-balancer https://git.lpgenerator.ru/sys/lpg-load-balancer/releases/download/lpg-load-balancer-freebsd-386
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/lpg-load-balancer
```

Run GitLab-Runner:

```bash
cd ~
lpg-load-balancer run
```

Voila! lpg-load-balancer is currently running, but it will not start automatically after system reboot.
