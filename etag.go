package etag

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"hash"
	"io"
	"net/http"
)

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

		h.ServeHTTP(bufw, r)

		if bufw.flushed {
			return
		}

		etag := hex.EncodeToString(bufw.hash.Sum(nil))
		if v := r.Header.Get("If-None-Match"); v != "" && (r.Method == "HEAD" || r.Method == "GET") && etag == v {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("etag", etag)

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
	headers http.Header
	rw      http.ResponseWriter
	hash    hash.Hash
	buf     *bytes.Buffer
	status  int
	flushed bool
}

func (w *rw) Write(b []byte) (int, error) {
	if w.flushed {
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
	w.status = status
}

func (w *rw) Header() http.Header {
	return w.headers
}

func (w *rw) Flush() {
	if f, ok := w.rw.(http.Flusher); ok {
		// if this is the first flush, immediately write
		// any buffered data to the underlying rw and flip
		// the bit so all future writes go to the underlying rw
		if !w.flushed {
			if w.buf.Len() > 0 {
				// todo(isao) - do something with this error?
				w.rw.Write(w.buf.Bytes())
			}
			w.flushed = true
		}

		f.Flush()
	}
}
