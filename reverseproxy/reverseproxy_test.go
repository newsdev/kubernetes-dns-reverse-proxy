package reverseproxy

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"testing"
)

type givenReverseProxyRequests struct {
	URL         string
	WantHost    string
	WantHeaders http.Header
}

func TestNewReverseProxy(t *testing.T) {
	tests := []struct {
		givenRoutesJSON string
		given           Config
		givenRequests   []givenReverseProxyRequests
	}{
		{
			`{
				"www.cats.com": {
					"/": "cats"
				}
			}`,
			Config{
			// --kubernetes-dns-domain=127.0.0.1.xip.io:8090 \
			// --routes test/routes.json
			},
			[]givenReverseProxyRequests{
				givenReverseProxyRequests{
					"http://www.cats.com/tabby",
					"cats.local",
					http.Header{},
				},
				givenReverseProxyRequests{
					"http://www.cats.com/tabby",
					"cats.local",
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

		rp, err := NewReverseProxy(&test.given)
		if err != nil {
			t.Fatal(err)
			t.Fatal("Can't create reverse proxy")
		}

		for _, request := range test.givenRequests {
			r, err := http.NewRequest("GET", request.URL, nil)
			if err != nil {
				t.Fatal("bad given test URL")
			}
			r.Header.Add("connection", "glerf")
			log.Println(r.URL.Host)
			rp.Director(r)
			log.Println(r.URL.Host)
			if r.Header.Get("connection") != "" {
				t.Error("Reverse Proxy should drop the connection header, it did not.")
			}
		}
		os.Remove(routefile.Name())

	}
}
