# GoMCP Examples

This directory contains example MCP servers and clients demonstrating the usage of the `gomcp` library. The examples are organized into subdirectories by functionality.

## Directory Structure

Each subdirectory contains an example demonstrating a specific feature or transport. Most examples are now self-contained Go modules.

- **`basic/`**: Demonstrates stdio communication with multiple tools (`echo`, `calculator`, `filesystem`).
- **`http/`**: Contains subdirectories for various HTTP frameworks/routers (Chi, Echo, Fiber, Gin, Go-Zero, Gorilla/Mux, HttpRouter, Beego, Iris, Net/HTTP) integrated with the SSE transport.
- **`websocket/`**: Demonstrates the WebSocket transport.
- **`configuration/`**: Shows loading server configuration from JSON, YAML, and TOML files, with separate server examples for each format.
- **`cmd/`**: Provides generic command-line client and server implementations (using stdio by default, potentially configurable).
- **`auth/`**: Demonstrates simple API key authentication (stdio).
- **`rate-limit/`**: Builds on the auth example, adding simple global rate limiting (stdio).
- **`billing/`**: Builds on the auth example, simulating billing/tracking (stdio).
- **`kitchen-sink/`**: A comprehensive server example combining multiple features (stdio).
<!-- - **`sqlite/`**: (Removed - verify existence if needed) -->

## Running the Examples

Most examples are now Go modules. The general way to run them is:

1.  Navigate (`cd`) into the specific example's directory (e.g., `examples/basic/server`).
2.  Run the server using `go run .`.
3.  If the example has a corresponding client within the same parent directory (e.g., `examples/basic/client`), open another terminal, `cd` into the client directory, and run it using `go run .`.

**Note:**

- Examples using **stdio** (`basic`, `auth`, `billing`, `rate-limit`, `kitchen-sink`) require the client and server to be connected. You can run them in separate terminals and manually copy/paste, or use shell piping (see specific examples below).
- Examples using **network transports** (SSE in `http/`, `websocket/`) require the server to be running first, and then a compatible client needs to connect to the specified address and port. The generic client in `examples/cmd/gomcp-client` might be adaptable, or you might need a specific client (e.g., a browser for SSE/WebSocket).
- The **configuration** examples (`configuration/{json,yaml,toml}/server/`) load their respective config files (`../config.{json,yaml,toml}`) and start the transport specified within that file.

### Basic Multi-Tool Example (`basic/`)

This is the main example demonstrating multiple tools.

**Using Piping:**

```bash
# cd into the examples directory first if you aren't already there
(cd basic/server && go run .) | (cd basic/client && go run .)
```

**Expected Output:**

When run successfully, you will see log messages printed to **stderr** from both the server and the client, detailing the steps:

1.  Server starts.
2.  Client starts.
3.  Handshake occurs.
4.  Client requests tool definitions.
5.  Server sends tool definitions (echo, calculator, filesystem).
6.  Client prints received definitions.
7.  Client uses the `echo` tool.
8.  Server executes `echo`, sends result.
9.  Client prints `echo` result.
10. Client uses the `calculator` tool (add, divide by zero, missing arg).
11. Server executes `calculator`, sends results/errors.
12. Client prints `calculator` results/errors.
13. Client uses the `filesystem` tool (list, write, read, read non-existent, write outside sandbox).
14. Server executes `filesystem`, sends results/errors.
15. Client prints `filesystem` results/errors.
16. Client finishes.
17. Server detects client disconnection (EOF) and finishes.

### Auth Example (`auth/server/` and `auth/client/`)

**Using Piping:**

```bash
# Set the required API key and run the auth server and client
export MCP_API_KEY="test-key-123"
(cd auth/server && go run .) | (cd auth/client && go run .)

# Example of running with the wrong key (server will likely exit quickly)
export MCP_API_KEY="wrong-key"
(cd auth/server && go run .) | (cd auth/client && go run .)
```

### Rate Limit Example (`rate-limit/server/` and `rate-limit/client/`)

**Using Piping:**

