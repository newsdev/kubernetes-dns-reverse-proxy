all: bin/kubernetes-dns-reverse-proxy

bin/kubernetes-dns-reverse-proxy:
	docker build -t kubernetes-dns-reverse-proxy-build .
	docker run --rm -v `pwd`/bin:/opt/bin kubernetes-dns-reverse-proxy-build go build -o /opt/bin/kubernetes-dns-reverse-proxy  

bin:
	mkdir bin

clean:
	docker rmi kubernetes-dns-reverse-proxy-build || true
	rm -rf bin
