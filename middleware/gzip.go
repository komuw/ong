package middleware

import (
	"bufio"
	stdGzip "compress/gzip"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
)

// Some of the code here is inspired by(or taken from):
//   (a) https://github.com/tmthrgd/gziphandler whose license(Apache License, Version 2.0) can be found here:                   https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/LICENSE.md
//   (b) https://github.com/klauspost/compress/tree/master/gzhttp whose license(Apache License, Version 2.0) can be found here: https://github.com/klauspost/compress/blob/4bc73d36928c39bbd7cf823171081d14c884edde/gzhttp/LICENSE
//   (c) https://github.com/CAFxX/httpcompression whose license(Apache License, Version 2.0) can be found here:                 https://github.com/CAFxX/httpcompression/blob/9d30d0704fe304b4586ae1585a54ee6eec47675f/LICENSE

const (
	// This middleware unlike others does not have a minimum size below which it does not compress.
	// It compresses all sizes.
	acceptEncodingHeader   = "Accept-Encoding"
	contentEncodingHeader  = "Content-Encoding"
	contentRangeHeader     = "Content-Range"
	acceptRangesHeader     = "Accept-Ranges"
	contentTypeHeader      = "Content-Type"
	contentLengthHeader    = "Content-Length"
	rangeHeader            = "Range"
	thisMiddlewareEncoding = "gzip"
)

// gzip is a middleware that transparently gzips the http response body, for clients that support it.
func gzip(wrappedHandler http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(varyHeader, acceptEncodingHeader)

		if !shouldGzipReq(r) {
			wrappedHandler.ServeHTTP(w, r)
			return
		}

		gzipWriter, _ := stdGzip.NewWriterLevel(w, stdGzip.BestSpeed)
		grw := &gzipRW{
			ResponseWriter: w,
			// Bytes written during ServeHTTP are redirected to this gzip writer
			// before being written to the underlying response.
			gw: gzipWriter,
		}
		defer func() { _ = grw.Close() }() // errcheck made me do this.

		// We do not handle range requests when compression is used, as the
		// range specified applies to the compressed data, not to the uncompressed one.
		// see: https://github.com/nytimes/gziphandler/issues/83
		r.Header.Del(rangeHeader)

		// todo: we could detect if `w` is a `http.CloseNotifier` and do something special here.
		// see: https://github.com/klauspost/compress/blob/4a97174a615ed745c450077edf0e1f7e97aabd58/gzhttp/compress.go#L383-L385
		// However `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)

		wrappedHandler.ServeHTTP(grw, r)
	}
}

// gzipRW provides an http.ResponseWriter interface, which gzips
// bytes before writing them to the underlying response. This doesn't close the
// writers, so don't forget to do that.
type gzipRW struct {
	http.ResponseWriter
	gw *stdGzip.Writer

	// code is the status code set by WriteHeader. It is sent to the underlying response
	// only after the decision to gzip(or not) has been made, since that decision changes the headers.
	code int
	// decided reports whether the decision to gzip(or not) has been made.
	// The decision is made only once per response.
	decided bool
	// zip reports whether the response is gzipped. It is only valid if decided is true.
	zip bool
}

