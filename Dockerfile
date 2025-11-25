# Runtime stage - using distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# Expect TARGETARCH to be set by docker buildx (amd64 or arm64)
ARG TARGETARCH

# Copy the pre-built binary for the target architecture from out/ directory
COPY out/nats-k8s-oidc-callout-linux-${TARGETARCH} /nats-k8s-oidc-callout

# Use nonroot user (UID 65532)
USER nonroot:nonroot

# Expose HTTP port for health checks and metrics
EXPOSE 8080

# Run the application
ENTRYPOINT ["/nats-k8s-oidc-callout"]
