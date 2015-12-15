FROM golang:1.5.2-alpine

# Add a system-user for the Go application.
RUN adduser -SHs /bin/false golang-app

# Add the source code and build the binary.
ENV GO15VENDOREXPERIMENT=1
ADD . $GOPATH/src/github.com/newsdev/kubernetes-dns-reverse-proxy
RUN cd $GOPATH/src/github.com/newsdev/kubernetes-dns-reverse-proxy && go install .

# Run as the system-user.
USER golang-app
ENTRYPOINT ["kubernetes-dns-reverse-proxy"]
