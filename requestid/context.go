package requestid

import (
	"context"
)

type requestIDCtxKey struct{}

// ContextWithRequestID adds the specified request ID to the context
func ContextWithRequestID(parent context.Context, requestID string) context.Context {
	return context.WithValue(parent, requestIDCtxKey{}, requestID)
}

// FromContext returns the request ID from the context. If there is no
// request ID in the context, ok will be false.
func FromContext(ctx context.Context) (_ string, ok bool) {
	v, ok := ctx.Value(requestIDCtxKey{}).(string)
	return v, ok
}
