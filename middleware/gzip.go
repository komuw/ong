package middleware

import (
	"net/http"
	"os"
	"strings"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/tmthrgd/gziphandler whose license(Apache License, Version 2.0) can be found here: https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/LICENSE.md

const (
	acHeader = "Accept-Encoding"
)

// Gzip is a middleware that transparently gzips the response body, for clients which support.
func Gzip(wrappedHandler http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Add(varyHeader, acHeader)

		if !shouldGzip(r) {
			wrappedHandler(w, r)
			return
		}

		gw := &responseWriter{
			ResponseWriter: w,
			h:              h,
			buf:            bufferPool.Get().(*[]byte),
		}
		defer func() {
			err := gw.Close()
			if err != nil && os.Getenv("GOWEB_RUNNING_IN_TESTS") != "" {
				panic(err)
			}
		}()

		var modifiedRw http.ResponseWriter = gw

		wrappedHandler(modifiedRw, r)
	}
}

func shouldGzip(r *http.Request) bool {
	// Examples of the `acHeader` are:
	//   Accept-Encoding: gzip
	//   Accept-Encoding: gzip, compress, br
	//   Accept-Encoding: br;q=1.0, gzip;q=0.8, *;q=0.1

	// This is a truly bad way to do this. We should do better.
	// see: https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/gzip.go#L364
	//      https://github.com/nytimes/gziphandler/issues/65
	val := r.Header.Get(acHeader)
	if strings.Contains(val, "gzip") {
		return true
	}
	return false
}
