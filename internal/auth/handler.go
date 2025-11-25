package auth

import (
	"github.com/portswigger-tim/nats-k8s-oidc-callout/internal/jwt"
)

// JWTValidator defines the interface for JWT validation
type JWTValidator interface {
	Validate(token string) (*jwt.Claims, error)
}

// PermissionsProvider defines the interface for retrieving ServiceAccount permissions
type PermissionsProvider interface {
	GetPermissions(namespace, name string) (pubPerms []string, subPerms []string, found bool)
}

// AuthRequest represents an authorization request
type AuthRequest struct {
	Token string
}

// AuthResponse represents the authorization response
type AuthResponse struct {
	Allowed              bool
	PublishPermissions   []string
	SubscribePermissions []string
	Error                string
}

// Handler handles authorization requests
type Handler struct {
	jwtValidator JWTValidator
	permProvider PermissionsProvider
}

// NewHandler creates a new authorization handler
func NewHandler(jwtValidator JWTValidator, permProvider PermissionsProvider) *Handler {
	return &Handler{
		jwtValidator: jwtValidator,
		permProvider: permProvider,
	}
}

// Authorize processes an authorization request and returns the response
func (h *Handler) Authorize(req *AuthRequest) *AuthResponse {
	// Validate input
	if req.Token == "" {
		return &AuthResponse{
			Allowed: false,
			Error:   "authorization failed",
		}
	}

	// Validate JWT and extract claims
	claims, err := h.jwtValidator.Validate(req.Token)
	if err != nil {
		// Generic error message to client, detailed logging would happen elsewhere
		return &AuthResponse{
			Allowed: false,
			Error:   "authorization failed",
		}
	}

	// Look up permissions from K8s ServiceAccount
	pubPerms, subPerms, found := h.permProvider.GetPermissions(claims.Namespace, claims.ServiceAccount)
	if !found {
		return &AuthResponse{
			Allowed: false,
			Error:   "authorization failed",
		}
	}

	// Success
	return &AuthResponse{
		Allowed:              true,
		PublishPermissions:   pubPerms,
		SubscribePermissions: subPerms,
	}
}
