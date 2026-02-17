package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"k8s.io/klog/v2"

	"github.com/openshift/must-gather-mcp-server/pkg/config"
)

// WellKnownEndpoints lists all well-known endpoints that should bypass authentication
var WellKnownEndpoints = []string{
	"/.well-known/oauth-authorization-server",
}

// OAuthServerMetadata represents the OAuth 2.0 Authorization Server Metadata
// as defined in RFC 8414
type OAuthServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint,omitempty"`
	TokenEndpoint                     string   `json:"token_endpoint,omitempty"`
	RegistrationEndpoint              string   `json:"registration_endpoint,omitempty"`
	ScopesSupported                   []string `json:"scopes_supported,omitempty"`
	ResponseTypesSupported            []string `json:"response_types_supported,omitempty"`
	GrantTypesSupported               []string `json:"grant_types_supported,omitempty"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported,omitempty"`
}

// WellKnownHandler returns a handler for .well-known OAuth discovery endpoints
func WellKnownHandler(staticConfig *config.StaticConfig, httpClient *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		switch r.URL.Path {
		case "/.well-known/oauth-authorization-server":
			handleOAuthServerMetadata(w, r, staticConfig, httpClient)
		default:
			http.NotFound(w, r)
		}
	})
}

// handleOAuthServerMetadata handles the OAuth 2.0 Authorization Server Metadata endpoint
func handleOAuthServerMetadata(w http.ResponseWriter, r *http.Request, staticConfig *config.StaticConfig, httpClient *http.Client) {
	// If OAuth is not required or no authorization URL is configured, return minimal metadata
	if !staticConfig.RequireOAuth || staticConfig.AuthorizationURL == "" {
		metadata := OAuthServerMetadata{
			Issuer: getIssuer(r, staticConfig),
		}
		writeJSON(w, metadata)
		return
	}

	// Fetch metadata from the configured authorization server
	metadataURL := strings.TrimSuffix(staticConfig.AuthorizationURL, "/") + "/.well-known/oauth-authorization-server"

	// Try OpenID Connect discovery first
	oidcMetadataURL := strings.TrimSuffix(staticConfig.AuthorizationURL, "/") + "/.well-known/openid-configuration"

	metadata, err := fetchOAuthMetadata(oidcMetadataURL, httpClient)
	if err != nil {
		klog.V(2).Infof("Failed to fetch OpenID Connect metadata from %s: %v, trying OAuth endpoint", oidcMetadataURL, err)
		// Fall back to OAuth metadata endpoint
		metadata, err = fetchOAuthMetadata(metadataURL, httpClient)
		if err != nil {
			klog.Warningf("Failed to fetch OAuth metadata from %s: %v", metadataURL, err)
			// Return minimal metadata on error
			metadata = &OAuthServerMetadata{
				Issuer:                 staticConfig.AuthorizationURL,
				ScopesSupported:        staticConfig.OAuthScopes,
				ResponseTypesSupported: []string{"code"},
				GrantTypesSupported:    []string{"authorization_code"},
			}
		}
	}

	// Override or add scopes from configuration
	if len(staticConfig.OAuthScopes) > 0 {
		metadata.ScopesSupported = staticConfig.OAuthScopes
	}

	// Remove registration endpoint if dynamic client registration is disabled
	if staticConfig.DisableDynamicClientRegistration {
		metadata.RegistrationEndpoint = ""
	}

	writeJSON(w, metadata)
}

// fetchOAuthMetadata fetches OAuth metadata from the given URL
func fetchOAuthMetadata(url string, httpClient *http.Client) (*OAuthServerMetadata, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch metadata: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var metadata OAuthServerMetadata
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("failed to decode metadata: %w", err)
	}

	return &metadata, nil
}

// getIssuer returns the issuer URL for this server
func getIssuer(r *http.Request, staticConfig *config.StaticConfig) string {
	if staticConfig.ServerURL != "" {
		return staticConfig.ServerURL
	}

	// Construct from request
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}

	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

// writeJSON writes a JSON response
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		klog.Warningf("Failed to encode JSON response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}
