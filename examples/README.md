# GoMCP Examples

This directory contains example MCP servers and clients demonstrating the usage of the `gomcp` library.

## Running the Examples

The examples are designed to communicate over standard input/output (stdio). To run them, you typically need two terminals or use shell piping to connect the server's stdout to the client's stdin and vice-versa.

### Multi-Tool Example (`server/` and `client/`)

This is the main example demonstrating multiple tools.

**Using Piping:**

```bash
# This command runs the multi-tool server and the standard client
go run ./examples/server/*.go | go run ./examples/client/main.go
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

## Examples Included

- **`server/` & `client/`**: The primary example demonstrating a server offering multiple tools (`echo`, `calculator`, `filesystem`) and a client that uses them all. The `filesystem` tool operates within a `./fs_sandbox` directory.
- **`auth-server/` & `auth-client/`**: Demonstrates a simple API key authentication mechanism (via environment variable `MCP_API_KEY=test-key-123`) required for the server to start.
- **`rate-limit-server/` & `rate-limit-client/`**: Builds on the auth example, adding simple global rate limiting (2 requests/sec, burst 4) to the server's tool. The client sends requests rapidly to demonstrate hitting the limit.
- **`billing-server/` & `billing-client/`**: Builds on the auth example, simulating billing/tracking by logging a structured event to stderr before executing a tool call.

### Auth Example (`auth-server/` and `auth-client/`)

**Using Piping:**

```bash
# Set the required API key and run the auth server and client
export MCP_API_KEY="test-key-123"
go run ./examples/auth-server/main.go | go run ./examples/auth-client/main.go

# Example of running with the wrong key (server will fail to start)
export MCP_API_KEY="wrong-key"
go run ./examples/auth-server/main.go | go run ./examples/auth-client/main.go
```

### Rate Limit Example (`rate-limit-server/` and `rate-limit-client/`)

**Using Piping:**

```bash
# Set the required API key and run the rate-limited server and client
export MCP_API_KEY="test-key-123"
go run ./examples/rate-limit-server/main.go | go run ./examples/rate-limit-client/main.go
```

### Billing/Tracking Simulation Example (`billing-server/` and `billing-client/`)

**Using Piping:**

```bash
# Set the required API key and run the billing server and client
export MCP_API_KEY="test-key-123"
go run ./examples/billing-server/main.go | go run ./examples/billing-client/main.go
```

### Running the Multi-Tool Server with Docker

A `Dockerfile` is provided in `examples/server/` to build a container image for the multi-tool server. A `docker-compose.yml` file is also provided at the project root primarily for building the image.

**Build the image:**

```bash
# From the repository root directory
docker compose build mcp-server
# Or using docker build directly:
# docker build -t gomcp-server-example -f examples/server/Dockerfile .
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
go run ./examples/client/main.go | docker exec -i my_mcp_server sh -c 'cat > /dev/stdin'
```

You will see the server logs within the `docker run` terminal, and the client logs in the terminal where you ran the client.

### Running the Multi-Tool Server with Kubernetes

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
go run ./examples/client/main.go | kubectl exec -i $POD_NAME -- sh -c 'cat > /dev/stdin'
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
