# NATS Kubernetes OIDC Auth Callout

A NATS auth callout service that authenticates and authorizes NATS clients using Kubernetes service account JWTs.

## Overview

This service integrates NATS with Kubernetes authentication, allowing workloads to connect to NATS using their projected service account tokens. It validates JWTs against the Kubernetes JWKS endpoint and maps Kubernetes identities to NATS subject permissions.

## Features

- **JWT Validation**: Validates Kubernetes service account tokens against JWKS endpoint
- **Namespace Isolation**: Default subject permissions scoped to pod namespace (`<namespace>.>`)
- **Cross-Namespace Access**: Opt-in via ServiceAccount annotations for broader permissions
- **Separate Pub/Sub Controls**: Fine-grained control over publish and subscribe permissions
- **Real-time Updates**: Kubernetes watch-based caching keeps permissions current
- **Cloud-Native**: 12-factor app design with environment-based configuration
- **Observability**: Health checks, Prometheus metrics, and structured logging

## Quick Start

### Prerequisites

- Kubernetes cluster with OIDC token projection configured
- NATS server with auth callout enabled
- Service account with permissions to watch ServiceAccounts cluster-wide

### Configuration

Configure via environment variables:

```bash
# NATS Connection
NATS_URL=nats://nats:4222
NATS_CREDS_FILE=/etc/nats/auth.creds
NATS_ACCOUNT=MyAccount

# Kubernetes JWT Validation
JWKS_URL=https://kubernetes.default.svc/openid/v1/jwks
JWT_ISSUER=https://kubernetes.default.svc
JWT_AUDIENCE=nats

# ServiceAccount Annotations
SA_ANNOTATION_PREFIX=nats.io/
CACHE_CLEANUP_INTERVAL=15m
```

### Granting Cross-Namespace Access

Annotate ServiceAccounts to grant additional subject permissions:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service
  namespace: foo
  annotations:
    nats.io/allowed-pub-subjects: "bar.>, platform.commands.*"
    nats.io/allowed-sub-subjects: "platform.events.*, shared.status"
```

This grants:
- **Publish**: `foo.>`, `bar.>`, `platform.commands.*`
- **Subscribe**: `foo.>`, `platform.events.*`, `shared.status`

## Development Status

### ✅ Implemented
- CLI application with graceful shutdown
- Environment-based configuration
- HTTP server with health checks and Prometheus metrics
- **JWT Validation** - Full token validation with:
  - JWKS-based signature verification (RS256)
  - Standard claims validation (iss, aud, exp, nbf, iat)
  - Kubernetes-specific claims extraction
  - Comprehensive test coverage using TDD

### 🚧 In Progress
- NATS client integration
- Kubernetes ServiceAccount cache
- Authorization request handler

### 📋 Planned
- NATS auth callout subscription
- Permission building from annotations
- End-to-end integration tests

## Architecture

See [design document](docs/plans/2025-11-24-nats-k8s-auth-design.md) for detailed architecture and design decisions.

### Key Components

- **JWT Validator**: ✅ Validates K8s tokens and claims
- **HTTP Server**: ✅ Health and metrics endpoints (port 8080)
- **NATS Client**: 📋 Subscribes to auth callout subjects
- **ServiceAccount Cache**: 📋 Real-time watch of K8s ServiceAccounts
- **Permission Builder**: 📋 Maps K8s identity to NATS permissions

## Observability

### Health Check

```bash
curl http://localhost:8080/health
```

### Metrics

Prometheus metrics available at `http://localhost:8080/metrics`:

- `nats_auth_requests_total` - Authorization request counts
- `jwt_validation_duration_seconds` - JWT validation latency
- `sa_cache_size` - Current cache size
- `k8s_api_calls_total` - Kubernetes API call counts

## Development

### Building

```bash
go build -o nats-k8s-auth ./cmd/server
```

### Testing

```bash
go test ./...
```

### Running Locally

```bash
# Set required environment variables
export NATS_URL=nats://localhost:4222
export JWKS_URL=https://kubernetes.default.svc/openid/v1/jwks
# ... other vars

./nats-k8s-auth
```

## License

See [LICENSE](LICENSE) file.
