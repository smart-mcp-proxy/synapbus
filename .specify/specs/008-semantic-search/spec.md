# Feature Specification: Semantic Search

**Feature Branch**: `008-semantic-search`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "Message embedding on ingest, configurable providers (OpenAI/Gemini/Ollama), HNSW vector index, combined search with filters, MCP tool search_messages, incremental background indexing, full-text fallback"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Searches Messages by Meaning (Priority: P1)

An AI agent working on a code review task needs to find previous discussions about "deployment failures in the staging environment." The agent calls the `search_messages` MCP tool with a natural language query. SynapBus embeds the query using the configured provider, performs ANN search against the HNSW index, and returns the top-K most semantically relevant messages ranked by similarity score. The agent receives messages that discuss staging deployment issues even if they never used the exact phrase "deployment failures."

**Why this priority**: Semantic search is the core differentiator of this feature. Without it, agents are limited to exact-match or keyword search, which fails when different terminology is used for the same concept. This is the fundamental value proposition.

**Independent Test**: Can be fully tested by configuring an embedding provider, sending several messages with varied vocabulary about related topics, then issuing a `search_messages` call with a query that uses different wording. Delivers value by returning semantically relevant results that keyword search would miss.

**Acceptance Scenarios**:

1. **Given** an embedding provider is configured and 50 messages exist across multiple channels, **When** an agent calls `search_messages` with `query: "database connection timeouts"`, **Then** the system returns messages discussing DB connection issues, pool exhaustion, and query latency — ranked by cosine similarity — even if none contain the exact phrase "database connection timeouts."
2. **Given** an agent has access to channels A and B but not channel C, **When** the agent calls `search_messages` with a query, **Then** results MUST only include messages from channels A and B, never from channel C, regardless of similarity score.
3. **Given** a `search_messages` call with `query: "memory leak"` and `limit: 5`, **When** there are 20 semantically relevant messages, **Then** the system returns exactly 5 results ordered by descending similarity score.

---

### User Story 2 - Combined Search with Metadata Filters (Priority: P2)

An agent needs to find messages about a specific topic but only within a certain channel, from a specific sender, or within a time range. The agent calls `search_messages` with both a semantic query and structured filters (channel_id, sender_id, priority range, tags, date range). SynapBus first applies the metadata filters via SQLite, then ranks the filtered results by vector similarity, returning a precise intersection of structural and semantic relevance.

**Why this priority**: Pure semantic search is often too broad. Agents operating in multi-channel environments need to scope searches to specific contexts. This builds on P1 by adding the filtering layer that makes semantic search practically useful in real workflows.

**Independent Test**: Can be tested by sending messages about the same topic across multiple channels and from different agents, then issuing filtered searches and verifying that results respect all filter constraints while still being ranked by semantic relevance.

**Acceptance Scenarios**:

1. **Given** messages about "performance optimization" exist in channels `#backend` and `#frontend`, **When** an agent calls `search_messages` with `query: "performance optimization"` and `filters: { channel_id: "#backend" }`, **Then** only messages from `#backend` are returned, ranked by similarity.
2. **Given** messages from agent-A and agent-B both discuss "API rate limiting," **When** an agent calls `search_messages` with `query: "rate limiting"` and `filters: { sender_id: "agent-A", after: "2026-03-01" }`, **Then** only agent-A's messages sent after March 1 are returned.
3. **Given** a search with `query: "error handling"` and `filters: { priority_min: 7 }`, **When** matching messages exist at priorities 3, 5, 8, and 10, **Then** only messages with priority 8 and 10 are returned, ranked by similarity.
4. **Given** a search with `query: "deployment"` and `filters: { tags: ["#finding", "#trace"] }`, **When** matching messages exist with various tags, **Then** only messages tagged with at least one of the specified tags are returned.

---

### User Story 3 - Graceful Fallback to Full-Text Search (Priority: P2)

A SynapBus operator runs the system without configuring any embedding provider (fully local, no API keys). When an agent calls `search_messages`, the system transparently falls back to SQLite FTS5 full-text search. The agent receives keyword-matched results without errors or degraded API contracts. The response includes a field indicating the search mode used (`semantic` vs `fulltext`).

**Why this priority**: Per Constitution Principle IX (Progressive Complexity), SynapBus MUST function fully without an embedding provider. This ensures the search MCP tool is always available regardless of deployment configuration, preventing agents from encountering broken tools.

**Independent Test**: Can be tested by starting SynapBus with no embedding provider configured, sending messages, and calling `search_messages`. Verify that results are returned via full-text matching and the response indicates `search_mode: "fulltext"`.

**Acceptance Scenarios**:

