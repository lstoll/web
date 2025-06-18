package slogctx

import (
	"context"
	"log/slog"
)

// Handler wraps a slog.Handler to inject attributes from the context
// into each log record.
type Handler struct {
	handler slog.Handler
}

// NewContextHandler creates a new ContextHandler that wraps the given handler.
func NewContextHandler(h slog.Handler) *Handler {
	return &Handler{handler: h}
}

// Enabled implements slog.Handler.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.handler.Enabled(ctx, level)
}

// Handle implements slog.Handler.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	// Get attributes from context
	attrs := AttrsFromContext(ctx)

	// Get attributes from registered extractors
	extractedAttrs := getExtractedAttributes(ctx)
	if len(extractedAttrs) > 0 {
		attrs = append(attrs, extractedAttrs...)
	}

	if len(attrs) > 0 {
		// Create a new record with the context attributes
		r = r.Clone()

		// Create a map of existing attribute keys
		existingKeys := make(map[string]struct{})
		r.Attrs(func(a slog.Attr) bool {
			existingKeys[a.Key] = struct{}{}
			return true
		})

		// Only add context attributes that don't already exist in the record
		var uniqueAttrs []slog.Attr
		for _, attr := range attrs {
			if _, exists := existingKeys[attr.Key]; !exists {
				uniqueAttrs = append(uniqueAttrs, attr)
			}
		}

		if len(uniqueAttrs) > 0 {
			r.AddAttrs(uniqueAttrs...)
		}
	}
	return h.handler.Handle(ctx, r)
}

// WithAttrs implements slog.Handler.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{handler: h.handler.WithAttrs(attrs)}
}

// WithGroup implements slog.Handler.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{handler: h.handler.WithGroup(name)}
}
