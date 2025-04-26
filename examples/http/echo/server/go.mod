module github.com/localrivet/gomcp/examples/http/echo/server

go 1.24.0 // Assuming the same Go version, adjust if needed

replace github.com/localrivet/gomcp => ../../../../

require (
	github.com/labstack/echo/v4 v4.13.3
	github.com/localrivet/gomcp v0.0.0-00010101000000-000000000000
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/labstack/gommon v0.4.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v1.2.2 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.11.0 // indirect
)
