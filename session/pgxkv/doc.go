// Package pgxkv provides a postgres-backed session store using the pgx driver
//
// Example Schema:
//	CREATE TABLE web_sessions (
//		id TEXT PRIMARY KEY,
//		data JSONB NOT NULL, -- if JSON serialized, if proto then bytea
//		expires_at TIMESTAMPTZ NOT NULL
//	);
//	CREATE INDEX web_sessions_expires_at_idx ON sessions (expires_at);
//
//	COMMENT ON TABLE public.web_sessions IS 'Store for Web/HTTP user sessions';
//	COMMENT ON COLUMN web_sessions.id IS 'ID of the stored session';
//	COMMENT ON COLUMN web_sessions.data IS 'Session data, JSON format';
//	COMMENT ON COLUMN web_sessions.expires_at IS 'When the data should no longer be returned, and is a candidate for garbage collection';

package pgxkv
