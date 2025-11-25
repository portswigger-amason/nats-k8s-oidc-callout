# NATS Auth Callout Client Package

This package provides the NATS client that handles auth callout subscriptions and integrates with the internal authorization handler.

## Architecture

The client uses the `synadia-io/callout.go` library to simplify auth callout handling, focusing implementation on the business logic of extracting tokens and building permissions.

### Components

**Client** (`client.go`)
- Manages NATS connection and auth callout service lifecycle
- Generates NKey for signing authorization responses
- Bridges NATS authorization requests to internal auth handler
- Converts auth responses to NATS user claims with permissions

**Key Features:**
- Automatic token extraction from NATS connection options
- Generic error handling (rejections via timeout pattern)
- 5-minute token expiry for generated user JWTs
- Clean shutdown with connection cleanup

## Token Flow

```
1. NATS client connects with JWT token in connection options
      ↓
2. NATS server sends AuthorizationRequest to our service
      ↓
3. Extract JWT from ConnectOptions.JWT or ConnectOptions.Token
      ↓
4. Call internal auth handler with token
      ↓
5. If authorized: Build NATS user claims with permissions
      ↓
6. Sign user claims with our NKey
      ↓
7. Return encoded JWT to NATS server
      ↓
8. Client gets connected with appropriate permissions!
```

## Usage

### Creating and Starting the Client

```go
import (
    "context"
    "github.com/portswigger-tim/nats-k8s-oidc-callout/internal/nats"
    "github.com/portswigger-tim/nats-k8s-oidc-callout/internal/auth"
)

// Create auth handler (combines JWT validator + K8s permissions)
authHandler := auth.NewHandler(jwtValidator, k8sClient)

// Create NATS client
client, err := nats.NewClient("nats://localhost:4222", authHandler)
if err != nil {
    log.Fatalf("Failed to create NATS client: %v", err)
}

// Start the auth callout service
ctx := context.Background()
err = client.Start(ctx)
if err != nil {
    log.Fatalf("Failed to start auth callout service: %v", err)
}
defer client.Shutdown(context.Background())

// Service is now listening for auth callout requests!
```

### NATS Server Configuration

The NATS server must be configured to use auth callouts:

```
authorization {
  auth_callout {
    # Public key from our signing NKey
    issuer: "AABCDEFGHIJKLMNOPQRSTUVWXYZ..."

    # Auth service account (must exist in NATS)
    account: "AUTH"
  }
}
```

## Token Extraction

The client extracts JWT tokens from NATS connection options in this priority:

1. **ConnectOptions.JWT** - Standard NATS JWT field (preferred)
2. **ConnectOptions.Token** - Alternative auth_token field
3. Empty string if no token provided → rejection

Example NATS client connection with JWT:

```go
nc, err := nats.Connect("nats://localhost:4222",
    nats.UserJWT(
        func() (string, error) {
            return kubernetesJWT, nil
        },
        func(nonce []byte) ([]byte, error) {
            return nil, nil // No signing needed for JWT-only auth
        },
    ),
)
```

## Permission Mapping

Auth responses from our internal handler are mapped to NATS user claims:

**Internal Auth Response:**
```go
&auth.AuthResponse{
    Allowed:              true,
    PublishPermissions:   []string{"hakawai.>", "platform.events.>"},
    SubscribePermissions: []string{"hakawai.>", "platform.commands.*"},
}
```

**NATS User Claims:**
```go
uc := jwt.NewUserClaims(userNkey)
uc.Pub.Allow.Add("hakawai.>", "platform.events.>")
uc.Sub.Allow.Add("hakawai.>", "platform.commands.*")
uc.Expires = time.Now().Add(5 * time.Minute).Unix()
```

## Error Handling

The client follows NATS auth callout best practices:

**Authorization Denied:**
- Does NOT return a user JWT
- Returns an error which causes NATS to timeout the connection
- Client receives generic connection timeout (no information leakage)

**Why Timeout?**
- Security: Prevents attackers from distinguishing between "invalid JWT" vs "no permissions"
- Simplicity: Single rejection mechanism
- Standard: Follows NATS auth callout patterns

## Testing

### Test Coverage
- **29.7%** coverage (unit tests)
- Comprehensive unit tests for core logic
- Integration tests with real NATS server via testcontainers

### Test Organization

**Unit Tests** (`client_test.go`)
- No external dependencies
- Fast execution
- Run by default with `go test`

**Integration Tests** (`integration_test.go`)
- Requires Docker
- Uses testcontainers to spin up real NATS server
- Tests full auth callout flow
- Run with `-tags=integration` flag

### Test Cases

**Token Extraction:**
- JWT field extraction
- Token field extraction (fallback)
- Precedence (JWT > Token)
- Empty token handling

**Authorization Flow:**
- Successful authorization with permissions
- Authorization denied
- Empty token rejection

**User Claims:**
- Permission mapping (pub/sub)
- Expiration validation
- Multiple permissions
- Empty permissions
- Wildcard permissions

**Client Lifecycle:**
- Creation with different URLs
- Signing key initialization
- Shutdown cleanup

### Running Tests

```bash
# Run unit tests only (fast, no Docker required)
go test ./internal/nats/

# Run integration tests (requires Docker)
go test -tags=integration -v ./internal/nats/

# Run all tests (unit + integration)
make test-all

# With coverage
go test -cover ./internal/nats/

# Verbose
go test -v ./internal/nats/
```

**Using Makefile:**
```bash
# Unit tests only
make test

# Integration tests only (requires Docker)
make test-integration

# All tests
make test-all

# Coverage report
make coverage
```

## Design Decisions

### Why Use callout.go Library?

**Benefits:**
- Handles all NATS protocol details automatically
- Manages auth callout subscription
- Handles request/response encryption (XKeys)
- Simplifies our code to just business logic

**Our Responsibility:**
- Extract JWT from request
- Call internal auth handler
- Build NATS user claims from response
- Sign and encode the JWT

### Why 5-Minute Token Expiry?

- **Short-lived**: Limits damage if token is compromised
- **Reconnection**: Forces periodic re-authorization
- **Reasonable**: Allows normal operation without constant re-auth
- **Configurable**: Can be adjusted via `DefaultTokenExpiry` constant

### Why Not Return Detailed Errors?

**Security Best Practice:**
- Generic timeouts prevent information leakage
- Attackers can't distinguish failure reasons
- Detailed logging happens server-side (not sent to client)

## Integration Points

### With Auth Handler (`internal/auth/`)
- Receives `AuthRequest` with JWT token
- Returns `AuthResponse` with permissions or denial
- Implements `AuthHandler` interface

### With NATS Server
- Subscribes to auth callout subject
- Receives `AuthorizationRequest` with connection details
- Returns signed `UserClaims` JWT or error

### With Callout Library (`synadia-io/callout.go`)
- Provides authorizer function
- Library handles all NATS protocol details
- Library manages encryption and signing

## Future Enhancements

- [ ] Metrics: authorization success/failure rates, latency
- [ ] Logging: structured logs for authorization attempts
- [ ] XKey encryption: Optional encryption for request/response
- [ ] Connection pooling: Multiple NATS connections for HA
- [ ] Health checks: Monitor auth callout subscription health