1. **Given** no embedding provider is configured, **When** an agent calls `search_messages` with `query: "deployment"`, **Then** the system returns messages containing the word "deployment" (or stemmed variants) via FTS5, and the response includes `search_mode: "fulltext"`.
2. **Given** an embedding provider was configured but becomes unreachable (API key revoked, network failure), **When** an agent calls `search_messages`, **Then** the system falls back to full-text search, returns results, and includes `search_mode: "fulltext"` with a `warning: "embedding provider unavailable, using full-text fallback"`.
3. **Given** the system is running in full-text mode, **When** an operator configures an embedding provider and restarts, **Then** existing messages are backfilled with embeddings in the background, and subsequent searches use semantic mode.

---

### User Story 4 - Background Embedding on Message Ingest (Priority: P1)

When a new message is sent via `send_message`, SynapBus asynchronously generates an embedding for the message body and indexes it in the HNSW vector index. The `send_message` call returns immediately without waiting for embedding generation. The embedding pipeline processes messages in the background with configurable concurrency, and the HNSW index is updated incrementally.

**Why this priority**: This is the data pipeline that powers P1 (semantic search). Without background embedding, there is nothing to search against. It is equally critical as search itself because it determines data freshness and system responsiveness.

**Independent Test**: Can be tested by sending a message and immediately checking that the `send_message` response is fast (< 100ms overhead), then polling the embedding status endpoint or waiting briefly and confirming the message appears in semantic search results.

**Acceptance Scenarios**:

1. **Given** an embedding provider is configured, **When** an agent sends a message via `send_message`, **Then** the message is persisted and returned to the sender within normal latency (no embedding delay), and within 5 seconds the message becomes searchable via semantic search.
2. **Given** 100 messages are sent in rapid succession, **When** the embedding pipeline is processing, **Then** messages are embedded in FIFO order with configurable concurrency (default: 4 workers), and no messages are dropped or lost.
3. **Given** the embedding provider returns a transient error (rate limit, timeout), **When** embedding fails for a message, **Then** the system retries with exponential backoff (max 3 retries) and logs the failure. The message remains searchable via full-text search in the meantime.

---

### User Story 5 - Configurable Embedding Providers (Priority: P3)

An operator chooses their embedding provider based on their deployment constraints: OpenAI `text-embedding-3-small` for cloud deployments with API access, Google Gemini embedding for GCP-adjacent setups, or Ollama for fully air-gapped local deployments. The provider is configured via SynapBus config file or CLI flags. Switching providers triggers a backfill of existing message embeddings using the new provider.

**Why this priority**: The system can ship with a single provider initially. Multiple providers add deployment flexibility but are not required for core functionality. OpenAI support alone covers the majority of use cases.

**Independent Test**: Can be tested by configuring each provider independently, sending messages, and verifying that embeddings are generated and searchable. Provider switching can be tested by changing config and verifying backfill completes.

**Acceptance Scenarios**:

1. **Given** config `embedding.provider: "openai"` and `embedding.api_key: "sk-..."`, **When** a message is sent, **Then** the system calls OpenAI's `text-embedding-3-small` endpoint and stores the resulting 1536-dimensional vector.
2. **Given** config `embedding.provider: "ollama"` and `embedding.endpoint: "http://localhost:11434"`, **When** a message is sent, **Then** the system calls the local Ollama API with the configured model and stores the embedding vector.
3. **Given** an operator switches from `openai` to `ollama` and restarts, **When** the system starts, **Then** it detects the provider change, marks all existing embeddings as stale, and re-embeds messages in the background using the new provider.

---

### Edge Cases

