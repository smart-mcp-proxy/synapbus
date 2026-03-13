# Tasks: Semantic Search

**Input**: Design documents from `/specs/008-semantic-search/`
**Prerequisites**: spec.md (required)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story. US1 (Semantic Search) and US4 (Background Embedding) are both P1 and deeply coupled — they are split across Phases 3 and 4 but share foundational infrastructure from Phase 2.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4, US5)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Add dependencies and create package scaffolding for semantic search

- [ ] T001 Add `TFMV/hnsw` dependency to `go.mod` — run `go get github.com/TFMV/hnsw`
- [ ] T002 [P] Create `internal/search/` package structure with placeholder files: `internal/search/search.go` (package declaration and doc comment), `internal/search/types.go` (shared types)
- [ ] T003 [P] Create `internal/search/embedding/` sub-package structure: `internal/search/embedding/embedding.go` (package declaration)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented — embedding provider interface, SQLite schema for embeddings/queue, HNSW index wrapper, and config plumbing

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Create SQLite migration `schema/002_semantic_search.sql` — add `embeddings` table (`id`, `message_id` FK, `provider`, `model`, `vector` BLOB, `dimensions` INTEGER, `created_at`), `embedding_queue` table (`id`, `message_id` FK, `status` enum pending/processing/completed/failed, `attempts` INTEGER DEFAULT 0, `last_error` TEXT, `created_at`, `completed_at`), add `tags` TEXT column to `messages` table (JSON array, DEFAULT '[]'), and relevant indexes (`idx_embeddings_message` UNIQUE, `idx_embedding_queue_status`, `idx_messages_tags`)
- [ ] T005 [P] Define embedding provider interface in `internal/search/embedding/provider.go` — `EmbeddingProvider` interface with methods: `Embed(ctx context.Context, text string) ([]float32, error)`, `EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)`, `Dimensions() int`, `Name() string`, `Model() string`, `MaxTokens() int`. Also define `ProviderConfig` struct (Type, APIKey, Endpoint, Model string)
- [ ] T006 [P] Define search domain types in `internal/search/types.go` — `SearchRequest` struct (Query string, Filters *SearchFilters, Limit int, SearchMode string), `SearchFilters` struct (ChannelID *int, SenderID *string, PriorityMin/PriorityMax *int, Tags []string, After/Before *time.Time), `SearchResult` struct (Message, SimilarityScore/RelevanceScore float64, SearchMode string), `SearchResponse` struct (Results []SearchResult, SearchMode string, TotalResults int, Warning string)
- [ ] T007 [P] Implement HNSW index wrapper in `internal/search/hnsw.go` — `HNSWIndex` struct wrapping `TFMV/hnsw`, methods: `NewHNSWIndex(dimensions int, dataDir string) (*HNSWIndex, error)`, `Add(id uint64, vector []float32) error`, `Remove(id uint64) error`, `Search(query []float32, k int) ([]HNSWResult, error)` returning (id, distance) pairs, `Save() error`, `Load() error`, `Rebuild(vectors map[uint64][]float32) error`. Use `sync.RWMutex` for concurrent read safety (FR-012). Config: efConstruction=200, M=16, efSearch=100
- [ ] T008 [P] Implement embedding repository in `internal/search/repository.go` — `EmbeddingRepository` struct with SQLite `*sql.DB`, methods: `SaveEmbedding(ctx, messageID int64, provider, model string, vector []float32, dimensions int) error`, `GetEmbedding(ctx, messageID int64) (*Embedding, error)`, `GetAllEmbeddings(ctx) ([]Embedding, error)`, `DeleteEmbedding(ctx, messageID int64) error`, `DeleteAllEmbeddings(ctx) error`, `GetEmbeddingCount(ctx) (int64, error)`. Serialize float32 vectors to/from BLOB using `encoding/binary`
- [ ] T009 [P] Implement embedding queue repository in `internal/search/queue.go` — `QueueRepository` struct with SQLite `*sql.DB`, methods: `Enqueue(ctx, messageID int64) error`, `Dequeue(ctx, batchSize int) ([]QueueItem, error)` (atomically sets status=processing), `MarkCompleted(ctx, messageID int64) error`, `MarkFailed(ctx, messageID int64, errMsg string) error`, `RetryFailed(ctx, maxAttempts int) (int64, error)` (re-queues items below max attempts), `PendingCount(ctx) (int64, error)`, `GetStaleItems(ctx, olderThan time.Duration) ([]QueueItem, error)`
- [ ] T010 Implement search configuration in `internal/search/config.go` — `Config` struct (Provider ProviderConfig, Workers int default 4, BatchSize int default 50, RetryMaxAttempts int default 3, RetryBaseDelay time.Duration default 1s, HNSWEfConstruction int default 200, HNSWM int default 16, HNSWEfSearch int default 100). Parse from environment variables (`SYNAPBUS_EMBEDDING_PROVIDER`, `SYNAPBUS_EMBEDDING_API_KEY`, `SYNAPBUS_OLLAMA_URL`) and config file. Method `IsEnabled() bool` returns true if provider is configured

