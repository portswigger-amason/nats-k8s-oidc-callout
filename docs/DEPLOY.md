# Deploying nats-k8s-oidc-callout

This guide covers how to deploy and configure the NATS Kubernetes OIDC auth callout service.

## Prerequisites

- Kubernetes cluster (1.20+)
- `kubectl` configured to access your cluster
- `nsc` (NATS CLI tools) installed
- NATS server (2.9+) with auth callout support
- Docker (for building images)

## Overview

The deployment process involves:
1. Setting up NATS server with auth callout configuration
2. Creating NATS credentials for the auth service
3. Configuring Kubernetes RBAC for ServiceAccount watching
4. Deploying the auth callout service
5. Configuring client ServiceAccounts with NATS permissions

## 1. NATS Server Setup

### Install NATS with Helm

```bash
# Add NATS Helm repository
helm repo add nats https://nats-io.github.io/k8s/helm/charts/
helm repo update

# Create namespace
kubectl create namespace nats

# Install NATS with auth callout enabled
helm install nats nats/nats \
  --namespace nats \
  --set config.cluster.enabled=true \
  --set config.jetstream.enabled=true
```

### Configure Auth Callout

Create a NATS configuration file with auth callout enabled:

```conf
# nats-auth-callout.conf
port: 4222

authorization {
  # Auth callout configuration
  auth_callout {
    # Account for clients to be assigned to
    account: AUTH_ACCOUNT

    # Subject the auth service listens on
    auth_users: [ "_AUTH_" ]

    # Authorization request subject
    xkey: <XKEY_PUBLIC_KEY>
  }
}

accounts {
  # Account for the auth callout service itself
  AUTH_SERVICE: {
    users: [
      {
        nkey: <AUTH_SERVICE_NKEY>
      }
    ]
  }

  # Account that clients will be assigned to
  AUTH_ACCOUNT: {
    # Will be populated by auth service
  }
}
```

**Key components:**
- `AUTH_SERVICE` account - Used by the auth callout service
- `AUTH_ACCOUNT` account - Clients get assigned here after authentication
- `xkey` - Public key for signing auth responses (XKey from auth service)
- `auth_users` - Subject the auth service subscribes to

## 2. Generate NATS Credentials

### Create Operator and Accounts

```bash
# Create operator
nsc add operator --name MyOperator

# Create AUTH_SERVICE account for the auth callout service
nsc add account --name AUTH_SERVICE
nsc add user --account AUTH_SERVICE --name auth-service

# Create AUTH_ACCOUNT for clients
nsc add account --name AUTH_ACCOUNT

# Generate credentials file
nsc generate creds --account AUTH_SERVICE --user auth-service > auth-service.creds

# Get the public XKey for the auth service (for NATS config)
# You'll generate this in the application startup or use nkeys CLI
```

### Extract Information from Credentials

```bash
# View the credentials
cat auth-service.creds

# Extract the seed (keep this secret!)
# The .creds file contains both JWT and seed
```

### Create Kubernetes Secret

```bash
# Create secret with NATS credentials
kubectl create secret generic nats-auth-creds \
  --from-file=auth.creds=auth-service.creds \
  --namespace=nats-auth
```

## 3. Kubernetes RBAC Setup

The auth service needs permission to watch ServiceAccounts cluster-wide.

### Create ServiceAccount

```yaml
# serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: nats-auth-callout
  namespace: nats-auth
```

### Create ClusterRole

```yaml
# clusterrole.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: nats-auth-callout
rules:
  # Watch ServiceAccounts cluster-wide
  - apiGroups: [""]
    resources: ["serviceaccounts"]
    verbs: ["get", "list", "watch"]

  # Create TokenRequest for testing (optional)
  - apiGroups: [""]
    resources: ["serviceaccounts/token"]
    verbs: ["create"]
```

### Create ClusterRoleBinding

```yaml
# clusterrolebinding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: nats-auth-callout
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: nats-auth-callout
subjects:
  - kind: ServiceAccount
    name: nats-auth-callout
    namespace: nats-auth
```

### Apply RBAC Resources

```bash
kubectl apply -f serviceaccount.yaml
kubectl apply -f clusterrole.yaml
kubectl apply -f clusterrolebinding.yaml
```

## 4. Get Kubernetes OIDC Information

### Find Your Cluster's OIDC Issuer

