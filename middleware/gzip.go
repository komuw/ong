package middleware

import (
	"compress/gzip"
	"net/http"
	"os"
	"strings"
	"sync"
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
			// h:              h,
			buf: bufferPool.Get().(*[]byte),
		}
		defer func() {
			err := gw.Close()
			if err != nil && os.Getenv("GOWEB_RUNNING_IN_TESTS") != "" {
				panic(err)
			}
		}()

		var modifiedRw http.ResponseWriter = gw

		_, cok := w.(http.CloseNotifier)
		_, hok := w.(http.Hijacker)
		_, pok := w.(http.Pusher)

		_ = cok
		_ = hok
		_ = pok

		// TODO: implement this
		// switch {
		// case cok && hok:
		// 	modifiedRw = closeNotifyHijackResponseWriter{gw}
		// case cok && pok:
		// 	modifiedRw = closeNotifyPusherResponseWriter{gw}
		// case cok:
		// 	modifiedRw = closeNotifyResponseWriter{gw}
		// case hok:
		// 	modifiedRw = hijackResponseWriter{gw}
		// case pok:
		// 	modifiedRw = pusherResponseWriter{gw}
		// }

		wrappedHandler(modifiedRw, r)
	}
}

// TODO: copy/add docs.
const defaultMinSize = 150

var bufferPool = &sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, defaultMinSize)
		return &buf
	},
}

// responseWriter provides an http.ResponseWriter interface,
// which gzips bytes before writing them to the underlying
// response. This doesn't close the writers, so don't forget
// to do that. It can be configured to skip response smaller
// than minSize.
type responseWriter struct {
	http.ResponseWriter

	// h *handler

	gw *gzip.Writer

	// Holds the first part of the write before reaching
	// the minSize or the end of the write.
	buf *[]byte

	// Saves the WriteHeader value.
	code int
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