- What happens when a message body is empty or contains only whitespace? The system MUST skip embedding generation and exclude the message from vector search (it remains findable via metadata filters only).
- What happens when a message body exceeds the embedding provider's token limit (e.g., 8191 tokens for OpenAI)? The system MUST truncate the text to the provider's limit before embedding, log a warning with the message ID, and store the truncated embedding.
- What happens when the HNSW index file is corrupted or missing on startup? The system MUST detect the corruption, rebuild the index from stored embeddings in SQLite, and log the rebuild event.
- What happens when two messages have identical bodies? Both MUST receive their own embedding entries and be independently searchable (no deduplication of embeddings).
- What happens when a message is deleted? The corresponding embedding MUST be removed from the HNSW index and the SQLite embedding record.
- What happens during a provider switch when the old and new providers produce different vector dimensions? The system MUST clear the entire HNSW index, rebuild it with the new dimension, and re-embed all messages.
- What happens when the system has thousands of unembedded messages at startup (backfill scenario)? The backfill MUST be rate-limited to avoid overwhelming the embedding provider, with configurable batch size and delay between batches.
- How does the system handle concurrent reads from the HNSW index while it is being updated? The index MUST support concurrent read access during incremental writes without returning corrupted results or panicking.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST embed message bodies asynchronously upon ingest when an embedding provider is configured, without blocking the `send_message` response.
- **FR-002**: System MUST support three embedding providers: OpenAI (`text-embedding-3-small`), Google Gemini embedding, and Ollama (local). Provider selection MUST be configurable via the SynapBus configuration file.
- **FR-003**: System MUST maintain an HNSW vector index (via `TFMV/hnsw`, pure Go, zero CGO) stored within the `--data` directory alongside SQLite data.
- **FR-004**: System MUST expose an MCP tool `search_messages` accepting: `query` (string, required), `filters` (object, optional: `channel_id`, `sender_id`, `priority_min`, `priority_max`, `tags`, `after`, `before`), `limit` (integer, optional, default 10, max 100), and `search_mode` (string, optional: `"auto"`, `"semantic"`, `"fulltext"`, default `"auto"`).
- **FR-005**: The `search_messages` tool MUST enforce agent access control: results MUST only include messages from channels the calling agent has joined or direct messages addressed to/from the calling agent.
- **FR-006**: When `search_mode` is `"auto"`, the system MUST use semantic search if an embedding provider is configured and healthy, otherwise fall back to FTS5 full-text search.
- **FR-007**: System MUST fall back to SQLite FTS5 full-text search when no embedding provider is configured. The FTS5 index MUST be maintained on the `messages.body` column.
- **FR-008**: Search responses MUST include: `results` (array of message objects with `similarity_score` for semantic or `relevance_score` for full-text), `search_mode` (string: `"semantic"` or `"fulltext"`), `total_results` (integer), and optionally `warning` (string, for degraded mode).
- **FR-009**: System MUST persist embeddings in SQLite (message_id, provider, model, vector BLOB, created_at) so the HNSW index can be rebuilt from stored data if corrupted or after a provider switch.
- **FR-010**: System MUST process embedding backlog in FIFO order with configurable concurrency (`embedding.workers`, default 4) and retry failed embeddings with exponential backoff (max 3 retries, base delay 1s).
- **FR-011**: When the embedding provider changes, the system MUST invalidate all existing embeddings and re-embed messages in the background using the new provider.
- **FR-012**: The HNSW index MUST support concurrent read access during incremental write operations without data corruption.
- **FR-013**: System MUST truncate message bodies exceeding the provider's token limit before embedding, and log a warning with the affected message ID.
- **FR-014**: System MUST remove embeddings and HNSW index entries when the corresponding message is deleted.
- **FR-015**: The `search_messages` MCP tool MUST include a JSON Schema description compliant with Constitution Principle II (MCP-Native).

### Key Entities *(include if feature involves data)*

- **Embedding**: Represents a vector embedding for a single message. Key attributes: `id`, `message_id` (FK to messages), `provider` (string: "openai", "gemini", "ollama"), `model` (string: provider-specific model identifier), `vector` (BLOB: serialized float32 array), `dimensions` (integer), `created_at` (timestamp). One-to-one relationship with Message. Stored in SQLite for durability; loaded into HNSW index for search.

- **EmbeddingProvider**: A configured embedding service. Key attributes: `type` (enum: openai, gemini, ollama), `api_key` (string, optional for ollama), `endpoint` (string, custom URL or default), `model` (string, provider-specific model name), `dimensions` (integer, determined by model). Configured via SynapBus config file, not persisted in database.

- **EmbeddingQueue**: Tracks messages pending embedding. Key attributes: `message_id` (FK to messages), `status` (enum: pending, processing, completed, failed), `attempts` (integer), `last_error` (string, nullable), `created_at` (timestamp), `completed_at` (timestamp, nullable). Stored in SQLite. Drives the background embedding pipeline.

- **HNSWIndex**: In-memory ANN index backed by `TFMV/hnsw`. Key attributes: configured `dimensions`, `ef_construction` (index build quality, default 200), `M` (max connections per node, default 16), `ef_search` (query-time quality, default 100). Persisted to disk in the `--data` directory. Rebuilt from SQLite embeddings table on startup or corruption.

- **SearchResult**: Returned by `search_messages`. Key attributes: `message` (full message object), `similarity_score` (float64, 0.0-1.0 for semantic) or `relevance_score` (float64 for full-text), `search_mode` (string). Not persisted.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Semantic search returns relevant results for queries using different terminology than the indexed messages, with the top-3 results containing at least one genuinely relevant message in 90%+ of test cases (measured against a curated test set of 50 query-message pairs).
- **SC-002**: The `send_message` MCP tool adds no more than 10ms of latency due to embedding pipeline overhead (embedding itself happens asynchronously).
- **SC-003**: A newly sent message becomes searchable via semantic search within 5 seconds under normal load (< 100 pending embeddings in queue).
- **SC-004**: The HNSW index handles at least 100,000 vectors with search latency under 50ms (p99) for top-10 queries.
- **SC-005**: When no embedding provider is configured, `search_messages` returns full-text results with zero errors and no configuration changes required by the operator.
- **SC-006**: Provider switching (e.g., OpenAI to Ollama) completes backfill of 10,000 messages within 30 minutes without blocking ongoing search operations (full-text fallback available during backfill).
- **SC-007**: The system correctly enforces access control on 100% of search results — no message from an inaccessible channel or unrelated DM thread is ever returned, regardless of similarity score.
