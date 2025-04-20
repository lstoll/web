package requestid

import (
	"context"
	"testing"
)

func TestContext(t *testing.T) {
	ctx := context.Background()

	if _, ok := FromContext(ctx); ok {
		t.Error("new context should not contain a request ID")
	}

	ctx, id := ContextWithNewRequestID(ctx)

	gotID, ok := FromContext(ctx)
	if !ok {
		t.Error("context should have a request ID")
	}
	if gotID != id {
		t.Errorf("wanted request ID %s, got: %s", id, gotID)
	}

	newID := newRequestID()
	ctx = ContextWithRequestID(ctx, newID)

	gotID, ok = FromContext(ctx)
	if !ok {
		t.Error("context should have a request ID")
	}
	if gotID != newID {
		t.Errorf("wanted request ID %s, got: %s", newID, gotID)
	}
}
