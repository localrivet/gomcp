# Auth Example

> **Secure MCP with JWT Authentication.** This example demonstrates robust JWT authentication for MCP servers and clients, showcasing best practices for securing tool access and managing user identity.

---

## Why JWT Authentication?

| Security Need       | How Auth Example Helps                              |
| ------------------- | --------------------------------------------------- |
| **Access Control**  | Restrict tool usage to valid, logged-in users       |
| **User Identity**   | Validate identities securely via JWT tokens         |
| **API Security**    | Prevent unauthorized access to sensitive operations |
| **Scalability**     | Utilize stateless JWTs suitable for microservices   |
| **Standardization** | Adhere to industry-standard authentication flows    |

The example provides a complete client-server flow:

1. **Server-side Authentication:** Validates incoming JWTs using JWKS and protects specific tools.
2. **Client-side Authentication:** Manages tokens (from env or test) and includes them in requests via hooks.

---

## Component Line-Up

| Component | Responsibility                                         |
| --------- | ------------------------------------------------------ |
| `server`  | Validates JWTs, uses auth hooks, serves protected tool |
| `client`  | Acquires token, uses auth hooks, calls protected tool  |

**Core Features & Tools**

- **Tool:** `secure-echo` - A simple tool that requires valid JWT authentication to execute.
- **JWT Validation:** Supports standard JWKS URLs for key discovery and validation.
- **Auth Hooks:** Demonstrates `BeforeHandleMessageHook` (server) and `BeforeSendRequestHook` (client).
- **Token Management:** Client uses environment variables or a fallback test token.
- **Context Propagation:** Securely passes authentication details via context.

---

## Architecture

```mermaid
flowchart TD
    subgraph Client Side
        direction LR
        Start --> GetToken[Get JWT Token (Env/Test)]
        GetToken --> Ctx[Add Token to Context]
        Ctx --> HookClient[Register Client Auth Hook]
        HookClient --> CallTool["Call 'secure-echo'"]
    end

    subgraph Server Side
        direction LR
        RecvReq["Receive Request w/ Token"] --> HookServer[Run Server Auth Hook]
        HookServer --> Validate[Validate Token (JWKS/Mock)]
        Validate -- Valid --> ExecTool[Execute 'secure-echo']
        Validate -- Invalid --> AuthError[Return Auth Error]
        ExecTool --> SendResp[Send Success Response]
    end

    CallTool --> RecvReq
    SendResp --> ClientResp[Client Receives Response]
    AuthError --> ClientResp
```

---

## Quick Start

> **Prerequisite:** Go 1.24+

1.  **Start the Server:**

    ```bash
    cd server
    go run .
    # Optionally set JWKS_URL, JWT_ISSUER, JWT_AUDIENCE env vars for real validation
    ```

2.  **Run the Client:**

    ```bash
    cd client
    # Uses test token by default
    go run .

    # Use a real JWT token
    JWT_TOKEN=your_real_jwt_token go run .
    ```

---

## Features Deep Dive

### Server-Side (`server/main.go`)

- **Token Validation:** Uses `auth.NewJWKSTokenValidator` or a mock validator.
- **Auth Hook:** Leverages `auth.NewAuthenticationHook` to intercept messages.
- **Protected Tool:** `secure-echo` handler only runs if the hook successfully validates the token.
- **Error Handling:** Returns specific `ErrorCodeMCPAuthenticationFailed` on validation failure.

### Client-Side (`client/main.go`)

- **Token Acquisition:** Reads `JWT_TOKEN` environment variable or uses a demo token.
- **Context Setup:** Uses `auth.ContextWithToken` to store the token.
- **Request Hook:** Implements `hooks.ClientBeforeSendRequestHook` (though in this SSE example, the transport uses the context token directly).
- **Authenticated Calls:** Demonstrates calling the protected tool with and without a valid token context.

---

## Extending the Example

- **Role-Based Access:** Modify the auth hook to check for specific roles or permissions within the JWT claims.
- **Other Auth Methods:** Integrate API Key authentication alongside or instead of JWT.
- **Token Refresh:** Implement client-side logic to handle JWT expiration and refresh.
- **Granular Security:** Apply authentication selectively to only certain tools or parameters.
- **Audit Logging:** Add detailed logging for authentication successes and failures.

---

## Project Structure

```
auth/
├── client/
│   └── main.go       # JWT auth client: manages tokens, calls server
├── server/
│   └── main.go       # Protected MCP server: validates tokens, serves secure tool
├── go.mod            # Module dependencies
└── README.md         # You are here
```
