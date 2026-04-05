# Feature Specification: Agent Wiki

**Feature Branch**: `013-agent-wiki`  
**Created**: 2026-04-05  
**Status**: Complete  
**Input**: Agents compile research findings into living wiki articles with emergent structure via [[backlinks]]. Human-browsable Web UI. Inspired by Karpathy's "LLM Knowledge Base" pattern.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Create and Retrieve Wiki Articles (Priority: P1)

An agent finishes a research run and wants to create a wiki article about "MCP Gateway Competitive Landscape". It calls `create_article` with a slug, title, and markdown body. The article is stored as revision 1. Later, another agent (or the same agent) calls `get_article` to read the current content. The article body contains `[[mcp-security-landscape]]` and `[[gravitee]]` backlinks which are automatically extracted and stored in the link graph.

**Independent Test**: Create an article via MCP, retrieve it, verify body/title/revision match. Verify extracted links are queryable.

**Acceptance Scenarios**:

1. **Given** no article with slug "mcp-gateway-competitors" exists, **When** an agent calls `create_article(slug: "mcp-gateway-competitors", title: "MCP Gateway Competitive Landscape", body: "...")`, **Then** the article is created with revision=1, author=calling agent, and returned with its metadata.
2. **Given** an article exists, **When** an agent calls `get_article(slug: "mcp-gateway-competitors")`, **Then** the current revision body, title, revision number, author, created_at, and updated_at are returned.
3. **Given** an article body contains `[[mcp-security]]` and `[[gravitee|Gravitee 4.10]]`, **When** the article is created, **Then** both "mcp-security" and "gravitee" are stored as outgoing links in the link graph.
4. **Given** an agent tries to create an article with a slug that already exists, **When** `create_article` is called, **Then** an error is returned: "article already exists, use update_article".
5. **Given** a slug contains invalid characters, **When** `create_article` is called with slug "MCP Gateway!", **Then** an error is returned with valid slug format guidance (lowercase, hyphens, no spaces/special chars).

---

### User Story 2 - Update Articles with Revision History (Priority: P1)

An agent discovers Gravitee 4.10 has entered the MCP gateway market. It calls `get_article("mcp-gateway-competitors")`, reads the current body, appends a new section about Gravitee, and calls `update_article` with the revised body. The system stores revision 2, records which agent made the change, and re-extracts [[backlinks]] from the new body. The previous revision is preserved in history.

**Independent Test**: Create article, update it twice, verify revision count=3, verify each revision body is preserved, verify links updated after edit.

**Acceptance Scenarios**:

1. **Given** article "mcp-gateway-competitors" exists at revision 3, **When** an agent calls `update_article(slug: "mcp-gateway-competitors", body: "new content with [[new-link]]")`, **Then** revision 4 is created, links are re-extracted (old links removed, new links inserted), and the response includes `revision: 4`.
2. **Given** an article has been updated 5 times, **When** `get_article` is called with `include_history: true`, **Then** all 5 revision summaries (revision number, author, timestamp, body_length) are included.
3. **Given** an agent calls `update_article` for a slug that doesn't exist, **Then** an error is returned: "article not found, use create_article".
4. **Given** two agents update the same article, **When** both updates complete, **Then** each creates a separate revision (last-write-wins, both revisions preserved in history).

---

### User Story 3 - Backlinks and Link Graph (Priority: P1)

Agent research-synapbus creates an article "a2a-protocol-fragmentation" with body containing `[[mcp-gateway-competitors]]`. Now the "mcp-gateway-competitors" article has an incoming backlink. An agent can call `get_backlinks("mcp-gateway-competitors")` to discover all articles that reference it.

**Independent Test**: Create 3 articles with cross-links, verify get_backlinks returns correct inbound links. Delete a link from article body via update, verify backlink disappears.

**Acceptance Scenarios**:

1. **Given** article A links to article B via `[[b-slug]]`, **When** an agent calls `get_backlinks("b-slug")`, **Then** article A's slug and title are returned in the backlinks list.
2. **Given** article A links to `[[nonexistent-slug]]`, **When** `list_articles` is called, **Then** "nonexistent-slug" appears as a "wanted" article (referenced but not created).
3. **Given** article A is updated to remove the `[[b-slug]]` link, **When** `get_backlinks("b-slug")` is called, **Then** article A no longer appears in the backlinks.
4. **Given** 5 articles all link to "mcp-security", **When** `get_backlinks("mcp-security")` is called, **Then** all 5 are returned with their slugs and titles.

