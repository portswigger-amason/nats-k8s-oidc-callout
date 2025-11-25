# Building nats-k8s-oidc-callout

This document describes how to build the nats-k8s-oidc-callout application for multiple architectures.

## Prerequisites

- Go 1.23 or later
- Docker with buildx support (for multi-arch image builds)
- Make

## Quick Start

```bash
# Build binaries for all architectures
make build-all

# Build Docker image
make docker-build
```

## Build Process

The project uses a two-stage build process:

1. **Binary compilation** - Go binaries are built outside Docker for multiple architectures
2. **Docker packaging** - Pre-built binaries are copied into minimal distroless images

This approach provides:
- Faster builds (native Go compilation)
- Better caching
- Easier multi-architecture support
- Smaller final images

### Output Directory

All build artifacts are placed in the `out/` directory, which is ignored by git. This keeps the repository clean and makes it easy to clean up build artifacts with `make clean`.

## Available Make Targets

### Build Targets

```bash
# Build for current architecture
make build

# Build for all architectures (amd64 + arm64)
make build-all

# Build for specific architecture
make build-amd64
make build-arm64
```

### Docker Targets

```bash
# Build multi-arch Docker image for local testing
make docker-build

# Build and push multi-arch Docker image to registry
make docker-push
```

### Test Targets

```bash
# Run unit tests
make test

# Run all tests (unit + integration + e2e)
make test-all

# Run tests with coverage
make coverage
```

### Utility Targets

```bash
# Display version information
make version

# Clean build artifacts
make clean

# Show help
make help
```

## Build Configuration

The build process uses the following configuration:

- **Binary name**: `nats-k8s-oidc-callout`
- **Version**: Determined from git tags (`git describe --tags`)
- **Commit**: Short git commit hash
- **Build date**: UTC timestamp
- **LDFLAGS**: Strips debug info for smaller binaries (`-w -s`)

## Docker Image

### Base Image

The Docker image uses `gcr.io/distroless/static-debian12:nonroot` which provides:
- Minimal attack surface (no shell, package manager, or unnecessary tools)
- Non-root user (UID 65532)
- Static binary support
- CA certificates for HTTPS connections

### Multi-Architecture Support

The Dockerfile uses Docker's `TARGETARCH` build argument to select the correct pre-built binary:

```dockerfile
ARG TARGETARCH
COPY out/nats-k8s-oidc-callout-linux-${TARGETARCH} /nats-k8s-oidc-callout
```

This allows building images for both amd64 and arm64 with a single Dockerfile.

### Building Images

```bash
# Build for local testing (loads into local Docker)
make docker-build

# Build and push to registry
make docker-push
```

The `docker-build` and `docker-push` targets automatically:
1. Build binaries for amd64 and arm64
2. Build Docker images for both architectures
3. Create a multi-arch manifest

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Build and Push Image

on:
  push:
    branches: [main]
    tags: ['v*']

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'

      - name: Build binaries
        run: make build-all

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v2

      - name: Login to registry
        uses: docker/login-action@v2
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push
        run: |
          export VERSION=${GITHUB_REF#refs/tags/}
          make docker-push
```

## Manual Build Steps

If you prefer to build manually without Make:

```bash
# Create output directory
mkdir -p out

# Build for amd64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s" \
  -o out/nats-k8s-oidc-callout-linux-amd64 \
  ./cmd/server

# Build for arm64
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -ldflags="-w -s" \
  -o out/nats-k8s-oidc-callout-linux-arm64 \
  ./cmd/server

# Build Docker image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t nats-k8s-oidc-callout:latest \
  --load \
  .
```

## Troubleshooting

### Docker buildx not available

```bash
# Create a new buildx builder
docker buildx create --use

# Verify buildx is working
docker buildx inspect --bootstrap
```

### Binary size

The binaries are statically compiled and stripped, resulting in ~30MB per architecture. To further reduce size:

1. Use UPX compression (not recommended for production)
2. Build with `-ldflags="-w -s"` (already enabled)
3. Use Go build tags to exclude unused features

### Cross-compilation issues

If you encounter cross-compilation errors:

1. Ensure you're using Go 1.23 or later
2. Set `CGO_ENABLED=0` to disable CGO
3. Verify `GOOS` and `GOARCH` are set correctly
