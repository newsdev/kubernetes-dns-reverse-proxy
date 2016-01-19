package router

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

type givenReverseProxyRequests struct {
	URL         string
	WantHost    string
	WantHeaders http.Header
}

func TestRouter(t *testing.T) {
	tests := []struct {
		givenRoutesJSON string
		given           RouterConfig
		givenRequests   []givenReverseProxyRequests
	}{
		{
			`{
				"www.cats.com": {
					"/": "cats"
				}
			}`,
			RouterConfig{
				"",
				"",
				"",
				"",
				32,
				4,
				time.Second,
				false,
				KubernetesConfig{
					"default",
					"cluster.local",
				},
				StaticBackendConfig{
					false,
					"http",
					"",
					"/",
				},
				FallbackConfig{
					false,
					"http",
					"",
					"/",
				},
			},
			[]givenReverseProxyRequests{
				givenReverseProxyRequests{
					"http://www.cats.com/tabby",
					"cats.default.cluster.local",
					http.Header{},
				},
				givenReverseProxyRequests{
					"http://www.cats.com/tabby",
					"cats.default.cluster.local",
					http.Header{},
				},
			},
		},
	}

	for _, test := range tests {
		// Use a temp file for the JSON
		routefile, err := ioutil.TempFile("", "cats")
		if err != nil {
			t.Fatal("we wrote something horrible")
		}
		test.given.RoutesFilename = routefile.Name()

		if _, err := routefile.WriteString(test.givenRoutesJSON); err != nil {
			t.Fatal("we wrote something horrible")
		}
		if err := routefile.Close(); err != nil {
			t.Fatal("we wrote something horrible")
		}

		for _, givenRequest := range test.givenRequests {
			router, err := NewKubernetesRouter(&test.given)
			if err != nil {
				t.Fatal(err)
				t.Fatal("Can't create router")
			}
			request, err := http.NewRequest("GET", givenRequest.URL, nil)
			if err != nil {
				t.Fatal("bad given test URL")
			}

			// Save the URL for later.
			request.Header.Add("X-Original-URL", request.URL.String())

			// Add a connection header, which the ReverseProxy should drop.
			request.Header.Add("connection", "glerf")

			responseRecorder := httptest.NewRecorder()
			router.Handler.ServeHTTP(responseRecorder, request)
			if request.URL.Host != givenRequest.WantHost {
				t.Errorf("The router should have rewritten the request %s to %s, but it was mapped to %s", request.Header.Get("X-Original-URL"), givenRequest.WantHost, request.URL.Host)
			}
			if request.Header.Get("connection") != "" {
				t.Error("The router should drop the connection header, it did not.")
			}
		}
		os.Remove(routefile.Name())

	}
}
