// Modified from
// https://github.com/gorilla/handlers

// Copyright 2013 The Gorilla Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package accesslog

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"unicode/utf8"
	// "log"
)

// combinedLoggingHandler is the http.Handler implementation for LoggingHandlerTo
// and its friends
type combinedLoggingHandler struct {
	writer  io.Writer
	handler http.Handler
}

// customLoggingHandlerHandler is our extended version of combinedLoggingHandler
type customLoggingHandler struct {
	writer  io.Writer
	handler http.Handler
}


func (h combinedLoggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	t := time.Now()
	logger := makeLogger(w)
	url := *req.URL
	h.handler.ServeHTTP(logger, req)
	writeCombinedLog(h.writer, req, url, t, logger.Status(), logger.Size())
}

func (h customLoggingHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	t := time.Now()
	logger := makeLogger(w)
	url := *req.URL
	// log.Println(req.URL.IsAbs(), req.Host, req.URL.String())
	originalHost := req.Host
	h.handler.ServeHTTP(logger, req)
	writeCustomLog(h.writer, req, url, t, logger.Status(), logger.Size(), originalHost)
}

func makeLogger(w http.ResponseWriter) loggingResponseWriter {
	var logger loggingResponseWriter = &responseLogger{w: w}
	if _, ok := w.(http.Hijacker); ok {
		logger = &hijackLogger{responseLogger{w: w}}
	}
	h, ok1 := logger.(http.Hijacker)
	c, ok2 := w.(http.CloseNotifier)
	if ok1 && ok2 {
		return hijackCloseNotifier{logger, h, c}
	}
	if ok2 {
		return &closeNotifyWriter{logger, c}
	}
	return logger
}

type loggingResponseWriter interface {
	http.ResponseWriter
	http.Flusher
	Status() int
	Size() int
}

// responseLogger is wrapper of http.ResponseWriter that keeps track of its HTTP
// status code and body size
type responseLogger struct {
	w      http.ResponseWriter
	status int
	size   int
}

func (l *responseLogger) Header() http.Header {
	return l.w.Header()
}

func (l *responseLogger) Write(b []byte) (int, error) {
	if l.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		l.status = http.StatusOK
	}
	size, err := l.w.Write(b)
	l.size += size
	return size, err
}

func (l *responseLogger) WriteHeader(s int) {
	l.w.WriteHeader(s)
	l.status = s
}

func (l *responseLogger) Status() int {
	return l.status
}

func (l *responseLogger) Size() int {
	return l.size
}

func (l *responseLogger) Flush() {
	f, ok := l.w.(http.Flusher)
	if ok {
		f.Flush()
	}
}

type hijackLogger struct {
	responseLogger
}

func (l *hijackLogger) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h := l.responseLogger.w.(http.Hijacker)
	conn, rw, err := h.Hijack()
	if err == nil && l.responseLogger.status == 0 {
		// The status will be StatusSwitchingProtocols if there was no error and
		// WriteHeader has not been called yet
		l.responseLogger.status = http.StatusSwitchingProtocols
	}
	return conn, rw, err
}

type closeNotifyWriter struct {
	loggingResponseWriter
	http.CloseNotifier
}

type hijackCloseNotifier struct {
	loggingResponseWriter
	http.Hijacker
	http.CloseNotifier
}

const lowerhex = "0123456789abcdef"

func appendQuoted(buf []byte, s string) []byte {
	var runeTmp [utf8.UTFMax]byte
	for width := 0; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1
		if r >= utf8.RuneSelf {
			r, width = utf8.DecodeRuneInString(s)
		}
		if width == 1 && r == utf8.RuneError {
			buf = append(buf, `\x`...)
			buf = append(buf, lowerhex[s[0]>>4])
			buf = append(buf, lowerhex[s[0]&0xF])
			continue
		}
		if r == rune('"') || r == '\\' { // always backslashed
			buf = append(buf, '\\')
			buf = append(buf, byte(r))
			continue
		}
		if strconv.IsPrint(r) {
			n := utf8.EncodeRune(runeTmp[:], r)
			buf = append(buf, runeTmp[:n]...)
			continue
		}
		switch r {
		case '\a':
			buf = append(buf, `\a`...)
		case '\b':
			buf = append(buf, `\b`...)
		case '\f':
			buf = append(buf, `\f`...)
		case '\n':
			buf = append(buf, `\n`...)
		case '\r':
			buf = append(buf, `\r`...)
		case '\t':
			buf = append(buf, `\t`...)
		case '\v':
			buf = append(buf, `\v`...)
		default:
			switch {
			case r < ' ':
				buf = append(buf, `\x`...)
				buf = append(buf, lowerhex[s[0]>>4])
				buf = append(buf, lowerhex[s[0]&0xF])
			case r > utf8.MaxRune:
				r = 0xFFFD
				fallthrough
			case r < 0x10000:
				buf = append(buf, `\u`...)
				for s := 12; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			default:
				buf = append(buf, `\U`...)
				for s := 28; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			}
		}
	}
	return buf

}

