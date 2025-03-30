# GoMCP Examples

This directory contains example MCP servers and clients demonstrating the usage of the `gomcp` library. The examples are organized into subdirectories by functionality.

## Directory Structure

- **`basic/`**: The primary example demonstrating a server offering multiple tools (`echo`, `calculator`, `filesystem`) and a client that uses them all.
- **`auth/`**: Demonstrates a simple API key authentication mechanism.
- **`rate-limit/`**: Builds on the auth example, adding simple global rate limiting.
- **`billing/`**: Builds on the auth example, simulating billing/tracking by logging structured events.
- **`kitchen-sink/`**: A comprehensive server example demonstrating various MCP features.
- **`sqlite/`**: Demonstrates using a SQLite database to store and query user data.

## Running the Examples

The examples are designed to communicate over standard input/output (stdio). To run them, you typically need two terminals or use shell piping to connect the server's stdout to the client's stdin and vice-versa.

### Basic Multi-Tool Example (`basic/server/` and `basic/client/`)

This is the main example demonstrating multiple tools.

**Using Piping:**

```bash
# This command runs the multi-tool server and the standard client
go run ./examples/basic/server/*.go | go run ./examples/basic/client/main.go
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
go run ./examples/auth/server/main.go | go run ./examples/auth/client/main.go

# Example of running with the wrong key (server will fail to start)
export MCP_API_KEY="wrong-key"
go run ./examples/auth/server/main.go | go run ./examples/auth/client/main.go
```

### Rate Limit Example (`rate-limit/server/` and `rate-limit/client/`)

**Using Piping:**

```bash
# Set the required API key and run the rate-limited server and client
export MCP_API_KEY="test-key-123"
go run ./examples/rate-limit/server/main.go | go run ./examples/rate-limit/client/main.go
```

### Billing/Tracking Simulation Example (`billing/server/` and `billing/client/`)

**Using Piping:**

```bash
# Set the required API key and run the billing server and client
export MCP_API_KEY="test-key-123"
go run ./examples/billing/server/main.go | go run ./examples/billing/client/main.go
```

### SQLite Database Example (`sqlite/server/` and `sqlite/client/`)

This example demonstrates using a SQLite database to store and query user data.

**Using Piping:**

```bash
# Run the SQLite server and client
go run ./examples/sqlite/server/main.go | go run ./examples/sqlite/client/main.go
```

The server provides the following tools:

- `getUserById`: Get a user by their ID
- `listUsers`: List all users, optionally filtered by active status
- `searchUsers`: Search for users by name or email

The client demonstrates using all of these tools to query the database.

### Kitchen Sink Server Example (`kitchen-sink/server/`)

This is a comprehensive server example demonstrating various MCP features.

```bash
# Run the kitchen-sink server
go run ./examples/kitchen-sink/server/main.go
```

## Running the Basic Multi-Tool Server with Docker

A `Dockerfile` is provided in `examples/basic/server/` to build a container image for the multi-tool server. A `docker-compose.yml` file is also provided at the project root primarily for building the image.

**Build the image:**

```bash
# From the repository root directory
docker compose build mcp-server
# Or using docker build directly:
# docker build -t gomcp-server-example -f examples/basic/server/Dockerfile .
```

**Run the container interactively:**

Since the server uses stdio, you need to run it interactively.

```bash
docker run -i --rm --name my_mcp_server gomcp-server-example
```

**Interact with the container (Requires separate terminal):**

You can run a client locally and pipe its output to the container. _Note: Piping stdio to/from Docker containers can be complex and shell-dependent._

```bash
# Example: Run the standard client and pipe to the running container
go run ./examples/basic/client/main.go | docker exec -i my_mcp_server sh -c 'cat > /dev/stdin'
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
