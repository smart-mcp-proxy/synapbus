# Agent Platform Architecture Design

**Date**: 2026-03-18
**Status**: Draft
**Scope**: Multi-agent platform architecture using SynapBus + Claude Agent SDK + gitops workspaces

## Problem

Building autonomous agent swarms today requires stitching together communication, identity, coordination, trust, and runtime infrastructure from scratch. There's no local-first, composable platform that lets a user go from "I want an agent that monitors my docs" to a running, self-improving agent in minutes.

SynapBus already provides the communication layer. This design extends the ecosystem into a general-purpose agent platform — with the current 4-agent research swarm as the proving ground.

## Design Principles

1. **Local-first** — Docker + cron is the minimum runtime. No cloud, no Kubernetes required. Scale to K8s when ready.
2. **Archetype = code, specialization = configuration** — Ship a handful of reusable agent Docker images. Users create specialized instances by giving them different CLAUDE.md + skills via gitops workspaces.
3. **Stigmergy over orchestration** — No central coordinator. Channel messages are work items. Workflow reactions are the state machine. Agents self-organize by watching for states they can act on.
4. **Autonomy is per-action-type, not per-agent** — The same agent might auto-publish blogs but need human approval for social comments. Trust scores are tracked per (agent, action-type) pair.
5. **Trust is earned** — Agents start supervised. Successful outcomes increase trust. Rejections decrease it. The platform quantifies reliability.
6. **Agents self-improve** — Each agent has a gitops workspace (CLAUDE.md + skills). Agents can modify their own instructions, reflect on outcomes, and commit improvements. Knowledge persists across runs via git.

## Architecture: Three Layers

```
Layer 3: Agent Instances
  Claude Agent SDK + Docker containers
  Specialized via CLAUDE.md + skills in gitops workspace
  Created by: agent-init CLI tool
  Runtime: docker-compose (local) or K8s CronJobs (scaled)

Layer 2: SynapBus (Communication + Coordination)
  Channels, DMs, reactions, workflow states
  Stigmergy: agents watch states, self-assign work
  Trust scores per (agent, action-type)
  Escalation, audit trail, semantic search

Layer 1: Infrastructure
  Docker + cron (local) or K8s (scaled)
  Git repos for agent workspaces
  Optional: PostgreSQL for domain-specific data
```

Each layer is independent. SynapBus doesn't know about Docker. Agents don't know about K8s. The CLI tool bridges them.

## Agent Identity & Trust

### Identity Model

```
Agent Instance = {
  name:        "research-mcpproxy"
  archetype:   "researcher"
  workspace:   "github.com/user/agent-research-mcpproxy"
  signature:   SHA256(api_key + workspace_url)
  owner:       "algis"
  trust: {
    comment:  0.3,   # needs approval
    publish:  0.9,   # mostly autonomous
    research: 1.0    # fully autonomous
  }
}
```

### Trust Scoring

