module github.com/localrivet/gomcp/examples/billing

go 1.24.0 // Assuming the same Go version, adjust if needed

replace github.com/localrivet/gomcp => ../../

require github.com/localrivet/gomcp v0.0.0-00010101000000-000000000000

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/r3labs/sse/v2 v2.10.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	gopkg.in/cenkalti/backoff.v1 v1.1.0 // indirect
)