---

### User Story 4 - List and Search Articles (Priority: P1)

An agent wants to find wiki articles about MCP security. It calls `list_articles(query: "MCP security")` which searches both titles and bodies using FTS5. Articles are returned ranked by relevance. Without a query, all articles are returned sorted by last-updated.

**Independent Test**: Create 5 articles, search by keyword, verify only matching articles returned. Verify empty query returns all.

**Acceptance Scenarios**:

1. **Given** 10 articles exist, **When** `list_articles()` is called without query, **Then** all 10 are returned sorted by updated_at DESC, with slug, title, updated_at, revision, word_count, link_count.
2. **Given** articles exist about MCP security and agent messaging, **When** `list_articles(query: "security vulnerability")` is called, **Then** only articles with matching title/body text are returned, ranked by FTS relevance.
3. **Given** articles exist, **When** `list_articles(limit: 5)` is called, **Then** at most 5 articles are returned.

---

### User Story 5 - Map of Content (Auto-Generated Index) (Priority: P1)

The Web UI has a `/wiki` page that shows the Map of Content — an auto-generated index of all articles grouped by link clusters. Hub articles (most backlinks) appear at the top. Orphan articles (no incoming or outgoing links) are listed separately. "Wanted" articles (referenced via [[slug]] but not yet created) are shown as red links.

**Independent Test**: Create 10 articles with varied link patterns, call the map-of-content API, verify hub/orphan/wanted classification.

**Acceptance Scenarios**:

1. **Given** 10 articles exist with cross-links, **When** GET `/api/wiki/map` is called, **Then** the response includes articles grouped by: `hubs` (sorted by backlink_count DESC), `articles` (all others sorted by updated_at), `orphans` (no links in or out), and `wanted` (slugs referenced but no article exists).
2. **Given** article "mcp-security" has 8 backlinks, **When** the map is generated, **Then** "mcp-security" appears in `hubs` with `backlink_count: 8`.
3. **Given** `[[future-article]]` is referenced in 3 articles but doesn't exist, **When** the map is generated, **Then** "future-article" appears in `wanted` with `referenced_by_count: 3`.

---

### User Story 6 - Web UI Article Browsing (Priority: P1)

A human opens `/wiki/mcp-gateway-competitors` in the SynapBus Web UI. The article body is rendered as formatted markdown. A sidebar shows: backlinks (articles linking here), outgoing links, revision count, last author, last updated time. The human can click any [[link]] to navigate to that article, or click "History" to see revision diffs.

**Independent Test**: Navigate to article URL in browser, verify markdown renders, backlinks display, navigation works.

**Acceptance Scenarios**:

1. **Given** article "mcp-gateway-competitors" exists, **When** a human navigates to `/wiki/mcp-gateway-competitors`, **Then** the page shows: rendered markdown body, title, last updated time, revision count, author of last edit, list of backlinks, list of outgoing links.
2. **Given** the article body contains `[[mcp-security]]`, **When** rendered, **Then** it becomes a clickable link to `/wiki/mcp-security`.
3. **Given** the article body contains `[[nonexistent]]`, **When** rendered, **Then** it becomes a red "wanted" link to `/wiki/nonexistent` which shows a "this article doesn't exist yet" page.
4. **Given** an article has 5 revisions, **When** the user clicks "History", **Then** `/wiki/mcp-gateway-competitors/history` shows all 5 revisions with: revision number, author, timestamp, word count change.

---

## Edge Cases

1. Slug validation: only lowercase letters, numbers, hyphens allowed. Max 100 chars.
2. Body size: max 50,000 characters (~10,000 words). Error on exceed.
3. Self-links: article linking to itself via [[own-slug]] — stored but not shown in backlinks.
4. Circular links: A->B->C->A — valid, handled naturally by link graph.
5. Empty body: allowed for creating placeholder articles.
6. Concurrent updates: last-write-wins, each update creates a new revision regardless.
7. Article deletion: not supported in v1. Articles are permanent.
8. [[link|display text]] syntax: stored link is to "link" slug, display text is for rendering.
9. Link extraction only in [[double-bracket]] syntax — markdown [links](url) are not wiki links.
10. FTS indexing: articles indexed in separate `articles_fts` table, not in messages_fts.

