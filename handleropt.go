package web

import "net/http"

// HandlerOpt are functions that can be used to provide options to middleware
// serving a request. When registered on a handler, they will be called before
// the request hits the middleware stack.
type HandlerOpt func(r *http.Request) *http.Request

// nolint:unused // TODO - either use or drop this.
type handlerWithOpts struct {
	http.Handler
	Opts []HandlerOpt
}