**Checkpoint**: Foundation ready — embedding provider interface defined, HNSW wrapper built, SQLite schema for embeddings and queue created, configuration plumbed. User story implementation can now begin in parallel.

---

## Phase 3: User Story 4 — Background Embedding on Message Ingest (Priority: P1)

**Goal**: When a message is sent, asynchronously generate an embedding and index it in HNSW. The `send_message` call returns immediately without blocking on embedding generation.

**Independent Test**: Send a message, verify `send_message` responds in < 100ms overhead, then wait briefly and confirm the message appears in the HNSW index with a valid embedding stored in SQLite.

**Rationale for Phase 3 (before US1)**: US1 (search) requires data in the index. The embedding pipeline must exist first so there is something to search against.

### Implementation for User Story 4

- [ ] T011 [P] [US4] Implement OpenAI embedding provider in `internal/search/embedding/openai.go` — `OpenAIProvider` struct implementing `EmbeddingProvider`, calls `https://api.openai.com/v1/embeddings` with model `text-embedding-3-small` (1536 dimensions). Handle HTTP POST with JSON body, parse response, extract float32 vectors. Support batch embedding (max 2048 inputs per batch per OpenAI limits). Truncate text to MaxTokens (8191) before sending. Return descriptive errors for 401/429/500 status codes
- [ ] T012 [P] [US4] Implement Ollama embedding provider in `internal/search/embedding/ollama.go` — `OllamaProvider` struct implementing `EmbeddingProvider`, calls `POST {endpoint}/api/embeddings` with configurable model (default `nomic-embed-text`, 768 dimensions). Support custom endpoint URL. Implement `EmbedBatch` by sequential calls (Ollama does not support batch). Handle connection refused and timeout errors gracefully
- [ ] T013 [P] [US4] Implement Gemini embedding provider in `internal/search/embedding/gemini.go` — `GeminiProvider` struct implementing `EmbeddingProvider`, calls Google Gemini embedding API (`POST https://generativelanguage.googleapis.com/v1beta/models/{model}:embedContent`) with model `text-embedding-004` (768 dimensions). API key passed as query param. Handle batch via `batchEmbedContents` endpoint
- [ ] T014 [US4] Implement provider factory in `internal/search/embedding/factory.go` — `NewProvider(cfg ProviderConfig) (EmbeddingProvider, error)` function that returns the appropriate provider based on `cfg.Type` ("openai", "gemini", "ollama"). Return clear error for unknown provider type. Validate required fields (API key for openai/gemini, endpoint for ollama)
- [ ] T015 [US4] Implement embedding pipeline worker in `internal/search/pipeline.go` — `Pipeline` struct with dependencies: `EmbeddingProvider`, `EmbeddingRepository`, `QueueRepository`, `*HNSWIndex`, `*slog.Logger`. Method `Start(ctx context.Context)` launches N worker goroutines (from config.Workers). Each worker loops: dequeue batch from queue, call `provider.EmbedBatch`, save embeddings to SQLite via repository, add vectors to HNSW index. Implement exponential backoff retry (base 1s, max 3 attempts per FR-010). Handle empty/whitespace message bodies by skipping (mark completed). Handle text truncation to provider's MaxTokens with slog warning (FR-013). Graceful shutdown via context cancellation
- [ ] T016 [US4] Implement message ingest hook in `internal/search/ingest.go` — `IngestHook` struct that receives new message events and enqueues them for embedding. Method `OnMessageCreated(ctx, messageID int64, body string)` — if body is empty/whitespace, skip; otherwise insert into embedding_queue with status=pending. Method `OnMessageDeleted(ctx, messageID int64)` — remove from embedding_queue if pending, delete embedding from repository, remove from HNSW index (FR-014). This must be non-blocking: use a buffered channel or direct SQLite insert (queue table)
- [ ] T017 [US4] Implement HNSW index initialization and recovery in `internal/search/index_manager.go` — `IndexManager` struct, method `Initialize(ctx) error`: load HNSW index from disk if file exists, validate dimensions match configured provider, if dimensions mismatch or file missing/corrupted then rebuild from SQLite embeddings table. Method `DetectProviderChange(ctx, currentProvider string) (bool, error)`: compare stored provider/model in embeddings table against current config, return true if changed. Method `TriggerBackfill(ctx) error`: mark all existing embeddings as stale (delete from embeddings, re-enqueue all message IDs to embedding_queue)

