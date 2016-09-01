## Manual installation and configuration

### Install

Simply download one of the binaries for your system:

```bash
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-amd64
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-arm64
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-386
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-arm
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/l0xyd
```

Create a l0xyd user (on Linux):

```
useradd --create-home l0xyd --shell /bin/bash
```

Install and run as service (on Linux):
```bash
sudo l0xyd install --user=l0xyd --working-directory=/home/l0xyd
sudo l0xyd start
```

### Update

Stop the service (you need elevated command prompt as before):

```bash
sudo l0xyd stop
```

Download the binary to replace LB's executable:

```bash
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-amd64
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-arm64
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-386
wget -O /usr/local/bin/l0xyd https://github.com/LPgenerator/L0xyd/releases/download/1.0.5/l0xyd-linux-arm
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/l0xyd
```

Start the service:

```bash
l0xyd start
```
