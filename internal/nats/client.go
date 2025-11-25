package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/jwt/v2"
	natsclient "github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
	"github.com/synadia-io/callout.go"

	"github.com/portswigger-tim/nats-k8s-oidc-callout/internal/auth"
)

const (
	// DefaultTokenExpiry is the default expiry time for generated NATS user tokens
	DefaultTokenExpiry = 5 * time.Minute
)

// AuthHandler defines the interface for authorization
type AuthHandler interface {
	Authorize(req *auth.AuthRequest) *auth.AuthResponse
}

// Client manages NATS connection and auth callout subscription
type Client struct {
	url         string
	authHandler AuthHandler
	conn        *natsclient.Conn
	service     *callout.AuthorizationService
	signingKey  nkeys.KeyPair
}

// NewClient creates a new NATS auth callout client
func NewClient(url string, authHandler AuthHandler) (*Client, error) {
	// Generate signing key for responses
	signingKey, err := nkeys.CreateAccount()
	if err != nil {
		return nil, fmt.Errorf("failed to create signing key: %w", err)
	}

	return &Client{
		url:         url,
		authHandler: authHandler,
		signingKey:  signingKey,
	}, nil
}

// SetSigningKey sets the signing key for the client (useful for testing)
func (c *Client) SetSigningKey(key nkeys.KeyPair) {
	c.signingKey = key
}

// Start connects to NATS and starts the auth callout service
func (c *Client) Start(ctx context.Context) error {
	// Connect to NATS with timeout
	conn, err := natsclient.Connect(c.url,
		natsclient.Timeout(5*time.Second),
		natsclient.Name("nats-k8s-oidc-callout"),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	c.conn = conn

	// Create authorizer function that bridges NATS and our auth handler
	authorizer := func(req *jwt.AuthorizationRequest) (string, error) {
		// Extract JWT token from request
		// The token is provided by the client in the connection options
		// For now, we'll extract it from the ConnectOptions if available
		token := extractToken(req)

		if token == "" {
			// Reject requests without a token by not returning a JWT
			// This causes the connection to timeout
			return "", fmt.Errorf("no token provided")
		}

		// Call our auth handler
		authReq := &auth.AuthRequest{
			Token: token,
		}
		authResp := c.authHandler.Authorize(authReq)

		// If denied, reject by not returning a JWT
		if !authResp.Allowed {
			return "", fmt.Errorf("authorization failed")
		}

		// Build NATS user claims
		uc := jwt.NewUserClaims(req.UserNkey)
		uc.Pub.Allow.Add(authResp.PublishPermissions...)
		uc.Sub.Allow.Add(authResp.SubscribePermissions...)
		uc.Expires = time.Now().Add(DefaultTokenExpiry).Unix()

		// Encode and return JWT
		return uc.Encode(c.signingKey)
	}

	// Create auth callout service
	service, err := callout.NewAuthorizationService(
		conn,
		callout.Authorizer(authorizer),
		callout.ResponseSignerKey(c.signingKey),
	)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create authorization service: %w", err)
	}

	c.service = service
	return nil
}

// Shutdown gracefully shuts down the client
func (c *Client) Shutdown(ctx context.Context) error {
	if c.service != nil {
		c.service.Stop()
	}

	if c.conn != nil {
		c.conn.Close()
	}

	return nil
}

// extractToken extracts the JWT token from the authorization request
// The token should be provided by the client in the connection options
func extractToken(req *jwt.AuthorizationRequest) string {
	// Check for JWT in connect options (standard field)
	if req.ConnectOptions.JWT != "" {
		return req.ConnectOptions.JWT
	}

	// Alternative: check for auth_token field
	if req.ConnectOptions.Token != "" {
		return req.ConnectOptions.Token
	}

	return ""
}