// buildCommonLogLine builds a log entry for req in Apache Common Log Format.
// ts is the timestamp with which the entry should be logged.
// status and size are used to provide the response HTTP status and size.
func buildCommonLogLine(req *http.Request, url url.URL, ts time.Time, status int, size int) []byte {
	username := "-"
	if url.User != nil {
		if name := url.User.Username(); name != "" {
			username = name
		}
	}

	host, _, err := net.SplitHostPort(req.RemoteAddr)

	if err != nil {
		host = req.RemoteAddr
	}

	uri := req.RequestURI

	// Requests using the CONNECT method over HTTP/2.0 must use
	// the authority field (aka r.Host) to identify the target.
	// Refer: https://httpwg.github.io/specs/rfc7540.html#CONNECT
	if req.ProtoMajor == 2 && req.Method == "CONNECT" {
		uri = req.Host
	}
	if uri == "" {
		uri = url.RequestURI()
	}

	buf := make([]byte, 0, 3*(len(host)+len(username)+len(req.Method)+len(uri)+len(req.Proto)+50)/2)
	buf = append(buf, host...)
	buf = append(buf, " - "...)
	buf = append(buf, username...)
	buf = append(buf, " ["...)
	buf = append(buf, ts.Format("02/Jan/2006:15:04:05 -0700")...)
	buf = append(buf, `] "`...)
	buf = append(buf, req.Method...)
	buf = append(buf, " "...)
	buf = appendQuoted(buf, uri)
	buf = append(buf, " "...)
	buf = append(buf, req.Proto...)
	buf = append(buf, `" `...)
	buf = append(buf, strconv.Itoa(status)...)
	buf = append(buf, " "...)
	buf = append(buf, strconv.Itoa(size)...)
	return buf
}

// writeLog writes a log entry for req to w in Apache Common Log Format.
// ts is the timestamp with which the entry should be logged.
// status and size are used to provide the response HTTP status and size.
// func writeLog(w io.Writer, req *http.Request, url url.URL, ts time.Time, status, size int) {
// 	buf := buildCommonLogLine(req, url, ts, status, size)
// 	buf = append(buf, '\n')
// 	w.Write(buf)
// }

// writeCombinedLog writes a log entry for req to w in Apache Combined Log Format.
// ts is the timestamp with which the entry should be logged.
// status and size are used to provide the response HTTP status and size.
func writeCombinedLog(w io.Writer, req *http.Request, url url.URL, ts time.Time, status, size int) {
	buf := buildCommonLogLine(req, url, ts, status, size)
	buf = append(buf, ` "`...)
	buf = appendQuoted(buf, req.Referer())
	buf = append(buf, `" "`...)
	buf = appendQuoted(buf, req.UserAgent())
	buf = append(buf, '"', '\n')
	w.Write(buf)
}

// CombinedLoggingHandler return a http.Handler that wraps h and logs requests to out in
// Apache Combined Log Format.
//
// See http://httpd.apache.org/docs/2.2/logs.html#combined for a description of this format.
//
// LoggingHandler always sets the ident field of the log to -
func CombinedLoggingHandler(out io.Writer, h http.Handler) http.Handler {
	return combinedLoggingHandler{out, h}
}



// writeCustomLog builds upon writeCombinedLog and adds the following fields
// - original hostname of request
// - new proxied hostname
// - SRCIP (added to request by a custom front end balancer)
// - X-Forwarded-For (added to request by other front end balancer)
func writeCustomLog(w io.Writer, req *http.Request, url url.URL, ts time.Time, status, size int, originalHost string) {
	buf := buildCommonLogLine(req, url, ts, status, size)
	
	buf = append(buf, ` `...)
	if originalHost != "" {
		buf = appendQuoted(buf, originalHost)	
	} else {
		buf = append(buf, `-`...)
	}
	
	buf = append(buf, ` `...)
	if req.URL.Host != "" {
		buf = appendQuoted(buf, req.URL.Host)
	} else {
		buf = append(buf, `-`...)
	}
	
	buf = append(buf, ` `...)
	if req.Header.Get("SRCIP") != "" {
		buf = appendQuoted(buf, req.Header.Get("SRCIP"))
	} else {
		buf = append(buf, `-`...)
	}

	buf = append(buf, ` `...)
	if req.Header.Get("X-Forwarded-For") != "" {
		buf = appendQuoted(buf, req.Header.Get("X-Forwarded-For"))
	} else {
		buf = append(buf, `-`...)
	}

	buf = append(buf, ` "`...)
	buf = appendQuoted(buf, req.Referer())
	buf = append(buf, `" "`...)
	buf = appendQuoted(buf, req.UserAgent())
	buf = append(buf, '"', '\n')
	w.Write(buf)
}

func CustomLoggingHandler(out io.Writer, h http.Handler) http.Handler {
	return customLoggingHandler{out, h}
}
