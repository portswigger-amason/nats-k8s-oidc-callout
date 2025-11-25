# Kubernetes Client Package

This package provides a thread-safe ServiceAccount cache with Kubernetes informer integration for the NATS auth callout service.

## Architecture

### Components

**Cache** (`cache.go`)
- Thread-safe in-memory cache using `sync.RWMutex`
- Key format: `namespace/name`
- Stores parsed NATS permissions (publish/subscribe subjects)
- Automatic permission building with namespace isolation

**Client** (`client.go`)
- Kubernetes informer wrapper
- Watches ServiceAccounts cluster-wide
- Event handlers for ADD/UPDATE/DELETE events
- Integrates with cache for automatic updates

### Permission Model

**Default Permissions**
- All ServiceAccounts get namespace isolation: `<namespace>.>`
- Applied to both publish and subscribe permissions

**Additional Permissions**
- Annotations: `nats.io/allowed-pub-subjects` and `nats.io/allowed-sub-subjects`
- Format: Comma-separated NATS subjects (e.g., `"platform.events.>, shared.metrics.*"`)
- Whitespace is automatically trimmed
- Empty values are handled gracefully

**Example**
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: my-service
  namespace: production
  annotations:
    nats.io/allowed-pub-subjects: "platform.events.>, shared.metrics.*"
    nats.io/allowed-sub-subjects: "platform.commands.*, shared.status"
```

Results in:
- **Publish**: `production.>`, `platform.events.>`, `shared.metrics.*`
- **Subscribe**: `production.>`, `platform.commands.*`, `shared.status`

## Usage

### Creating a Client

```go
import (
    "k8s.io/client-go/informers"
    "k8s.io/client-go/kubernetes"
    "github.com/portswigger-tim/nats-k8s-oidc-callout/internal/k8s"
)

// Create Kubernetes client
kubeClient := kubernetes.NewForConfigOrDie(config)

// Create informer factory
informerFactory := informers.NewSharedInformerFactory(kubeClient, 0)

// Create our client
k8sClient := k8s.NewClient(informerFactory)

// Start the informer
stopCh := make(chan struct{})
informerFactory.Start(stopCh)
informerFactory.WaitForCacheSync(stopCh)

// Use the client
pubPerms, subPerms, found := k8sClient.GetPermissions("production", "my-service")
if found {
    fmt.Printf("Publish: %v\n", pubPerms)
    fmt.Printf("Subscribe: %v\n", subPerms)
}

// Shutdown gracefully
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
k8sClient.Shutdown(ctx)
```

## Testing

### Test Coverage
- **81.2%** coverage (exceeds 80% target)
- TDD approach (red-green-refactor)
- Uses fake Kubernetes client for testing
- Tests cover all event types (ADD/UPDATE/DELETE)

### Running Tests

```bash
# Run tests
go test ./internal/k8s/

# With coverage
go test -cover ./internal/k8s/

# Verbose
go test -v ./internal/k8s/
```

### Test Cases

**Cache Tests** (`cache_test.go`)
- ServiceAccount lookup with various annotation combinations
- Annotation parsing with edge cases (whitespace, trailing commas, etc.)
- Cache operations (upsert, delete)
- Thread-safety (implicitly tested)

**Client Tests** (`client_test.go`)
- Informer event handling (ADD/UPDATE/DELETE)
- Graceful shutdown
- Integration with cache

## Implementation Details

### Thread Safety
- `sync.RWMutex` for read-heavy workloads
- Read locks for `Get()` operations
- Write locks for `upsert()` and `delete()` operations

### Event Handling
- **ADD**: New ServiceAccount → cache entry created
- **UPDATE**: Existing ServiceAccount → cache entry updated
- **DELETE**: ServiceAccount removed → cache entry deleted
- Tombstone handling for delayed deletions

### Error Handling
- Uses `k8s.io/apimachinery/pkg/util/runtime.HandleError()` for non-fatal errors
- Type assertions with proper error reporting
- Graceful degradation on unexpected object types

## Future Enhancements

- [ ] Metrics: cache hits/misses, event processing times
- [ ] Logging: structured logging for event handling
- [ ] Validation: annotation format validation
- [ ] Performance: consider LRU cache if memory becomes a concern
