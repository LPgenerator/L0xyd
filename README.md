## L0xyd

Simple load balancer with Http API.


### Usage

Add instance to LB

    curl -X PUT --user lb:7eNQ4iWLgDw4Q6w -d 'url=127.0.0.1:8081&weight=1' -H "Accept: application/json" -s -i http://127.0.0.1:9090

List all instances under LB

    curl -X GET --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090

Remove instance from LB

    curl -X DELETE --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090/127.0.0.1:8081

Get LB statistics

    curl -X GET --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090/stats

Get LB status

    curl -X GET --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090/status


### Control

Add instance to LB

    l0xyd ctl -a add -b 127.0.0.1:8081

List all instances under LB

    l0xyd ctl -a list

Remove instance from LB

    l0xyd ctl -a delete -b 127.0.0.1:8081

Get LB statistics

    l0xyd ctl -a stats

Get LB status

    l0xyd ctl -a status


### Default configuration

API listen port

    api-address = "0.0.0.0:9090"

LB listen port

    lb-address = "127.0.0.1:8080"

LB access log

    lb-log-file = "/var/log/l0xyd.log"

Servers examples

    [servers]
      [servers.web-1]
      url = "http://127.0.0.1:8081"
      weight = 0
    
      [servers.web-2]
      url = "http://127.0.0.1:8082"
      weight = 0


### Contributing

The official repository for this project is on [Github.com](https://github.com/LPgenerator/L0xyd).

* [Development](docs/development/README.md)
* [Issues](https://github.com/LPgenerator/L0xyd/issues)
* [Pull Requests](https://github.com/LPgenerator/L0xyd/pulls)


### Requirements

**None:** L0xyd is run as a single binary.

This project is designed for the Linux, OS X and Windows operating systems.

### Installation

* [Install on OSX (preferred)](docs/install/osx.md)
* [Install on Windows (preferred)](docs/install/windows.md)
* [Install on FreeBSD](docs/install/freebsd.md)
* [Install development environment](docs/development/README.md)

### Statistics

* 10K+ req per sec
* 16 MB memory usage
* 10 MB on fs

### Help

```bash
$ l0xyd --help

NAME:
   L0xyd - Simple load balancer with Http API.

USAGE:
   l0xyd [global options] command [command options] [arguments...]
   
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
