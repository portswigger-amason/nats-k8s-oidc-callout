package auth

import (
	"errors"
	"testing"

	"github.com/portswigger-tim/nats-k8s-oidc-callout/internal/jwt"
)

// Mock JWT validator for testing
type mockJWTValidator struct {
	validateFunc func(token string) (*jwt.Claims, error)
}

func (m *mockJWTValidator) Validate(token string) (*jwt.Claims, error) {
	return m.validateFunc(token)
}

// Mock permissions provider for testing
type mockPermissionsProvider struct {
	getPermissionsFunc func(namespace, name string) ([]string, []string, bool)
}

func (m *mockPermissionsProvider) GetPermissions(namespace, name string) ([]string, []string, bool) {
	return m.getPermissionsFunc(namespace, name)
}

// TestHandler_Authorize_Success tests successful authorization flow
func TestHandler_Authorize_Success(t *testing.T) {
	// Mock JWT validator that returns valid claims
	jwtValidator := &mockJWTValidator{
		validateFunc: func(token string) (*jwt.Claims, error) {
			return &jwt.Claims{
				Namespace:      "hakawai",
				ServiceAccount: "hakawai-litellm-proxy",
			}, nil
		},
	}

	// Mock permissions provider that returns permissions
	permProvider := &mockPermissionsProvider{
		getPermissionsFunc: func(namespace, name string) ([]string, []string, bool) {
			if namespace == "hakawai" && name == "hakawai-litellm-proxy" {
				return []string{"hakawai.>", "platform.events.>"}, []string{"hakawai.>", "platform.commands.*"}, true
			}
			return nil, nil, false
		},
	}

	handler := NewHandler(jwtValidator, permProvider)

	req := &AuthRequest{
		Token: "valid.jwt.token",
	}

	resp := handler.Authorize(req)

	if !resp.Allowed {
		t.Error("Expected authorization to be allowed")
	}

	if resp.Error != "" {
		t.Errorf("Expected no error, got: %s", resp.Error)
	}

	expectedPub := []string{"hakawai.>", "platform.events.>"}
	expectedSub := []string{"hakawai.>", "platform.commands.*"}

	if !equalStringSlices(resp.PublishPermissions, expectedPub) {
		t.Errorf("PublishPermissions = %v, want %v", resp.PublishPermissions, expectedPub)
	}

	if !equalStringSlices(resp.SubscribePermissions, expectedSub) {
		t.Errorf("SubscribePermissions = %v, want %v", resp.SubscribePermissions, expectedSub)
	}
}

// TestHandler_Authorize_InvalidJWT tests JWT validation failures
func TestHandler_Authorize_InvalidJWT(t *testing.T) {
	tests := []struct {
		name        string
		jwtError    error
		expectedMsg string
	}{
		{
			name:        "Expired token",
			jwtError:    jwt.ErrExpiredToken,
			expectedMsg: "authorization failed",
		},
		{
			name:        "Invalid signature",
			jwtError:    jwt.ErrInvalidSignature,
			expectedMsg: "authorization failed",
		},
		{
			name:        "Invalid claims",
			jwtError:    jwt.ErrInvalidClaims,
			expectedMsg: "authorization failed",
		},
		{
			name:        "Missing K8s claims",
			jwtError:    jwt.ErrMissingK8sClaims,
			expectedMsg: "authorization failed",
		},
		{
			name:        "Generic error",
			jwtError:    errors.New("some validation error"),
			expectedMsg: "authorization failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock JWT validator that returns an error
			jwtValidator := &mockJWTValidator{
				validateFunc: func(token string) (*jwt.Claims, error) {
					return nil, tt.jwtError
				},
			}

			// Permissions provider won't be called
			permProvider := &mockPermissionsProvider{
				getPermissionsFunc: func(namespace, name string) ([]string, []string, bool) {
					t.Error("GetPermissions should not be called when JWT validation fails")
					return nil, nil, false
				},
			}

			handler := NewHandler(jwtValidator, permProvider)

			req := &AuthRequest{
				Token: "invalid.jwt.token",
			}

			resp := handler.Authorize(req)

			if resp.Allowed {
				t.Error("Expected authorization to be denied")
			}

			if resp.Error != tt.expectedMsg {
				t.Errorf("Error = %q, want %q", resp.Error, tt.expectedMsg)
			}

			if resp.PublishPermissions != nil {
				t.Error("Expected no PublishPermissions on failure")
			}

			if resp.SubscribePermissions != nil {
				t.Error("Expected no SubscribePermissions on failure")
			}
		})
	}
}

// TestHandler_Authorize_ServiceAccountNotFound tests when SA doesn't exist
func TestHandler_Authorize_ServiceAccountNotFound(t *testing.T) {
	// Mock JWT validator that returns valid claims
	jwtValidator := &mockJWTValidator{
		validateFunc: func(token string) (*jwt.Claims, error) {
			return &jwt.Claims{
				Namespace:      "production",
				ServiceAccount: "nonexistent-sa",
			}, nil
		},
	}

	// Mock permissions provider that returns not found
	permProvider := &mockPermissionsProvider{
		getPermissionsFunc: func(namespace, name string) ([]string, []string, bool) {
			return nil, nil, false
		},
	}

	handler := NewHandler(jwtValidator, permProvider)

	req := &AuthRequest{
		Token: "valid.jwt.token",
	}

	resp := handler.Authorize(req)

	if resp.Allowed {
		t.Error("Expected authorization to be denied")
	}

	if resp.Error != "authorization failed" {
		t.Errorf("Error = %q, want %q", resp.Error, "authorization failed")
	}

	if resp.PublishPermissions != nil {
		t.Error("Expected no PublishPermissions on failure")
	}

	if resp.SubscribePermissions != nil {
		t.Error("Expected no SubscribePermissions on failure")
	}
}

// TestHandler_Authorize_EmptyToken tests empty token handling
func TestHandler_Authorize_EmptyToken(t *testing.T) {
	// JWT validator shouldn't be called
	jwtValidator := &mockJWTValidator{
		validateFunc: func(token string) (*jwt.Claims, error) {
			t.Error("Validate should not be called with empty token")
			return nil, errors.New("should not be called")
		},
	}

	permProvider := &mockPermissionsProvider{
		getPermissionsFunc: func(namespace, name string) ([]string, []string, bool) {
			t.Error("GetPermissions should not be called with empty token")
			return nil, nil, false
		},
	}

	handler := NewHandler(jwtValidator, permProvider)

	req := &AuthRequest{
		Token: "",
	}

	resp := handler.Authorize(req)

	if resp.Allowed {
		t.Error("Expected authorization to be denied")
	}

	if resp.Error != "authorization failed" {
		t.Errorf("Error = %q, want %q", resp.Error, "authorization failed")
	}
}

// Helper function to compare string slices
func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
