# NATS Kubernetes OIDC Auth Callout - Claude Context

## Project Overview

This is a Go-based NATS auth callout service that validates Kubernetes service account JWTs and provides subject-based authorization for NATS clients running in Kubernetes clusters.

## Key Design Decisions

### Architecture Pattern
- **NATS subject-based auth callout**: Service subscribes to NATS authorization request subjects
- **Kubernetes informer pattern**: Watch ServiceAccounts cluster-wide for annotation changes
- **Lazy-load caching**: Cache on first auth request, with K8s watch keeping cache up-to-date
- **12-factor app**: All configuration via environment variables

### Permission Model
- **Default**: Namespace isolation (services can only pub/sub to `<namespace>.>`)
- **Opt-in cross-namespace**: ServiceAccounts use annotations to grant additional access
- **Separate pub/sub controls**: `nats.io/allowed-pub-subjects` and `nats.io/allowed-sub-subjects`

### Security Principles
- **Generic errors to clients**: "authorization failed" for all validation failures
- **Detailed logging/metrics**: Capture specific failure reasons for debugging
- **Principle of least privilege**: Default to minimal access, explicit grants only
- **Full JWT validation**: Signature, standard claims, K8s-specific claims

## Project Structure

```
cmd/server/main.go          - Entry point, wiring components
internal/config/            - Environment variable configuration
internal/nats/              - NATS connection & subscription handling
internal/jwt/               - JWT validation & JWKS handling
internal/k8s/               - ServiceAccount cache (informer pattern)
internal/auth/              - Authorization request handler & permission builder
internal/http/              - Health & metrics endpoints
```

## Dependencies

- `github.com/nats-io/nats.go` - NATS client
- `github.com/nats-io/nkeys` - NKey cryptography
- `k8s.io/client-go` - Kubernetes API client
- `github.com/golang-jwt/jwt/v5` - JWT parsing
- `github.com/MicahParks/keyfunc/v2` - JWKS key fetching
- `github.com/prometheus/client_golang` - Metrics
- `go.uber.org/zap` - Structured logging

## Open Implementation Questions

These details will be validated during implementation:

1. **NATS subject pattern**: Exact subject for auth callout subscription
2. **Request/response format**: NATS authorization JWT structure and encryption (XKey)
3. **Auth service NKey**: How to generate and manage the service's signing key
4. **JWKS caching**: TTL and refresh strategy for K8s public keys

## Testing Strategy

- **Unit tests**: Each internal package with mocks
- **Integration tests**: Embedded NATS server + envtest for K8s
- **Manual testing**: Deploy to kind/k3s with real NATS server
- **Coverage target**: >80% on business logic

## Related Documentation

- Design document: `docs/plans/2025-11-24-nats-k8s-auth-design.md`
- NATS auth callout docs: https://docs.nats.io/running-a-nats-service/configuration/securing_nats/auth_callout
- NATS auth callout example: https://natsbyexample.com/examples/auth/callout/cli

## Development Guidelines

- Follow standard Go project layout
- Use structured logging (zap) with consistent fields
- Instrument all operations with Prometheus metrics
- Handle errors gracefully with proper context
- Write tests alongside implementation
- Document public APIs and complex logic
