# LPG LOAD BALANCER

Simple load balancer with Http API.


## Usage

Add instance to LB

    curl "http://127.0.0.1:8182/add/?url=http://127.0.0.1:8081"

Remove instance from LB

    curl "http://127.0.0.1:8182/del/?url=http://127.0.0.1:8081"
