FROM golang:1.5.2
ENV CGO_ENABLED=0
COPY . /go/src/github.com/newsdev/kubernetes-dns-reverse-proxy
WORKDIR /go/src/github.com/newsdev/kubernetes-dns-reverse-proxy
