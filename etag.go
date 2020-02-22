package etag

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"io"
	"net/http"
)

// Handler returns a new http.Handler that writes an etag header
// based off the contents of the body
func Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hsh := sha1.New()
		buf := bytes.NewBuffer(nil)
		bufw := &rw{
			headers: w.Header(),
			w:       io.MultiWriter(hsh, buf),
		}

		h.ServeHTTP(bufw, r)

		etag := hex.EncodeToString(hsh.Sum(nil))
		w.Header().Set("etag", etag)

		if bufw.status != 0 {
			w.WriteHeader(bufw.status)
		}

		if buf.Len() > 0 {
			// todo(isao) - what to do with error?
			_, _ = w.Write(buf.Bytes())
		}
	})
}

type rw struct {
	headers http.Header
	w       io.Writer
	status  int
}

func (w *rw) Write(b []byte) (int, error) {
	return w.w.Write(b)
}

func (w *rw) WriteHeader(status int) {
	w.status = status
}

func (w *rw) Header() http.Header {
	return w.headers
}
