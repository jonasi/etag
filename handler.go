package etag

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/hex"
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
			etag = `"` + hex.EncodeToString(bufw.hash.Sum(nil)) + `"`
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
