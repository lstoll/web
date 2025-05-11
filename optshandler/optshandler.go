package optshandler

import (
	"context"
	"net/http"
)

type handlerOptsCtxKey struct{}

// HandlerOpts is an option that can be passed through handlers. Specific
// implementations are per-handler
type HandlerOpt any

// ContextWithHandlerOpts appends the options to the context, making them
// available to downstream Handlers.
func ContextWithHandlerOpts(ctx context.Context, opts ...HandlerOpt) context.Context {
	return context.WithValue(ctx,
		handlerOptsCtxKey{},
		append(HandlerOptsFromContext(ctx), opts...))
}

// HandlerOptsFromContext returns the options from the context. If no options
// are present, an empty slice is returned.
func HandlerOptsFromContext(ctx context.Context) []HandlerOpt {
	v, ok := ctx.Value(handlerOptsCtxKey{}).([]HandlerOpt)
	if !ok {
		return nil
	}
	return v
}

func ContextHasOpt[OptT HandlerOpt](ctx context.Context) (OptT, bool) {
	opts := HandlerOptsFromContext(ctx)
	for _, opt := range opts {
		if opt, ok := opt.(OptT); ok {
			return opt, true
		}
	}
	var zero OptT
	return zero, false
}

// OptsHandler is an interface a handler can implement, to provide options for
// when it is called. These options are unpacked at the root of the [Server]'s
// Handler, and passed via the context. Usage outside of [Server] will require
// equivalent processing to be implemented.
type OptsHandler interface {
	http.Handler
	HandleOpts() []HandlerOpt
}

type handlerWithOpts struct {
	http.Handler
	Opts []HandlerOpt
}

func (o *handlerWithOpts) HandleOpts() []HandlerOpt {
	return o.Opts
}

// HandlerWithOpts returns a new OptsHandler for the passed options.
func HandlerWithOpts(h http.Handler, opts ...HandlerOpt) OptsHandler {
	return &handlerWithOpts{h, opts}
}

func HandlerFuncWithOpts(fn func(w http.ResponseWriter, r *http.Request), opts ...HandlerOpt) OptsHandler {
	return &handlerWithOpts{http.HandlerFunc(fn), opts}
}
