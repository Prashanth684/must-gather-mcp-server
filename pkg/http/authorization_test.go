package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/stretchr/testify/suite"

	"github.com/openshift/must-gather-mcp-server/pkg/config"
)

type AuthorizationSuite struct {
	suite.Suite
}

func (s *AuthorizationSuite) TestParseJWTClaims() {
	s.Run("parses valid JWT", func() {
		// Create a simple JWT for testing
		token := s.createTestToken(time.Now().Add(time.Hour), "test-audience")

		claims, err := ParseJWTClaims(token)
		s.NoError(err)
		s.NotNil(claims)
		s.Equal(token, claims.Token)
	})

	s.Run("returns error for invalid JWT", func() {
		_, err := ParseJWTClaims("invalid.jwt.token")
		s.Error(err)
	})

	s.Run("parses JWT with scopes", func() {
		token := s.createTestTokenWithScopes(time.Now().Add(time.Hour), "test-aud", "read write admin")

		claims, err := ParseJWTClaims(token)
		s.NoError(err)
		s.NotNil(claims)

		scopes := claims.GetScopes()
		s.Len(scopes, 3)
		s.Contains(scopes, "read")
		s.Contains(scopes, "write")
		s.Contains(scopes, "admin")
	})
}

func (s *AuthorizationSuite) TestValidateOffline() {
	s.Run("validates token with correct audience", func() {
		token := s.createTestToken(time.Now().Add(time.Hour), "test-audience")
		claims, err := ParseJWTClaims(token)
		s.Require().NoError(err)

		err = claims.ValidateOffline("test-audience")
		s.NoError(err)
	})

	s.Run("validates token without audience check", func() {
		token := s.createTestToken(time.Now().Add(time.Hour), "any-audience")
		claims, err := ParseJWTClaims(token)
		s.Require().NoError(err)

		err = claims.ValidateOffline("")
		s.NoError(err)
	})

	s.Run("rejects expired token", func() {
		token := s.createTestToken(time.Now().Add(-time.Hour), "test-audience")
		claims, err := ParseJWTClaims(token)
		s.Require().NoError(err)

		err = claims.ValidateOffline("test-audience")
		s.Error(err)
		s.Contains(err.Error(), "validation")
	})

	s.Run("rejects token with wrong audience", func() {
		token := s.createTestToken(time.Now().Add(time.Hour), "wrong-audience")
		claims, err := ParseJWTClaims(token)
		s.Require().NoError(err)

		err = claims.ValidateOffline("expected-audience")
		s.Error(err)
	})
}

func (s *AuthorizationSuite) TestAuthorizationMiddleware() {
	s.Run("allows requests when OAuth not required", func() {
		cfg := &config.StaticConfig{
			RequireOAuth: false,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		req := httptest.NewRequest("GET", "/mcp", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
	})

	s.Run("allows health check without auth", func() {
		cfg := &config.StaticConfig{
			RequireOAuth: true,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		req := httptest.NewRequest("GET", "/healthz", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
	})

	s.Run("allows well-known endpoints without auth", func() {
		cfg := &config.StaticConfig{
			RequireOAuth: true,
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
	})

	s.Run("rejects request without bearer token", func() {
		cfg := &config.StaticConfig{
			RequireOAuth:  true,
			OAuthAudience: "test-aud",
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		req := httptest.NewRequest("GET", "/mcp", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusUnauthorized, rec.Code)
		s.Contains(rec.Header().Get("WWW-Authenticate"), "Bearer")
		s.Contains(rec.Header().Get("WWW-Authenticate"), "audience=\"test-aud\"")
	})

	s.Run("rejects request with invalid token", func() {
		cfg := &config.StaticConfig{
			RequireOAuth:  true,
			OAuthAudience: "test-aud",
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		req := httptest.NewRequest("GET", "/mcp", nil)
		req.Header.Set("Authorization", "Bearer invalid.token.here")
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusUnauthorized, rec.Code)
	})

	s.Run("accepts request with valid token", func() {
		cfg := &config.StaticConfig{
			RequireOAuth:  true,
			OAuthAudience: "test-aud",
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		token := s.createTestToken(time.Now().Add(time.Hour), "test-aud")

		req := httptest.NewRequest("GET", "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusOK, rec.Code)
	})

	s.Run("rejects expired token", func() {
		cfg := &config.StaticConfig{
			RequireOAuth:  true,
			OAuthAudience: "test-aud",
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		token := s.createTestToken(time.Now().Add(-time.Hour), "test-aud")

		req := httptest.NewRequest("GET", "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusUnauthorized, rec.Code)
	})

	s.Run("rejects token with wrong audience", func() {
		cfg := &config.StaticConfig{
			RequireOAuth:  true,
			OAuthAudience: "expected-aud",
		}

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := AuthorizationMiddleware(cfg, nil)
		wrapped := middleware(handler)

		token := s.createTestToken(time.Now().Add(time.Hour), "wrong-aud")

		req := httptest.NewRequest("GET", "/mcp", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)
		s.Equal(http.StatusUnauthorized, rec.Code)
	})
}

func (s *AuthorizationSuite) TestValidateWithProvider() {
	s.Run("skips validation when provider is nil", func() {
		token := s.createTestToken(time.Now().Add(time.Hour), "test-aud")
		claims, err := ParseJWTClaims(token)
		s.Require().NoError(err)

		err = claims.ValidateWithProvider(context.Background(), "test-aud", nil)
		s.NoError(err)
	})

	// Note: Testing with a real OIDC provider would require a mock OIDC server
	// which is complex to set up. In production code, integration tests would
	// cover this scenario with a real or mock OIDC provider.
}

// Helper functions

func (s *AuthorizationSuite) createTestToken(exp time.Time, audience string) string {
	return s.createTestTokenWithScopes(exp, audience, "")
}

func (s *AuthorizationSuite) createTestTokenWithScopes(exp time.Time, audience, scopes string) string {
	// Create a simple test key
	key := []byte("test-secret-key-for-testing-purposes-only")

	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.HS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	s.Require().NoError(err)

	claims := JWTClaims{
		Claims: jwt.Claims{
			Subject:   "test-user",
			Issuer:    "test-issuer",
			Audience:  jwt.Audience{audience},
			Expiry:    jwt.NewNumericDate(exp),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-time.Minute)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-time.Minute)),
		},
		Scope: scopes,
	}

	token, err := jwt.Signed(signer).Claims(claims).Serialize()
	s.Require().NoError(err)

	return token
}

func TestAuthorizationSuite(t *testing.T) {
	suite.Run(t, new(AuthorizationSuite))
}