**Checkpoint**: At this point, messages sent via `send_message` are asynchronously embedded and indexed. The HNSW index contains searchable vectors. The pipeline handles errors, retries, and edge cases.

---

## Phase 4: User Story 1 — Agent Searches Messages by Meaning (Priority: P1) MVP

**Goal**: An agent calls the `search_messages` MCP tool with a natural language query and receives semantically relevant messages ranked by cosine similarity, with access control enforced.

**Independent Test**: Configure an embedding provider, send several messages with varied vocabulary about related topics, then issue a `search_messages` call with different wording. Verify the top results are semantically relevant even without exact keyword matches.

### Implementation for User Story 1

- [ ] T018 [US1] Implement semantic search engine in `internal/search/semantic.go` — `SemanticEngine` struct with dependencies: `EmbeddingProvider`, `*HNSWIndex`, `EmbeddingRepository`, `*sql.DB`. Method `Search(ctx, query string, limit int) ([]SearchResult, error)`: embed the query text using the provider, call `HNSWIndex.Search(queryVec, limit * 3)` for over-fetch (to account for access control filtering), convert HNSW distances to cosine similarity scores (1.0 - distance), look up message objects by ID from SQLite, return results sorted by descending similarity. Handle provider errors by returning error (caller decides fallback)
- [ ] T019 [US1] Implement full-text search engine in `internal/search/fulltext.go` — `FullTextEngine` struct with `*sql.DB`. Method `Search(ctx, query string, limit int) ([]SearchResult, error)`: query `messages_fts` using FTS5 `MATCH` with `bm25()` ranking function, join to `messages` table for full message data, return results with `relevance_score` (BM25 score normalized). Handle FTS5 syntax errors gracefully (e.g., special characters in query)
- [ ] T020 [US1] Implement unified search service in `internal/search/service.go` — `Service` struct orchestrating semantic and full-text engines. Method `Search(ctx, req SearchRequest, callerAgent string) (*SearchResponse, error)`: determine search mode (auto/semantic/fulltext per FR-006), if semantic: try `SemanticEngine.Search()`, on error fall back to full-text with warning. Apply access control filter: query `channel_members` to get channels the callerAgent has joined, filter results to only include messages from those channels or DMs to/from callerAgent (FR-005). Enforce limit (default 10, max 100). Build `SearchResponse` with `search_mode` and optional `warning` fields (FR-008)
- [ ] T021 [US1] Implement `search_messages` MCP tool in `internal/mcp/search_tool.go` — register MCP tool `search_messages` with JSON Schema per FR-015: input schema with `query` (string, required), `filters` (object, optional), `limit` (integer, optional, default 10, max 100), `search_mode` (string, optional, enum: auto/semantic/fulltext, default auto). Handler: extract caller agent identity from MCP session context, build `SearchRequest` from tool input, call `search.Service.Search()`, marshal `SearchResponse` to MCP tool result. Include field descriptions in JSON Schema for agent discoverability
- [ ] T022 [US1] Wire search service into server startup in `cmd/synapbus/main.go` — in `runServe()`: parse search config from env/config, if embedding provider configured: create provider via factory, initialize HNSW index via IndexManager (with recovery), create repositories, start Pipeline workers, create SemanticEngine. Always create FullTextEngine. Create search Service with available engines. Register `search_messages` MCP tool. Handle graceful shutdown of pipeline workers

