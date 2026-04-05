# Implementation Plan: Agent Wiki

## Phase 1: Backend (schema + store + service)
1. Create migration `schema/017_wiki.sql` with articles, article_revisions, article_links, articles_fts tables
2. Create `internal/wiki/` package with store.go (SQLite CRUD), service.go (business logic), types.go (Article, Revision, Link structs)
3. Link extraction: parse [[slug]] and [[slug|text]] from markdown body
4. Service methods: CreateArticle, GetArticle, UpdateArticle, ListArticles, GetBacklinks, GetMapOfContent
5. Tests: store_test.go with table-driven tests for all CRUD + link extraction

## Phase 2: MCP Actions + REST API
1. Register wiki actions in action registry: create_article, get_article, update_article, list_articles, get_backlinks
2. Add wiki bridge methods in MCP bridge.go or new wiki_bridge.go
3. REST API handlers in `internal/api/wiki.go`: GET/POST /api/wiki/articles, GET /api/wiki/articles/:slug, GET /api/wiki/articles/:slug/history, GET /api/wiki/map
4. Wire into main server setup

## Phase 3: Web UI
1. Svelte route `/wiki` — Map of Content page
2. Svelte route `/wiki/[slug]` — Article view with markdown rendering + backlinks sidebar
3. Svelte route `/wiki/[slug]/history` — Revision history
4. API client methods in client.ts
5. Sidebar navigation link to Wiki

## Phase 4: Build, Test, Deploy
1. Run `make test` — verify all tests pass including new wiki tests
2. Run `make build` — verify binary compiles
3. Docker build for linux/amd64, deploy to kubic
4. Verify via MCP tools (create/read/update articles)
5. Verify via Web UI in Chrome
