# Authorization Handler Package

This package provides the core authorization logic that ties together JWT validation and Kubernetes ServiceAccount permissions.

## Architecture

### Clean Interface Design

The handler uses **dependency injection** with interfaces for testability:

```go
type JWTValidator interface {
    Validate(token string) (*jwt.Claims, error)
}

type PermissionsProvider interface {
    GetPermissions(namespace, name string) ([]string, []string, bool)
}
```

This design allows:
- Easy unit testing with mock implementations
- Decoupling from concrete implementations
- Future flexibility (e.g., caching, multiple providers)

### Data Flow

```
AuthRequest (JWT token)
    ↓
JWT Validation (extract namespace + service account)
    ↓
Permissions Lookup (from K8s cache)
    ↓
AuthResponse (pub/sub permissions or error)
```

## Usage

### Creating a Handler

```go
import (
    "github.com/portswigger-tim/nats-k8s-oidc-callout/internal/auth"
    "github.com/portswigger-tim/nats-k8s-oidc-callout/internal/jwt"
    "github.com/portswigger-tim/nats-k8s-oidc-callout/internal/k8s"
)

// Create dependencies
jwtValidator := jwt.NewValidatorFromURL("https://k8s.example.com/.well-known/jwks.json")
k8sClient := k8s.NewClient(informerFactory)

// Create handler
authHandler := auth.NewHandler(jwtValidator, k8sClient)

// Process authorization request
req := &auth.AuthRequest{
    Token: "eyJhbGciOiJSUzI1NiIsImtpZCI6...",
}

resp := authHandler.Authorize(req)

if resp.Allowed {
    fmt.Printf("Publish: %v\n", resp.PublishPermissions)
    fmt.Printf("Subscribe: %v\n", resp.SubscribePermissions)
} else {
    fmt.Printf("Denied: %s\n", resp.Error)
}
```

### Request/Response

**AuthRequest:**
```go
type AuthRequest struct {
    Token string  // JWT token from client
}
```

**AuthResponse:**
```go
type AuthResponse struct {
    Allowed              bool      // Authorization decision
    PublishPermissions   []string  // NATS subjects client can publish to
    SubscribePermissions []string  // NATS subjects client can subscribe to
    Error                string    // Generic error message (if denied)
}
```

## Security Design

### Generic Error Messages

All authorization failures return the same generic message: `"authorization failed"`

**Why?**
- Prevents information leakage to clients
- Attackers can't distinguish between:
  - Invalid JWT
  - Expired token
  - ServiceAccount not found
  - Missing permissions

**Detailed logging** happens elsewhere (with structured fields for debugging).

### Validation Flow

1. **Empty token check** - Fast fail for missing tokens
2. **JWT validation** - Signature, claims, expiration
3. **Permissions lookup** - ServiceAccount must exist in cache
4. **Success response** - Only if all checks pass

## Testing

### Test Coverage
- **100%** coverage (all statements)
- TDD approach (tests written first)
- Mock implementations for dependencies
- Comprehensive error handling tests

### Test Cases

**Success Flow:**
- Valid JWT + existing ServiceAccount → permissions returned

**JWT Validation Failures:**
- Expired token
- Invalid signature
- Invalid claims
- Missing Kubernetes claims
- Generic validation errors

**ServiceAccount Failures:**
- ServiceAccount not found in cache
- Empty token

### Running Tests

```bash
# Run tests
go test ./internal/auth/

# With coverage
go test -cover ./internal/auth/

# Verbose
go test -v ./internal/auth/
```

### Example Test

```go
func TestHandler_Authorize_Success(t *testing.T) {
    // Mock JWT validator
    jwtValidator := &mockJWTValidator{
        validateFunc: func(token string) (*jwt.Claims, error) {
            return &jwt.Claims{
                Namespace:      "production",
                ServiceAccount: "my-service",
            }, nil
        },
    }

    // Mock permissions provider
    permProvider := &mockPermissionsProvider{
        getPermissionsFunc: func(ns, name string) ([]string, []string, bool) {
            return []string{"production.>"}, []string{"production.>"}, true
        },
    }

    handler := auth.NewHandler(jwtValidator, permProvider)

    req := &auth.AuthRequest{Token: "valid.jwt.token"}
    resp := handler.Authorize(req)

    if !resp.Allowed {
        t.Error("Expected authorization to be allowed")
    }
}
```

## Integration Points

### With JWT Validator (`internal/jwt/`)
- Implements `JWTValidator` interface
- Extracts Kubernetes namespace and service account claims
- Returns typed errors for different failure modes

### With K8s Client (`internal/k8s/`)
- Implements `PermissionsProvider` interface
- Returns publish and subscribe permissions
- Handles ServiceAccount not found case

### With NATS Client (`internal/nats/`)
- Will be called by NATS auth callout subscription handler
- Receives requests, returns responses
- Next component to implement!

## Design Decisions

### Why Interfaces?
- **Testability**: Mock implementations for unit tests
- **Flexibility**: Easy to swap implementations
- **Decoupling**: Handler doesn't depend on concrete types

### Why No Logging in Handler?
- **Separation of concerns**: Handler is pure business logic
- **Flexibility**: Caller controls logging (structured logs, levels, etc.)
- **Testability**: No side effects in tests

### Why Generic Errors?
- **Security**: Prevent information leakage
- **Consistency**: Same experience for all failures
- **Simplicity**: Single error code for NATS clients

## Future Enhancements

- [ ] Metrics: authorization success/failure rates
- [ ] Caching: Short-lived cache for repeated authorizations
- [ ] Audit logging: Structured logs with all auth attempts
- [ ] Rate limiting: Per-namespace or per-ServiceAccount limits
