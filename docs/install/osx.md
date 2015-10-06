### Install on OSX

Download the binary for your system:

```bash
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/v1.0/l0xyd-OSX
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/l0xyd
```

Install L0xyd as service and start it:

```bash
cd ~
l0xyd install
l0xyd start
```

Voila! L0xyd is installed and will be run after system reboot.

### Update

Stop the service:

```bash
l0xyd stop
```
