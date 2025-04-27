module github.com/localrivet/gomcp/examples/basic

go 1.24.0 // Assuming the same Go version, adjust if needed

replace github.com/localrivet/gomcp => ../../

require github.com/localrivet/gomcp v0.0.0-00010101000000-000000000000

require (
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/gobwas/pool v0.2.1 // indirect
	github.com/gobwas/ws v1.4.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/r3labs/sse/v2 v2.10.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	gopkg.in/cenkalti/backoff.v1 v1.1.0 // indirect
)
