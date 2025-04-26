module github.com/localrivet/gomcp/examples/http/httprouter/server

go 1.24.0 // Assuming the same Go version, adjust if needed

replace github.com/localrivet/gomcp => ../../../../

require (
	github.com/julienschmidt/httprouter v1.3.0
	github.com/localrivet/gomcp v0.0.0-00010101000000-000000000000
)

require github.com/google/uuid v1.6.0 // indirect