```bash
# For EKS
aws eks describe-cluster --name <cluster-name> \
  --query 'cluster.identity.oidc.issuer' --output text

# For GKE
gcloud container clusters describe <cluster-name> \
  --format='value(workloadIdentityConfig.issuerUri)'

# For generic Kubernetes (from API server flags)
kubectl cluster-info dump | grep oidc-issuer-url
```

### Get JWKS URL

The JWKS URL is typically:
- **In-cluster**: `https://kubernetes.default.svc/openid/v1/jwks` (default)
- **External**: `https://<oidc-issuer>/openid/v1/jwks`

### Verify OIDC Discovery

```bash
# Test OIDC discovery endpoint (from within cluster)
curl https://kubernetes.default.svc/.well-known/openid-configuration

# Or from outside (if accessible)
curl https://<oidc-issuer>/.well-known/openid-configuration
```

## 5. Deploy Auth Callout Service

### Create Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nats-auth-callout
  namespace: nats-auth
  labels:
    app: nats-auth-callout
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nats-auth-callout
  template:
    metadata:
      labels:
        app: nats-auth-callout
    spec:
      serviceAccountName: nats-auth-callout
      containers:
        - name: nats-auth-callout
          image: nats-k8s-oidc-callout:latest
          ports:
            - containerPort: 8080
              name: http
              protocol: TCP
          env:
            # NATS connection
            - name: NATS_URL
              value: "nats://nats.nats.svc.cluster.local:4222"
            - name: NATS_CREDS_FILE
              value: "/etc/nats/auth.creds"
            - name: NATS_ACCOUNT
              value: "AUTH_ACCOUNT"

            # Kubernetes JWT validation (defaults work for in-cluster)
            - name: K8S_IN_CLUSTER
              value: "true"
            # JWKS_URL and JWT_ISSUER will auto-default
            - name: JWT_AUDIENCE
              value: "nats"

            # Optional: Override defaults if needed
            # - name: JWKS_URL
            #   value: "https://kubernetes.default.svc/openid/v1/jwks"
            # - name: JWT_ISSUER
            #   value: "https://kubernetes.default.svc"

            # Logging
            - name: LOG_LEVEL
              value: "info"

            # HTTP server
            - name: PORT
              value: "8080"

          volumeMounts:
            - name: nats-creds
              mountPath: /etc/nats
              readOnly: true

          livenessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 10
            periodSeconds: 30

          readinessProbe:
            httpGet:
              path: /healthz
              port: http
            initialDelaySeconds: 5
            periodSeconds: 10

          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi

      volumes:
        - name: nats-creds
          secret:
            secretName: nats-auth-creds
```

### Create Service

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: nats-auth-callout
  namespace: nats-auth
  labels:
    app: nats-auth-callout
spec:
  type: ClusterIP
  ports:
    - port: 8080
      targetPort: http
      protocol: TCP
      name: http
  selector:
    app: nats-auth-callout
```

### Apply Deployment

```bash
kubectl create namespace nats-auth
kubectl apply -f deployment.yaml
kubectl apply -f service.yaml
```

### Verify Deployment

```bash
# Check pods are running
kubectl get pods -n nats-auth

# Check logs
kubectl logs -n nats-auth -l app=nats-auth-callout

# Check health endpoint
kubectl port-forward -n nats-auth svc/nats-auth-callout 8080:8080
curl http://localhost:8080/healthz
```

## 6. Configure Client ServiceAccounts

Clients use ServiceAccount annotations to define NATS permissions.

### Create ServiceAccount with Permissions

```yaml
# client-serviceaccount.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-nats-client
  namespace: my-app
  annotations:
    # Publish permissions
    nats.io/allowed-pub-subjects: "my-app.requests.>,my-app.events.>"

    # Subscribe permissions
    nats.io/allowed-sub-subjects: "my-app.responses.>,my-app.commands.>"
```

**Permission patterns:**
- `my-app.>` - All subjects under my-app
- `my-app.requests.*` - Single-level wildcard
- `my-app.requests.>` - Multi-level wildcard
- Separate multiple patterns with commas

### Default Permissions

Without annotations, clients can only access their own namespace:
- Publish: `<namespace>.>`
- Subscribe: `<namespace>.>`

## 7. Client Application Setup

### Get ServiceAccount Token

Your application needs a Kubernetes ServiceAccount token with the correct audience.

