package slogctx

import (
	"context"
	"log/slog"
)

type attrsContextKey struct{}
type handleContextKey struct{}

// WithAttrs adds the given attributes to the context.
func WithAttrs(ctx context.Context, attrs ...slog.Attr) context.Context {
	if h, ok := ctx.Value(handleContextKey{}).(*Handle); ok {
		h.attrs = append(h.attrs, attrs...)
		return ctx
	}
	existing := AttrsFromContext(ctx)
	all := append(existing, attrs...)
	return context.WithValue(ctx, attrsContextKey{}, all)
}

// AttrsFromContext returns the attributes from the context.
func AttrsFromContext(ctx context.Context) []slog.Attr {
	if h, ok := ctx.Value(handleContextKey{}).(*Handle); ok {
		return h.attrs
	}
	if attrs, ok := ctx.Value(attrsContextKey{}).([]slog.Attr); ok {
		return attrs
	}
	return nil
}

// Handle is used to track the attributes across a series of child contexts. The
// handle can always retrieve the attributes that were added to the context and
// its children.
type Handle struct {
	attrs []slog.Attr
}

// Attrs returns the current attributes for the handle.
func (h *Handle) Attrs() []slog.Attr {
	return h.attrs
}

// WithHandle returns a new context with a new handle. If the context already
// has a handle, it will be returned.
func WithHandle(ctx context.Context) (context.Context, *Handle) {
	if h, ok := ctx.Value(handleContextKey{}).(*Handle); ok {
		return ctx, h
	}
	h := &Handle{attrs: AttrsFromContext(ctx)}
	return context.WithValue(ctx, handleContextKey{}, h), h
}
