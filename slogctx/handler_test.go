package slogctx

import (
	"context"
	"log/slog"
	"testing"
)

// testExtractor is a simple attribute extractor for testing
func testExtractor(ctx context.Context) []slog.Attr {
	if v, ok := ctx.Value("test_key").(string); ok {
		return []slog.Attr{slog.String("extracted_key", v)}
	}
	return nil
}

func TestHandler(t *testing.T) {
	// Register our test extractor
	RegisterAttributeExtractor("test", testExtractor)

	// Create a test handler that records all records
	records := make([]slog.Record, 0)
	handler := &testHandler{
		handleFunc: func(ctx context.Context, r slog.Record) error {
			records = append(records, r)
			return nil
		},
	}

	// Wrap it with our context handler
	ctxHandler := NewContextHandler(handler)

	// Test cases
	tests := []struct {
		name      string
		ctx       context.Context
		record    slog.Record
		wantAttrs map[string]slog.Value
	}{
		{
			name: "no context attrs",
			ctx:  context.Background(),
			record: slog.Record{
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantAttrs: map[string]slog.Value{},
		},
		{
			name: "with context attrs",
			ctx: WithAttrs(context.Background(),
				slog.String("user_id", "123"),
				slog.Int("count", 42),
			),
			record: slog.Record{
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantAttrs: map[string]slog.Value{
				"user_id": slog.StringValue("123"),
				"count":   slog.IntValue(42),
			},
		},
		{
			name: "with existing record attrs",
			ctx: WithAttrs(context.Background(),
				slog.String("ctx_attr", "value"),
			),
			record: slog.Record{
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantAttrs: map[string]slog.Value{
				"ctx_attr": slog.StringValue("value"),
			},
		},
		{
			name: "with extracted attrs",
			ctx:  context.WithValue(context.Background(), "test_key", "extracted_value"),
			record: slog.Record{
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantAttrs: map[string]slog.Value{
				"extracted_key": slog.StringValue("extracted_value"),
			},
		},
		{
			name: "with both context and extracted attrs",
			ctx: WithAttrs(
				context.WithValue(context.Background(), "test_key", "extracted_value"),
				slog.String("ctx_attr", "value"),
			),
			record: slog.Record{
				Level:   slog.LevelInfo,
				Message: "test message",
			},
			wantAttrs: map[string]slog.Value{
				"ctx_attr":      slog.StringValue("value"),
				"extracted_key": slog.StringValue("extracted_value"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			records = records[:0] // Clear records

			// Add some attributes to the record
			tt.record.AddAttrs(slog.String("record_attr", "value"))

			// Handle the record
			if err := ctxHandler.Handle(tt.ctx, tt.record); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}

			// Verify we got exactly one record
			if len(records) != 1 {
				t.Fatalf("expected 1 record, got %d", len(records))
			}

			record := records[0]
			if record.Message != tt.record.Message {
				t.Errorf("expected message %q, got %q", tt.record.Message, record.Message)
			}

			// Collect all attributes from the record
			gotAttrs := make(map[string]slog.Value)
			record.Attrs(func(a slog.Attr) bool {
				gotAttrs[a.Key] = a.Value
				return true
			})

			// Verify context attributes are present
			for k, want := range tt.wantAttrs {
				if got, ok := gotAttrs[k]; !ok {
					t.Errorf("missing attribute %q", k)
				} else if got.Any() != want.Any() {
					t.Errorf("attribute %q = %v, want %v", k, got, want)
				}
			}

			// Verify record attributes are preserved
			if got, ok := gotAttrs["record_attr"]; !ok {
				t.Error("missing record attribute 'record_attr'")
			} else if got.String() != "value" {
				t.Errorf("record_attr = %v, want 'value'", got)
			}
		})
	}

	// Test deregistration
	t.Run("deregister extractor", func(t *testing.T) {
		// Verify extractor is registered
		if !DeregisterAttributeExtractor("test") {
			t.Error("expected to find and remove test extractor")
		}

		// Verify it's gone
		if DeregisterAttributeExtractor("test") {
			t.Error("expected test extractor to be already removed")
		}

		// Verify it no longer adds attributes
		records = records[:0]
		ctx := context.WithValue(context.Background(), "test_key", "extracted_value")
		if err := ctxHandler.Handle(ctx, slog.Record{
			Level:   slog.LevelInfo,
			Message: "test message",
		}); err != nil {
			t.Fatalf("Handle() error = %v", err)
		}

		if len(records) != 1 {
			t.Fatalf("expected 1 record, got %d", len(records))
		}

		// Verify no extracted attributes are present
		gotAttrs := make(map[string]slog.Value)
		records[0].Attrs(func(a slog.Attr) bool {
			gotAttrs[a.Key] = a.Value
			return true
		})

		if _, ok := gotAttrs["extracted_key"]; ok {
			t.Error("expected extracted_key to be removed")
		}
	})
}

func TestHandlerWithAttrs(t *testing.T) {
	records := make([]slog.Record, 0)
	handler := &testHandler{
		handleFunc: func(ctx context.Context, r slog.Record) error {
			records = append(records, r)
			return nil
		},
	}

	ctxHandler := NewContextHandler(handler)

	// Test WithAttrs
	newHandler := ctxHandler.WithAttrs([]slog.Attr{
		slog.String("handler_attr", "value"),
	})

	// Handle a record
	if err := newHandler.Handle(context.Background(), slog.Record{
		Level:   slog.LevelInfo,
		Message: "test message",
	}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify the handler attribute was added
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	attrs := make(map[string]slog.Value)
	records[0].Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value
		return true
	})

	if v, ok := attrs["handler_attr"]; !ok || v.String() != "value" {
		t.Errorf("expected handler_attr=value, got %v", v)
	}
}

func TestHandlerWithGroup(t *testing.T) {
	records := make([]slog.Record, 0)
	handler := &testHandler{
		handleFunc: func(ctx context.Context, r slog.Record) error {
			records = append(records, r)
			return nil
		},
	}

	ctxHandler := NewContextHandler(handler)

	// Test WithGroup
	newHandler := ctxHandler.WithGroup("test_group")

	// Handle a record
	if err := newHandler.Handle(context.Background(), slog.Record{
		Level:   slog.LevelInfo,
		Message: "test message",
	}); err != nil {
		t.Fatalf("Handle() error = %v", err)
	}

	// Verify the handler was properly wrapped
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
}

// testHandler is a simple slog.Handler implementation for testing
type testHandler struct {
	handleFunc func(context.Context, slog.Record) error
	attrs      []slog.Attr
}

func (h *testHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testHandler) Handle(ctx context.Context, r slog.Record) error {
	// Add our stored attributes to the record
	if len(h.attrs) > 0 {
		r = r.Clone()
		r.AddAttrs(h.attrs...)
	}
	return h.handleFunc(ctx, r)
}

func (h *testHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &testHandler{
		handleFunc: h.handleFunc,
		attrs:      append(h.attrs, attrs...),
	}
}

func (h *testHandler) WithGroup(name string) slog.Handler {
	return h
}
