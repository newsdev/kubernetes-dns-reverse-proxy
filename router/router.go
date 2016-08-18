package router

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	// "net"
	"net/http"
	"net/http/httputil"
	"os"
	"path"
	"strings"
	"time"
	// "context"

	"github.com/newsdev/kubernetes-dns-reverse-proxy/accesslog"
	"github.com/newsdev/kubernetes-dns-reverse-proxy/director"
	"github.com/newsdev/kubernetes-dns-reverse-proxy/httpwrapper"
)

// Config is a configuration data structure for the router.
type Config struct {
	Address, StatusAddress        string
	DomainSuffixesRaw             string
	RoutesFilename                string
	Concurrency, CompressionLevel int
	Timeout                       time.Duration
	ValidateRoutes                bool

	Kubernetes KubernetesConfig

	Static   StaticBackendConfig
	Fallback FallbackConfig
}

// KubernetesConfig describes properties of the Kubernetes back-end.
type KubernetesConfig struct {
	Namespace, DNSDomain string
}

// StaticBackendConfig describes properties of the static file backend.
type StaticBackendConfig struct {
	Enable             bool
	Scheme, Host, Path string
}

// FallbackConfig describes properties of the fallback. What's the fallback for?
type FallbackConfig struct {
	Enable             bool
	Scheme, Host, Path string
}

// DomainSuffixes gets a comma separated list of the service domain suffixes.
func (c *Config) DomainSuffixes() []string {
	return strings.Split(c.DomainSuffixesRaw, ",")
}

// KubernetesServiceDomainSuffix gets the Kubernetes service domain suffix.
// When appended to a service name, gives a hostname that a service is available on.
func (c *Config) KubernetesServiceDomainSuffix() string {
	return fmt.Sprintf(".%s.%s", c.Kubernetes.Namespace, c.Kubernetes.DNSDomain)
}

// NewKubernetesRouter gives you a router instance.
func NewKubernetesRouter(config *Config) (*http.Server, error) {

	log.Println("Domain suffixes:", config.DomainSuffixes())
	log.Println("Kubernetes service domain suffix:", config.KubernetesServiceDomainSuffix())

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
	reverseProxy := &httputil.ReverseProxy{
		// Specify a custom transport which rate limits requests and compresses responses.
		Transport: &httpwrapper.Transport{
			MaxConcurrencyPerHost: config.Concurrency,
			CompressionLevel:      config.CompressionLevel,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: config.Concurrency,
				// DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {

				// 	// TODO:
				// 	// resolve timeouts here

				// 	return net.DialTimeout(network, addr, config.Timeout)
				// 	// return net.Dial(network, addr)
				// },
			},
		},
		// The Director has the opportunity to modify the HTTP request before it
		// is handed off to the Transport.
		Director: func(req *http.Request) {
			// empty director atm
		},
	}

	return &http.Server{
			Addr: config.Address,
			Handler: accesslog.CustomLoggingHandler(
				os.Stdout, 
				http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
					// Drop the connection header to ensure keepalives are maintained.
					req.Header.Del("connection")

					if root, err := dir.Service(req.Host, req.URL.Path); err != nil {
						// The director didn't find a match, handle it gracefully.

						if err != director.NoMatchingServiceError {
							log.Println("Error:", req.Host, req.URL.Path, err)
						} else {

							// If NoMatchingServiceError is thrown, check against the domain suffixes, e.g. {service}.local
							for _, domainSuffix := range config.DomainSuffixes() {
								if root := strings.TrimSuffix(req.Host, domainSuffix); root != req.Host {
									req.URL.Scheme = "http"
									req.URL.Host = root + config.KubernetesServiceDomainSuffix()
									log.Println("Domain Suffix Match:", req.Host, req.URL.Host, req.URL.Path)
									reverseProxy.ServeHTTP(w, req)
									return
								}
							}

							// Otherwise, send traffic to the fallback.
							if config.Fallback.Enable {

								// Set the URL scheme, host, and path.
								req.URL.Scheme = config.Fallback.Scheme
								req.URL.Host = config.Fallback.Host
								req.URL.Path = path.Join(config.Fallback.Path, req.URL.Path)

								log.Println("Fallback:", req.Host, req.URL.Path, "to", req.URL.Host)
							} else {
								log.Println("Error: no route matched and fallback not enabled for", req.Host, req.URL.Path)
							}

						}

					} else {
						// The director found a match.

						if config.Static.Enable && strings.HasPrefix(root, "/") {
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
							req.Header.Add("x-static-root", path.Join(config.Static.Path, root)+"/")
							req.Header.Add("x-original-url", req.Host+req.URL.String())

							// Set the URL scheme, host, and path.
							req.URL.Scheme = config.Static.Scheme
							req.URL.Host = config.Static.Host

							// log.Println("Path: ", req.URL.Path)
							trailing := strings.HasSuffix(req.URL.Path, "/")

							req.URL.Path = path.Join(config.Static.Path, root, req.URL.Path)
							if trailing && !strings.HasSuffix(req.URL.Path, "/") {
								req.URL.Path += "/"
							}

							// Set the request host (used as the "Host" header value).
							req.Host = config.Static.Host

							// Drop cookies given that the response should not vary.
							req.Header.Del("cookie")

							// log.Println("Static:", req.Header.Get("x-original-url"), "to", req.URL.Host+req.URL.Path)

						} else if url := strings.TrimPrefix(root, ">"); url != root {
							url += req.URL.Path
							if req.URL.RawQuery != "" {
								url += "?" + req.URL.RawQuery
							}
							//TODO: pass query string along with
							log.Printf("Redirect: %s%s to %s", req.Host, req.URL.Path, url)
							http.Redirect(w, req, url, 301)
							return
						} else {
							// Handle an arbitrary URL routing to a service.

							req.URL.Scheme = "http"
							req.URL.Host = root + config.KubernetesServiceDomainSuffix()
							log.Println("Proxy:", req.Host+req.URL.Path, "to", req.URL.Host)
						}
					}

					reverseProxy.ServeHTTP(w, req)
				}),
			),
		},
		nil
}
