package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"

	"github.com/buth/kubernetes-dns-reverse-proxy/director"
)

var config struct {
	address             string
	domainSuffixes      string
	kubernetesDNSDomain string
	kubernetesNamespace string
	routesFilename      string
}

func init() {
	flag.StringVar(&config.address, "address", ":8080", "domain suffixes")
	flag.StringVar(&config.domainSuffixes, "domain-suffixes", ".local", "domain suffixes")
	flag.StringVar(&config.kubernetesDNSDomain, "kubernetes-dns-domain", "cluster.local", "Kubernetes DNS domain")
	flag.StringVar(&config.kubernetesNamespace, "kubernetes-namespace", "default", "Kubernetes namespace to server")
	flag.StringVar(&config.routesFilename, "routes", "", "path to a routes file")
}

func main() {
	flag.Parse()

	// Set domain suffixes based on the config.
	domainSuffixes := strings.Split(config.domainSuffixes, ",")
	log.Println("domain suffixes:", domainSuffixes)

	// Set the kubernetes suffix based on the config.
	kubernetesSuffix := fmt.Sprintf(".%s.%s", config.kubernetesNamespace, config.kubernetesDNSDomain)
	log.Println("kubernetes suffix:", kubernetesSuffix)

	// Create a new director object.
	d := director.NewDirector()

	// Check for a routes JSON file.
	if config.routesFilename != "" {

		routesFile, err := os.Open(config.routesFilename)
		if err != nil {
			log.Fatal(err)
		}

		routesJSON, err := ioutil.ReadAll(routesFile)
		if err != nil {
			log.Fatal(err)
		}

		if err := routesFile.Close(); err != nil {
			log.Fatal(err)
		}

		var routes map[string]map[string]string
		if err := json.Unmarshal(routesJSON, &routes); err != nil {
			log.Fatal(err)
		}

		for domain, prefixMap := range routes {
			for prefix, service := range prefixMap {
				d.SetService(domain, prefix, service)
			}
		}
	}

	// Build the reverse proxy HTTP handler.
	reverseProxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = "http"

			// First check against the domain suffixes.
			for _, domainSuffix := range domainSuffixes {
				if root := strings.TrimSuffix(req.Host, domainSuffix); root != req.Host {
					req.URL.Host = root + kubernetesSuffix
					return
				}
			}

			// Then try the director.
			if root, err := d.Service(req.Host, req.URL.Path); err != nil {
				if err != director.NoMatchingServiceError {
					log.Println(err)
				}
			} else {
				req.URL.Host = root + kubernetesSuffix
			}
		},
	}

	// Add the handler and start the server.
	http.Handle("/", reverseProxy)
	log.Println("starting server on", config.address)
	log.Fatal(http.ListenAndServe(config.address, nil))
}