```bash
# Set the required API key and run the rate-limited server and client
export MCP_API_KEY="test-key-123"
(cd rate-limit/server && go run .) | (cd rate-limit/client && go run .)
```

### Billing/Tracking Simulation Example (`billing/server/` and `billing/client/`)

**Using Piping:**

```bash
# Set the required API key and run the billing server and client
export MCP_API_KEY="test-key-123"
(cd billing/server && go run .) | (cd billing/client && go run .)
```

### Kitchen Sink Server Example (`kitchen-sink/`)

This is a comprehensive server example demonstrating various MCP features.

```bash
# Run the kitchen-sink server and client (assuming a client exists)
# Replace client command if needed
(cd kitchen-sink/server && go run .) | (cd kitchen-sink/client && go run .)
```

### HTTP Framework Examples (`http/`)

These examples demonstrate integrating `gomcp` with various Go web frameworks using the SSE transport.

1.  Choose a framework (e.g., `gin`).
2.  Start the server: `cd examples/http/gin/server && go run .`
3.  Connect using an SSE-compatible MCP client (e.g., configure and run `examples/cmd/gomcp-client` or use a browser-based client) targeting the server's address (e.g., `http://127.0.0.1:8084`).

### WebSocket Example (`websocket/`)

This example demonstrates the WebSocket transport.

1.  Start the server: `cd examples/websocket/server && go run .`
2.  Connect using a WebSocket-compatible MCP client (e.g., configure and run `examples/cmd/gomcp-client` or use a browser-based client) targeting the server's WebSocket endpoint (e.g., `ws://127.0.0.1:8092/mcp`).

### Configuration Examples (`configuration/`)

These examples show loading server settings from different file formats.

1.  Choose a format (e.g., `json`).
2.  Start the server: `cd examples/configuration/json/server && go run .`
3.  The server will load `../config.json` and start the transport specified within (e.g., WebSocket for the default JSON config).
4.  Connect using a client compatible with the transport defined in the config file.

---

## Deployment Examples (Docker, Kubernetes, systemd)

**Note:** The following deployment examples currently target the **`basic/server`** (stdio multi-tool) example. Adapting them for network-based transports (SSE, WebSocket) would require additional configuration (e.g., exposing ports, handling network clients).

## Running the Basic Multi-Tool Server with Docker

A `Dockerfile` is provided in `examples/basic/server/` to build a container image for the multi-tool server. A `docker-compose.yml` file is also provided at the project root primarily for building the image.

**Build the image:**

```bash
# From the repository root directory
docker compose build gomcp-server
# Or using docker build directly:
# docker build -t gomcp-server-example -f examples/basic/server/Dockerfile .
```

**Run the container interactively:**

Since the server uses stdio, you need to run it interactively.

```bash
docker run -i --rm --name my_gomcp_server gomcp-server-example
```

**Interact with the container (Requires separate terminal):**

You can run a client locally and pipe its output to the container. _Note: Piping stdio to/from Docker containers can be complex and shell-dependent._

```bash
# Example: Run the standard client and pipe to the running container
go run ./examples/basic/client/main.go | docker exec -i my_gomcp_server sh -c 'cat > /dev/stdin'
```

You will see the server logs within the `docker run` terminal, and the client logs in the terminal where you ran the client.

### Running the Basic Multi-Tool Server with Kubernetes

Basic Kubernetes manifests (`Deployment`, headless `Service`) are provided in the `deployments/kubernetes/` directory.

**Prerequisites:**

- A running Kubernetes cluster.
- `kubectl` configured to interact with your cluster.
- The `gomcp-server-example:latest` Docker image built (see Docker section above) and pushed to a registry accessible by your cluster (or available locally on cluster nodes if using `imagePullPolicy: IfNotPresent`). You may need to update the `image:` field in `deployment.yaml` to point to your specific registry.

**Deploy:**

```bash
# From the repository root directory
kubectl apply -f deployments/kubernetes/deployment.yaml
kubectl apply -f deployments/kubernetes/service.yaml
```

**Check Status:**

```bash
kubectl get deployment gomcp-server-deployment
kubectl get pods -l app=gomcp-server
```

