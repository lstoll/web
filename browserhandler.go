package web

import (
	"context"
	"fmt"
	"net/http"

	"lds.li/web/httperror"
	"lds.li/web/internal"
)

type BrowserHandlerFunc func(context.Context, ResponseWriter, *Request) error

func (b BrowserHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw, ok := w.(ResponseWriter)
	if !ok {
		rw = NewResponseWriter(w)
	}
	if err := r.ParseForm(); err != nil {
		b.handleError(rw, r, fmt.Errorf("parsing form: %w", err))
		return
	}

	br := &Request{
		r: r,
	}

	// Call handler with response writer
	err := b(r.Context(), rw, br)
	if err != nil {
		b.handleError(rw, br.r, err)
		return
	}
}

func (b BrowserHandlerFunc) handleError(rw ResponseWriter, r *http.Request, err error) {
	errh, ok := internal.UnwrapResponseWriterTo[httperror.ResponseWriter](rw)
	if ok {
		errh.WriteError(err)
	} else {
		httperror.DefaultErrorHandler(rw, r, err)
	}
}
