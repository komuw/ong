package middleware

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/tmthrgd/gziphandler whose license(Apache License, Version 2.0) can be found here:                   https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/LICENSE.md
//   (b) https://github.com/klauspost/compress/tree/master/gzhttp whose license(Apache License, Version 2.0) can be found here: https://github.com/klauspost/compress/blob/4bc73d36928c39bbd7cf823171081d14c884edde/gzhttp/LICENSE
//   (c) https://github.com/CAFxX/httpcompression whose license(Apache License, Version 2.0) can be found here:                 https://github.com/CAFxX/httpcompression/blob/9d30d0704fe304b4586ae1585a54ee6eec47675f/LICENSE

const (
	// defaultMinSize is the default minimum size for which we enable gzip compression.
	// - compressing very small payloads may actually increase their size.
	// - compressing small payloads may actually decrease end-to-end performance.
	//
	// nginx recommends 20 bytes; apache/mod_gzip, 500 bytes; apache/pagespeed, 0 bytes.
	// In the past, google recommended 150 bytes and akamai 860 bytes, but both of these recommendations seem to have disappeared from their current documentation.
	// klauspost/compress recommends 1024; based on the fact that the MTU size is 1500 bytes;
	//  (even if you compress something from 1300bytes to 800bytes, it still gets transmitted in 1500bytes MTU; so u have done zero work.)
	defaultMinSize = 150

	acceptEncodingHeader   = "Accept-Encoding"
	contentEncodingHeader  = "Content-Encoding"
	contentRangeHeader     = "Content-Range"
	acceptRangesHeader     = "Accept-Ranges"
	contentTypeHeader      = "Content-Type"
	contentLengthHeader    = "Content-Length"
	rangeHeader            = "Range"
	thisMiddlewareEncoding = "gzip"
)

// Gzip is a middleware that transparently gzips the response body, for clients which support.
func Gzip(wrappedHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(varyHeader, acceptEncodingHeader)

		if !shouldGzipReq(r) {
			wrappedHandler(w, r)
			return
		}

		gzipWriter, _ := gzip.NewWriterLevel(w, gzip.BestSpeed)
		grw := &gzipRW{
			ResponseWriter: w,
			// Bytes written during ServeHTTP are redirected to this gzip writer
			// before being written to the underlying response.
			gw:      gzipWriter,
			minSize: defaultMinSize,
		}
		defer func() { _ = grw.Close() }() // errcheck made me do this.

		// We do not handle range requests when compression is used, as the
		// range specified applies to the compressed data, not to the uncompressed one.
		// see: https://github.com/nytimes/gziphandler/issues/83
		r.Header.Del(rangeHeader)

		// todo: we could detect if `w` is a `http.CloseNotifier` and do something special here.
		// see: https://github.com/klauspost/compress/blob/4a97174a615ed745c450077edf0e1f7e97aabd58/gzhttp/compress.go#L383-L385
		// However `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)

		wrappedHandler(grw, r)
	}
}

// gzipRW provides an http.ResponseWriter interface, which gzips
// bytes before writing them to the underlying response. This doesn't close the
// writers, so don't forget to do that.
// It can be configured to skip response smaller than minSize.
type gzipRW struct {
	http.ResponseWriter
	gw *gzip.Writer

	code int // Saves the WriteHeader value.

	minSize int    // Specifies the minimum response size to gzip. If the response length is bigger than this value, it is compressed.
	buf     []byte // Holds the first part of the write before reaching the minSize or the end of the write.

	handledZip bool // whether this has yet to handle a zipped response.
}

