### Install on OSX

Download the binary for your system:

```bash
wget -O /usr/local/bin/lpg-load-balancer https://git.lpgenerator.ru/sys/lpg-load-balancer/releases/download/lpg-load-balancer-OSX
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/lpg-load-balancer
```

**The rest of commands execute as the user who will run lpg-load-balancer.**

Install LB as service and start it:

```bash
cd ~
lpg-load-balancer install
lpg-load-balancer start
```

Voila! lpg-load-balancer is installed and will be run after system reboot.

### Update

Stop the service:

```bash
lpg-load-balancer stop
```
