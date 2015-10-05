## LPG LOAD BALANCER

Simple load balancer with Http API.

[![Build Status](http://ci.lpgenerator.ru/projects/7/status.png?ref=master)](http://ci.lpgenerator.ru/projects/7?ref=master)


### Usage

Add instance to LB

    curl -X PUT --user lb:7eNQ4iWLgDw4Q6w -d 'url=127.0.0.1:8081&weight=1' -H "Accept: application/json" -s -i http://127.0.0.1:9090

List all instances under LB

    curl -X GET --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090

Remove instance from LB

    curl -X DELETE --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090/127.0.0.1:8081

Get LB statistics

    curl -X GET --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090/stats


### Control

Add instance to LB

    lpg-load-balancer ctl -a add -b 127.0.0.1:8081

List all instances under LB

    lpg-load-balancer ctl -a list

Remove instance from LB

    lpg-load-balancer ctl -a delete -b 127.0.0.1:8081

Get LB statistics

    lpg-load-balancer ctl -a stats


### Default configuration

API listen port

    api-address = "0.0.0.0:9090"

LB listen port

    lb-address = "127.0.0.1:8080"

LB access log

    lb-log-file = "/var/log/lpg-lb.log"

Servers examples

    [servers]
      [servers.web-1]
      url = "http://127.0.0.1:8081"
      weight = 0
    
      [servers.web-2]
      url = "http://127.0.0.1:8082"
      weight = 0


### Contributing

The official repository for this project is on [Github.com](https://github.com/gotlium/lpg-load-balancer).

* [Development](docs/development/README.md)
* [Issues](https://github.com/gotlium/lpg-load-balancer/issues)
* [Pull Requests](https://github.com/gotlium/lpg-load-balancer/pulls)


### Requirements

**None:** lpg-load-balancer is run as a single binary.

This project is designed for the Linux, OS X and Windows operating systems.

### Installation

* [Install on OSX (preferred)](docs/install/osx.md)
* [Install on Windows (preferred)](docs/install/windows.md)
* [Use on FreeBSD](docs/install/freebsd.md)
* [Install development environment](docs/development/README.md)

### Statistics

* 10K+ req per sec
* 12 MB memory usage

### Help

```bash
$ lpg-load-balancer --help

NAME:
   lpg-load-balancer - Simple load balancer with Http API.

USAGE:
   lpg-load-balancer [global options] command [command options] [arguments...]
   
VERSION:
   1.0~beta.0.g4badd3b (4badd3b)
   
AUTHOR(S):
   GoTLiuM InSPiRiT <gotlium@gmail.com> 
   
COMMANDS:
   run          Run Load Balancer
   ctl          Control utility
   install      Install service
   uninstall    Uninstall service
   start        Start service
   stop         Stop service
   restart      Restart service
   http         Run simple HTTP server
   verify       Verify configuration
   help, h      Shows a list of commands or help for one command
   
GLOBAL OPTIONS:
   --debug                      Debug mode [$DEBUG]
   --log-level, -l "error"      Log level (options: debug, info, warn, error, fatal, panic)
   --help, -h                   Show help
   --version, -v                Print the version
```

### License

GPLv3
