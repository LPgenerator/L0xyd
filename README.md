# LPG LOAD BALANCER

Simple load balancer with Http API.


## Usage

Add instance to LB

    curl -X PUT --user lb:7eNQ4iWLgDw4Q6w -d 'url=127.0.0.1:8081&weight=0' -H "Accept: application/json" -s -i http://127.0.0.1:9090

List all instances under LB

    curl -X GET --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090

Remove instance from LB

    curl -X DELETE --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:9090/127.0.0.1:8081


## Default configuration

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
