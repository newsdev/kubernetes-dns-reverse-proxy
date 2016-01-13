package reverseproxy

import (
	"encoding/json"
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

type Config struct {
	Address, StatusAddress        string
	DomainSuffixesRaw             string
	RoutesFilename                string
	Concurrency, CompressionLevel int
	Timeout                       time.Duration
	ValidateRoutes                bool

	Kubernetes struct {
		Namespace, DNSDomain string
	}

	Static, Fallback struct {
		Enable             bool
		Scheme, Host, Path string
	}
}

func (c *Config) DomainSuffixes() []string {
	return strings.Split(c.DomainSuffixesRaw, ",")
}

func (c *Config) KubernetesSuffixes() string {
	return fmt.Sprintf(".%s.%s", c.Kubernetes.Namespace, c.Kubernetes.DNSDomain)
}

func NewReverseProxy(config *Config) (*httputil.ReverseProxy, error) {

	// Create a new director object.
	dir := director.NewDirector()

	// Check for a routes JSON file.
	if config.ValidateRoutes || config.RoutesFilename != "" {

		routesFile, err := os.Open(config.RoutesFilename)
		if err != nil {
			return nil, err
		}

		routesJSON, err := ioutil.ReadAll(routesFile)
		if err != nil {
			return nil, err
		}

		if err := routesFile.Close(); err != nil {
			return nil, err
		}

		var routes map[string]map[string]string
		if err := json.Unmarshal(routesJSON, &routes); err != nil {
			return nil, err
		}

		for domain, prefixMap := range routes {
			for prefix, service := range prefixMap {
				dir.SetService(domain, prefix, service)
			}
		}
	}
	// Build the reverse proxy HTTP handler.
	return &httputil.ReverseProxy{
		// Specify a custom transport which rate limits requests and compresses responses.
		Transport: &httpwrapper.Transport{
			MaxConcurrencyPerHost: config.Concurrency,
			CompressionLevel:      config.CompressionLevel,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: config.Concurrency,
				Dial: func(network, addr string) (net.Conn, error) {
					return net.DialTimeout(network, addr, config.Timeout)
				},
			},
		},
		// The Director has the opportunity to modify the HTTP request before it
		// is handed off to the Transport.
		Director: func(req *http.Request) {

			// Drop the connection header to ensure keepalives are maintained.
			req.Header.Del("connection")

			// Check if the request is for a default domain-based service routing
			// i.e. http://{servicename}.{domain-suffix}/
			for _, domainSuffix := range config.DomainSuffixes() {
				if root := strings.TrimSuffix(req.Host, domainSuffix); root != req.Host {
					req.URL.Scheme = "http"
					req.URL.Host = root + config.KubernetesSuffixes()
					log.Println("Domain Suffix Match:", req.Host, req.URL.Path)
					return
				}
			}

			// Then, try the director.
			if root, err := dir.Service(req.Host, req.URL.Path); err != nil {
				// The director didn't find a match, handle it gracefully.
				if err != director.NoMatchingServiceError {

					log.Println("Error:", req.Host, req.URL.Path, err)
				} else {
					// Send traffic to the fallback.
					if config.Fallback.Enable {

						// Set the URL scheme, host, and path.
						req.URL.Scheme = config.Fallback.Scheme
						req.URL.Host = config.Fallback.Host
						req.URL.Path = path.Join(config.Fallback.Path, req.URL.Path)

						log.Println("Fallback:", req.Host, req.URL.Path, "to", req.URL.Host)
					} else {
						log.Println("The fallback was required but not enabled")
					}
				}

			} else {
				// The director found a match.
				if config.Static.Enable && strings.HasPrefix(root, "/") {
					// Handle static file requests.

					// Set the URL scheme, host, and path.
					req.URL.Scheme = config.Static.Scheme
					req.URL.Host = config.Static.Host
					req.URL.Path = path.Join(config.Static.Path, root, req.URL.Path)

					// Set the request host (used as the "Host" header value).
					req.Host = config.Static.Host

					// Drop cookies given that the response should not vary.
					req.Header.Del("cookie")

					log.Println("Static:", req.Host, req.URL.Path, "to", req.URL.Host)
				} else {
					// Handle an arbitrary URL routing to a service.

					req.URL.Scheme = "http"
					// "cats" + ".local"
					req.URL.Host = root + config.KubernetesSuffixes()
					log.Println("Proxy:", req.Host, req.URL.Path, "to", req.URL.Host)
				}
			}
		},
	}, nil
}
