package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strings"
	"time"

	"github.com/newsdev/kubernetes-dns-reverse-proxy/director"
	"github.com/newsdev/kubernetes-dns-reverse-proxy/httpwrapper"
)

var config struct {
	address, statusAddress        string
	domainSuffixes                string
	routesFilename                string
	concurrency, compressionLevel int
	timeout                       time.Duration
	validateRoutes                bool

	kubernetes struct {
		namespace, dnsDomain string
	}

	static, fallback struct {
		enable             bool
		scheme, host, path string
	}
}

func init() {
	flag.StringVar(&config.address, "address", ":8080", "address to run the proxy server on")
	flag.StringVar(&config.statusAddress, "status-address", ":8081", "address to run the status server on")
	flag.StringVar(&config.domainSuffixes, "domain-suffixes", ".local", "domain suffixes")
	flag.StringVar(&config.kubernetes.dnsDomain, "kubernetes-dns-domain", "cluster.local", "Kubernetes DNS domain")
	flag.StringVar(&config.kubernetes.namespace, "kubernetes-namespace", "default", "Kubernetes namespace to server")
	flag.BoolVar(&config.static.enable, "static", false, "enable static proxy")
	flag.StringVar(&config.static.scheme, "static-scheme", "http", "static scheme")
	flag.StringVar(&config.static.host, "static-host", "", "static host")
	flag.StringVar(&config.static.path, "static-path", "/", "static path")
	flag.BoolVar(&config.fallback.enable, "fallback", false, "enable fallback proxy")
	flag.StringVar(&config.fallback.scheme, "fallback-scheme", "http", "fallback scheme")
	flag.StringVar(&config.fallback.host, "fallback-host", "", "fallback host")
	flag.StringVar(&config.fallback.path, "fallback-path", "/", "fallback path")
	flag.StringVar(&config.routesFilename, "routes", "", "path to a routes file")
	flag.BoolVar(&config.validateRoutes, "validate-routes", false, "validate routes file and exit")
	flag.IntVar(&config.concurrency, "concurrency", 32, "concurrency per host")
	flag.IntVar(&config.compressionLevel, "compression-level", 4, "gzip compression level (0 to disable)")
	flag.DurationVar(&config.timeout, "timeout", time.Second, "dial timeout")
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
	if config.validateRoutes || config.routesFilename != "" {

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

		log.Println("routes are valid!")
		if config.validateRoutes {
			return
		}
	}

	// Build the reverse proxy HTTP handler.
	reverseProxy := &httputil.ReverseProxy{
		// Specify a custom transport which rate limits requests and compresses responses.
		Transport: &httpwrapper.Transport{
			MaxConcurrencyPerHost: config.concurrency,
			CompressionLevel:      config.compressionLevel,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: config.concurrency,
				Dial: func(network, addr string) (net.Conn, error) {
					return net.DialTimeout(network, addr, config.timeout)
				},
			},
		},
		// The Director has the opportunity to modify the HTTP request before it
		// is handed off to the Transport.
		Director: func(req *http.Request) {
			// empty director atm
		},
	}

	mainServer := &http.Server{
		Addr: config.address,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			// TODO: 
			// doesn't seem to be a way of routing something that matches the domain suffix
			// into either static or redirect

			// Drop the connection header to ensure keepalives are maintained.
			req.Header.Del("connection")

			// First, check if the request is for a default domain-based service routing
			// i.e. http://{servicename}.{domain-suffix}/
			for _, domainSuffix := range domainSuffixes {
				if root := strings.TrimSuffix(req.Host, domainSuffix); root != req.Host {
					req.URL.Scheme = "http"
					req.URL.Host = root + kubernetesSuffix
					log.Println("Domain Suffix Match:", req.Host, req.URL.Path)
					reverseProxy.ServeHTTP(w, req)
					return
				}
			}

			// Then, try the director.
			if root, err := d.Service(req.Host, req.URL.Path); err != nil {
				// The director didn't find a match, handle it gracefully.

				if err != director.NoMatchingServiceError {

					log.Println("Error:", req.Host, req.URL.Path, err)
				} else {

					// Send traffic to the fallback.
					if config.fallback.enable {

						// Set the URL scheme, host, and path.
						req.URL.Scheme = config.fallback.scheme
						req.URL.Host = config.fallback.host
						req.URL.Path = path.Join(config.fallback.path, req.URL.Path)

						log.Println("Fallback:", req.Host, req.URL.Path, "to", req.URL.Host)
					}
				}

			} else {
				// The director found a match.

				if config.static.enable && strings.HasPrefix(root, "/") {
					// Handle static file requests.

					// we need to modify response
					// with equivalent of nginx
					// proxy_redirect /<%= application.name %>/ /;
					// http://nginx.org/en/docs/http/ngx_http_proxy_module.html#proxy_redirect
					// // Sets the text that should be changed in the “Location” and “Refresh” header
					// // fields of a proxied server response.
					// Otherwise, AWS returned redirects will have wrong paths
					//
					// for example
					// curl -v http://well.127.0.0.1.xip.io:8080/projects/workouts
					// Location: /well_workout/projects/workouts/
					// needs to get rewritten to
					// Location: /projects/workouts/
					// so
					// here we set headers so that
					// in httpwrapper.Transport.RoundTrip we know what's needed to  be replaced
					req.Header.Add("x-static-root", path.Join(config.static.path, root)+"/")
					req.Header.Add("x-original-url", req.Host+req.URL.String())

					// Set the URL scheme, host, and path.
					req.URL.Scheme = config.static.scheme
					req.URL.Host = config.static.host

					log.Println("Path: ", req.URL.Path)
					trailing := strings.HasSuffix(req.URL.Path, "/")

					req.URL.Path = path.Join(config.static.path, root, req.URL.Path)
					if trailing && !strings.HasSuffix(req.URL.Path, "/") {
						req.URL.Path += "/"
					}

					// Set the request host (used as the "Host" header value).
					req.Host = config.static.host

					// Drop cookies given that the response should not vary.
					req.Header.Del("cookie")

					log.Println("Static:", req.Header.Get("x-original-url"), "to", req.URL.Host+req.URL.Path)


				} else if url := strings.TrimPrefix(root, ">"); url != root {
					url += req.URL.Path
					if req.URL.RawQuery != "" {
						url += "?"+req.URL.RawQuery
					}
					//TODO: pass query string along with
					log.Printf("Redirect: %s%s to %s", req.Host, req.URL.Path, url)
					http.Redirect(w, req, url, 301)
					return
				} else {
					// Handle an arbitrary URL routing to a service.

					req.URL.Scheme = "http"
					req.URL.Host = root + kubernetesSuffix
					log.Println("Proxy:", req.Host+req.URL.Path, "to", req.URL.Host)
				}
			}

			reverseProxy.ServeHTTP(w, req)
		}),
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
		errs <- mainServer.ListenAndServe()
	}()

	go func() {
		log.Println("starting status server on", config.statusAddress)
		errs <- statusServer.ListenAndServe()
	}()

	// Any error is fatal, so we only need to listen for the first one.
	log.Fatal(<-errs)
}
