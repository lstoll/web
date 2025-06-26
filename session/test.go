package session

import (
	"context"
	"time"
)

type TestResult struct {
	ctx *Session
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
	return t.ctx.sessdata.Data
}

// TestContext attaches a session to a context, to be used for testing. The
// returned TestResult can be used to verify the actions against the session. The session
// is optional, if omitted a new session is created.
func TestContext(ctx context.Context, s *Session) (context.Context, *TestResult) {
	if s == nil {
		s = &Session{
			sessdata: persistedSession{
				Data:      make(map[string]any),
				CreatedAt: time.Now(),
			},
		}
	}
	return context.WithValue(ctx, sessionContextKey{}, s), nil
}
