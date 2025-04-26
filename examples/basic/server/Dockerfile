# Use an official Go runtime as a parent image
# Choose a specific version for reproducibility
FROM golang:1.24-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache for dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project source code
# This includes the root library files and the examples directory
COPY . .

# Build the server executable within the examples/server directory
# Use CGO_ENABLED=0 for static linking, useful for alpine base
# Use -o flag to specify output path
# Ensure we build all .go files in the server directory
WORKDIR /app/examples/server
RUN CGO_ENABLED=0 go build -o /server_app *.go

# --- Final Stage ---
# Use a minimal base image for the final container
FROM alpine:latest

# Set working directory
WORKDIR /app/

# Copy the built executable from the builder stage
COPY --from=builder /server_app .

# Copy the fs_sandbox directory if it exists and has content needed at runtime
# For this example, the server creates it if needed, so copying isn't strictly necessary
# COPY --from=builder /app/examples/server/fs_sandbox ./fs_sandbox

# (Optional) Expose ports if using network transport later
# EXPOSE 8080

# Command to run the executable
# The server reads from stdin and writes to stdout
ENTRYPOINT ["./server_app"]