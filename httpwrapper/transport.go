package httpwrapper

import (
	"compress/gzip"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
)

const (
	GzipCompressionLevel = 2
	MTUSize              = 1000
)

var (
	compressableTypes = []string{
		"application/atom+xml",
		"application/javascript",
		"application/json",
		"application/rss+xml",
		"application/vnd.ms-fontobject",
		"application/x-font-ttf",
		"application/x-web-app-manifest+json",
		"application/xhtml+xml",
		"application/xml",
		"font/opentype",
		"image/svg+xml",
		"image/x-icon",
		"text/css",
		"text/plain",
		"text/x-component",
	}
	nothing = struct{}{}
)

type readCloserSem struct {
	io.ReadCloser
	sem chan struct{}
}

func (c *readCloserSem) Close() error {
	err := c.ReadCloser.Close()
	<-c.sem
	return err
}

type Transport struct {
	Transport             http.RoundTripper
	MaxConcurrencyPerHost int

	// Unexported attributes.
	mu  sync.Mutex
	sem map[string]chan struct{}
}

func closeLogError(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Println(err)
	}
}

func compressResponse(resp *http.Response) error {

	// Establish a new pipe.
	pipeReader, pipeWriter := io.Pipe()

	// In a seperate Go routine, compress the request body and copy it to the
	// pipe.
	go func(r io.ReadCloser) {

		// Defer the closing of both the reader and writer.
		defer closeLogError(r)
		defer closeLogError(pipeWriter)

		// Create a new gzip writer, wrapping the original writer, and defer its
		// closing.
		gzipWriter, err := gzip.NewWriterLevel(pipeWriter, GzipCompressionLevel)
		if err != nil {
			log.Println(err)
			return
		}
		defer closeLogError(gzipWriter)

		// Copy the response body to the gzip writer.
		if _, err := io.Copy(gzipWriter, r); err != nil {
			log.Println(err)
		}
	}(resp.Body)

	resp.Header.Set("content-encoding", "gzip")
	resp.Header.Del("content-length")
	resp.Body = pipeReader
	return nil
}

func compressionEnabledRequest(req *http.Request) bool {

	// Check if the request defines the Accept-Encoding header.
	requestAcceptEncoding := req.Header.Get("accept-encoding")
	if requestAcceptEncoding == "" {
		return false
	}

	// Check if the list of accepted encodings includes gzip.
	return strings.Contains(requestAcceptEncoding, "gzip")
}

func compressableResponse(resp *http.Response) bool {

	// Check if content length was defined. If it is and its value is lower than
	// the MTU size, this isn't something we should try and compress.
	if resp.ContentLength >= 0 && resp.ContentLength < MTUSize {
		return false
	}

	// Check if the content has already been gzipped.
	responseEncoding := resp.Header.Get("content-encoding")
	if responseEncoding != "" && strings.Contains(responseEncoding, "gzip") {
		return false
	}

	// Check if a response type has been defined.
	responseType := resp.Header.Get("content-type")
	if responseType == "" {
		return false
	}

	// Then look through the list.
	for _, compressableType := range compressableTypes {
		if strings.Contains(responseType, compressableType) {
			return true
		}
	}
	return false
}

func (t *Transport) getSem(req *http.Request) chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if the sem map has been initialized.
	if t.sem == nil {
		t.sem = make(map[string]chan struct{})
	}

	// Get the host-specific sem, initializing it beforehand if necessare.
	sem, ok := t.sem[req.URL.Host]
	if !ok {
		sem = make(chan struct{}, t.MaxConcurrencyPerHost)
		t.sem[req.URL.Host] = sem
	}
	return sem
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	// Get the sem for this request and try to aquire it.
	var sem chan struct{}
	if t.MaxConcurrencyPerHost > 0 {
		sem = t.getSem(req)
		sem <- nothing
	}

	// Make the request.
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {

		// Check if we need to release the sem.
		if t.MaxConcurrencyPerHost > 0 {
			<-sem
		}

		// Return the error.
		return nil, err
	}

	// Set a few debug headers.
	resp.Header.Set("x-kubernetes-url", req.URL.String())

	// Set up a sem release linked to the response being read.
	if t.MaxConcurrencyPerHost > 0 {
		resp.Body = &readCloserSem{resp.Body, sem}
	}

	// Check if we should compress the respondse
	if compressionEnabledRequest(req) && compressableResponse(resp) {
		if err := compressResponse(resp); err != nil {
			return nil, err
		}
	}

	return resp, nil
}
