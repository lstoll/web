package httperror

import (
	"errors"
	"net/http"
)

// ResponseWriter extends the standard http.ResponseWriter with a method to
// write typed errors.
type ResponseWriter interface {
	http.ResponseWriter
	WriteError(err error)
}

type errorResponseWriter struct {
	http.ResponseWriter
	baseReq *http.Request
	handler ErrorHandler
}

func (e *errorResponseWriter) WriteError(r *http.Request, err error) {
	e.handler.HandleError(e.ResponseWriter, r, err)
}

// WriteHeader implements the http.ResponseWriter interface, intercepting the error code
// and passing it to the typed WriteError method.å
func (e *errorResponseWriter) WriteHeader(status int) {
	switch {
	case status >= 400:
		// TODO - how does this work with the test from http.Error?
		e.WriteError(e.baseReq, &httpErr{
			error: errors.New(http.StatusText(status)),
			code:  status,
		})
	default:
		// if it's not a case we explicitly handle, just pass it through.
		e.ResponseWriter.WriteHeader(status)
	}
}
