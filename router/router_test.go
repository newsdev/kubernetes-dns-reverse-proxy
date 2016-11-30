package router

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestRouter(t *testing.T) {
	tests := []struct {
		givenRoutesJSON string
		given           Config
	}{
		{
			`{
				"www.cats.com": {
					"/": "cats"
				},
				"www.dogs.com": {
					"/brown": ">https://www.cats.com",
					"/": ">https://www.cats.com"
				}
			}`,
			Config{
				"",
				"",
				"",
				"",
				32,
				4,
				time.Minute,
				false,
				false,
				KubernetesConfig{
					"default",
					"svc.cluster.local",
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

		router, err := NewKubernetesRouter(&test.given)
		if err != nil {
			t.Fatal(err)
			t.Fatal("Can't create router")
		}


		request, err := http.NewRequest("GET", "http://www.cats.com/tabby", nil)
		if err != nil {
			t.Fatal("bad given test URL")
		}
		// Save the URL for later.
		request.Header.Add("X-Original-URL", request.URL.String())
		// Add a connection header, which the ReverseProxy should drop.
		request.Header.Add("connection", "glerf")
		responseRecorder := httptest.NewRecorder()
		router.Handler.ServeHTTP(responseRecorder, request)
		if request.URL.Host != "cats.default.svc.cluster.local" {
			t.Errorf("The router should have rewritten the request %s to %s, but it was mapped to %s", request.Header.Get("X-Original-URL"), "cats.default.svc.cluster.local", request.URL.Host)
		}
		if request.Header.Get("connection") != "" {
			t.Error("The router should drop the connection header, it did not.")
		}

		request, err = http.NewRequest("GET", "http://www.dogs.com/", nil)
		responseRecorder = httptest.NewRecorder()
		router.Handler.ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != 301 {
			t.Errorf("Should return a 301, but it returned %s", responseRecorder.Code)
		}
		if responseRecorder.HeaderMap.Get("Location") != "https://www.cats.com/" {
			t.Errorf("Should return a redirect Location to https://www.cats.com/, but it returned %s", responseRecorder.HeaderMap.Get("Location"))
		}


		request, err = http.NewRequest("GET", "http://www.dogs.com/brown/good", nil)
		responseRecorder = httptest.NewRecorder()
		router.Handler.ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != 301 {
			t.Errorf("Should return a 301, but it returned %s", responseRecorder.Code)
		}
		if responseRecorder.HeaderMap.Get("Location") != "https://www.cats.com/good" {
			t.Errorf("Should return a redirect Location to https://www.cats.com/good, but it returned %s", responseRecorder.HeaderMap.Get("Location"))
		}

		request, err = http.NewRequest("GET", "http://www.dogs.com/yellow", nil)
		responseRecorder = httptest.NewRecorder()
		router.Handler.ServeHTTP(responseRecorder, request)
		if responseRecorder.Code != 301 {
			t.Errorf("Should return a 301, but it returned %s", responseRecorder.Code)
		}
		if responseRecorder.HeaderMap.Get("Location") != "https://www.cats.com/yellow" {
			t.Errorf("Should return a redirect Location to https://www.cats.com/yellow, but it returned %s", responseRecorder.HeaderMap.Get("Location"))
		}

		os.Remove(routefile.Name())
	}
}