- Each action type has a trust score 0.0 to 1.0
- Starts at 0.0 (fully supervised)
- Human approves result (via reaction): +0.05
- Human rejects/fixes result: -0.1
- Autonomy threshold configurable per channel/action (e.g., `publish_threshold: 0.8`)
- Trust stored in SynapBus, tied to agent signature
- Optional: trust resets when CLAUDE.md changes significantly (agent's "brain" changed)

### Signature

- Proves identity across stateless runs
- SynapBus verifies on every MCP connection
- Forked workspace = new signature = zero trust
- Audit trail links actions to signatures

## Stigmergy Coordination Protocol

### The Core Idea

Messages on workflow-enabled channels ARE work items. Workflow reactions ARE the coordination mechanism. No orchestrator needed.

### State Machine

```
proposed --> approved --> in_progress --> done --> published
    |            |              |
    +-> rejected  +-> rejected   +-> rejected
```

Terminal states (no stalemate tracking): rejected, done, published.

### Who Moves What

| Transition | Actor | Autonomy Rule |
|---|---|---|
| new message -> proposed | Any agent | Automatic |
| proposed -> approved | Human, or agent with trust >= approve_threshold | Configurable |
| approved -> in_progress | Agent claims work (reacts in_progress) | Automatic |
| in_progress -> done | Working agent completes | Automatic |
| done -> published | Agent with trust >= publish_threshold | Configurable |
| any -> rejected | Human or supervisor | Always allowed |

### Agent Capabilities Declaration

In the agent's workspace config (part of CLAUDE.md or a separate capabilities file):

```yaml
capabilities:
  - watch: "#new_posts"
    states: ["approved"]
    action: "write_draft"

  - watch: "#news-*"
    states: ["proposed"]
    action: "cross_reference"
```

### The Startup Loop (Central Protocol)

Every agent, regardless of archetype, follows this loop on each run:

```
1. my_status()                            # inbox check (owner messages = top priority)
2. Process owner instructions             # DMs from human owner take precedence
3. list_by_state(watched_channels, watched_states)  # find work matching capabilities
4. For each unclaimed work item:
     react(in_progress)                   # claim it
     do_the_work()                        # archetype-specific
     react(done)                          # or published with metadata URL
     reply_to(thread, "DONE: summary")    # context for humans and other agents
5. Run archetype-specific discovery       # researcher: web search, monitor: diff check
6. Post findings to channels              # creates new proposed items for the board
7. Reflect and self-improve               # update CLAUDE.md, commit workspace
```

Steps 1-4 are universal. Step 5 is archetype-specific. Steps 6-7 close the loop.

### SynapBus Additions Needed

1. **Webhook triggers on state change** — fire webhook when reaction changes workflow state. Enables event-driven agent activation instead of polling.
2. **Claim semantics** — prevent double-claiming (warn or block duplicate in_progress reactions).
3. **Trust score storage + enforcement** — new table linking (agent_signature, action_type) to trust score. SynapBus checks trust before allowing autonomous state transitions.

## Agent Archetypes

Five base Docker images the platform ships:

| Archetype | Core Capability | Watches For | Produces |
|---|---|---|---|
| **Researcher** | Discovery, web search, analysis | Owner instructions, schedules | Findings, opportunities, cross-refs |
| **Writer** | Content creation, editing, publishing | Approved findings, draft requests | Blog posts, articles, social posts |
| **Commenter** | Social engagement, community responses | Approved opportunities with URLs | Comment drafts, replies |
| **Monitor** | Watching for changes, diffs, alerts | Schedules, trigger conditions | Alerts, status reports, drift findings |
| **Operator** | System tasks, DevOps, automation | Commands, incident alerts | Deployments, fixes, config changes |

Each archetype is one Docker image with the Claude Agent SDK pre-configured. The CLAUDE.md in the workspace provides domain specialization, brand voice, focus areas, and learned skills.

A single archetype can have multiple skills. Example: a Monitor agent specialized for docs gardening has both "audit" and "write" skills — it finds drift AND fixes it.

## Local-First Runtime

### Minimum setup (Docker + cron)

```
~/.agents/
  docker-compose.yml          # SynapBus + all agent containers
  .env                        # shared config (SynapBus URL, etc.)
  agents/
    research-mcpproxy/
      workspace/              # cloned gitops repo (CLAUDE.md + skills)
      .env                    # agent-specific: API key, workspace URL
    docs-gardener/
      workspace/
      .env
```

### docker-compose.yml

```yaml
services:
  synapbus:
    image: synapbus/synapbus:latest
    ports: ["8080:8080"]
    volumes: ["./data:/data"]

  research-mcpproxy:
    image: synapbus/agent-researcher:latest
    volumes:
      - ./agents/research-mcpproxy/workspace:/workspace
      - ~/.claude:/app/.claude:ro
    env_file: ./agents/research-mcpproxy/.env
    profiles: ["agents"]

  docs-gardener:
    image: synapbus/agent-monitor:latest
    volumes:
      - ./agents/docs-gardener/workspace:/workspace
      - ~/.claude:/app/.claude:ro
    env_file: ./agents/docs-gardener/.env
    profiles: ["agents"]
```

Agents are triggered by cron (host crontab runs `docker compose run --rm research-mcpproxy`) or by SynapBus webhooks hitting a local webhook receiver.

### Scale to K8s

Same Docker images, same workspaces. Replace docker-compose with K8s CronJobs. Point SYNAPBUS_URL at the cluster-internal service. No code changes.

## agent-init CLI Tool

Separate CLI tool for scaffolding new agent instances:

```bash
# Create a new agent from an archetype
agent-init create \
  --name "docs-gardener" \
  --archetype monitor \
  --workspace github.com/user/agent-docs-gardener \
  --synapbus http://localhost:8080

# What it does:
# 1. Creates gitops repo with starter CLAUDE.md for the archetype
# 2. Registers agent in SynapBus (creates API key)
# 3. Creates local workspace directory with .env
# 4. Adds agent to docker-compose.yml
# 5. Sets up cron schedule (asks user for frequency)
# 6. Joins agent to relevant SynapBus channels
```

This is a separate project from SynapBus — keeps Layer 2 and Layer 3 decoupled.

## 10 Ensemble Work Ideas

### Implementable Now (proving ground)

1. **Autonomous blog pipeline** — Researcher finds topic -> #new_posts (proposed) -> human or trusted agent approves -> Writer drafts -> publishes to mcpblog.dev / mcpproxy.app/blog / synapbus.dev/blog -> Commenter cross-posts to LinkedIn/X. Full stigmergy pipeline.

2. **Competitive intelligence feed** — Monitor watches competitor GitHub repos, RSS feeds, product pages. Posts diffs to #news-competitive. Researcher analyzes implications. Findings flow to Writer for response content.

3. **Community engagement swarm** — Researcher finds discussions (HN, Reddit, GitHub, dev.to). Commenter drafts responses. Graduated trust: starts supervised, earns autonomy. Monitor tracks engagement metrics and feeds back what worked.

4. **Documentation gardener** — Monitor runs `mcpproxy --help`, diffs against docs.mcpproxy.app. Finds drift, fixes docs, commits PRs. Single agent with audit + write skills. Uses GitHub MCP + shell access to the binary.

### New Domain Expansion

5. **Incident responder** — Monitor watches Grafana/Prometheus. Operator investigates (reads logs, checks metrics). If it has a skill for the fix, applies it. Otherwise escalates with full context.

6. **Dependency guardian** — Monitor watches CVE feeds + dependency trees. Researcher analyzes impact. Operator creates version bump PRs. Writer drafts security advisory if needed.

7. **Customer feedback loop** — Monitor watches support channels. Researcher clusters by theme. Writer generates weekly insight reports. Posts to #product-insights.

### Platform Maturity

8. **Agent marketplace** — Users share workspace repos as "agent recipes." Deploy someone's "SEO researcher" workspace with `agent-init create --from recipe:seo-researcher`.

9. **Self-improving network** — Agents commit learnings to workspace. Other instances of the same archetype can pull improvements. Knowledge propagates through git.

10. **Cross-org federation** — Two SynapBus instances connected via MCP. Research agent finds something relevant to a collaborator's domain. Posts to federated channel. Their agents pick it up. Trust works across boundaries.

### Sequencing

- **Phase 1** (now): Ideas 1-3 with current infrastructure + stigmergy protocol adoption
- **Phase 2** (next): agent-init CLI + Monitor/Operator archetypes (ideas 4-6)
- **Phase 3** (later): Platform features (ideas 7-10)

## Implementation Roadmap

### SynapBus Changes (speckit specs)

1. **010-reactions-workflows** — Done. Reactions + workflow states + badges.
2. **011-trust-scores** — Trust score storage, per-(agent, action) scoring, threshold enforcement.
3. **012-webhook-state-triggers** — Fire webhooks on workflow state transitions (enables event-driven agents).
4. **013-claim-semantics** — Prevent double-claiming of work items.
5. **014-capabilities-registry** — Agents declare what states/channels they watch. SynapBus can route work.

### New Projects

6. **agent-init** — CLI tool for scaffolding agents. Separate repo.
7. **agent-archetypes** — Docker images for researcher, writer, commenter, monitor, operator. Separate repo.
8. **Website docs** — Update synapbus.dev, mcpproxy.app docs with platform architecture.

### Searcher Migration

9. Refactor current 4 agents to use the archetype model (researcher archetype + domain CLAUDE.md).
10. Validate stigmergy loop with current #new_posts -> social-commenter pipeline.
