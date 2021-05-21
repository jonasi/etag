package etag

import (
	"bytes"
	"hash"
	"io"
	"net/http"
)

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

var (
	_ http.ResponseWriter = &rw{}
	_ http.Flusher        = &rw{}
)