var (
	// make sure we support http optional interfaces.
	// https://github.com/komuw/ong/issues/15
	// https://blog.merovius.de/2017/07/30/the-trouble-with-optional-interfaces.html
	_ http.ResponseWriter = &gzipRW{}
	_ http.Flusher        = &gzipRW{}
	_ http.Hijacker       = &gzipRW{}
	_ http.Pusher         = &gzipRW{}
	_ io.WriteCloser      = &gzipRW{}
	_ io.ReaderFrom       = &gzipRW{}
	_ httpRespCtrler      = &gzipRW{}
	// _ http.CloseNotifier  = &gzipRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)

// Write writes b to the gzip writer or to the underlying response.
func (grw *gzipRW) Write(b []byte) (int, error) {
	// todo: we have the ability to re-use the grw.gw if it already exists.
	// see: https://github.com/klauspost/compress/blob/4a97174a615ed745c450077edf0e1f7e97aabd58/gzhttp/compress.go#L81-L84 for implementation.

	// todo: In future, we could buffer small writes and only gzip if the response is big enough.
	// According to our benchmarks, any value of 256/512/1024 bytes would be ideal.

	if !grw.decided {
		grw.decide(b, true)
	}
	if grw.zip {
		return grw.gw.Write(b)
	}
	return grw.ResponseWriter.Write(b)
}

// WriteHeader stores the status code until the decision to gzip(or not) has been made.
func (grw *gzipRW) WriteHeader(statusCode int) {
	if grw.decided || isInformational(statusCode) {
		grw.ResponseWriter.WriteHeader(statusCode)
		return
	}
	if grw.code != 0 {
		// Same as net/http; only the first call takes effect.
		return
	}

	grw.code = statusCode
	if !bodyAllowedForStatus(statusCode) {
		grw.decide(nil, false)
	}
}

// decide decides whether to gzip the response, sets the headers accordingly and then sends the status code.
// The b argument is used to detect the content-type if it has not been set.
func (grw *gzipRW) decide(b []byte, canZip bool) {
	grw.decided = true
	h := grw.Header()

	// We need to do it even if we do not gzip, because the Gzip handler has already stripped the range header anyway.
	h.Del(acceptRangesHeader)

	ct := h.Get(contentTypeHeader)
	if ct == "" && len(b) > 0 {
		// If a Content-Type wasn't specified, infer it from the current data.
		ct = http.DetectContentType(b)
	}

	grw.zip = canZip &&
		ct != "" &&
		shouldGzipCt(ct) &&
		(grw.code == 0 || bodyAllowedForStatus(grw.code)) &&
		// Only gzip if they didn't already choose an encoding.
		h.Get(contentEncodingHeader) == "" &&
		h.Get(contentRangeHeader) == ""

	if grw.zip {
		// Set the header only if the key does not exist.
		// There are some cases where a nil content-type is set intentionally(eg some http/fs)
		if _, ok := h[contentTypeHeader]; !ok {
			h.Set(contentTypeHeader, ct)
		}

		h.Set(contentEncodingHeader, thisMiddlewareEncoding)

		// if the Content-Length is already set, then calls to Write on gzip
		// will fail to set the Content-Length header since its already set
		// See: https://github.com/golang/go/issues/14975.
		h.Del(contentLengthHeader)
	}

	if grw.code != 0 {
		grw.ResponseWriter.WriteHeader(grw.code)
	}
}

// Close will close the stdGzip.Writer.
func (grw *gzipRW) Close() error {
	if !grw.decided {
		// Nothing was written. Do not gzip an empty body.
		grw.decide(nil, false)
	}
	if !grw.zip {
		return nil
	}

	return grw.gw.Close() // will also call gzip flush()
}

// Flush flushes the underlying *stdGzip.Writer and then the
// underlying http.ResponseWriter if it is an http.Flusher.
// This makes gzipRW an http.Flusher.
func (grw *gzipRW) Flush() {
	if !grw.decided {
		grw.decide(nil, true)
	}
	if grw.zip {
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
	return nil, nil, fmt.Errorf("ong/middleware/gzip: http.Hijacker interface is not supported")
}

// ReadFrom implements io.ReaderFrom
// It is necessary for the sendfile syscall
// https://github.com/caddyserver/caddy/pull/5022
// https://github.com/caddyserver/caddy/blob/v2.7.4/modules/caddyhttp/responsewriter.go#L45-L49
func (grw *gzipRW) ReadFrom(src io.Reader) (n int64, err error) {
	if grw.decided && !grw.zip {
		return io.Copy(grw.ResponseWriter, src)
	}
	// Hide the ReadFrom method of grw so that io.Copy does not call it recursively.
	return io.Copy(struct{ io.Writer }{grw}, src)
}

// Push implements http.Pusher
func (grw *gzipRW) Push(target string, opts *http.PushOptions) error {
	if p, ok := grw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return fmt.Errorf("ong/middleware/gzip: http.Pusher interface is not supported")
}

// Unwrap implements http.ResponseController.
// It returns the underlying ResponseWriter,
// which is necessary for http.ResponseController to work correctly.
func (grw *gzipRW) Unwrap() http.ResponseWriter {
	return grw.ResponseWriter
}

// bodyAllowedForStatus reports whether a given response status code permits a body.
// See RFC 7230, section 3.3.
func bodyAllowedForStatus(statusCode int) bool {
	switch {
	case statusCode >= 100 && statusCode <= 199:
		return false
	case statusCode == http.StatusNoContent:
		return false
	case statusCode == http.StatusNotModified:
		return false
	}
	return true
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
	// Don't compress any audio/video types, or images that are already compressed.
	excludePrefixDefault := []string{"video/", "audio/", "image/"}

	// Skip a bunch of compressed types that contains this string.
	// Curated by supposedly still active formats on https://en.wikipedia.org/wiki/List_of_archive_formats
	excludeContainsDefault := []string{"compress", "zip", "snappy", "lzma", "xz", "zstd", "brotli", "stuffit"}

	ct = strings.TrimSpace(strings.ToLower(ct))
	if ct == "" {
		return true
	}
	if strings.HasPrefix(ct, "image/svg") {
		// svg is text.
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