```yaml
# client-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-nats-client
  namespace: my-app
spec:
  template:
    spec:
      serviceAccountName: my-nats-client
      containers:
        - name: app
          image: my-app:latest
          env:
            - name: NATS_URL
              value: "nats://nats.nats.svc.cluster.local:4222"

            # Token will be mounted automatically
            - name: NATS_TOKEN_FILE
              value: "/var/run/secrets/nats/token"

          volumeMounts:
            # Projected volume for token with correct audience
            - name: nats-token
              mountPath: /var/run/secrets/nats
              readOnly: true

      volumes:
        - name: nats-token
          projected:
            sources:
              - serviceAccountToken:
                  path: token
                  expirationSeconds: 3600
                  audience: nats
```

### Connect to NATS

Example Go code:

```go
package main

import (
    "os"
    "github.com/nats-io/nats.go"
)

func main() {
    // Read token
    token, err := os.ReadFile("/var/run/secrets/nats/token")
    if err != nil {
        panic(err)
    }

    // Connect with token
    nc, err := nats.Connect(
        os.Getenv("NATS_URL"),
        nats.Token(string(token)),
    )
    if err != nil {
        panic(err)
    }
    defer nc.Close()

    // Use connection
    nc.Publish("my-app.requests.test", []byte("hello"))
}
```

## 8. Testing & Verification

### Test Authentication

```bash
# Get a test token
kubectl create token my-nats-client \
  --namespace=my-app \
  --audience=nats \
  --duration=1h > test-token.txt

# Test connection using nats CLI
nats --server=nats://nats.nats.svc.cluster.local:4222 \
  --token=$(cat test-token.txt) \
  pub my-app.test "hello"
```

### Check Metrics

```bash
# Port forward to metrics endpoint
kubectl port-forward -n nats-auth svc/nats-auth-callout 8080:8080

# View Prometheus metrics
curl http://localhost:8080/metrics
```

### View Logs

```bash
# Follow logs
kubectl logs -n nats-auth -l app=nats-auth-callout -f

# Check for auth events
kubectl logs -n nats-auth -l app=nats-auth-callout | grep "authorization"
```

## 9. Troubleshooting

### Connection Issues

```bash
# Verify NATS server is accessible
kubectl run -it --rm debug --image=natsio/nats-box --restart=Never \
  -- nats server check --server=nats://nats.nats.svc.cluster.local:4222
```

### Token Validation Failures

Check JWT claims:

```bash
# Decode token (example using jwt.io or jwt CLI)
echo "TOKEN" | jwt decode -

# Verify issuer and audience match configuration
```

### Permission Denials

```bash
# Check ServiceAccount annotations
kubectl get serviceaccount my-nats-client -n my-app -o yaml

# Check auth service logs for permission decisions
kubectl logs -n nats-auth -l app=nats-auth-callout | grep "permission"
```

### JWKS Errors

```bash
# Verify JWKS endpoint is accessible from pod
kubectl exec -it -n nats-auth <pod-name> -- \
  wget -O- https://kubernetes.default.svc/openid/v1/jwks
```

## 10. Production Considerations

### High Availability

- Run multiple replicas (2-3 minimum)
- Use pod anti-affinity to spread across nodes
- Configure resource requests and limits appropriately

### Security

- Use Network Policies to restrict traffic
- Regularly rotate NATS credentials
- Monitor for unauthorized access attempts
- Use TLS for NATS connections in production

### Monitoring

- Set up alerts on authentication failures
- Monitor cache hit rates
- Track permission denial patterns
- Set up dashboards for key metrics

### Scaling

The auth service is stateless and scales horizontally. Consider:
- HorizontalPodAutoscaler based on CPU/memory
- Connection pooling if using many clients
- Cache tuning for large clusters

## Reference

### Environment Variables

See main README.md for complete list of configuration options.

Required:
- `NATS_CREDS_FILE` - Path to NATS credentials
- `NATS_ACCOUNT` - NATS account to assign clients

Optional (with defaults for in-cluster):
- `NATS_URL` - Default: `nats://nats:4222`
- `JWKS_URL` - Default: `https://kubernetes.default.svc/openid/v1/jwks`
- `JWT_ISSUER` - Default: `https://kubernetes.default.svc`
- `JWT_AUDIENCE` - Default: `nats`
- `LOG_LEVEL` - Default: `info`

### Annotation Format

ServiceAccount annotations:
- `nats.io/allowed-pub-subjects` - Comma-separated publish subjects
- `nats.io/allowed-sub-subjects` - Comma-separated subscribe subjects

Subject patterns:
- `*` - Single token wildcard
- `>` - Multi-token wildcard (must be last token)
