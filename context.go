package web

import (
	"context"

	"github.com/lstoll/web/static"
)

type staticHandlerCtxKey struct{}

func contextWithStaticHandler(ctx context.Context, sh *static.FileHandler) context.Context {
	return context.WithValue(ctx, staticHandlerCtxKey{}, sh)
}

func staticHandlerFromContext(ctx context.Context) *static.FileHandler {
	return ctx.Value(staticHandlerCtxKey{}).(*static.FileHandler)
}
