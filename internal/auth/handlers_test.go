package auth

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func setupHandlers(t *testing.T) (*Handlers, *sql.DB) {
	t.Helper()
	db := newTestDB(t)

	cfg := DefaultConfig()
	cfg.BcryptCost = 10 // faster for tests
	cfg.Secret = make([]byte, 32)
	for i := range cfg.Secret {
		cfg.Secret[i] = byte(i)
	}

	userStore := NewSQLiteUserStore(db, cfg.BcryptCost)
	sessionStore := NewSQLiteSessionStore(db)
	clientStore := NewSQLiteClientStore(db, cfg.BcryptCost)
	fositeStore := NewFositeStore(db, cfg.BcryptCost)
	provider := NewOAuthProvider(cfg, fositeStore)
	handlers := NewHandlers(userStore, sessionStore, clientStore, provider, cfg)

	return handlers, db
}

func TestHandlers_Register(t *testing.T) {
	h, _ := setupHandlers(t)

	t.Run("successful registration", func(t *testing.T) {
		body := `{"username":"newuser","password":"password123","display_name":"New User"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleRegister(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d. Body: %s", rr.Code, http.StatusCreated, rr.Body.String())
		}

		var user User
		if err := json.NewDecoder(rr.Body).Decode(&user); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if user.Username != "newuser" {
			t.Errorf("Username = %q, want %q", user.Username, "newuser")
		}
		if user.DisplayName != "New User" {
			t.Errorf("DisplayName = %q, want %q", user.DisplayName, "New User")
		}
	})

	t.Run("duplicate username returns 409", func(t *testing.T) {
		body := `{"username":"newuser","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleRegister(rr, req)

		if rr.Code != http.StatusConflict {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusConflict)
		}
	})

	t.Run("short password returns 400", func(t *testing.T) {
		body := `{"username":"shortpw","password":"short"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleRegister(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid username returns 400", func(t *testing.T) {
		body := `{"username":"ab","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleRegister(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", bytes.NewBufferString("{invalid"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleRegister(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

func TestHandlers_Login(t *testing.T) {
	h, db := setupHandlers(t)
	ctx := context.Background()

	// Create a test user
	userStore := NewSQLiteUserStore(db, 10)
	userStore.CreateUser(ctx, "loginuser", "password123", "Login User")

	t.Run("successful login", func(t *testing.T) {
		body := `{"username":"loginuser","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleLogin(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		// Check session cookie
		cookies := rr.Result().Cookies()
		var sessionCookie *http.Cookie
		for _, c := range cookies {
			if c.Name == SessionCookieName {
				sessionCookie = c
				break
			}
		}
		if sessionCookie == nil {
			t.Fatal("session cookie not set")
		}
		if !sessionCookie.HttpOnly {
			t.Error("session cookie should be HttpOnly")
		}
		if sessionCookie.SameSite != http.SameSiteLaxMode {
			t.Error("session cookie should be SameSite=Lax")
		}
	})

	t.Run("wrong password returns 401", func(t *testing.T) {
		body := `{"username":"loginuser","password":"wrongpassword"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleLogin(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})

	t.Run("non-existent user returns 401", func(t *testing.T) {
		body := `{"username":"nobody","password":"password123"}`
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		h.HandleLogin(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})
}

func TestHandlers_Logout(t *testing.T) {
	h, db := setupHandlers(t)
	ctx := context.Background()

	userStore := NewSQLiteUserStore(db, 10)
	sessStore := NewSQLiteSessionStore(db)

	user, _ := userStore.CreateUser(ctx, "logoutuser", "password123", "")
	session, _ := sessStore.CreateSession(ctx, user.ID, 24*time.Hour)

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session.SessionID})
	// Add user to context (normally done by middleware)
	req = req.WithContext(ContextWithUser(req.Context(), user))
	rr := httptest.NewRecorder()

	h.HandleLogout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	// Cookie should be cleared
	cookies := rr.Result().Cookies()
	for _, c := range cookies {
		if c.Name == SessionCookieName && c.MaxAge != -1 {
			t.Error("session cookie should be cleared (MaxAge = -1)")
		}
	}

	// Session should be deleted
	_, err := sessStore.GetSession(ctx, session.SessionID)
	if err != ErrSessionNotFound {
		t.Errorf("session should be deleted after logout")
	}
}

func TestHandlers_Me(t *testing.T) {
	h, db := setupHandlers(t)
	ctx := context.Background()

	userStore := NewSQLiteUserStore(db, 10)
	user, _ := userStore.CreateUser(ctx, "meuser", "password123", "Me User")

	t.Run("authenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		req = req.WithContext(ContextWithUser(req.Context(), user))
		rr := httptest.NewRecorder()

		h.HandleMe(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusOK)
		}

		var resp User
		json.NewDecoder(rr.Body).Decode(&resp)
		if resp.Username != "meuser" {
			t.Errorf("Username = %q, want %q", resp.Username, "meuser")
		}
	})

	t.Run("unauthenticated", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
		rr := httptest.NewRecorder()

		h.HandleMe(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusUnauthorized)
		}
	})
}

func TestHandlers_ChangePassword(t *testing.T) {
	h, db := setupHandlers(t)
	ctx := context.Background()

	userStore := NewSQLiteUserStore(db, 10)
	sessStore := NewSQLiteSessionStore(db)

	user, _ := userStore.CreateUser(ctx, "chpwuser", "oldpassword1", "")
	session, _ := sessStore.CreateSession(ctx, user.ID, 24*time.Hour)
	// Create another session that should be invalidated
	sess2, _ := sessStore.CreateSession(ctx, user.ID, 24*time.Hour)

	t.Run("successful password change", func(t *testing.T) {
		body := `{"current_password":"oldpassword1","new_password":"newpassword1"}`
		req := httptest.NewRequest(http.MethodPut, "/auth/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ContextWithUser(req.Context(), user))
		req = req.WithContext(ContextWithSessionID(req.Context(), session.SessionID))
		rr := httptest.NewRecorder()

		h.HandleChangePassword(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d. Body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		// Verify old password no longer works
		_, err := userStore.VerifyPassword(ctx, "chpwuser", "oldpassword1")
		if err != ErrInvalidPassword {
			t.Error("old password should not work")
		}

		// Verify new password works
		_, err = userStore.VerifyPassword(ctx, "chpwuser", "newpassword1")
		if err != nil {
			t.Errorf("new password should work: %v", err)
		}

		// Other sessions should be invalidated
		_, err = sessStore.GetSession(ctx, sess2.SessionID)
		if err != ErrSessionNotFound {
			t.Error("other sessions should be invalidated after password change")
		}

		// Current session should still exist
		_, err = sessStore.GetSession(ctx, session.SessionID)
		if err != nil {
			t.Errorf("current session should still exist: %v", err)
		}
	})

	t.Run("wrong current password returns 403", func(t *testing.T) {
		body := `{"current_password":"wrongpassword","new_password":"newpassword2"}`
		req := httptest.NewRequest(http.MethodPut, "/auth/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ContextWithUser(req.Context(), user))
		rr := httptest.NewRecorder()

		h.HandleChangePassword(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusForbidden)
		}
	})

	t.Run("short new password returns 400", func(t *testing.T) {
		body := `{"current_password":"newpassword1","new_password":"short"}`
		req := httptest.NewRequest(http.MethodPut, "/auth/password", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ContextWithUser(req.Context(), user))
		rr := httptest.NewRecorder()

		h.HandleChangePassword(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusBadRequest)
		}
	})
}

// T018: Test OAuth metadata endpoint returns valid JSON with required fields.
func TestHandlers_OAuthMetadata(t *testing.T) {
	h, _ := setupHandlers(t)
	h.config.IssuerURL = "http://localhost:8080"

	req := httptest.NewRequest(http.MethodGet, "/.well-known/oauth-authorization-server", nil)
	rr := httptest.NewRecorder()

	h.HandleOAuthMetadata(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	contentType := rr.Header().Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", contentType)
	}

	var metadata map[string]any
	if err := json.NewDecoder(rr.Body).Decode(&metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}

	// Verify required fields
	requiredFields := []string{
		"issuer",
		"authorization_endpoint",
		"token_endpoint",
		"token_endpoint_auth_methods_supported",
		"response_types_supported",
		"grant_types_supported",
		"code_challenge_methods_supported",
		"scopes_supported",
	}

	for _, field := range requiredFields {
		if metadata[field] == nil {
			t.Errorf("missing required field: %s", field)
		}
	}

	// Verify issuer matches config
	if metadata["issuer"] != "http://localhost:8080" {
		t.Errorf("issuer = %v, want http://localhost:8080", metadata["issuer"])
	}

	// Verify endpoints contain base URL
	if authEndpoint, ok := metadata["authorization_endpoint"].(string); ok {
		if authEndpoint != "http://localhost:8080/oauth/authorize" {
			t.Errorf("authorization_endpoint = %q, want http://localhost:8080/oauth/authorize", authEndpoint)
		}
	}

	if tokenEndpoint, ok := metadata["token_endpoint"].(string); ok {
		if tokenEndpoint != "http://localhost:8080/oauth/token" {
			t.Errorf("token_endpoint = %q, want http://localhost:8080/oauth/token", tokenEndpoint)
		}
	}

	// Verify S256 is supported
	if methods, ok := metadata["code_challenge_methods_supported"].([]any); ok {
		found := false
		for _, m := range methods {
			if m == "S256" {
				found = true
			}
		}
		if !found {
			t.Error("S256 should be in code_challenge_methods_supported")
		}
	}

	// Verify "mcp" scope is supported
	if scopes, ok := metadata["scopes_supported"].([]any); ok {
		found := false
		for _, s := range scopes {
			if s == "mcp" {
				found = true
			}
		}
		if !found {
			t.Error("mcp should be in scopes_supported")
		}
	}
}

// T018: Test OAuth metadata with POST returns 405.
func TestHandlers_OAuthMetadata_MethodNotAllowed(t *testing.T) {
	h, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodPost, "/.well-known/oauth-authorization-server", nil)
	rr := httptest.NewRecorder()

	h.HandleOAuthMetadata(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

// mockAgentLister implements AgentLister for testing.
type mockAgentLister struct {
	agents []AgentInfo
}

func (m *mockAgentLister) ListAgentsByOwner(ctx context.Context, ownerID int64) ([]AgentInfo, error) {
	return m.agents, nil
}

// T019: Test authorize page renders HTML with login form when unauthenticated.
func TestHandlers_AuthorizeGet_LoginForm(t *testing.T) {
	h, _ := setupHandlers(t)

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=test-client&redirect_uri=http://localhost:3000&state=abc123&scope=mcp&code_challenge=challenge&code_challenge_method=S256", nil)
	rr := httptest.NewRecorder()

	h.HandleAuthorizeGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	// Content-Type header should be HTML
	ct := rr.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	// Should contain login form elements
	if !strings.Contains(body, "<form") {
		t.Error("expected HTML form in response")
	}
	if !strings.Contains(body, "username") {
		t.Error("expected username field in login form")
	}
	if !strings.Contains(body, "password") {
		t.Error("expected password field in login form")
	}
	// Should contain hidden OAuth params
	if !strings.Contains(body, "test-client") {
		t.Error("expected client_id in hidden fields")
	}
	if !strings.Contains(body, "abc123") {
		t.Error("expected state in hidden fields")
	}
}

// T019: Test authorize page renders agent selector when logged in.
func TestHandlers_AuthorizeGet_AgentSelector(t *testing.T) {
	h, db := setupHandlers(t)
	ctx := context.Background()

	// Set up agent lister
	h.agentLister = &mockAgentLister{
		agents: []AgentInfo{
			{Name: "my-bot", DisplayName: "My Bot", Type: "ai"},
			{Name: "my-human", DisplayName: "My Human", Type: "human"},
		},
	}

	// Create user and session
	userStore := NewSQLiteUserStore(db, 10)
	sessStore := NewSQLiteSessionStore(db)
	user, _ := userStore.CreateUser(ctx, "authzuser", "password123", "Authz User")
	session, _ := sessStore.CreateSession(ctx, user.ID, 24*time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=test-client&redirect_uri=http://localhost:3000&state=xyz&scope=mcp&code_challenge=ch&code_challenge_method=S256", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session.SessionID})
	rr := httptest.NewRecorder()

	h.HandleAuthorizeGet(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	body := rr.Body.String()

	// Should show agent selector, not login form
	if !strings.Contains(body, "agent_name") {
		t.Error("expected agent_name selector in response")
	}
	if !strings.Contains(body, "my-bot") {
		t.Error("expected agent name 'my-bot' in dropdown")
	}
	if !strings.Contains(body, "my-human") {
		t.Error("expected agent name 'my-human' in dropdown")
	}
	// Should show logged-in user
	if !strings.Contains(body, "authzuser") {
		t.Error("expected username in response")
	}
	// Should have Authorize button
	if !strings.Contains(body, "Authorize") {
		t.Error("expected Authorize button")
	}
}

// T019: Test authorize page shows message when user has no agents.
func TestHandlers_AuthorizeGet_NoAgents(t *testing.T) {
	h, db := setupHandlers(t)
	ctx := context.Background()

	h.agentLister = &mockAgentLister{agents: []AgentInfo{}}

	userStore := NewSQLiteUserStore(db, 10)
	sessStore := NewSQLiteSessionStore(db)
	user, _ := userStore.CreateUser(ctx, "noagentuser", "password123", "")
	session, _ := sessStore.CreateSession(ctx, user.ID, 24*time.Hour)

	req := httptest.NewRequest(http.MethodGet, "/oauth/authorize?response_type=code&client_id=test-client&redirect_uri=http://localhost:3000&state=xyz&scope=mcp&code_challenge=ch&code_challenge_method=S256", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: session.SessionID})
	rr := httptest.NewRecorder()

	h.HandleAuthorizeGet(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, "No agents registered yet") {
		t.Error("expected 'No agents registered yet' message when user has no agents")
	}
}