**Interact (Requires separate terminal):**

Since the service is headless and the application uses stdio, you need to interact directly with the Pod using `kubectl attach` or `kubectl exec`.

```bash
# Find the pod name (it will have a random suffix)
POD_NAME=$(kubectl get pods -l app=gomcp-server -o jsonpath='{.items[0].metadata.name}')

# Option 1: Attach (might require specific terminal settings)
# kubectl attach -i $POD_NAME

# Option 2: Exec and pipe (similar to Docker, can be tricky)
# Run the client locally and pipe its output to the pod's stdin
go run ./examples/basic/client/main.go | kubectl exec -i $POD_NAME -- sh -c 'cat > /dev/stdin'
```

**View Logs:**

```bash
kubectl logs deployment/gomcp-server-deployment -f
```

**Cleanup:**

```bash
kubectl delete -f deployments/kubernetes/service.yaml
kubectl delete -f deployments/kubernetes/deployment.yaml
```

### Running the Basic Multi-Tool Server with systemd (Linux VM)

Files are provided in `deployments/systemd/` to run the multi-tool server as a `systemd` service on a Linux virtual machine (e.g., EC2, DigitalOcean Droplet).

**Prerequisites:**

- A Linux VM with `systemd`.
- Go installed on the VM (or just the pre-built binary).

**Setup Steps:**

1.  **Build the Executable:** On your development machine or the VM (if Go is installed), build the server executable specifically for Linux:
    ```bash
    # From the repository root directory
    CGO_ENABLED=0 GOOS=linux go build -o ./deployments/systemd/multi-tool-server ./examples/basic/server/*.go
    ```
2.  **Create Deployment Directory on VM:**
    ```bash
    # On the target VM
    sudo mkdir -p /opt/gomcp-server
    sudo chown your_user:your_group /opt/gomcp-server # Or the user who will run the service
    ```
    _(Replace `your_user:your_group` appropriately. Consider creating a dedicated user.)_
3.  **Copy Files to VM:** Copy the built `multi-tool-server` executable and the `start-gomcp-server.sh` script to `/opt/gomcp-server` on the VM.
    ```bash
    # On your development machine (using scp example)
    scp ./deployments/systemd/multi-tool-server user@your_vm_ip:/opt/gomcp-server/
    scp ./deployments/systemd/start-gomcp-server.sh user@your_vm_ip:/opt/gomcp-server/
    ```
4.  **Make Script Executable on VM:**
    ```bash
    # On the target VM
    chmod +x /opt/gomcp-server/start-gomcp-server.sh
    ```
5.  **Copy systemd Unit File:** Copy `deployments/systemd/gomcp-server.service` to `/etc/systemd/system/` on the VM.
    ```bash
    # On your development machine (using scp example)
    scp ./deployments/systemd/gomcp-server.service user@your_vm_ip:/tmp/
    # On the target VM
    sudo mv /tmp/gomcp-server.service /etc/systemd/system/
    ```
6.  **Edit Unit File (IMPORTANT):** Edit `/etc/systemd/system/gomcp-server.service` on the VM.
    - Ensure `WorkingDirectory` and `ExecStart` paths match your deployment directory (`/opt/gomcp-server` in this example).
    - Change the `User` and `Group` to an appropriate non-root user if you created one.
7.  **Enable and Start the Service:**
    ```bash
    # On the target VM
    sudo systemctl daemon-reload
    sudo systemctl enable gomcp-server.service
    sudo systemctl start gomcp-server.service
    ```

**Check Status:**

```bash
sudo systemctl status gomcp-server.service
journalctl -u gomcp-server.service -f # View logs
```

**Interact:**

Since this runs as a background service, direct stdio interaction isn't possible in the same way as the piped `go run` command. You would typically need:

- A client running on the same VM that can interact with the service's process (complex).
- Or, modify the server/library to use a different transport mechanism like network sockets or WebSockets, which can then be accessed remotely.

**Stop/Disable:**

```bash
sudo systemctl stop gomcp-server.service
sudo systemctl disable gomcp-server.service
```
