package httpwrapper

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
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

func compressResponse(resp *http.Response) error {

	// Setup a new gzip writer built arround a bytes buffer.
	buf := bytes.NewBuffer([]byte{})
	gzipBuf, err := gzip.NewWriterLevel(buf, GzipCompressionLevel)
	if err != nil {
		return err
	}

	// Copy the response body to the gzip writer.
	if _, err := io.Copy(gzipBuf, resp.Body); err != nil {
		return err
	}
	resp.Body.Close()

	// Flush and close the gzip buffer, as it won't be written to again.
	gzipBuf.Flush()
	gzipBuf.Close()

	// Set content headers.
	resp.Header.Set("Content-Encoding", "gzip")
	resp.Header.Set("Content-Length", strconv.Itoa(buf.Len()))
	resp.Body = ioutil.NopCloser(buf)
	return nil
}

func compressionEnabledRequest(req *http.Request) bool {

	// Check if the request defines the Accept-Encoding header.
	requestAcceptEncoding := req.Header.Get("Accept-Encoding")
	if requestAcceptEncoding == "" {
		return false
	}

	// Check if the list of accepted encodings includes gzip.
	return strings.Contains(requestAcceptEncoding, "gzip")
}

func compressableResponse(resp *http.Response) bool {

	// Check if content length was defined. If it is and its value is lower than
	// the MTU size, this isn't something we should try and compress.
	responseContentLength := resp.Header.Get("Content-Length")
	if size, err := strconv.Atoi(responseContentLength); err == nil && size < MTUSize {
		return false
	}

	// Check if a response type has been defined.
	responseType := resp.Header.Get("Content-Type")
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
	sem, ok := t.sem[req.Host]
	if !ok {
		sem = make(chan struct{}, t.MaxConcurrencyPerHost)
		t.sem[req.Host] = sem
	}
	return sem
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {

	// Get the sem for this request and try to aquire it.
	var sem chan struct{}
	if t.MaxConcurrencyPerHost > 0 {
		sem = t.getSem(req)
		sem <- struct{}{}
	}

	// Make the request.
	resp, err := t.Transport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

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
