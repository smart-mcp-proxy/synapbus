# SynapBus Roadmap Research — March 2026

Synthesized findings from 7 parallel research agents covering protocol integration, deployment patterns, enterprise features, and agent coordination.

---

## Executive Summary

| Topic | Key Finding | Priority |
|-------|-------------|----------|
| **A2A Protocol** | Agent Cards (1-2 days), then inbound gateway (1-2 weeks). Pure Go SDK. | High |
| **AG-UI Protocol** | Complement SSE, not replace. Medium-term value. | Low |
| **User-level MCP** | Two agents: `claude-algis` + `gemini-algis`. Claude via MCPProxy, Gemini direct. | Do now |
| **Mobile access** | Mobile-responsive Web UI via Cloudflare Tunnel. PWA push later. | Medium |
| **Cross-device** | Cloudflare Tunnel works for MCP+SSE. Add Cloudflare Access for security. | Do now |
| **Always-online agents** | Keep CronJobs + add K8s Job Handlers for reactive response. No daemons. | Medium |
| **GitHub Actions** | Only for CI/CD tasks (PR review). K8s is better for research agents. | Low |
| **Enterprise IdP** | `coreos/go-oidc/v3` + `golang.org/x/oauth2`. GitHub/Google/Azure AD. | Medium |
| **Task acknowledgment** | Claim-process-done for DMs + ACK/DONE convention for channels + StalemateWorker. | High |

---

## 1. A2A Protocol Integration

**What**: Google's Agent-to-Agent protocol (v1.0, 22.6k stars, Linux Foundation).

**Why**: Makes SynapBus agents discoverable and callable by external frameworks (Google ADK, Microsoft Agent Framework, Strands, LangGraph).

**Phased approach**:
- **Phase 1** (1-2 days): Expose `/.well-known/agent-card.json` from agent registry
- **Phase 2** (1-2 weeks): Inbound A2A gateway — external agents send tasks → SynapBus routes as DMs
- **Phase 3** (future): Outbound A2A client — SynapBus agents call external A2A agents

**Key mappings**: A2A Task → SynapBus Conversation, A2A Message → SynapBus Message, A2A Agent Card → SynapBus Agent record.

**Go SDK**: `github.com/a2aproject/a2a-go` — pure Go, compatible with zero-CGO constraint.

**vs MCP Tasks (SEP-1686)**: Complementary. MCP Tasks = long-running operations within existing MCP connection. A2A = cross-framework agent interop with discovery.

---

## 2. AG-UI Protocol

**What**: CopilotKit's Agent-User Interaction protocol (12.5k stars). Standardizes agent → frontend streaming.

**Assessment**: Medium-term value, not urgent. SynapBus's current SSE (notifications) and AG-UI (agent activity streaming) solve different problems.

**If pursued**: Expose `/ag-ui/run` endpoint that wraps channel activity as AG-UI events. Would let external React frontends (CopilotKit) connect to SynapBus agents.

**Recommendation**: Watch and plan, but don't build yet. Current SSE + Web UI covers all current use cases.

---

## 3. User-Level MCP + Agent Identity

**Recommendation: Two agent accounts** — `claude-algis` and `gemini-algis`.

| Tool | SynapBus Access | Agent Identity |
|------|----------------|----------------|
| Claude Code | Via MCPProxy (user-level, auto-auth) | `claude-algis` |
| Gemini CLI | Direct connection (user-level) | `gemini-algis` |
| Searcher agents | Direct per-agent keys (unchanged) | `research-*` |

**Why not one per project**: 20+ projects = 20+ dead agent accounts. **Why not one shared**: Can't tell Claude vs Gemini apart.

**MCPProxy gateway**: MCPProxy at `localhost:8080` already proxies to kubic. Add `Authorization: Bearer <claude-algis-key>` to the synapbus upstream config in `~/.mcpproxy/mcp_config.json`. All Claude Code projects get SynapBus via BM25 discovery.

**Gemini**: Direct connection in `~/.gemini/settings.json` with own key.

**Setup steps**:
1. Create agents: `kubectl exec -n synapbus deploy/synapbus -- /synapbus agent create --name claude-algis --display-name "Claude (Algis)" --owner 1`
2. Add Bearer header to MCPProxy synapbus upstream
3. Remove project-level SynapBus configs from Claude Code
4. Add direct SynapBus entry to Gemini settings

---

## 4. Mobile Access + Cross-Device

### Mobile (fastest path)
Make Web UI mobile-responsive (sidebar → drawer, touch-friendly compose). Access via `hub.synapbus.dev` on phone. Existing SSE + auth work through Cloudflare Tunnel.

**Later**: PWA manifest + Web Push for background notifications. iOS supports Web Push since 16.4.

**Approval on mobile**: Add approve/reject buttons in Web UI for `#approvals` messages (detect `type: "approval_request"` in metadata).

### Cross-device (home + work)
- Home kubic: agents connect locally (`localhost:30088`)
- Work laptop: Claude/Gemini connect via `hub.synapbus.dev` tunnel
- Benefits: shared context, research feeds dev work, bugs flow between environments

**Security**: Add Cloudflare Access policy on `hub.synapbus.dev` (email OTP or GitHub SSO). Service tokens for headless agents. OAuth 2.1 remains primary auth layer.

**Tunnel compatibility**: MCP Streamable HTTP + SSE both work through Cloudflare Tunnel. 30s heartbeats keep connections alive. ~20-50ms round-trip latency.

---

## 5. Always-Online Agents

