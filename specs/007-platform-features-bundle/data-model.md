# Data Model: SynapBus v0.6.0

## Existing Entities (Modified)

### Messages (existing table)
- No schema changes. StalemateWorker queries existing `status`, `claimed_at`, `created_at`, `to_agent`, `from_agent` columns.
- `reply_to` column already exists (added in migration 007_threads.sql).

### Agents (existing table)
- `capabilities` column already exists (JSON, currently unused).
- A2A Agent Cards will read from this column.
- Admin CLI `agent update-capabilities` will write to this column.

## New Entities

### A2A Tasks (migration 010)

| Field | Type | Description |
|-------|------|-------------|
| id | TEXT (UUID) PRIMARY KEY | A2A task identifier |
| context_id | TEXT | Groups related tasks |
| target_agent | TEXT NOT NULL | SynapBus agent name |
| source_agent | TEXT | External agent identifier |
| conversation_id | INTEGER | Maps to SynapBus conversation |
| state | TEXT NOT NULL | SUBMITTED, WORKING, COMPLETED, FAILED, CANCELED |
| created_at | TIMESTAMP | Auto-set |
| updated_at | TIMESTAMP | Auto-set on state change |

### User Identities (migration 011)

| Field | Type | Description |
|-------|------|-------------|
| id | INTEGER PRIMARY KEY | Auto-increment |
| user_id | INTEGER FK → users(id) | Local user link |
| provider | TEXT NOT NULL | 'github', 'google', 'azuread' |
| external_id | TEXT NOT NULL | Provider's stable user ID |
| email | TEXT | Email from provider |
| display_name | TEXT | Name from provider |
| raw_claims | TEXT DEFAULT '{}' | Full JSON claims |
| created_at | TIMESTAMP | Auto-set |
| UNIQUE(provider, external_id) | | |

### Identity Providers (migration 011)

| Field | Type | Description |
|-------|------|-------------|
| id | TEXT PRIMARY KEY | 'github', 'google', 'azuread-gcore' |
| type | TEXT NOT NULL | 'github', 'oidc' |
| display_name | TEXT NOT NULL | Button label |
| client_id | TEXT NOT NULL | OAuth client ID |
| client_secret_encrypted | TEXT NOT NULL | Encrypted secret |
| issuer_url | TEXT | OIDC discovery URL (NULL for GitHub) |
| scopes | TEXT DEFAULT '[]' | JSON array |
| allowed_domains | TEXT DEFAULT '[]' | JSON array |
| group_mapping | TEXT DEFAULT '{}' | JSON: external group → role |
| tenant_id | TEXT | Azure AD tenant |
| enabled | INTEGER DEFAULT 1 | Active flag |
| created_at | TIMESTAMP | Auto-set |

## State Transitions

### A2A Task States
```
SUBMITTED → WORKING → COMPLETED (terminal)
                   → FAILED (terminal)
                   → CANCELED (terminal)
```

### Message Stalemate Flow
```
pending ──(4h)──→ system reminder DM
pending ──(48h)──→ escalation to #approvals
processing ──(24h)──→ auto-fail with "claim timeout exceeded"
```
