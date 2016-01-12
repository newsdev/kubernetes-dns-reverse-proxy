package main

// A reverse proxy to route incoming HTTP requests

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/newsdev/kubernetes-dns-reverse-proxy/reverseproxy"
)

var config reverseproxy.Config

func init() {
	flag.StringVar(&config.Address, "address", ":8080", "address to run the proxy server on")
	flag.StringVar(&config.StatusAddress, "status-address", ":8081", "address to run the status server on")
	flag.StringVar(&config.DomainSuffixesRaw, "domain-suffixes", ".local", "domain suffixes")
	flag.StringVar(&config.Kubernetes.DNSDomain, "kubernetes-dns-domain", "cluster.local", "Kubernetes DNS domain")
	flag.StringVar(&config.Kubernetes.Namespace, "kubernetes-namespace", "default", "Kubernetes namespace to server")
	flag.BoolVar(&config.Static.Enable, "static", false, "enable static proxy")
	flag.StringVar(&config.Static.Scheme, "static-scheme", "http", "static scheme")
	flag.StringVar(&config.Static.Host, "static-host", "", "static host")
	flag.StringVar(&config.Static.Path, "static-path", "/", "static path")
	flag.BoolVar(&config.Fallback.Enable, "fallback", false, "enable fallback proxy")
	flag.StringVar(&config.Fallback.Scheme, "fallback-scheme", "http", "fallback scheme")
	flag.StringVar(&config.Fallback.Host, "fallback-host", "", "fallback host")
	flag.StringVar(&config.Fallback.Path, "fallback-path", "/", "fallback path")
	flag.StringVar(&config.RoutesFilename, "routes", "", "path to a routes file")
	flag.BoolVar(&config.ValidateRoutes, "validate-routes", false, "validate routes file and exit")
	flag.IntVar(&config.Concurrency, "concurrency", 32, "concurrency per host")
	flag.IntVar(&config.CompressionLevel, "compression-level", 4, "gzip compression level (0 to disable)")
	flag.DurationVar(&config.Timeout, "timeout", time.Second, "dial timeout")
}

func main() {
	flag.Parse()

	// Set domain suffixes based on the config.
	domainSuffixes := config.DomainSuffixes()
	log.Println("domain suffixes:", domainSuffixes)

	// Set the kubernetes suffix based on the config.
	kubernetesSuffix := fmt.Sprintf(".%s.%s", config.Kubernetes.Namespace, config.Kubernetes.DNSDomain)
	log.Println("kubernetes suffix:", kubernetesSuffix)

	reverseProxy, err := reverseproxy.NewReverseProxy(&config)
	if err != nil {
		log.Fatal("Unable to instantiate reverse proxy:", err)
	}

	log.Println("routes are valid!")
	if config.ValidateRoutes {
		return
	}

	reverseProxyServer := &http.Server{
		Addr:    config.Address,
		Handler: reverseProxy,
	}

	statusServer := &http.Server{
		Addr: config.StatusAddress,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "ok")
		}),
	}

	// Each server could return a fatal error, so make a channel to signal on.
	errs := make(chan error)

	go func() {
		log.Println("starting server on", config.Address)
		errs <- reverseProxyServer.ListenAndServe()
	}()

	go func() {
		log.Println("starting status server on", config.StatusAddress)
		errs <- statusServer.ListenAndServe()
	}()

	// Any error is fatal, so we only need to listen for the first one.
	log.Fatal(<-errs)
}
