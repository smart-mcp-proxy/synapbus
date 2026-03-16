# Identity Provider Routes Contract

## List Providers

```
GET /auth/providers
Response: 200 OK
```

```json
{
  "providers": [
    {"id": "github", "type": "github", "display_name": "GitHub"},
    {"id": "google", "type": "oidc", "display_name": "Google"},
    {"id": "azuread-gcore", "type": "oidc", "display_name": "Microsoft (Gcore)"}
  ]
}
```

Returns empty array if no IdPs configured. Only enabled providers are returned.

## Initiate Login

```
GET /auth/login/{provider}
Response: 302 Redirect to external IdP authorization URL
```

Sets a `state` cookie for CSRF protection. Redirects to the IdP's authorization endpoint with configured client_id, redirect_uri, and scopes.

## Callback

```
GET /auth/callback/{provider}?code=...&state=...
Response: 302 Redirect to /
```

Validates state parameter, exchanges code for tokens, fetches/verifies user identity, creates or links local user, sets session cookie, redirects to Web UI home.

Error case: Redirects to `/login?error=<reason>`.