var (
	// make sure we support http optional interfaces.
	// https://github.com/komuw/ong/issues/15
	// https://blog.merovius.de/2017/07/30/the-trouble-with-optional-interfaces.html
	_ http.ResponseWriter = &gzipRW{}
	_ http.Flusher        = &gzipRW{}
	_ http.Hijacker       = &gzipRW{}
	_ io.WriteCloser      = &gzipRW{}
	// _ http.CloseNotifier  = &gzipRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)

// Write appends data to the gzip writer.
func (grw *gzipRW) Write(b []byte) (int, error) {
	// todo: we have the ability to re-use the grw.gw if it already exists.
	// see: https://github.com/klauspost/compress/blob/4a97174a615ed745c450077edf0e1f7e97aabd58/gzhttp/compress.go#L81-L84 for implementation.

	// Save the write into a buffer for later use in GZIP responseWriter (if content is long enough) or at close with regular responseWriter.
	// On the first write, w.buf changes from nil to a valid slice
	grw.buf = append(grw.buf, b...)

	nonGzipped := func() (int, error) {
		if err := grw.handleNonGzipped(); err != nil {
			return 0, err
		}
		return len(b), nil
	}

	// Only continue if they didn't already choose an encoding .
	if grw.Header().Get(contentEncodingHeader) != "" &&
		grw.Header().Get(contentEncodingHeader) != thisMiddlewareEncoding ||
		grw.Header().Get(contentRangeHeader) != "" {
		return nonGzipped()
	}

	cl := 0
	if clStr := grw.Header().Get(contentLengthHeader); clStr != "" {
		cl, _ = strconv.Atoi(clStr)
	}
	if cl < grw.minSize && cl > 0 {
		// if content-length == 0, it means that the header was not set.
		// for those, we actually want to call `handleGzipped`; so we exempt them from this branch.
		return nonGzipped()
	}

	ct := ""
	if ct = grw.Header().Get(contentTypeHeader); ct == "" {
		// If a Content-Type wasn't specified, infer it from the current buffer.
		ct = http.DetectContentType(grw.buf)
	}
	if !shouldGzipCt(ct) {
		return nonGzipped()
	}

	// If the current buffer is less than minSize, then wait until we have more data.
	if len(grw.buf) < grw.minSize {
		return len(b), nil
	}

	// The current buffer is larger than minSize, continue.
	//
	// Set the header only if the key does not exist. There are some cases where a nil content-type is set intentionally(eg some http/fs)
	if _, ok := grw.Header()[contentTypeHeader]; !ok && ct != "" {
		grw.Header().Set(contentTypeHeader, ct)
	}

	// gzip response.
	if err := grw.handleGzipped(); err != nil {
		return 0, err
	}
	return len(b), nil
}

// handleNonGzipped writes to the underlying ResponseWriter without gzip.
func (grw *gzipRW) handleNonGzipped() error {
	grw.handledZip = false
	// We need to do it even in this case because the Gzip handler has already stripped the range header anyway.
	grw.Header().Del(acceptRangesHeader)

	if grw.code != 0 {
		grw.ResponseWriter.WriteHeader(grw.code)
		// Ensure that no other WriteHeader's happen
		grw.code = 0
	}

	// If Write was never called then don't call Write on the underlying ResponseWriter.
	if len(grw.buf) == 0 {
		return nil
	}
	_, err := grw.ResponseWriter.Write(grw.buf)

	grw.buf = grw.buf[:0]
	return err
}

// handleGzipped initializes a GZIP writer and writes the buffer.
func (grw *gzipRW) handleGzipped() error {
	grw.handledZip = true

	// Set the GZIP header.
	grw.Header().Set(contentEncodingHeader, thisMiddlewareEncoding)

	// if the Content-Length is already set, then calls to Write on gzip
	// will fail to set the Content-Length header since its already set
	// See: https://github.com/golang/go/issues/14975.
	grw.Header().Del(contentLengthHeader)

	// Delete Accept-Ranges.
	// see: https://github.com/nytimes/gziphandler/issues/83
	grw.Header().Del(acceptRangesHeader)

	// Write the header to gzip response.
	if grw.code != 0 {
		grw.ResponseWriter.WriteHeader(grw.code)
		// Ensure that no other WriteHeader's happen
		grw.code = 0
	}

	// Flush the buffer into the gzip response if there are any bytes.
	// If there aren't any, we shouldn't initialize it yet because on Close it will
	// write the gzip header even if nothing was ever written.
	if len(grw.buf) > 0 {
		_, err := grw.gw.Write(grw.buf)
		grw.buf = grw.buf[:0]
		return err
	}
	return nil
}

// WriteHeader just saves the response code until close or GZIP effective writes.
func (grw *gzipRW) WriteHeader(statusCode int) {
	if grw.code == 0 {
		grw.code = statusCode
	}
}

// Close will close the gzip.Writer.
func (grw *gzipRW) Close() error {
	if !grw.handledZip {
		// GZIP not triggered yet, write out regular response.
		return grw.handleNonGzipped()
	}

	return grw.gw.Close()
}

// Flush flushes the underlying *gzip.Writer and then the
// underlying http.ResponseWriter if it is an http.Flusher.
// This makes gzipRW an http.Flusher.
func (grw *gzipRW) Flush() {
	if grw.gw == nil && grw.buf != nil {
		// Fix for NYTimes/gziphandler#58:
		//  Only flush once startGzip or
		//  startPassThrough has been called.
		//
		// Flush is thus a no-op until the written
		// body exceeds minSize, or we've decided
		// not to compress.
		return
	}

	if grw.gw != nil {
		_ = grw.gw.Flush()
	}

	if fw, ok := grw.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}

// Hijack implements http.Hijacker. If the underlying ResponseWriter is a
// Hijacker, its Hijack method is returned. Otherwise an error is returned.
func (grw *gzipRW) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := grw.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("http.Hijacker interface is not supported")
}

// shouldGzipReq checks whether the request is eligible to be gzipped.
func shouldGzipReq(r *http.Request) bool {
	// Examples of the `acceptEncodingHeader` are:
	//   Accept-Encoding: gzip
	//   Accept-Encoding: gzip, compress, br
	//   Accept-Encoding: br;q=1.0, gzip;q=0.8, *;q=0.1

	// This is a truly bad way to do this. We should do better.
	// see: https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/gzip.go#L364
	//      https://github.com/nytimes/gziphandler/issues/65

	// Note that we don't request this for HEAD requests,
	// due to a bug in nginx:
	//   https://trac.nginx.org/nginx/ticket/358
	//   https://golang.org/issue/5522

	if r.Method == http.MethodHead {
		return false
	}

	if strings.Contains(r.Header.Get(acceptEncodingHeader), thisMiddlewareEncoding) {
		return true
	}

	return false
}

// shouldGzipCt checks whether the supplied content-type is eligible to be gzipped.
// It excludes common compressed audio, video and archive formats.
func shouldGzipCt(ct string) bool {
	// Don't compress any audio/video types.
	excludePrefixDefault := []string{"video/", "audio/", "image/jp"}

	// Skip a bunch of compressed types that contains this string.
	// Curated by supposedly still active formats on https://en.wikipedia.org/wiki/List_of_archive_formats
	excludeContainsDefault := []string{"compress", "zip", "snappy", "lzma", "xz", "zstd", "brotli", "stuffit"}

	ct = strings.TrimSpace(strings.ToLower(ct))
	if ct == "" {
		return true
	}
	for _, s := range excludeContainsDefault {
		if strings.Contains(ct, s) {
			return false
		}
	}

	for _, prefix := range excludePrefixDefault {
		if strings.HasPrefix(ct, prefix) {
			return false
		}
	}
	return true
}
