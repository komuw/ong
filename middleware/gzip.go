package middleware

import (
	"compress/gzip"
	"fmt"
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

	defaultLevel = bestSpeed // TODO: should this be the default?

	// TODO: copy/add docs.
	// TODO: check other impls for other sizes that they use.
	defaultMinSize = 150
)

// Gzip is a middleware that transparently gzips the response body, for clients which support.
func Gzip(wrappedHandler http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		w.Header().Add(varyHeader, acHeader)

		if !shouldGzip(r) {
			wrappedHandler(w, r)
			return
		}

		gw := &gzipRW{
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

// gzipRW provides an http.ResponseWriter interface,
// which gzips bytes before writing them to the underlying
// response. This doesn't close the writers, so don't forget
// to do that. It can be configured to skip response smaller
// than minSize.
type gzipRW struct {
	http.ResponseWriter

	// h *handler

	gw *gzip.Writer

	// Holds the first part of the write before reaching
	// the minSize or the end of the write.
	buf *[]byte

	// Saves the WriteHeader value.
	code int
}

// WriteHeader just saves the response code until close or
// GZIP effective writes.
func (w *gzipRW) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}

// Write appends data to the gzip writer.
func (w *gzipRW) Write(b []byte) (int, error) {
	switch {
	case w.buf != nil && w.gw != nil:
		panic("gziphandler: both buf and gw are non nil in call to Write")
	// GZIP responseWriter is initialized. Use the GZIP
	// responseWriter.
	case w.gw != nil:
		return w.gw.Write(b)
	// We're operating in pass through mode.
	case w.buf == nil:
		return w.ResponseWriter.Write(b)
	}

	w.WriteHeader(http.StatusOK)

	// This may succeed if the Content-Type header was
	// explicitly set.
	if w.shouldPassThrough() {
		if err := w.startPassThrough(); err != nil {
			return 0, err
		}

		return w.ResponseWriter.Write(b)
	}

	if w.shouldBuffer(b) {
		// Save the write into a buffer for later.
		// This buffer will be flushed in either
		// startGzip or startPassThrough.
		*w.buf = append(*w.buf, b...)
		return len(b), nil
	}

	// w.inferContentType(b) // TODO: maybe do this in future

	// Now that we've called inferContentType, we have
	// a Content-Type header.
	if w.shouldPassThrough() {
		if err := w.startPassThrough(); err != nil {
			return 0, err
		}

		return w.ResponseWriter.Write(b)
	}

	if err := w.startGzip(); err != nil {
		return 0, err
	}

	return w.gw.Write(b)
}

// startGzip initialize any GZIP specific informations.
func (w *gzipRW) startGzip() (err error) {
	h := w.Header()

	// Set the GZIP header.
	h.Set("Content-Encoding", "gzip")

	// if the Content-Length is already set, then calls
	// to Write on gzip will fail to set the
	// Content-Length header since its already set
	// See: https://github.com/golang/go/issues/14975.
	h.Del("Content-Length")

	// Write the header to gzip response.
	w.ResponseWriter.WriteHeader(w.code)

	// Bytes written during ServeHTTP are redirected to
	// this gzip writer before being written to the
	// underlying response.
	w.gw = gzipWriterGet(w.ResponseWriter, defaultLevel)

	if buf := *w.buf; len(buf) != 0 {
		// Flush the buffer into the gzip response.
		_, err = w.gw.Write(buf)
	}

	w.releaseBuffer() // TODO: this should be `defer w.releaseBuffer()` ??
	return err
}

// Close will close the gzip.Writer and will put it back in
// the gzipWriterPool.
func (w *gzipRW) Close() error {
	switch {
	case w.buf != nil && w.gw != nil:
		panic("gziphandler: both buf and gw are non nil in call to Close")
	// Buffer not nil means the regular response must
	// be returned.
	case w.buf != nil:
		fmt.Println("nonZipped")
		return w.closeNonGzipped()
	// If the GZIP responseWriter is not set no need
	// to close it.
	case w.gw != nil:
		fmt.Println("Zipped")
		return w.closeGzipped()
	// Both buf and gw nil means we are operating in
	// pass through mode.
	default:
		fmt.Println("passThrough")
		return nil
	}
}

func (w *gzipRW) closeGzipped() error {
	err := w.gw.Close()

	gzipWriterPut(w.gw, defaultLevel)
	w.gw = nil

	return err
}

func (w *gzipRW) closeNonGzipped() error {
	// w.inferContentType(nil) // TODO: maybe do this in future

	w.WriteHeader(http.StatusOK)

	return w.startPassThrough()
}

func (w *gzipRW) shouldPassThrough() bool {
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

func (w *gzipRW) startPassThrough() (err error) {
	w.ResponseWriter.WriteHeader(w.code)

	if buf := *w.buf; len(buf) != 0 {
		_, err = w.ResponseWriter.Write(buf)
	}

	w.releaseBuffer() // TODO: this should be `defer w.releaseBuffer()` ??
	return err
}

func (w *gzipRW) releaseBuffer() {
	if w.buf == nil {
		panic("gziphandler: w.buf is nil in call to emptyBuffer")
	}

	*w.buf = (*w.buf)[:0]
	bufferPool.Put(w.buf)
	w.buf = nil
}

func (w *gzipRW) shouldBuffer(b []byte) bool {
	// If the all writes to date are bigger than the
	// minSize, we no longer need to buffer and we can
	// decide whether to enable compression or whether
	// to operate in pass through mode.
	return len(*w.buf)+len(b) < defaultMinSize
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
