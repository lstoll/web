package kvtest

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"lds.li/web/session"
)

// RunComplianceTest runs a standard suite of tests against a KV implementation to ensure
// it behaves correctly according to the KV interface contract.
//
// The cleanup function should reset the store to a clean state (e.g., delete all keys).
// It will be called before each test.
func RunComplianceTest(t *testing.T, kv session.KV, cleanup func()) {
	if cleanup != nil {
		cleanup()
		t.Cleanup(cleanup)
	}

	t.Run("SetGetDelete", func(t *testing.T) {
		if cleanup != nil {
			cleanup()
		}

		ctx := context.Background()
		key := "testkey1"
		value := []byte(`{"value":1}`)
		expiresAt := time.Now().Add(time.Hour)

		// Set
		err := kv.Set(ctx, key, expiresAt, value)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Get
		retrievedValue, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !found {
			t.Fatalf("Get() found = %v, want %v", found, true)
		}
		assertJSONEqual(t, value, retrievedValue)

		// Delete
		err = kv.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete() error = %v", err)
		}

		// Get after delete
		_, found, err = kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		if cleanup != nil {
			cleanup()
		}

		ctx := context.Background()
		key := "nonexistentkey"

		_, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}
	})

	t.Run("Upsert", func(t *testing.T) {
		if cleanup != nil {
			cleanup()
		}

		ctx := context.Background()
		key := "testkey_upsert"
		value1 := []byte(`{"value":1}`)
		value2 := []byte(`{"value":2}`)
		expiresAt := time.Now().Add(time.Hour)

		// Set initial value
		err := kv.Set(ctx, key, expiresAt, value1)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Upsert with new value
		err = kv.Set(ctx, key, expiresAt, value2)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Get and verify
		retrievedValue, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !found {
			t.Fatalf("Get() found = %v, want %v", found, true)
		}
		assertJSONEqual(t, value2, retrievedValue)
	})

	t.Run("GetExpiredKey", func(t *testing.T) {
		if cleanup != nil {
			cleanup()
		}

		ctx := context.Background()
		key := "expiredkey"
		value := []byte(`{"value":"expired"}`)
		// Set expiration to 1 second in the past
		expiresAt := time.Now().Add(-time.Second)

		// Set the expired key
		err := kv.Set(ctx, key, expiresAt, value)
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Get the expired key (should not be found)
		_, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}
	})

	// Additional tests for any KV implementations that support GC
	t.Run("GC", testGC(kv, cleanup))
}

// assertJSONEqual checks if two JSON byte slices are semantically equal
func assertJSONEqual(t testing.TB, want, got []byte) {
	t.Helper()

	var (
		wm map[string]any
		gm map[string]any
	)
	if err := json.Unmarshal(want, &wm); err != nil {
		t.Fatalf("unmarshaling want %s: %v", want, err)
	}
	if err := json.Unmarshal(got, &gm); err != nil {
		t.Fatalf("unmarshaling got %s: %v", got, err)
	}
	if !reflect.DeepEqual(wm, gm) {
		t.Errorf("want %s, got: %s", want, got)
	}
}

// GC is an optional interface for KV implementations that support garbage collection
type GC interface {
	GC(ctx context.Context) (deleted int, _ error)
}

// testGC tests garbage collection functionality if the KV implements the GC interface
func testGC(kv session.KV, cleanup func()) func(t *testing.T) {
	return func(t *testing.T) {
		// Skip if GC is not implemented
		gc, ok := kv.(GC)
		if !ok {
			t.Skip("KV implementation does not support GC")
		}

		if cleanup != nil {
			cleanup()
		}

		ctx := context.Background()

		// Set expired key
		err := kv.Set(ctx, "expiredkey", time.Now().Add(-time.Hour), []byte(`{"value":1}`))
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// Set not expired key
		err = kv.Set(ctx, "validkey", time.Now().Add(time.Hour), []byte(`{"value":2}`))
		if err != nil {
			t.Fatalf("Set() error = %v", err)
		}

		// GC
		deleted, err := gc.GC(ctx)
		if err != nil {
			t.Fatalf("GC() error = %v", err)
		}
		if deleted < 1 {
			t.Errorf("GC() deleted = %v, want at least 1", deleted)
		}

		// Get expired key
		_, found, err := kv.Get(ctx, "expiredkey")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}

		// Get not expired key
		_, found, err = kv.Get(ctx, "validkey")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if !found {
			t.Errorf("Get() found = %v, want %v", found, true)
		}
	}
}
