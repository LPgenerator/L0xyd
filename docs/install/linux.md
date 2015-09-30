## Manual installation and configuration

### Install

Simply download one of the binaries for your system:

```bash
wget -O /usr/local/bin/lpg-load-balancer https://github.com/gotlium/lpg-load-balancer/releases/download/lpg-load-balancer-linux-386
wget -O /usr/local/bin/lpg-load-balancer https://github.com/gotlium/lpg-load-balancer/releases/download/lpg-load-balancer-linux-amd64
wget -O /usr/local/bin/lpg-load-balancer https://github.com/gotlium/lpg-load-balancer/releases/download/lpg-load-balancer-linux-arm
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/lpg-load-balancer
```

Create a lpg-load-balancer user (on Linux):

```
useradd --create-home lpg-load-balancer --shell /bin/bash
```

Install and run as service (on Linux):
```bash
sudo lpg-load-balancer install --user=lpg-load-balancer --working-directory=/home/lpg-load-balancer
sudo lpg-load-balancer start
```

### Update

Stop the service (you need elevated command prompt as before):

```bash
sudo lpg-load-balancer stop
```

Download the binary to replace runner's executable:

```bash
wget -O /usr/local/bin/lpg-load-balancer https://lpg-load-balancer-downloads.s3.amazonaws.com/latest/binaries/lpg-load-balancer-linux-386
wget -O /usr/local/bin/lpg-load-balancer https://lpg-load-balancer-downloads.s3.amazonaws.com/latest/binaries/lpg-load-balancer-linux-amd64
wget -O /usr/local/bin/lpg-load-balancer https://github.com/gotlium/lpg-load-balancer/releases/download/lpg-load-balancer-linux-arm
```

Give it permissions to execute:

```bash
chmod +x /usr/local/bin/lpg-load-balancer
```

Start the service:

```bash
lpg-load-balancer start
```
