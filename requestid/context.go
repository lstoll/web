package requestid

import (
	"context"

	"github.com/google/uuid"
)

type requestIDCtxKey struct{}

// ContextWithRequestID adds the specified request ID to the context
func ContextWithRequestID(parent context.Context, requestID string) context.Context {
	return context.WithValue(parent, requestIDCtxKey{}, requestID)
}

// ContextWithNewRequestID sets a new request ID on the context, returning the
// ID.
func ContextWithNewRequestID(parent context.Context) (_ context.Context, requestID string) {
	rid := newRequestID()
	return context.WithValue(parent, requestIDCtxKey{}, rid), rid
}

// FromContext returns the request ID from the context. If there is no
// request ID in the context, ok will be false.
func FromContext(ctx context.Context) (_ string, ok bool) {
	v, ok := ctx.Value(requestIDCtxKey{}).(string)
	return v, ok
}

func newRequestID() string {
	return uuid.Must(uuid.NewV7()).String()
}
