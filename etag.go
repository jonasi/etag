package etag

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"hash"
	"io"
	"net/http"
)

type ctx struct{}

// Disable disables etag processing for this request
func Disable(r *http.Request) {
	r.Context().Value(ctx{}).(func())()
}

// Handler returns a new http.Handler that writes an etag header
// based off the contents of the body
func Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bufw := &rw{
			headers: w.Header(),
			rw:      w,
			hash:    sha1.New(),
			buf:     bytes.NewBuffer(nil),
		}

		r = r.WithContext(context.WithValue(r.Context(), ctx{}, bufw.disable))

		h.ServeHTTP(bufw, r)

		if bufw.disabled {
			return
		}

		var (
			etag          = bufw.headers.Get("Etag")
			etagSet       = etag != ""
			statusSuccess = bufw.status == 0 || (bufw.status >= 200 && bufw.status < 300)
		)

		if !etagSet {
			etag = hex.EncodeToString(bufw.hash.Sum(nil))
		}

		if v := r.Header.Get("If-None-Match"); v != "" && statusSuccess && (r.Method == "HEAD" || r.Method == "GET") && etag == v {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		if !etagSet {
			w.Header().Set("etag", etag)
		}

		if bufw.status != 0 {
			w.WriteHeader(bufw.status)
		}

		if bufw.buf.Len() > 0 {
			// todo(isao) - what to do with error?
			_, _ = w.Write(bufw.buf.Bytes())
		}
	})
}

type rw struct {
	headers  http.Header
	rw       http.ResponseWriter
	hash     hash.Hash
	buf      *bytes.Buffer
	status   int
	disabled bool
}

func (w *rw) Write(b []byte) (int, error) {
	if w.disabled {
		return w.rw.Write(b)
	}

	if n, err := write(w.buf, b); err != nil {
		return n, err
	}

	return write(w.hash, b)
}

func write(w io.Writer, b []byte) (int, error) {
	n, err := w.Write(b)
	if err != nil {
		return n, err
	}

	if n != len(b) {
		return n, io.ErrShortWrite
	}

	return n, nil
}

func (w *rw) WriteHeader(status int) {
	if w.disabled {
		w.rw.WriteHeader(status)
		return
	}

	w.status = status
}

func (w *rw) Header() http.Header {
	return w.headers
}

func (w *rw) disable() {
	if w.disabled {
		return
	}

	w.disabled = true
	// if this is the first flush, immediately write
	// any buffered data to the underlying rw and flip
	// the bit so all future writes go to the underlying rw
	if w.buf.Len() > 0 {
		// todo(isao) - do something with this error?
		_, _ = w.rw.Write(w.buf.Bytes())
	}
}

func (w *rw) Flush() {
	if f, ok := w.rw.(http.Flusher); ok {
		w.disable()
		f.Flush()
	}
}
