package session

import (
	"context"
	"time"
)

type TestResult struct {
	ctx *sessCtx
}

func (t *TestResult) Saved() bool {
	return t.ctx.save
}

func (t *TestResult) Deleted() bool {
	return t.ctx.delete
}

func (t *TestResult) Reset() bool {
	return t.ctx.reset
}

func (t *TestResult) Result() map[string]any {
	return t.ctx.data
}

// TestContext attaches a session to a context, to be used for testing. The
// returned TestResult can be used to verify the actions against the session
func TestContext(mgr *Manager, ctx context.Context, sess map[string]any) (context.Context, *TestResult) {
	return context.WithValue(ctx, mgrSessCtxKey{inst: mgr}, &sessCtx{
		metadata: &sessionMetadata{
			CreatedAt: time.Now(),
		},
		data: sess,
	}), nil
}
