#!/bin/bash

# Script to generate Go code from Protocol Buffer definitions

set -e

# Check if protoc is installed
if ! command -v protoc &>/dev/null; then
    echo "Error: protoc is not installed or not in PATH"
    echo "Please install protobuf compiler from https://github.com/protocolbuffers/protobuf/releases"
    exit 1
fi

# Check if protoc-gen-go and protoc-gen-go-grpc are installed
if ! command -v protoc-gen-go &>/dev/null; then
    echo "Error: protoc-gen-go is not installed or not in PATH"
    echo "Please install with: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"
    exit 1
fi

if ! command -v protoc-gen-go-grpc &>/dev/null; then
    echo "Error: protoc-gen-go-grpc is not installed or not in PATH"
    echo "Please install with: go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest"
    exit 1
fi

# Set directories
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROTO_DIR="${SCRIPT_DIR}/proto"
OUTPUT_DIR="${SCRIPT_DIR}/proto/gen"

# Ensure output directory exists
mkdir -p "${OUTPUT_DIR}"

# Find all proto files
PROTO_FILES=$(find "${PROTO_DIR}" -name "*.proto" -type f)

# Print what we're about to do
echo "Generating Go code from Protocol Buffer definitions..."
echo "Proto files: ${PROTO_FILES}"
echo "Output directory: ${OUTPUT_DIR}"

# Generate Go code
protoc \
    --proto_path="${PROTO_DIR}" \
    --go_out="${OUTPUT_DIR}" \
    --go_opt=paths=source_relative \
    --go-grpc_out="${OUTPUT_DIR}" \
    --go-grpc_opt=paths=source_relative \
    ${PROTO_FILES}

echo "Code generation complete."
echo "Generated files:"
find "${OUTPUT_DIR}" -type f | sort
