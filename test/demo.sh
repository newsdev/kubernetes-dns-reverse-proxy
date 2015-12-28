#!/bin/bash
#
# This runs a back-end server on port 8090,
# and sets the Kubernetes DNS domain suffix appropriately for routing to them.
go run test/server/server.go & go run kubernetes-dns-reverse-proxy.go \
                                       --kubernetes-dns-domain=127.0.0.1.xip.io:8090 \
                                       --domain-suffixes=.pub.stg.127.0.0.1.xip.io:8080 \
                                       --static \
                                       --static-scheme=http \
                                       --static-host=int-static-stg.s3-website-us-east-1.amazonaws.com \
                                       --static-path=/ \
                                       --routes test/routes.json \
