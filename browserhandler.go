package web

import (
	"context"
	"fmt"
	"net/http"
)

type BrowserHandlerFunc func(context.Context, ResponseWriter, *Request) error

func (b BrowserHandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rw, ok := w.(ResponseWriter)
	if !ok {
		rw = newResponseWriter(w, r, nil)
	}
	if err := r.ParseForm(); err != nil {
		rw.WriteError(fmt.Errorf("parsing form: %w", err))
		return
	}

	br := &Request{
		r: r,
	}

	// Call handler with response writer
	err := b(r.Context(), rw, br)
	if err != nil {
		rw.WriteError(fmt.Errorf("parsing form: %w", err))
		return
	}
}
