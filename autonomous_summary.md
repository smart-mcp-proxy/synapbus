# Autonomous Implementation Summary: Message Reactions & Workflow States

**Branch**: `010-reactions-workflows`
**Date**: 2026-03-18
**Status**: Complete (StalemateWorker extension deferred)

## What Was Built

### Message Reactions
- **Toggle semantics**: Add a reaction → added. Add same reaction again → removed. One per type per agent per message.
- **5 reaction types**: approve, reject, in_progress, done, published
- **Metadata support**: JSON metadata on reactions (e.g., `{"url": "https://..."}` for published)
- **100-reaction limit** per message (safety)

### Workflow State Derivation
- State computed from reactions: published > done > rejected > in_progress > approved > proposed
- No denormalization — state derived on read from reaction list
- Channel messages with no reactions → "proposed" state
- Terminal states (rejected, done, published) don't trigger stalemate checks

### Channel Workflow Settings
- `auto_approve` — skip proposed state for new messages
- `stalemate_remind_after` — duration before reminder DM (default 24h)
- `stalemate_escalate_after` — duration before escalation to #approvals (default 72h)

### REST API
- `POST /api/messages/{id}/reactions` — toggle reaction (add or remove)
- `GET /api/messages/{id}/reactions` — get reactions + workflow state
- `DELETE /api/messages/{id}/reactions/{reaction}` — remove reaction
- `PUT /api/channels/{name}/settings` — update workflow settings
- `GET /api/channels/{name}/messages/by-state?state=X` — list messages by state

### MCP Tools (via execute bridge)
- `react` — add/toggle reaction on a message
- `unreact` — remove a reaction
- `get_reactions` — query reactions and workflow state
- `list_by_state` — list messages by workflow state in a channel

### Web UI
- **WorkflowBadge** component: colored pills (yellow/green/blue/red/gray/cyan) per state
- **ReactionPills** component: grouped reaction pills with count, agent names on hover, click-to-toggle
- Published reactions with URL show clickable link icon
- Integrated into channel message view

### Admin CLI
- `synapbus channels update --name X --auto-approve=true --stalemate-remind-after=12h --stalemate-escalate-after=48h`

## Files Created/Modified

### New Files
| File | Description |
|------|-------------|
| `internal/storage/schema/013_reactions.sql` | Migration: message_reactions table + channel columns |
| `internal/reactions/model.go` | Reaction types, state derivation, constants |
| `internal/reactions/store.go` | SQLite CRUD for reactions |
| `internal/reactions/service.go` | Business logic: toggle, remove, get, list by state |
| `internal/reactions/model_test.go` | 23 test cases for model functions |
| `internal/reactions/store_test.go` | 6 test functions for store operations |
| `internal/api/reactions_handler.go` | REST API handlers for reactions |
| `web/src/lib/components/WorkflowBadge.svelte` | Colored state badge component |
| `web/src/lib/components/ReactionPills.svelte` | Reaction toggle pills component |

### Modified Files
| File | Changes |
|------|---------|
| `internal/messaging/types.go` | Added WorkflowState, Reactions, ReactionInfo to Message |
| `internal/messaging/service.go` | Added ReactionEnricher interface, enrichment in EnrichMessages |
| `internal/channels/types.go` | Added AutoApprove, StalemateRemindAfter, StalemateEscalateAfter, ChannelSettings |
| `internal/channels/store.go` | Updated SELECT queries for new columns, added UpdateChannelSettings |
| `internal/channels/service.go` | Added UpdateChannelSettings method |
| `internal/api/router.go` | Registered reaction and channel settings routes |
| `internal/api/channels_handler.go` | Added UpdateSettings, ListByState handlers |
| `internal/mcp/bridge.go` | Added react/unreact/get_reactions/list_by_state bridge methods |
| `internal/mcp/tools_hybrid.go` | Added reactionService to registrar |
| `internal/mcp/server.go` | Added reactionService parameter |
| `internal/actions/registry.go` | Registered 4 new reaction actions |
| `cmd/synapbus/main.go` | Wired reaction service, adapter, passed to router+MCP |
| `cmd/synapbus/admin.go` | Added channels update CLI command |
| `internal/admin/socket.go` | Added channels.update_settings handler |
| `web/src/lib/api/client.ts` | Added reactions.toggle/get methods |
| `web/src/routes/channels/[name]/+page.svelte` | Integrated WorkflowBadge + ReactionPills |

## Test Results

- **25 Go test packages**: all pass, 0 failures
- **New tests**: 29+ test cases (model: 23, store: 6)
- **Integration tests**: 9 E2E tests pass
- **Web build**: Svelte SPA builds successfully
- **Binary build**: Compiles cleanly

## Deferred

- **StalemateWorker extension** (T023-T025): The data model, channel settings, and query infrastructure are in place. The worker just needs a scan loop added to detect stale messages and send DMs/escalations. This is a straightforward follow-up task.

## Architecture Decisions

1. **Separate reactions package**: Clean domain separation from messaging
2. **Toggle semantics**: INSERT if absent, DELETE if present — simple, atomic, idempotent
3. **Derived workflow state**: No denormalization; state computed from reactions on read
4. **Bridge actions (not hybrid tools)**: Consistent with attachments pattern — 4 hybrid tools are stable surface area
5. **ReactionEnricher adapter**: Avoids circular dependency between reactions and messaging packages
