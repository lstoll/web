package web

import (
	"context"
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
