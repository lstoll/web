package slogctx

import (
	"context"
	"log/slog"
	"testing"
)

func TestWithAttrs(t *testing.T) {
	tests := []struct {
		name       string
		ctx        context.Context
		attrs      []slog.Attr
		wantLen    int
		withHandle bool
	}{
		{
			name:       "empty context",
			ctx:        context.Background(),
			attrs:      []slog.Attr{slog.String("key1", "value1")},
			wantLen:    1,
			withHandle: false,
		},
		{
			name:       "context with existing attrs",
			ctx:        WithAttrs(context.Background(), slog.String("key1", "value1")),
			attrs:      []slog.Attr{slog.String("key2", "value2")},
			wantLen:    2,
			withHandle: false,
		},
		{
			name: "context with handle",
			ctx: func() context.Context {
				ctx, _ := WithHandle(context.Background())
				return WithAttrs(ctx, slog.String("key1", "value1"))
			}(),
			attrs:      []slog.Attr{slog.String("key2", "value2")},
			wantLen:    2,
			withHandle: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := WithAttrs(tt.ctx, tt.attrs...)
			got := AttrsFromContext(ctx)
			if len(got) != tt.wantLen {
				t.Errorf("AttrsFromContext() got %d attrs, want %d", len(got), tt.wantLen)
			}
		})
	}
}

func TestWithHandle(t *testing.T) {
	tests := []struct {
		name    string
		ctx     context.Context
		attrs   []slog.Attr
		wantLen int
	}{
		{
			name:    "new handle",
			ctx:     WithAttrs(context.Background(), slog.String("key1", "value1")),
			attrs:   []slog.Attr{slog.String("key2", "value2")},
			wantLen: 2,
		},
		{
			name: "existing handle",
			ctx: func() context.Context {
				ctx, _ := WithHandle(context.Background())
				return WithAttrs(ctx, slog.String("key1", "value1"))
			}(),
			attrs:   []slog.Attr{slog.String("key2", "value2")},
			wantLen: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, handle := WithHandle(tt.ctx)
			ctx = WithAttrs(ctx, tt.attrs...)

			// Test handle's Attrs method
			got := handle.Attrs()
			if len(got) != tt.wantLen {
				t.Errorf("Handle.Attrs() got %d attrs, want %d", len(got), tt.wantLen)
			}

			// Test AttrsFromContext
			got = AttrsFromContext(ctx)
			if len(got) != tt.wantLen {
				t.Errorf("AttrsFromContext() got %d attrs, want %d", len(got), tt.wantLen)
			}
		})
	}
}
