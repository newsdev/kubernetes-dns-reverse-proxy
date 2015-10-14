#!/bin/bash
set -e

echo 'setup'
apt-get update > /dev/null
apt-get install -y apache2-utils curl > /dev/null

wupiao(){
  until curl -o /dev/null -sIf -H host:blah $1; do \
    sleep 1 && echo '.';
  done;
}

echo 'running the test backend'
go run test/server/server.go &
wupiao http://127.0.0.1:8090/status

echo 'running the proxy'
go run kubernetes-dns-reverse-proxy.go --static --static-host 127.0.0.1:8090 --routes test/routes.json &
wupiao http://127.0.0.1:8080/status

echo '+ ab -c 100 status'
ab -qS -H host:blah -n 10000 -c 100 localhost:8080/status

echo '+ ab -c 100 lorem'
ab -qS -H host:blah -n 10000 -c 100 localhost:8080/lorem

echo '+ ab -c 100 status (gzip)'
ab -qS -H host:blah -H accept-encoding:gzip -n 10000 -c 100 localhost:8080/status

echo '+ ab -c 100 lorem (gzip)'
ab -qS -H host:blah -H accept-encoding:gzip -n 10000 -c 100 localhost:8080/lorem
