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

	buf []byte // Holds the first part of the write before reaching the minSize or the end of the write.

	handledZip bool // whether this has yet to handle a zipped response.
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
	_ httpRespCtrler      = &logRW{}
	// _ http.CloseNotifier  = &gzipRW{} // `http.CloseNotifier` has been deprecated sinc Go v1.11(year 2018)
)

// Write appends data to the gzip writer.
func (grw *gzipRW) Write(b []byte) (int, error) {
	// todo: we have the ability to re-use the grw.gw if it already exists.
	// see: https://github.com/klauspost/compress/blob/4a97174a615ed745c450077edf0e1f7e97aabd58/gzhttp/compress.go#L81-L84 for implementation.

	// Save the write into a buffer for later use in GZIP responseWriter (if content is long enough) or at close with regular responseWriter.
	// On the first write, w.buf changes from nil to a valid slice
	grw.buf = append(grw.buf, b...)

	ct := ""
	{
		// Only continue if they didn't already choose an encoding.
		var shouldNotUseGzip bool = !grw.handledZip && grw.Header().Get(contentEncodingHeader) != "" || grw.Header().Get(contentRangeHeader) != ""
		// this is expensive, so we should probably just check shouldNotUseGzip and return
		if ct = grw.Header().Get(contentTypeHeader); ct == "" {
			// If a Content-Type wasn't specified, infer it from the current buffer.
			ct = http.DetectContentType(grw.buf)
		}
		shouldNotUseGzip = !shouldGzipCt(ct) || shouldNotUseGzip
		if shouldNotUseGzip {
			return grw.handleNonGzipped(len(b))
		}
	}

	{
		// todo: enable this in future.
		// According to our benchmarks, any value of 256/512/1024 bytes would be ideal.
		// if len(grw.buf) < 256 {
		// 	// don't call underlying gzip writer if the buffer is not big enough;
		// 	// wait for it to have enough.
		// 	return len(b), nil
		// }

		// gzip response.
		return grw.handleGzipped(ct, len(b))
	}
}

// handleNonGzipped writes to the underlying ResponseWriter without gzip.
func (grw *gzipRW) handleNonGzipped(lenB int) (int, error) {
	grw.handledZip = false
	// We need to do it even in this case because the Gzip handler has already stripped the range header anyway.
	grw.Header().Del(acceptRangesHeader)

	// If Write was never called then don't call Write on the underlying ResponseWriter.
	if len(grw.buf) == 0 {
		return lenB, nil
	}
	_, err := grw.ResponseWriter.Write(grw.buf)
	grw.buf = grw.buf[:0]
	if err != nil {
		return 0, err
	}
	return lenB, nil
}

// handleGzipped initializes a GZIP writer and writes the buffer.
func (grw *gzipRW) handleGzipped(ct string, lenB int) (int, error) {
	grw.handledZip = true

	// Set the header only if the key does not exist.
	// There are some cases where a nil content-type is set intentionally(eg some http/fs)
	if _, ok := grw.Header()[contentTypeHeader]; !ok && ct != "" {
		grw.Header().Set(contentTypeHeader, ct)
	}

	// Set the GZIP header.
	grw.Header().Set(contentEncodingHeader, thisMiddlewareEncoding)

	// if the Content-Length is already set, then calls to Write on gzip
	// will fail to set the Content-Length header since its already set
	// See: https://github.com/golang/go/issues/14975.
	grw.Header().Del(contentLengthHeader)

	// Delete Accept-Ranges.
	// see: https://github.com/nytimes/gziphandler/issues/83
	grw.Header().Del(acceptRangesHeader)

	// Flush the buffer into the gzip response if there are any bytes.
	// If there aren't any, we shouldn't initialize it yet because on Close it will
	// write the gzip header even if nothing was ever written.
	if len(grw.buf) > 0 {
		_, err := grw.gw.Write(grw.buf)
		grw.buf = grw.buf[:0]
		if err != nil {
			return 0, err
		}
	}
	return lenB, nil
}

// Close will close the stdGzip.Writer.
func (grw *gzipRW) Close() error {
	if !grw.handledZip {
		// GZIP not triggered yet, write out regular response.
		_, err := grw.handleNonGzipped(len(grw.buf))
		return err
	}

	return grw.gw.Close() // will also call gzip flush()
}

// Flush flushes the underlying *stdGzip.Writer and then the
// underlying http.ResponseWriter if it is an http.Flusher.
// This makes gzipRW an http.Flusher.
func (grw *gzipRW) Flush() {
	if grw.gw == nil && grw.buf != nil {
		// Fix for NYTimes/gziphandler#58:
		//  Only flush once startGzip or
		//  startPassThrough has been called.
		//
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
	return nil, nil, fmt.Errorf("ong/middleware: http.Hijacker interface is not supported")
}

// ReadFrom implements io.ReaderFrom
// It is necessary for the sendfile syscall
// https://github.com/caddyserver/caddy/pull/5022
// https://github.com/caddyserver/caddy/blob/v2.7.4/modules/caddyhttp/responsewriter.go#L45-L49
func (grw *gzipRW) ReadFrom(src io.Reader) (n int64, err error) {
	return io.Copy(grw.ResponseWriter, src)
}

// Push implements http.Pusher
func (grw *gzipRW) Push(target string, opts *http.PushOptions) error {
	if p, ok := grw.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return fmt.Errorf("ong/middleware: http.Pusher interface is not supported")
}

// Unwrap implements http.ResponseController.
// It returns the underlying ResponseWriter,
// which is necessary for http.ResponseController to work correctly.
func (grw *gzipRW) Unwrap() http.ResponseWriter {
	return grw.ResponseWriter
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