**Checkpoint**: At this point, the `search_messages` MCP tool is functional. Agents can search by meaning with results ranked by cosine similarity. Access control is enforced. Full system is wired end-to-end.

---

## Phase 5: User Story 2 — Combined Search with Metadata Filters (Priority: P2)

**Goal**: An agent calls `search_messages` with both a semantic query and structured filters (channel_id, sender_id, priority range, tags, date range). Results respect all filters while being ranked by semantic relevance.

**Independent Test**: Send messages about the same topic across multiple channels and from different agents. Issue filtered searches and verify results respect all filter constraints while maintaining semantic ranking.

### Implementation for User Story 2

- [ ] T023 [US2] Implement filter query builder in `internal/search/filters.go` — `FilterBuilder` struct, method `BuildWhereClause(filters *SearchFilters) (clause string, args []interface{})`: generate SQL WHERE conditions for channel_id, from_agent (sender_id), priority BETWEEN min/max, created_at after/before, tags containment (JSON `json_each` for SQLite JSON array matching). Return composable clause and parameterized args. Handle nil filters (no-op). Handle tags filter using `EXISTS (SELECT 1 FROM json_each(messages.metadata, '$.tags') WHERE json_each.value IN (...))`  or use the `tags` column added in T004
- [ ] T024 [US2] Integrate filters into semantic search engine in `internal/search/semantic.go` — modify `SemanticEngine.Search()` to accept `*SearchFilters`, apply pre-filtering strategy: first query SQLite with filters to get candidate message IDs, then intersect with HNSW ANN results. If filter is very selective (< 1000 candidates), compute cosine similarity directly against candidate vectors instead of using HNSW (brute-force on small set). This avoids HNSW returning results that are later all filtered out
- [ ] T025 [US2] Integrate filters into full-text search engine in `internal/search/fulltext.go` — modify `FullTextEngine.Search()` to accept `*SearchFilters`, append filter WHERE clauses to FTS5 query JOIN, ensuring filters and FTS ranking work together
- [ ] T026 [US2] Update search service and MCP tool for filter support in `internal/search/service.go` and `internal/mcp/search_tool.go` — parse `filters` object from MCP tool input into `SearchFilters` struct (channel_id string, sender_id string, priority_min/priority_max int, tags []string, after/before RFC3339 strings parsed to time.Time). Pass filters through to search engines. Update MCP tool JSON Schema to document filter fields with types and descriptions

**Checkpoint**: At this point, `search_messages` supports full metadata filtering combined with semantic or full-text ranking. US1 and US2 are both functional and independently testable.

---

## Phase 6: User Story 3 — Graceful Fallback to Full-Text Search (Priority: P2)

**Goal**: When no embedding provider is configured or the provider becomes unreachable, `search_messages` transparently falls back to FTS5 full-text search. The response indicates the search mode used.

**Independent Test**: Start SynapBus with no embedding provider configured. Send messages and call `search_messages`. Verify results come from FTS5 and the response includes `search_mode: "fulltext"`.

### Implementation for User Story 3

