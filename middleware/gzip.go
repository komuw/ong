package middleware

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Most of the code here is insipired by(or taken from):
//   (a) https://github.com/tmthrgd/gziphandler whose license(Apache License, Version 2.0) can be found here:                   https://github.com/tmthrgd/gziphandler/blob/9e3dc377f14f3554d9ae767761e33a87b38ed3f4/LICENSE.md
//   (b) https://github.com/klauspost/compress/tree/master/gzhttp whose license(Apache License, Version 2.0) can be found here: https://github.com/klauspost/compress/blob/4bc73d36928c39bbd7cf823171081d14c884edde/gzhttp/LICENSE

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

	// HeaderNoCompression can be used to disable compression.
	// Any header value will disable compression.
	// The Header is always removed from output.
	// HeaderNoCompression = "No-Gzip-Compression"
)

const (
	// TODO: vet this.
	acceptEncoding  = "Accept-Encoding"
	contentEncoding = "Content-Encoding"
	contentRange    = "Content-Range"
	acceptRanges    = "Accept-Ranges"
	contentType     = "Content-Type"
	contentLength   = "Content-Length"
)

// Gzip is a middleware that transparently gzips the response body, for clients which support.
func Gzip(wrappedHandler http.HandlerFunc) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add(varyHeader, acHeader)

		if !shouldGzipReq(r) {
			wrappedHandler(w, r)
			return
		}

		gzipWriter, _ := gzip.NewWriterLevel(w, defaultLevel)
		grw := &gzipRW{
			ResponseWriter: w,
			// Bytes written during ServeHTTP are redirected to this gzip writer
			// before being written to the underlying response.
			gw:      gzipWriter,
			level:   defaultLevel,
			minSize: defaultMinSize,
		}
		defer grw.Close()

		// We do not handle range requests when compression is used, as the
		// range specified applies to the compressed data, not to the uncompressed one.
		// see: https://github.com/nytimes/gziphandler/issues/83
		r.Header.Del("Range")

		if _, ok := w.(http.CloseNotifier); ok {
			// TODO: handle this case.

			// gwcn := GzipResponseWriterWithCloseNotify{gw}
			// wrappedHandler(gwcn, r)
			// return
		}

		wrappedHandler(grw, r)
	}
}

// gzipRW provides an http.ResponseWriter interface, which gzips
// bytes before writing them to the underlying response. This doesn't close the
// writers, so don't forget to do that.
// It can be configured to skip response smaller than minSize.
type gzipRW struct {
	http.ResponseWriter
	level int
	gw    *gzip.Writer

	code int // Saves the WriteHeader value.

	minSize int    // Specifies the minimum response size to gzip. If the response length is bigger than this value, it is compressed.
	buf     []byte // Holds the first part of the write before reaching the minSize or the end of the write.

	handledZip bool // whether this has yet to handle a zipped response.
}

var (
	// TODO: make sure these optional interfaces are implemented
	_ http.ResponseWriter = &gzipRW{}
	// _ http.Flusher        = &gzipRW{}
	// _ http.Hijacker       = &gzipRW{}
	// _ http.CloseNotifier  = &gzipRW{}
	_ io.WriteCloser = &gzipRW{}
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
	if grw.Header().Get(contentEncoding) != "" || grw.Header().Get(contentRange) != "" {
		return nonGzipped()
	}

	cl := 0
	if clStr := grw.Header().Get(contentLength); clStr != "" {
		cl, _ = strconv.Atoi(clStr)
	}
	if cl < grw.minSize && cl > 0 {
		// if content-length == 0, it means that the header was not set.
		// for those, we actually want to call `handleGzipped`; so we exempt them from this branch.
		return nonGzipped()
	}

	ct := ""
	if ct = grw.Header().Get(contentType); ct == "" {
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
	if _, ok := grw.Header()[contentType]; !ok && ct != "" {
		grw.Header().Set(contentType, ct)
	}

	// Initialize the GZIP writer.
	if err := grw.handleGzipped(); err != nil {
		return 0, err
	}
	return len(b), nil
}

// handleNonGzipped writes to the underlying ResponseWriter without gzip.
func (grw *gzipRW) handleNonGzipped() error {
	fmt.Println("\n\t handleNonGzipped called.")
	grw.handledZip = false
	// We need to do it even in this case because the Gzip handler has already stripped the range header anyway.
	grw.Header().Del(acceptRanges)

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
	fmt.Println("\n\t handleGzipped called.")
	grw.handledZip = true

	// Set the GZIP header.
	grw.Header().Set(contentEncoding, "gzip")

	// if the Content-Length is already set, then calls to Write on gzip
	// will fail to set the Content-Length header since its already set
	// See: https://github.com/golang/go/issues/14975.
	grw.Header().Del(contentLength)

	// Delete Accept-Ranges.
	// see: https://github.com/nytimes/gziphandler/issues/83
	grw.Header().Del(acceptRanges)

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
func (grw *gzipRW) WriteHeader(code int) {
	if grw.code == 0 {
		grw.code = code
	}
}

// Close will close the gzip.Writer and will put it back in the gzipWriterPool.
func (grw *gzipRW) Close() error {
	if !grw.handledZip {
		// GZIP not triggered yet, write out regular response.
		return grw.handleNonGzipped()
	}

	err := grw.gw.Close()
	return err
}

// shouldGzipReq checks whether the request is eligible to be gzipped.
func shouldGzipReq(r *http.Request) bool {
	// Examples of the `acHeader` are:
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

	if strings.Contains(r.Header.Get(acHeader), "gzip") {
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
