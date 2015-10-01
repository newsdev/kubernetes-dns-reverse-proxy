FROM golang:1.5.1
ENV CGO_ENABLED=0
COPY . /go/github.com/buth/kubernetes-reverse-proxy
WORKDIR /go/github.com/buth/kubernetes-reverse-proxy
