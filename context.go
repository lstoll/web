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

// The following functions are deprecated and only kept for backward compatibility
// since we've moved to secfetch for CSRF protection.

// Deprecated: CSRF protection is now handled by Sec-Fetch headers.
type ctxKeyCSRFExempt struct{}

// Deprecated: CSRF protection is now handled by Sec-Fetch headers.
func contextWithCSRFExempt(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKeyCSRFExempt{}, true)
}

// Deprecated: CSRF protection is now handled by Sec-Fetch headers.
func isRequestCSRFExempt(r *http.Request) bool {
	v, ok := r.Context().Value(ctxKeyCSRFExempt{}).(bool)
	return ok && v
}