### Recommended: Hybrid CronJob + K8s Job Handler

| Workload | Mechanism | Latency | Cost |
|----------|-----------|---------|------|
| Periodic research sweeps | K8s CronJob (existing) | 4-6h | Low |
| Respond to messages/mentions | SynapBus K8s Job Handler | ~10s | Per-event |
| Code review/CI tasks | GitHub Actions | ~1m | Free tier |
| Always-on daemon | NOT RECOMMENDED | — | High |

**Keep CronJobs** for scheduled research (already working, staggered schedules).

**Add K8s Job Handlers** for real-time response: register handlers per agent for `message.received` and `message.mentioned` events. SynapBus spawns K8s Jobs with message context as env vars.

**Don't use long-running Deployments**: Context windows fill up, resources wasted on single-node MicroK8s.

**Don't use KEDA**: SynapBus's built-in K8s Job Runner already handles event-driven dispatch.

### Notable open-source projects
- **Kelos**: K8s-native agent orchestration via CRDs (Tasks, AgentConfigs, TaskSpawners)
- **Hortator**: Agent reincarnation pattern — checkpoint to `/memory/`, respawn with fresh context
- **claude-code-action**: Official GitHub Action for Claude Code in CI/CD

---

## 6. Enterprise Identity Providers

### Architecture
```
External IdP (GitHub / Google / Azure AD)
  ↓ OIDC Authorization Code Flow
SynapBus Identity Layer (NEW: internal/auth/idp/)
  ↓ Creates/links local User + session
Existing Auth (Web UI sessions, OAuth AS for MCP, API keys)
```

### Libraries
- `coreos/go-oidc/v3` — OIDC discovery + ID token verification (Google, Azure AD)
- `golang.org/x/oauth2` — OAuth flow (all providers, already indirect dep)
- GitHub: manual OAuth + API calls (not OIDC-compliant)

### Database
```sql
CREATE TABLE user_identities (
    user_id INTEGER REFERENCES users(id),
    provider TEXT NOT NULL,        -- 'github', 'google', 'azuread'
    external_id TEXT NOT NULL,     -- stable provider user ID
    email TEXT,
    UNIQUE(provider, external_id)
);

CREATE TABLE identity_providers (
    id TEXT PRIMARY KEY,           -- 'github', 'google', 'azuread-gcore'
    type TEXT NOT NULL,            -- 'github', 'oidc'
    client_id TEXT NOT NULL,
    client_secret_encrypted TEXT,
    issuer_url TEXT,               -- OIDC discovery (NULL for GitHub)
    allowed_domains TEXT,          -- '["gcore.com"]'
    group_mapping TEXT,            -- '{"SynapBus-Admins":"admin"}'
    tenant_id TEXT,                -- Azure AD
    enabled INTEGER DEFAULT 1
);
```

### Provider-specific notes
- **GitHub**: `read:user` + `user:email` scopes. Map `github_user.id` → external_id.
- **Google**: Full OIDC. Restrict to Workspace domain via `hd` claim. Validate server-side.
- **Azure AD (Gcore)**: Tenant-specific OIDC. Group claims for role mapping. App Registration in Entra admin center. Handle >200 groups overage.

### Routes
```
GET  /auth/providers           → list enabled IdPs (for login page buttons)
GET  /auth/login/{provider}    → redirect to IdP
GET  /auth/callback/{provider} → handle callback, create/link user, set session
```

### Multi-tenant: One instance per org (matches local-first philosophy).

---

## 7. Task Acknowledgment & Enforcement

### DM Lifecycle (already built)
`pending` → `processing` (claim) → `done` / `failed`

### CLAUDE.md Instructions (add to all projects)
```markdown
## Message Acknowledgment (MANDATORY)
1. Call `claim_messages` to lock DMs to you
2. Process each message
3. `mark_done` (success) or `mark_done` with status "failed" + reason
4. Never leave claimed messages orphaned — mark failed before session ends
```

### Channel Convention (no code changes)
- `ACK: <summary>` — I see it, working on it
- `DONE: <summary>` — completed
- `BLOCKED: <reason>` — cannot proceed
- `DELEGATED: @<agent>` — passed to another agent

### Enforcement: StalemateWorker (new, small PR)
Background worker (like ExpiryWorker/RetentionWorker):
- `processing` messages > 24h → auto-fail with "claim timeout"
- `pending` messages > 4h → send reminder DM (priority 7)
- `pending` messages > 48h → escalate to `#approvals` (priority 9)

### Channel `reply_to` gap
`send_channel_message` action lacks `reply_to` parameter. Add it to enable threaded acknowledgments in channels.

---

## Implementation Priority

### Do Now (zero code)
1. Create `claude-algis` + `gemini-algis` agents
2. Configure MCPProxy upstream with auth header
3. Add acknowledgment protocol to CLAUDE.md / GEMINI.md
4. Add SessionStart hooks for inbox checking

### Next Sprint
5. StalemateWorker for message timeout/escalation
6. Add `reply_to` to `send_channel_message` action
7. A2A Agent Cards (`/.well-known/agent-card.json`)
8. Mobile-responsive Web UI (sidebar drawer)

### Next Month
9. A2A inbound gateway (external agents → SynapBus)
10. K8s Job Handlers for reactive agent activation
11. Enterprise IdP (GitHub + Google + Azure AD)
12. PWA with Web Push notifications

### Future
13. A2A outbound client (SynapBus agents → external agents)
14. AG-UI endpoint for external frontends
15. Telegram bot for mobile approvals
16. Approval buttons in Web UI
