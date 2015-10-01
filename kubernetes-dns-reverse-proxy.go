package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"strings"
)

var config struct {
	address             string
	domainSuffixes      string
	kubernetesDNSDomain string
	kubernetesNamespace string
}

func init() {
	flag.StringVar(&config.address, "address", ":8080", "domain suffixes")
	flag.StringVar(&config.domainSuffixes, "domain-suffixes", "local", "domain suffixes")
	flag.StringVar(&config.kubernetesDNSDomain, "kubernetes-dns-domain", "cluster.local", "Kubernetes DNS domain")
	flag.StringVar(&config.kubernetesNamespace, "kubernetes-namespace", "default", "Kubernetes namespace to serve")
}

func main() {
	flag.Parse()

	// Set suffixes based on the config.
	domainSuffixes := strings.Split(config.domainSuffixes, ",")
	kubernetesSuffix := fmt.Sprintf("%s.%s", config.kubernetesNamespace, config.kubernetesDNSDomain)

	// Build the reverse proxy HTTP handler.
	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"
			for _, domainSuffix := range domainSuffixes {
				if root := strings.TrimSuffix(req.Host, domainSuffix); root != req.Host {
					req.URL.Host = root + kubernetesSuffix
					break
				}
			}
		},
	}

	// Add the handler and start the server.
	http.Handle("/", reverseProxy)
	log.Fatal(http.ListenAndServe(config.address, nil))
}
