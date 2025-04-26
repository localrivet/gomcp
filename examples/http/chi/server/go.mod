module github.com/localrivet/gomcp/examples/http/chi/server

go 1.24.0

replace github.com/localrivet/gomcp => ../../../../

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/localrivet/gomcp v0.0.0-00010101000000-000000000000
)

require github.com/google/uuid v1.6.0 // indirect
