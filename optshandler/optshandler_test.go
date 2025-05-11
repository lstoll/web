package optshandler

import (
	"context"
	"testing"
)

// TestOpt is a simple HandlerOpt implementation for testing
type TestOpt struct {
	Value string
}

func TestContextWithHandlerOpts(t *testing.T) {
	ctx := context.Background()

	// Test adding a single option
	opt1 := TestOpt{Value: "test1"}
	ctx = ContextWithHandlerOpts(ctx, opt1)

	opts := HandlerOptsFromContext(ctx)
	if len(opts) != 1 {
		t.Errorf("expected 1 option, got %d", len(opts))
	}

	if got, ok := opts[0].(TestOpt); !ok {
		t.Error("failed to type assert option to TestOpt")
	} else if got.Value != "test1" {
		t.Errorf("expected value 'test1', got %q", got.Value)
	}

	// Test adding multiple options
	opt2 := TestOpt{Value: "test2"}
	ctx = ContextWithHandlerOpts(ctx, opt2)

	opts = HandlerOptsFromContext(ctx)
	if len(opts) != 2 {
		t.Errorf("expected 2 options, got %d", len(opts))
	}

	// Test empty context
	emptyCtx := context.Background()
	emptyOpts := HandlerOptsFromContext(emptyCtx)
	if len(emptyOpts) != 0 {
		t.Errorf("expected 0 options for empty context, got %d", len(emptyOpts))
	}
}

func TestContextHasOpt(t *testing.T) {
	ctx := context.Background()

	// Test when option doesn't exist
	if opt, exists := ContextHasOpt[TestOpt](ctx); exists {
		t.Errorf("expected no TestOpt to exist, but got %v", opt)
	}

	// Test when option exists
	opt1 := TestOpt{Value: "test1"}
	ctx = ContextWithHandlerOpts(ctx, opt1)

	if opt, exists := ContextHasOpt[TestOpt](ctx); !exists {
		t.Error("expected TestOpt to exist, but it doesn't")
	} else if opt.Value != "test1" {
		t.Errorf("expected value 'test1', got %q", opt.Value)
	}

	// Test with multiple options of different types
	type OtherOpt struct {
		Value int
	}

	opt2 := OtherOpt{Value: 42}
	ctx = ContextWithHandlerOpts(ctx, opt2)

	// Should still find TestOpt
	if opt, exists := ContextHasOpt[TestOpt](ctx); !exists {
		t.Error("expected TestOpt to exist, but it doesn't")
	} else if opt.Value != "test1" {
		t.Errorf("expected value 'test1', got %q", opt.Value)
	}

	// Should find OtherOpt
	if opt, exists := ContextHasOpt[OtherOpt](ctx); !exists {
		t.Error("expected OtherOpt to exist, but it doesn't")
	} else if opt.Value != 42 {
		t.Errorf("expected value 42, got %d", opt.Value)
	}
}
