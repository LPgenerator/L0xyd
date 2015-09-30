# LPG LOAD BALANCER

Simple load balancer with Http API.


## Usage

Add instance to LB

    curl -X PUT --user lb:7eNQ4iWLgDw4Q6w -d 'url=127.0.0.1:8081&weight=0' -H "Accept: application/json" -s -i http://127.0.0.1:8182

List all instances under LB

    curl -X GET --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:8182

Remove instance from LB

    curl -X DELETE --user lb:7eNQ4iWLgDw4Q6w -H "Accept: application/json" -s -i http://127.0.0.1:8182/127.0.0.1:8081


## Default configuration

API listen port

    api-address = "0.0.0.0:8081"

LB listen port

    lb-address = "127.0.0.1:8080"
