module github.com/lstoll/web/session/storee2e

go 1.25

require (
	github.com/go-sql-driver/mysql v1.8.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/lstoll/web v0.0.0
	github.com/mattn/go-sqlite3 v1.14.28
)

require (
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
)

replace github.com/lstoll/web => ../..
