#!/bin/bash

go run test/server/server.go & go run kubernetes-dns-reverse-proxy.go \
                                       --kubernetes-dns-domain=127.0.0.1.xip.io:8090 \
                                       --routes test/routes.json