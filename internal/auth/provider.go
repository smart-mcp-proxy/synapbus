package auth

import (
	"context"
	"crypto/rand"
	"net/url"
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"
	"github.com/ory/fosite/token/hmac"
)

// NewOAuthProvider creates a configured fosite OAuth 2.1 provider.
// It supports authorization code with PKCE (S256 only), client credentials, and refresh token rotation.
func NewOAuthProvider(cfg Config, store *FositeStore) fosite.OAuth2Provider {
	secret := cfg.Secret
	if len(secret) < 32 {
		// Generate a random secret if not configured
		secret = make([]byte, 32)
		rand.Read(secret)
	}

	config := &fosite.Config{
		AccessTokenLifespan:           cfg.AccessTokenTTL,
		RefreshTokenLifespan:          cfg.RefreshTokenLifetime,
		AuthorizeCodeLifespan:         10 * time.Minute,
		GlobalSecret:                  secret,
		SendDebugMessagesToClients:    cfg.DevMode,
		EnforcePKCE:                   true,
		EnforcePKCEForPublicClients:   true,
		EnablePKCEPlainChallengeMethod: false,
		TokenURL:                      tokenURL(cfg.IssuerURL),
		HashCost:                      cfg.BcryptCost,
		// Allow localhost with any port for native MCP clients (RFC 8252 Section 7.3)
		RedirectSecureChecker: func(_ context.Context, u *url.URL) bool {
			if u == nil {
				return false
			}
			return u.Scheme == "https" || u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1"
		},
	}

	// HMACSHAStrategy for token generation
	hmacStrategy := &hmac.HMACStrategy{
		Config: config,
	}

	_ = hmacStrategy

	return compose.Compose(
		config,
		store,
		&compose.CommonStrategy{
			CoreStrategy: compose.NewOAuth2HMACStrategy(config),
		},
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2ClientCredentialsGrantFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2PKCEFactory,
		compose.OAuth2TokenIntrospectionFactory,
	)
}

// tokenURL returns the token endpoint URL. When issuerURL is set, it's used as the base.
// When empty (auto-detect mode), fosite's TokenURL is set to a relative path since
// the actual URL is exposed via OAuth metadata from the request Host header.
func tokenURL(issuerURL string) string {
	if issuerURL != "" {
		return issuerURL + "/oauth/token"
	}
	return "/oauth/token"
}

// NewSession creates a new fosite session for a user.
func NewSession(user *User) fosite.Session {
	return &fositeSession{
		UserID:   user.ID,
		Username: user.Username,
		Subject:  user.Username,
	}
}

// NewSessionWithAgent creates a new fosite session for a user with an agent identity.
func NewSessionWithAgent(user *User, agentName string) fosite.Session {
	return &fositeSession{
		UserID:    user.ID,
		Username:  user.Username,
		Subject:   user.Username,
		AgentName: agentName,
	}
}
