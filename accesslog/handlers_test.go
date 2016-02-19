// Modified from
// https://github.com/gorilla/handlers

// Copyright 2013 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package accesslog

import (
	"bytes"
	"net"
	"net/http"
	"net/url"
	"testing"
	"time"
)

const (
	ok         = "ok\n"
	notAllowed = "Method not allowed\n"
)

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte(ok))
})

func newRequest(method, url string) *http.Request {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		panic(err)
	}
	return req
}

func TestWriteCombinedLog(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		panic(err)
	}
	ts := time.Date(1983, 05, 26, 3, 30, 45, 0, loc)

	// A typical request with an OK response
	req := newRequest("GET", "http://example.com")
	req.RemoteAddr = "192.168.100.5"
	req.Header.Set("Referer", "http://example.com")
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) AppleWebKit/537.33 "+
			"(KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33",
	)

	buf := new(bytes.Buffer)
	writeCombinedLog(buf, req, *req.URL, ts, http.StatusOK, 100)
	log := buf.String()

	expected := "192.168.100.5 - - [26/May/1983:03:30:45 +0200] \"GET / HTTP/1.1\" 200 100 \"http://example.com\" " +
		"\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) " +
		"AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33\"\n"
	if log != expected {
		t.Fatalf("wrong log, got %q want %q", log, expected)
	}

	// CONNECT request over http/2.0
	req1 := &http.Request{
		Method:     "CONNECT",
		Host:       "www.example.com:443",
		Proto:      "HTTP/2.0",
		ProtoMajor: 2,
		ProtoMinor: 0,
		RemoteAddr: "192.168.100.5",
		Header:     http.Header{},
		URL:        &url.URL{Host: "www.example.com:443"},
	}
	req1.Header.Set("Referer", "http://example.com")
	req1.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) AppleWebKit/537.33 "+
			"(KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33",
	)

	buf = new(bytes.Buffer)
	writeCombinedLog(buf, req1, *req1.URL, ts, http.StatusOK, 100)
	log = buf.String()

	expected = "192.168.100.5 - - [26/May/1983:03:30:45 +0200] \"CONNECT www.example.com:443 HTTP/2.0\" 200 100 \"http://example.com\" " +
		"\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) " +
		"AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33\"\n"
	if log != expected {
		t.Fatalf("wrong log, got %q want %q", log, expected)
	}

	// Request with an unauthorized user
	req.URL.User = url.User("kamil")

	buf.Reset()
	writeCombinedLog(buf, req, *req.URL, ts, http.StatusUnauthorized, 500)
	log = buf.String()

	expected = "192.168.100.5 - kamil [26/May/1983:03:30:45 +0200] \"GET / HTTP/1.1\" 401 500 \"http://example.com\" " +
		"\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) " +
		"AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33\"\n"
	if log != expected {
		t.Fatalf("wrong log, got %q want %q", log, expected)
	}

	// Test with remote ipv6 address
	req.RemoteAddr = "::1"

	buf.Reset()
	writeCombinedLog(buf, req, *req.URL, ts, http.StatusOK, 100)
	log = buf.String()

	expected = "::1 - kamil [26/May/1983:03:30:45 +0200] \"GET / HTTP/1.1\" 200 100 \"http://example.com\" " +
		"\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) " +
		"AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33\"\n"
	if log != expected {
		t.Fatalf("wrong log, got %q want %q", log, expected)
	}

	// Test remote ipv6 addr, with port
	req.RemoteAddr = net.JoinHostPort("::1", "65000")

	buf.Reset()
	writeCombinedLog(buf, req, *req.URL, ts, http.StatusOK, 100)
	log = buf.String()

	expected = "::1 - kamil [26/May/1983:03:30:45 +0200] \"GET / HTTP/1.1\" 200 100 \"http://example.com\" " +
		"\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) " +
		"AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33\"\n"
	if log != expected {
		t.Fatalf("wrong log, got %q want %q", log, expected)
	}
}

func TestWriteCustomLog(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		panic(err)
	}
	ts := time.Date(1983, 05, 26, 3, 30, 45, 0, loc)

	// A typical request with an OK response
	req := newRequest("GET", "http://example.com")
	req.RemoteAddr = "192.168.100.5"
	req.Header.Set("Referer", "http://example.com")
	req.Header.Set("SRCIP", "10.0.0.0")
	req.Header.Set("X-Forwarded-For", "127.0.0.1, 127.0.0.1")
	req.Header.Set(
		"User-Agent",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) AppleWebKit/537.33 "+
			"(KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33",
	)

	buf := new(bytes.Buffer)
	writeCustomLog(buf, req, *req.URL, ts, http.StatusOK, 100, "example.org")
	log := buf.String()

	expected := "192.168.100.5 - - [26/May/1983:03:30:45 +0200] \"GET / HTTP/1.1\" 200 100 example.org example.com 10.0.0.0 127.0.0.1, 127.0.0.1 \"http://example.com\" " +
		"\"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_8_2) " +
		"AppleWebKit/537.33 (KHTML, like Gecko) Chrome/27.0.1430.0 Safari/537.33\"\n"
	if log != expected {
		t.Fatalf("wrong log, got %q want %q", log, expected)
	}
}
