package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/tmthrgd/gziphandler whose license(Apache License, Version 2.0) can be found here: https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/LICENSE.md

const (
	acHeader           = "Accept-Encoding"
	noCompression      = gzip.NoCompression
	bestSpeed          = gzip.BestSpeed
	bestCompression    = gzip.BestCompression
	defaultCompression = gzip.DefaultCompression
	huffmanOnly        = gzip.HuffmanOnly

	defaultLevel = bestSpeed
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
// TODO: check other impls for other sizes that they use.
const defaultMinSize = 150

var bufferPool = &sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 0, defaultMinSize)
		return &buf
	},
}

var gzipWriterPools [gzip.BestCompression - gzip.HuffmanOnly + 1]sync.Pool

func gzipWriterPool(level int) *sync.Pool {
	return &gzipWriterPools[level-gzip.HuffmanOnly]
}

func gzipWriterGet(w io.Writer, level int) *gzip.Writer {
	if gw, ok := gzipWriterPool(level).Get().(*gzip.Writer); ok {
		gw.Reset(w)
		return gw
	}

	gw, _ := gzip.NewWriterLevel(w, level)
	return gw
}

func gzipWriterPut(gw *gzip.Writer, level int) {
	gzipWriterPool(level).Put(gw)
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

// Close will close the gzip.Writer and will put it back in
// the gzipWriterPool.
func (w *responseWriter) Close() error {
	switch {
	case w.buf != nil && w.gw != nil:
		panic("gziphandler: both buf and gw are non nil in call to Close")
	// Buffer not nil means the regular response must
	// be returned.
	case w.buf != nil:
		return w.closeNonGzipped()
	// If the GZIP responseWriter is not set no need
	// to close it.
	case w.gw != nil:
		return w.closeGzipped()
	// Both buf and gw nil means we are operating in
	// pass through mode.
	default:
		return nil
	}
}

func (w *responseWriter) closeGzipped() error {
	err := w.gw.Close()

	gzipWriterPut(w.gw, defaultLevel)
	w.gw = nil

	return err
}

func (w *responseWriter) closeNonGzipped() error {
	// w.inferContentType(nil) // TODO: maybe do this in future

	w.WriteHeader(http.StatusOK)

	return w.startPassThrough()
}

func (w *responseWriter) shouldPassThrough() bool {
	if w.Header().Get("Content-Encoding") != "" {
		return true
	}

	/*
		TODO: maybe implement `handleContentType`
		It allows people to specify(thro config) content-types that they want to handle.
		!w.handleContentType()
	*/
	return true
}

func (w *responseWriter) startPassThrough() (err error) {
	w.ResponseWriter.WriteHeader(w.code)

	if buf := *w.buf; len(buf) != 0 {
		_, err = w.ResponseWriter.Write(buf)
	}

	w.releaseBuffer()
	return err
}

func (w *responseWriter) releaseBuffer() {
	if w.buf == nil {
		panic("gziphandler: w.buf is nil in call to emptyBuffer")
	}

	*w.buf = (*w.buf)[:0]
	bufferPool.Put(w.buf)
	w.buf = nil
}

func shouldGzip(r *http.Request) bool {
	// Examples of the `acHeader` are:
	//   Accept-Encoding: gzip
	//   Accept-Encoding: gzip, compress, br
	//   Accept-Encoding: br;q=1.0, gzip;q=0.8, *;q=0.1

	// This is a truly bad way to do this. We should do better.
	// see: https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/gzip.go#L364
	//      https://github.com/nytimes/gziphandler/issues/65

	if strings.Contains(r.Header.Get(acHeader), "gzip") {
		return true
	}
	return false
}
