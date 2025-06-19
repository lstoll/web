package httperror

import (
	"bytes"
	"net/http"

	"github.com/lstoll/web/internal"
)

// ResponseWriter extends the standard http.ResponseWriter with a method to
// write typed errors.
type ResponseWriter interface {
	http.ResponseWriter
	WriteError(err error)
}

var (
	_ internal.UnwrappableResponseWriter = (*responseWriter)(nil)
	_ ResponseWriter                     = (*responseWriter)(nil)
)

// responseWriter wraps an http.ResponseWriter to intercept error responses
type responseWriter struct {
	http.ResponseWriter
	err           error
	code          int
	headerWritten bool

	buffer bytes.Buffer
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{
		ResponseWriter: w,
		code:           http.StatusOK,
	}
}

func (w *responseWriter) WriteHeader(code int) {
	w.code = code

	if code < 400 && !w.headerWritten {
		w.ResponseWriter.WriteHeader(code)
		w.headerWritten = true
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.code >= 400 {
		return w.buffer.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *responseWriter) WriteError(err error) {
	w.err = err
}

func (w *responseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
