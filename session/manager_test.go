package session

import (
	"testing"
	"time"
)

func TestItem_InvalidAt(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		item        persistedSession
		maxLifetime *time.Duration
		idleTimeout *time.Duration
		want        time.Time
	}{
		{
			name:        "Max lifetime only",
			item:        persistedSession{CreatedAt: now},
			maxLifetime: ptr(2 * time.Hour),
			want:        now.Add(2 * time.Hour),
		},
		{
			name:        "Idle timeout only (CreatedAt)",
			item:        persistedSession{CreatedAt: now},
			idleTimeout: ptr(1 * time.Hour),
			want:        now.Add(1 * time.Hour),
		},
		{
			name:        "Idle timeout only (UpdatedAt)",
			item:        persistedSession{CreatedAt: now, UpdatedAt: now.Add(30 * time.Minute)},
			idleTimeout: ptr(1 * time.Hour),
			want:        now.Add(30 * time.Minute).Add(1 * time.Hour),
		},
		{
			name:        "Both timeouts, MaxLifetime earlier",
			item:        persistedSession{CreatedAt: now, UpdatedAt: now.Add(30 * time.Minute)},
			maxLifetime: ptr(1 * time.Hour),
			idleTimeout: ptr(2 * time.Hour),
			want:        now.Add(1 * time.Hour),
		},
		{
			name:        "Both timeouts, IdleTimeout earlier (CreatedAt)",
			item:        persistedSession{CreatedAt: now},
			maxLifetime: ptr(2 * time.Hour),
			idleTimeout: ptr(1 * time.Hour),
			want:        now.Add(1 * time.Hour),
		},
		{
			name:        "Both timeouts, IdleTimeout earlier (UpdatedAt)",
			item:        persistedSession{CreatedAt: now, UpdatedAt: now.Add(1 * time.Hour)},
			maxLifetime: ptr(2 * time.Hour),
			idleTimeout: ptr(1 * time.Hour),
			want:        now.Add(1 * time.Hour).Add(1 * time.Hour), // 2 hours from original CreatedAt
		},
		{
			name:        "UpdatedAt is nil, Idle Timeout",
			item:        persistedSession{CreatedAt: now},
			idleTimeout: ptr(1 * time.Hour),
			want:        now.Add(1 * time.Hour),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := &Manager{}
			if tt.maxLifetime != nil {
				mgr.opts.MaxLifetime = *tt.maxLifetime
			}
			if tt.idleTimeout != nil {
				mgr.opts.IdleTimeout = *tt.idleTimeout
			}

			got := mgr.calculateExpiry(tt.item)
			if !got.Equal(tt.want) {
				t.Errorf("InvalidAt() = %v, want %v", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
