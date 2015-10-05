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
	"path"
	"strings"

	"github.com/buth/kubernetes-dns-reverse-proxy/director"
)

var config struct {
	address, statusAddress string
	domainSuffixes         string
	routesFilename         string

	kubernetes struct {
		namespace, dnsDomain string
	}

	static struct {
		enable        bool
		address, path string
	}
}

func init() {
	flag.StringVar(&config.address, "address", ":8080", "address to run the proxy server on")
	flag.StringVar(&config.statusAddress, "status-address", ":8081", "address to run the status server on")
	flag.StringVar(&config.domainSuffixes, "domain-suffixes", ".local", "domain suffixes")
	flag.StringVar(&config.kubernetes.dnsDomain, "kubernetes-dns-domain", "cluster.local", "Kubernetes DNS domain")
	flag.StringVar(&config.kubernetes.namespace, "kubernetes-namespace", "default", "Kubernetes namespace to server")
	flag.BoolVar(&config.static.enable, "static", false, "enable static proxy")
	flag.StringVar(&config.static.address, "static-address", "", "static address")
	flag.StringVar(&config.static.path, "static-path", "/", "static path")
	flag.StringVar(&config.routesFilename, "routes", "", "path to a routes file")
}

func main() {
	flag.Parse()

	// Set domain suffixes based on the config.
	domainSuffixes := strings.Split(config.domainSuffixes, ",")
	log.Println("domain suffixes:", domainSuffixes)

	// Set the kubernetes suffix based on the config.
	kubernetesSuffix := fmt.Sprintf(".%s.%s", config.kubernetes.namespace, config.kubernetes.dnsDomain)
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

				// Check if the static proxy is enabled and the director-returned root
				// is a path prefix.
				if config.static.enable && strings.HasPrefix(root, "/") {
					req.URL.Path = path.Join(config.static.path, root, req.URL.Path)
					req.Host = config.static.address
					req.URL.Host = config.static.address
				} else {
					req.URL.Host = root + kubernetesSuffix
				}
			}
		},
	}

	reverseProxyServer := &http.Server{
		Addr:    config.address,
		Handler: reverseProxy,
	}

	statusServer := &http.Server{
		Addr: config.statusAddress,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "ok")
		}),
	}

	// Each server could return a fatal error, so make a channel to signal on.
	errs := make(chan error)

	go func() {
		log.Println("starting server on", config.address)
		errs <- reverseProxyServer.ListenAndServe()
	}()

	go func() {
		log.Println("starting status server on", config.statusAddress)
		errs <- statusServer.ListenAndServe()
	}()

	// Any error is fatal, so we only need to listen for the first one.
	log.Fatal(<-errs)
}
