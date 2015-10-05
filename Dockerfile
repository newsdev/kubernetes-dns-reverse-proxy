FROM golang:1.5.1
ENV CGO_ENABLED=0
COPY . /go/src/github.com/buth/kubernetes-dns-reverse-proxy
WORKDIR /go/src/github.com/buth/kubernetes-dns-reverse-proxy
