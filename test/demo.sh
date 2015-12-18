#!/bin/bash

go run test/server/server.go & go run kubernetes-dns-reverse-proxy.go \
                                       --kubernetes-dns-domain=127.0.0.1.xip.io:8090 \
                                       --domain-suffixes=.pub.stg.127.0.0.1.xip.io:8080 \
                                       --static-host=int-static-prd.s3-website-us-east-1.amazonaws.com \
                                       --static \
                                       --static-scheme=http \
                                       --static-path=/ \
                                       --routes test/routes.json \
                                       
