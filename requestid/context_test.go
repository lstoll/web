package requestid

import (
	"context"
	"testing"

	"github.com/lstoll/web/internal"
)

func TestContext(t *testing.T) {
	ctx := context.Background()

	if _, ok := FromContext(ctx); ok {
		t.Error("new context should not contain a request ID")
	}

	id := internal.NewUUIDV4().String()
	ctx = ContextWithRequestID(ctx, id)

	gotID, ok := FromContext(ctx)
	if !ok {
		t.Error("context should have a request ID")
	}
	if gotID != id {
		t.Errorf("wanted request ID %s, got: %s", id, gotID)
	}

	newID := internal.NewUUIDV4().String()
	ctx = ContextWithRequestID(ctx, newID)

	gotID, ok = FromContext(ctx)
	if !ok {
		t.Error("context should have a request ID")
	}
	if gotID != newID {
		t.Errorf("wanted request ID %s, got: %s", newID, gotID)
	}
}
