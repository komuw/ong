package middleware

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
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

var grwPool = sync.Pool{New: func() interface{} { return &gzipRW{} }}

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

		if !shouldGzip(r) {
			wrappedHandler(w, r)
			return
		}

		grw := grwPool.Get().(*gzipRW)
		*grw = gzipRW{
			ResponseWriter: w,
			// Note: do not set `gw` here, it will be set when `startGzip` is called.
			level:             defaultLevel,
			minSize:           defaultMinSize,
			buf:               grw.buf,
			contentTypeFilter: defaultContentTypeFilter,
		}
		if len(grw.buf) > 0 {
			grw.buf = grw.buf[:0]
		}
		defer func() {
			grw.Close()
			grwPool.Put(grw)
		}()

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

	minSize          int    // Specifies the minimum response size to gzip. If the response length is bigger than this value, it is compressed.
	buf              []byte // Holds the first part of the write before reaching the minSize or the end of the write.
	ignore           bool   // If true, then we immediately passthru writes to the underlying ResponseWriter.
	keepAcceptRanges bool   // Keep "Accept-Ranges" header.

	contentTypeFilter func(ct string) bool // Only compress if the response is one of these content-types. All are accepted if empty.
}

// Write appends data to the gzip writer.
func (grw *gzipRW) Write(b []byte) (int, error) {
	// GZIP responseWriter is initialized. Use the GZIP responseWriter.
	if grw.gw != nil {
		fmt.Println("\n\t kkkkkkkkk")
		return grw.gw.Write(b)
	}

	// If we have already decided not to use GZIP, immediately passthrough.
	if grw.ignore {
		return grw.ResponseWriter.Write(b)
	}

	// Save the write into a buffer for later use in GZIP responseWriter
	// (if content is long enough) or at close with regular responseWriter.
	wantBuf := 512
	if grw.minSize > wantBuf {
		wantBuf = grw.minSize
	}
	toAdd := len(b)
	if len(grw.buf)+toAdd > wantBuf {
		toAdd = wantBuf - len(grw.buf)
	}
	grw.buf = append(grw.buf, b[:toAdd]...)
	remain := b[toAdd:]

	// Only continue if they didn't already choose an encoding or a known unhandled content length or type.
	if grw.Header().Get(contentEncoding) == "" && grw.Header().Get(contentRange) == "" {
		// Check more expensive parts now.
		pi, _ := strconv.ParseInt(grw.Header().Get(contentLength), 10, 0)
		cl := int(pi)
		ct := grw.Header().Get(contentType)
		if cl == 0 || cl >= grw.minSize && (ct == "" || grw.contentTypeFilter(ct)) {
			// If the current buffer is less than minSize and a Content-Length isn't set, then wait until we have more data.
			if len(grw.buf) < grw.minSize && cl == 0 {
				return len(b), nil
			}

			// If the Content-Length is larger than minSize or the current buffer is larger than minSize, then continue.
			if cl >= grw.minSize || len(grw.buf) >= grw.minSize {
				// If a Content-Type wasn't specified, infer it from the current buffer.
				if ct == "" {
					ct = http.DetectContentType(grw.buf)
				}

				// Handles the intended case of setting a nil Content-Type (as for http/server or http/fs)
				// Set the header only if the key does not exist
				if _, ok := grw.Header()[contentType]; !ok {
					grw.Header().Set(contentType, ct)
				}

				// If the Content-Type is acceptable to GZIP, initialize the GZIP writer.
				if grw.contentTypeFilter(ct) {
					if err := grw.startGzip(); err != nil {
						return 0, err
					}
					if len(remain) > 0 {
						if _, err := grw.gw.Write(remain); err != nil {
							return 0, err
						}
					}
					return len(b), nil
				}
			}
		}
	}

	// If we got here, we should not GZIP this response.
	if err := grw.nonGzipped(); err != nil {
		return 0, err
	}
	if len(remain) > 0 {
		if _, err := grw.ResponseWriter.Write(remain); err != nil {
			return 0, err
		}
	}
	return len(b), nil
}

// startGzip initializes a GZIP writer and writes the buffer.
func (grw *gzipRW) startGzip() error {
	fmt.Println("\n\t startGzip called.")
	// Set the GZIP header.
	grw.Header().Set(contentEncoding, "gzip")

	// if the Content-Length is already set, then calls to Write on gzip
	// will fail to set the Content-Length header since its already set
	// See: https://github.com/golang/go/issues/14975.
	grw.Header().Del(contentLength)

	// Delete Accept-Ranges.
	if !grw.keepAcceptRanges {
		grw.Header().Del(acceptRanges)
	}

	// Write the header to gzip response.
	if grw.code != 0 {
		grw.ResponseWriter.WriteHeader(grw.code)
		// Ensure that no other WriteHeader's happen
		grw.code = 0
	}

	// Initialize and flush the buffer into the gzip response if there are any bytes.
	// If there aren't any, we shouldn't initialize it yet because on Close it will
	// write the gzip header even if nothing was ever written.
	if len(grw.buf) > 0 {
		// Initialize the GZIP response.
		//
		// Bytes written during ServeHTTP are redirected to this gzip writer
		// before being written to the underlying response.
		gnw, _ := gzip.NewWriterLevel(grw.ResponseWriter, grw.level)
		grw.gw = gnw

		_, err := grw.gw.Write(grw.buf)

		grw.buf = grw.buf[:0]
		return err
	}
	return nil
}

// nonGzipped writes to the underlying ResponseWriter without gzip.
func (grw *gzipRW) nonGzipped() error {
	if grw.code != 0 {
		grw.ResponseWriter.WriteHeader(grw.code)
		// Ensure that no other WriteHeader's happen
		grw.code = 0
	}

	// TODO: we might not need `grw.ignore`. remove it??
	grw.ignore = true

	// If Write was never called then don't call Write on the underlying ResponseWriter.
	if len(grw.buf) == 0 {
		return nil
	}
	_, err := grw.ResponseWriter.Write(grw.buf)

	grw.buf = grw.buf[:0]
	return err
}

// Close will close the gzip.Writer and will put it back in the gzipWriterPool.
func (grw *gzipRW) Close() error {
	defer func() {
		grw.ResponseWriter = nil
	}()

	if grw.ignore {
		return nil
	}

	if grw.gw == nil {
		// GZIP not triggered yet, write out regular response.
		err := grw.nonGzipped()
		return err
	}

	err := grw.gw.Close()
	grw.gw = nil
	return err
}

func shouldGzip(r *http.Request) bool {
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

// defaultContentTypeFilter excludes common compressed audio, video and archive formats.
func defaultContentTypeFilter(ct string) bool {
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
