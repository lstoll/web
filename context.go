package web

import (
	"context"
	"net/http"
)

type ctxKeyScriptNonce struct{}

func contextWithScriptNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, ctxKeyScriptNonce{}, nonce)
}

type ctxKeyStyleNonce struct{}

func contextWithStyleNonce(ctx context.Context, nonce string) context.Context {
	return context.WithValue(ctx, ctxKeyStyleNonce{}, nonce)
}

type ctxKeyCSRFExempt struct{}

func contextWithCSRFExempt(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyCSRFExempt{}, true)
}

func isRequestCSRFExempt(r *http.Request) bool {
	v, ok := r.Context().Value(ctxKeyCSRFExempt{}).(bool)
	return ok && v
}
