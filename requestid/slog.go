package requestid

import (
	"context"
	"log/slog"

	"lds.li/web/slogctx"
)

func init() {
	// Register the request ID extractor
	slogctx.RegisterAttributeExtractor("requestid", func(ctx context.Context) []slog.Attr {
		if id, ok := FromContext(ctx); ok {
			return []slog.Attr{slog.String("request_id", id)}
		}
		return nil
	})
}
