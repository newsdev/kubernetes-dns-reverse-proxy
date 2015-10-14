all: bin/kubernetes-dns-reverse-proxy

bin/kubernetes-dns-reverse-proxy: kubernetes-dns-reverse-proxy-build
	docker run --rm -v `pwd`/bin:/opt/bin kubernetes-dns-reverse-proxy-build go build -o /opt/bin/kubernetes-dns-reverse-proxy  

bin:
	mkdir bin

kubernetes-dns-reverse-proxy-build:
	docker build -t kubernetes-dns-reverse-proxy-build .

benchmark: kubernetes-dns-reverse-proxy-build
	docker run --rm kubernetes-dns-reverse-proxy-build test/benchmark.sh

clean:
	docker rmi kubernetes-dns-reverse-proxy-build || true
	rm -rf bin

.PHONY: all benchmark clean kubernetes-dns-reverse-proxy-build
