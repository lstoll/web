// Package sqlkv provides a SQL-backed session store using database/sql
//
// This implementation is designed to work across multiple SQL database engines
// with appropriate dialect settings. Supported dialects include Generic (default),
// MySQL, PostgreSQL, SQLite, and SQL Server.
//
// Example Schema:
//
//	CREATE TABLE web_sessions (
//		id TEXT PRIMARY KEY,
//		data BLOB NOT NULL,
//		expires_at TIMESTAMP NOT NULL
//	);
//	CREATE INDEX web_sessions_expires_at_idx ON web_sessions (expires_at);
//
// Usage with SQLite:
//
//	db, err := sql.Open("sqlite3", ":memory:")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	// Create a new KV store
//	kv := sqlkv.New(db, &sqlkv.Opts{
//		Dialect: sqlkv.SQLite,
//		TableName: "my_sessions", // optional, defaults to "web_sessions"
//	})
//
//	// Create the table if it doesn't exist
//	if err := kv.CreateTable(context.Background()); err != nil {
//		log.Fatal(err)
//	}
//
//	// Configure the session manager to use this KV store
//	manager := session.NewManager(&session.ManagerOpts{
//		StorageMode: session.StorageModeKV,
//		KV:          kv,
//	})
//
// Garbage Collection:
//
// The KV store supports garbage collection to remove expired sessions:
//
//	// Run garbage collection once
//	deleted, err := kv.GC(context.Background())
//	if err != nil {
//		log.Printf("GC error: %v", err)
//	}
//	log.Printf("Deleted %d expired sessions", deleted)
//
//	// Or run garbage collection in the background at regular intervals
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	kv.RunGC(ctx, 10*time.Minute, log.New(os.Stdout, "GC: ", log.LstdFlags))

package sqlkv