- [ ] T027 [US3] Implement provider health checker in `internal/search/embedding/health.go` — `HealthChecker` struct wrapping `EmbeddingProvider`, method `IsHealthy(ctx) bool`: attempt a lightweight embed call (e.g., embed "health check" string) with short timeout (2s), cache result for 30s to avoid hammering provider. Method `LastError() error`. Used by search service to determine if semantic search is available at query time
- [ ] T028 [US3] Implement fallback logic in search service `internal/search/service.go` — modify `Search()` method: when `search_mode` is "auto", check `HealthChecker.IsHealthy()` — if provider unavailable, use full-text with warning "embedding provider unavailable, using full-text fallback" (FR-006, acceptance scenario 2). When `search_mode` is "semantic" but provider is down, return error explaining provider is unavailable. When no provider is configured at all, always use full-text mode with no warning (it is the expected mode)
- [ ] T029 [US3] Implement startup backfill trigger in `internal/search/index_manager.go` — extend `Initialize()`: after loading index, if provider was just configured (embeddings table empty but messages exist), enqueue all existing message IDs to embedding_queue for background processing. Use configurable batch size and delay between batches to avoid overwhelming the provider (edge case: thousands of unembedded messages). Log progress: "backfill: enqueued N messages for embedding"

**Checkpoint**: SynapBus works fully without an embedding provider (FTS5 fallback). Runtime provider failures are handled gracefully with automatic fallback and warning messages. Provider addition triggers backfill.

---

## Phase 7: User Story 5 — Configurable Embedding Providers (Priority: P3)

**Goal**: Operators can choose between OpenAI, Gemini, and Ollama embedding providers. Switching providers triggers a full re-embedding backfill.

**Independent Test**: Configure each provider independently, send messages, verify embeddings are generated. Switch provider in config and restart — verify old embeddings are invalidated and messages are re-embedded with the new provider.

### Implementation for User Story 5

- [ ] T030 [US5] Implement provider switch detection and re-embedding in `internal/search/index_manager.go` — extend `DetectProviderChange()`: on startup, query the embeddings table for the most recent provider/model values, compare against current config. If changed: log "provider changed from X to Y, triggering re-embed", call `DeleteAllEmbeddings()` on repository, call `HNSWIndex.Rebuild(nil)` to clear the index (edge case: dimension mismatch requires new index), enqueue all message IDs to embedding_queue. Ensure full-text search remains available during backfill
- [ ] T031 [US5] Add provider configuration validation in `internal/search/embedding/factory.go` — extend `NewProvider()` with validation: OpenAI requires non-empty APIKey, Gemini requires non-empty APIKey, Ollama requires valid endpoint URL (try HEAD request with 2s timeout). Return actionable error messages: "openai provider requires SYNAPBUS_EMBEDDING_API_KEY to be set". Add `ValidateConfig(cfg ProviderConfig) error` exported function for use during startup config validation
- [ ] T032 [US5] Add embedding provider status to server info in `internal/search/service.go` — add method `Status() SearchStatus` returning current provider name/model, index size (vector count), queue depth (pending embeddings), provider health, and whether a backfill is in progress. This can be exposed via REST API for the Web UI dashboard and via a future MCP resource

