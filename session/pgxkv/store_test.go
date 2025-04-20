package pgxkv

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

const createTable = `CREATE TABLE IF NOT EXISTS web_sessions (
	id TEXT PRIMARY KEY,
	data JSONB NOT NULL, -- if JSON serialized, if proto then bytea
	expires_at TIMESTAMPTZ NOT NULL
);`

func clearTable(t *testing.T, conn DBConn) {
	if _, err := conn.Exec(context.Background(), `DELETE FROM web_sessions`); err != nil {
		t.Fatal(err)
	}

}

func TestKV_E2E(t *testing.T) {
	dburl := os.Getenv("PGXKV_TEST_DATABASE_URL")
	if dburl == "" {
		t.Skip("PGXKV_TEST_DATABASE_URL not set")
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	conn, err := pgx.Connect(ctx, dburl)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := conn.Exec(ctx, createTable); err != nil {
		t.Fatal(err)
	}
	clearTable(t, conn)

	kv := New(conn, nil)

	t.Run("E2E_SetGetDelete", func(t *testing.T) {
		clearTable(t, conn)

		key := "testkey1"
		value := []byte(`{"value":1}`)
		expiresAt := time.Now().Add(time.Hour)

		// Set
		err := kv.Set(ctx, key, expiresAt, value)
		if err != nil {
			t.Fatalf("Set() error = %v, wantErr %v", err, nil)
		}

		// Get
		retrievedValue, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v, wantErr %v", err, nil)
		}
		if !found {
			t.Fatalf("Get() found = %v, want %v", found, true)
		}
		assertJSONeq(t, value, retrievedValue)

		// Delete
		err = kv.Delete(ctx, key)
		if err != nil {
			t.Fatalf("Delete() error = %v, wantErr %v", err, nil)
		}

		// Get after delete
		_, found, err = kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v, wantErr %v", err, nil)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}
	})

	t.Run("E2E_GetNotFound", func(t *testing.T) {
		clearTable(t, conn)

		key := "nonexistentkey"

		_, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v, wantErr %v", err, nil)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}
	})

	t.Run("E2E_GC", func(t *testing.T) {
		clearTable(t, conn)

		// Set expired key
		err := kv.Set(ctx, "expiredkey", time.Now().Add(-time.Hour), []byte(`{"value":1}`))
		if err != nil {
			t.Fatalf("Set() error = %v, wantErr %v", err, nil)
		}

		// Set not expired key
		err = kv.Set(ctx, "validkey", time.Now().Add(time.Hour), []byte(`{"value":2}`))
		if err != nil {
			t.Fatalf("Set() error = %v, wantErr %v", err, nil)
		}

		// GC
		deleted, err := kv.GC(ctx)
		if err != nil {
			t.Fatalf("GC() error = %v, wantErr %v", err, nil)
		}
		if deleted != 1 {
			t.Errorf("GC() deleted = %v, want %v", deleted, 1)
		}

		// Get expired key
		_, found, err := kv.Get(ctx, "expiredkey")
		if err != nil {
			t.Fatalf("Get() error = %v, wantErr %v", err, nil)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}

		// Get not expired key
		_, found, err = kv.Get(ctx, "validkey")
		if err != nil {
			t.Fatalf("Get() error = %v, wantErr %v", err, nil)
		}
		if !found {
			t.Errorf("Get() found = %v, want %v", found, true)
		}
	})
	t.Run("E2E_Upsert", func(t *testing.T) {
		clearTable(t, conn)

		key := "testkey_upsert"
		value1 := []byte(`{"value":1}`)
		value2 := []byte(`{"value":2}`)
		expiresAt := time.Now().Add(time.Hour)

		// Set initial value
		err := kv.Set(ctx, key, expiresAt, value1)
		if err != nil {
			t.Fatalf("Set() error = %v, wantErr %v", err, nil)
		}

		// Upsert with new value
		err = kv.Set(ctx, key, expiresAt, value2)
		if err != nil {
			t.Fatalf("Set() error = %v, wantErr %v", err, nil)
		}

		// Get and verify
		retrievedValue, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v, wantErr %v", err, nil)
		}
		if !found {
			t.Fatalf("Get() found = %v, want %v", found, true)
		}
		assertJSONeq(t, value2, retrievedValue)
	})

	t.Run("E2E_GetExpiredKey_Not_GCd", func(t *testing.T) {
		clearTable(t, conn)

		key := "expiredkey_not_gcd"
		value := []byte(`{"value":"expired"}`)
		// Set expiration to 1 second in the past
		expiresAt := time.Now().Add(-time.Second)

		// Set the expired key
		err := kv.Set(ctx, key, expiresAt, value)
		if err != nil {
			t.Fatalf("Set() error = %v, wantErr %v", err, nil)
		}

		// Get the expired key (should not be found even if not GC'd yet)
		_, found, err := kv.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get() error = %v, wantErr %v", err, nil)
		}
		if found {
			t.Errorf("Get() found = %v, want %v", found, false)
		}
	})
}

func assertJSONeq(t testing.TB, want, got []byte) {
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
