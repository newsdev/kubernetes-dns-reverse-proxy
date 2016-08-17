FROM golang:1.7

# Add a system-user for the Go application.
RUN adduser --system golang-app

# Add the source code and build the binary.
ENV GO15VENDOREXPERIMENT=1
ADD . $GOPATH/src/github.com/newsdev/kubernetes-dns-reverse-proxy
WORKDIR $GOPATH/src/github.com/newsdev/kubernetes-dns-reverse-proxy
RUN go install .

# Run as the system-user.
USER golang-app
CMD ["kubernetes-dns-reverse-proxy"]
