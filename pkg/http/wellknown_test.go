package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/openshift/must-gather-mcp-server/pkg/config"
)

type WellKnownSuite struct {
	suite.Suite
}

func (s *WellKnownSuite) TestWellKnownHandler() {
	s.Run("returns 404 for unknown well-known path", func() {
		cfg := &config.StaticConfig{}
		handler := WellKnownHandler(cfg, nil)

		req := httptest.NewRequest("GET", "/.well-known/unknown", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusNotFound, rec.Code)
	})

	s.Run("returns 405 for non-GET request", func() {
		cfg := &config.StaticConfig{}
		handler := WellKnownHandler(cfg, nil)

		req := httptest.NewRequest("POST", "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusMethodNotAllowed, rec.Code)
	})

	s.Run("returns minimal metadata when OAuth not required", func() {
		cfg := &config.StaticConfig{
			RequireOAuth: false,
			ServerURL:    "https://example.com",
		}
		handler := WellKnownHandler(cfg, nil)

		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusOK, rec.Code)
		s.Equal("application/json", rec.Header().Get("Content-Type"))

		var metadata OAuthServerMetadata
		err := json.NewDecoder(rec.Body).Decode(&metadata)
		s.NoError(err)
		s.Equal("https://example.com", metadata.Issuer)
	})

	s.Run("returns minimal metadata when no authorization URL", func() {
		cfg := &config.StaticConfig{
			RequireOAuth: true,
			ServerURL:    "https://example.com",
		}
		handler := WellKnownHandler(cfg, nil)

		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusOK, rec.Code)

		var metadata OAuthServerMetadata
		err := json.NewDecoder(rec.Body).Decode(&metadata)
		s.NoError(err)
		s.Equal("https://example.com", metadata.Issuer)
	})

	s.Run("includes configured scopes in metadata", func() {
		// Create a mock OIDC server
		var serverURL string
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/openid-configuration" {
				metadata := OAuthServerMetadata{
					Issuer:                 serverURL,
					AuthorizationEndpoint:  serverURL + "/authorize",
					TokenEndpoint:          serverURL + "/token",
					ScopesSupported:        []string{"openid"},
					ResponseTypesSupported: []string{"code"},
					GrantTypesSupported:    []string{"authorization_code"},
				}
				json.NewEncoder(w).Encode(metadata)
				return
			}
			http.NotFound(w, r)
		}))
		defer mockServer.Close()
		serverURL = mockServer.URL

		cfg := &config.StaticConfig{
			RequireOAuth:     true,
			AuthorizationURL: mockServer.URL,
			OAuthScopes:      []string{"openid", "profile", "email"},
			ServerURL:        "https://example.com",
		}
		handler := WellKnownHandler(cfg, mockServer.Client())

		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusOK, rec.Code)

		var metadata OAuthServerMetadata
		err := json.NewDecoder(rec.Body).Decode(&metadata)
		s.NoError(err)
		s.Equal(mockServer.URL, metadata.Issuer)
		s.Contains(metadata.ScopesSupported, "openid")
		s.Contains(metadata.ScopesSupported, "profile")
		s.Contains(metadata.ScopesSupported, "email")
	})

	s.Run("removes registration endpoint when disabled", func() {
		// Create a mock OIDC server
		var serverURL string
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/.well-known/openid-configuration" {
				metadata := OAuthServerMetadata{
					Issuer:               serverURL,
					RegistrationEndpoint: serverURL + "/register",
				}
				json.NewEncoder(w).Encode(metadata)
				return
			}
			http.NotFound(w, r)
		}))
		defer mockServer.Close()
		serverURL = mockServer.URL

		cfg := &config.StaticConfig{
			RequireOAuth:                     true,
			AuthorizationURL:                 mockServer.URL,
			DisableDynamicClientRegistration: true,
			ServerURL:                        "https://example.com",
		}
		handler := WellKnownHandler(cfg, mockServer.Client())

		req := httptest.NewRequest("GET", "/.well-known/oauth-authorization-server", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		s.Equal(http.StatusOK, rec.Code)

		var metadata OAuthServerMetadata
		err := json.NewDecoder(rec.Body).Decode(&metadata)
		s.NoError(err)
		s.Empty(metadata.RegistrationEndpoint)
	})
}

func (s *WellKnownSuite) TestGetIssuer() {
	s.Run("uses ServerURL from config", func() {
		cfg := &config.StaticConfig{
			ServerURL: "https://example.com",
		}

		req := httptest.NewRequest("GET", "/test", nil)
		req.Host = "localhost:8080"

		issuer := getIssuer(req, cfg)
		s.Equal("https://example.com", issuer)
	})

	s.Run("constructs from request when no ServerURL", func() {
		cfg := &config.StaticConfig{}

		req := httptest.NewRequest("GET", "/test", nil)
		req.Host = "localhost:8080"

		issuer := getIssuer(req, cfg)
		s.Equal("http://localhost:8080", issuer)
	})

	s.Run("uses X-Forwarded-Proto header", func() {
		cfg := &config.StaticConfig{}

		req := httptest.NewRequest("GET", "/test", nil)
		req.Host = "example.com"
		req.Header.Set("X-Forwarded-Proto", "https")

		issuer := getIssuer(req, cfg)
		s.Equal("https://example.com", issuer)
	})
}

func TestWellKnownSuite(t *testing.T) {
	suite.Run(t, new(WellKnownSuite))
}