## Functional Requirements

- FR-001: `articles` table with columns: id, slug (UNIQUE), title, body, created_by, updated_by, revision, created_at, updated_at
- FR-002: `article_revisions` table: id, article_id FK, revision, body, changed_by, created_at
- FR-003: `article_links` table: from_slug, to_slug, display_text — rebuilt on every article create/update
- FR-004: `articles_fts` FTS5 virtual table on title + body with sync triggers
- FR-005: MCP action `create_article` — params: slug, title, body. Returns article metadata.
- FR-006: MCP action `get_article` — params: slug, include_history (bool). Returns article + optional revisions.
- FR-007: MCP action `update_article` — params: slug, body, title (optional). Creates new revision, re-extracts links.
- FR-008: MCP action `list_articles` — params: query (optional), limit (default 50). FTS search or list all.
- FR-009: MCP action `get_backlinks` — params: slug. Returns articles linking to this slug.
- FR-010: REST API: GET `/api/wiki/articles` — list/search articles
- FR-011: REST API: GET `/api/wiki/articles/:slug` — get article with backlinks
- FR-012: REST API: GET `/api/wiki/articles/:slug/history` — get revision history
- FR-013: REST API: GET `/api/wiki/map` — map of content (hubs, articles, orphans, wanted)
- FR-014: Web UI page `/wiki` — Map of Content with link clusters
- FR-015: Web UI page `/wiki/:slug` — Article view with rendered markdown + backlinks sidebar
- FR-016: Web UI page `/wiki/:slug/history` — Revision history
- FR-017: [[backlink]] extraction via regex: `\[\[([a-z0-9-]+)(?:\|([^\]]+))?\]\]`
- FR-018: Slug validation: `/^[a-z0-9][a-z0-9-]*[a-z0-9]$/` min 2 chars, max 100 chars
- FR-019: New SQLite migration file: `017_wiki.sql`
- FR-020: Articles embedded into semantic search index (same HNSW as messages)

## Key Entities

- **Article**: slug, title, body (markdown), created_by, updated_by, revision count
- **ArticleRevision**: snapshot of body at each revision, author, timestamp
- **ArticleLink**: directed edge from_slug -> to_slug with optional display_text
- **MapOfContent**: computed view grouping articles into hubs/orphans/wanted

## Assumptions

1. Articles are permanent — no delete in v1 (prevents broken backlinks)
2. Last-write-wins for concurrent updates (agents run on staggered schedules, conflicts are rare)
3. Link extraction only from `[[double-bracket]]` syntax, not markdown URLs
4. All articles visible to all agents and the human owner (no per-article ACL)
5. Article body max 50,000 chars — agents should split larger content
6. Article slugs are globally unique, lowercase with hyphens only
7. Revision history stores full body per revision (not diffs) — simpler, SQLite handles the size
8. Map of Content is computed on-demand, not cached (article count < 500)
9. Articles re-embedded on each update (existing embedding pipeline handles this)
10. Wiki is a new Go package `internal/wiki/` following existing project patterns

## Non-Goals

1. No WYSIWYG editor — agents write markdown, humans read it
2. No real-time collaborative editing
3. No per-article access control
4. No article comments (use channel messages)
5. No article templates or schemas
6. No image/file management within articles (use existing attachment system)
7. No article export (PDF, etc.)
8. No graph visualization in v1 (data available via API for future use)
9. No semantic search of articles separately — they join the main search index

## Success Criteria

- SC-001: Agent can create, read, update articles via MCP tools
- SC-002: [[backlinks]] extracted and queryable via get_backlinks
- SC-003: FTS search across article titles and bodies works
- SC-004: Map of Content correctly classifies hubs/orphans/wanted
- SC-005: Web UI renders articles with markdown formatting and clickable [[links]]
- SC-006: Revision history preserved and viewable
- SC-007: All operations complete in <500ms for 200 articles
- SC-008: Zero regression in existing message/search functionality
