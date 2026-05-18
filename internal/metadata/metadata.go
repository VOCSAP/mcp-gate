// Package metadata serves RFC 9728 Protected Resource Metadata.
//
// It provides an HTTP handler for the /.well-known/oauth-protected-resource
// endpoint, returning a static JSON document that directs MCP clients to the
// appropriate OAuth 2.1 authorization server.
package metadata

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ProtectedResourceMetadata represents the RFC 9728 Protected Resource Metadata response.
// Defined inline to avoid heavy transitive dependencies from go-sdk/oauthex.
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	ScopesSupported        []string `json:"scopes_supported,omitempty"`
	BearerMethodsSupported []string `json:"bearer_methods_supported,omitempty"`
	ResourceName           string   `json:"resource_name,omitempty"`
	ResourceDocumentation  string   `json:"resource_documentation,omitempty"`
}

// Handler returns an http.HandlerFunc that serves the given metadata as JSON.
// The JSON is pre-marshaled at construction time to avoid repeated encoding and
// to surface marshaling errors at startup rather than at request time.
func Handler(meta ProtectedResourceMetadata) (http.HandlerFunc, error) {
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, fmt.Errorf("metadata marshal: %w", err)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		// no-store: per the Anthropic claude.ai MCP troubleshooting guidance
		// (anthropics/claude-ai-mcp #136) the resource-metadata response must
		// not be cached anywhere along the path. Caching by an intermediary
		// (Cloudflare, browser, claude.ai proxy) was implicated as one of the
		// factors breaking the OAuth callback in older claude.ai versions and
		// is still recommended by Anthropic for any RFC 9728 endpoint.
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	}, nil
}