**Checkpoint**: All three embedding providers are implemented, validated, and switchable. Provider changes trigger automatic re-embedding. Operators have visibility into embedding status.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T033 [P] Add structured logging throughout search package — ensure all operations in `internal/search/` use `slog` with consistent attributes: `component=search`, `provider`, `message_id`, `queue_depth`, `duration_ms`. Log at appropriate levels: Info for lifecycle events (start/stop/backfill), Warn for truncation and fallback, Error for provider failures
- [ ] T034 [P] Add vector serialization helpers in `internal/search/vector.go` — `SerializeVector(v []float32) []byte` and `DeserializeVector(b []byte) []float32` using `encoding/binary.LittleEndian`. Add `CosineSimilarity(a, b []float32) float64` utility. Add `ValidateVector(v []float32, expectedDim int) error`. These support the repository and HNSW wrapper
- [ ] T035 Implement HNSW index persistence and corruption recovery in `internal/search/hnsw.go` — extend `Save()` to write to `{dataDir}/hnsw.idx` with temp-file-then-rename for atomic writes. Extend `Load()` to detect corrupted files (invalid header, dimension mismatch) and trigger rebuild from SQLite. Add periodic auto-save (every 5 minutes or every N insertions) via background goroutine
- [ ] T036 [P] Add search-related trace logging in `internal/search/service.go` — after each `Search()` call, write a trace record (agent_name, action="search_messages", details JSON with query/filters/mode/result_count/duration_ms) to the `traces` table per Constitution Principle VIII
- [ ] T037 Code cleanup and edge case hardening — review all search package files for: empty body handling (skip embedding), duplicate message embedding (upsert semantics), concurrent HNSW access under load, queue stale item cleanup (items stuck in "processing" for > 5 minutes reset to "pending"), and message deletion cascading to embedding cleanup
- [ ] T038 Run `make lint` and `make test` — ensure all new code passes linting and existing tests are not broken. Fix any issues

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion — BLOCKS all user stories
- **US4 - Background Embedding (Phase 3)**: Depends on Foundational — must complete before US1 (provides data to search)
- **US1 - Semantic Search (Phase 4)**: Depends on Foundational + US4 (needs vectors in index)
- **US2 - Filtered Search (Phase 5)**: Depends on US1 (extends search engines)
- **US3 - Fallback (Phase 6)**: Depends on US1 (modifies search service); can proceed in parallel with US2
- **US5 - Provider Config (Phase 7)**: Depends on US4 (extends provider factory and index manager); can proceed in parallel with US2/US3
- **Polish (Phase 8)**: Depends on all user stories being complete

### User Story Dependencies

- **US4 (P1)**: Can start after Foundational (Phase 2) — provides the embedding pipeline that US1 depends on
- **US1 (P1)**: Depends on US4 — search requires vectors to exist in the index
- **US2 (P2)**: Depends on US1 — extends search engines with filter support
- **US3 (P2)**: Depends on US1 — modifies search service fallback logic. Can be done in parallel with US2
- **US5 (P3)**: Depends on US4 — extends provider factory and index manager. Can be done in parallel with US2/US3

### Within Each User Story

- Models/types before repositories
- Repositories before services
- Services before MCP tool handlers
- MCP tool before wiring in main.go
- Core implementation before integration

### Parallel Opportunities

- All Foundational tasks T005-T010 marked [P] can run in parallel (different files)
- T011, T012, T013 (embedding providers) can run in parallel (different files)
- US2 and US3 can proceed in parallel after US1 completes
- US5 can proceed in parallel with US2/US3 after US4 completes
- All Phase 8 tasks marked [P] can run in parallel

---

## Implementation Strategy

### MVP First (US4 + US1 Only)

1. Complete Phase 1: Setup (add dependencies)
2. Complete Phase 2: Foundational (schema, interfaces, HNSW wrapper, repos, config)
3. Complete Phase 3: US4 — Background Embedding (providers, pipeline, ingest hook)
4. Complete Phase 4: US1 — Semantic Search (search engines, MCP tool, wiring)
5. **STOP and VALIDATE**: Send messages, call `search_messages`, verify semantic results with access control

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. Add US4 (embedding pipeline) -> Messages are being embedded
3. Add US1 (search) -> Test independently -> Deploy/Demo (MVP!)
4. Add US2 (filters) -> Test independently -> Deploy/Demo
5. Add US3 (fallback) -> Test independently -> Deploy/Demo
6. Add US5 (multi-provider) -> Test independently -> Deploy/Demo
7. Each story adds value without breaking previous stories

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- US4 is placed before US1 because search requires indexed vectors — the pipeline must exist first
- The HNSW index uses `sync.RWMutex` for concurrent read-during-write safety (FR-012)
- Vector serialization uses `encoding/binary.LittleEndian` for cross-platform consistency
- All embedding providers are pure Go HTTP clients — zero CGO (Constitution Principle III)
- FTS5 index already exists in `schema/001_initial.sql` — US3 fallback builds on existing infrastructure
- Commit after each task or logical group
