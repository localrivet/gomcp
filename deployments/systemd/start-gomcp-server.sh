#!/bin/bash

# Simple startup script for the GoMCP Multi-Tool Server managed by systemd.

# Determine the directory where this script and the executable reside.
# This makes the script location-independent within the deployment directory.
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" &>/dev/null && pwd)"

# --- Configuration ---
# Set any required environment variables here.
# Example for a server requiring an API key:
# export MCP_API_KEY="your_production_key_here"

# Path to the server executable (expected to be in the same directory as this script)
EXECUTABLE="$DIR/multi-tool-server"

# --- Pre-flight Checks ---
# Check if the executable file exists
if [ ! -f "$EXECUTABLE" ]; then
    echo "Error: Server executable not found at $EXECUTABLE" >&2
    exit 1 # Exit with error code
fi

# Check if the file is actually executable
if [ ! -x "$EXECUTABLE" ]; then
    echo "Error: Server executable at $EXECUTABLE is not executable (run chmod +x)" >&2
    exit 1 # Exit with error code
fi

# --- Execution ---
echo "Starting GoMCP Multi-Tool Server from $DIR..."
# Use 'exec' to replace the shell process with the server process.
# This ensures signals (like SIGTERM from systemd) are passed directly to the Go application.
exec "$EXECUTABLE"

# The script should not reach here if exec is successful
echo "Error: Failed to exec $EXECUTABLE" >&2
exit 1
