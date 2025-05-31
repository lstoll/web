module github.com/lstoll/web

go 1.24

require (
	// TODO - use the stlib version when
	// https://github.com/golang/go/issues/73626 lands, or revert to our old
	// version.
	filippo.io/csrf v0.0.0-20250517103426-cfb6fbb0fbe3
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	golang.org/x/crypto v0.38.0
)

require golang.org/x/sys v0.33.0 // indirect
