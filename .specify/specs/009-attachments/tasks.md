# Tasks: Attachments

**Input**: Design documents from `/specs/009-attachments/`
**Prerequisites**: spec.md (required), constitution.md (required)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Package scaffolding and directory structure for the attachments feature

- [ ] T001 Create the `internal/attachments/` package directory and `doc.go` with package documentation in `internal/attachments/doc.go`
- [ ] T002 [P] Create attachment domain types (Attachment struct, AttachmentFile, error sentinels) in `internal/attachments/model.go`
- [ ] T003 [P] Create configuration constants (MaxFileSize=50MB, HashAlgorithm=SHA-256, sharding depth, default MIME type, supported image MIME types) in `internal/attachments/config.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core storage layer and database operations that ALL user stories depend on

**CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Verify the `attachments` table exists in `schema/001_initial.sql` and confirm the schema matches FR-004 (hash, original_filename, size, mime_type, message_id, uploaded_by, created_at). No migration needed since the table is already defined in the initial schema.
- [ ] T005 Implement the `Store` interface (repository pattern) in `internal/attachments/store.go` with methods: `InsertMetadata(ctx, Attachment) error`, `GetByHash(ctx, hash) ([]Attachment, error)`, `GetByMessageID(ctx, messageID) ([]Attachment, error)`, `DeleteOrphans(ctx) (int64, error)`, `CountReferences(ctx, hash) (int64, error)`. Define the interface only.
- [ ] T006 Implement `SQLiteStore` (the `Store` interface backed by `modernc.org/sqlite`) in `internal/attachments/sqlite_store.go`. All queries use parameterized statements. Context propagation on every method. Structured logging via `slog`.
- [ ] T007 Implement content-addressable storage (CAS) engine in `internal/attachments/cas.go`: `Write(ctx, reader io.Reader) (hash string, size int64, err error)` computes SHA-256 while streaming to a temp file, then atomically renames to `{dataDir}/attachments/{hash[0:2]}/{hash[2:4]}/{hash}`. Creates shard directories as needed. Cleans up temp files on failure (FR-013). Returns error on zero-byte content (FR-012).
- [ ] T008 [P] Implement `Exists(ctx, hash) (bool, error)` and `Read(ctx, hash) (io.ReadCloser, error)` and `Delete(ctx, hash) error` on the CAS engine in `internal/attachments/cas.go`. `Read` returns `ErrNotFound` if the file is missing from disk.
- [ ] T009 [P] Implement MIME type detection from file content (magic bytes) using `net/http.DetectContentType` with fallback to `application/octet-stream` in `internal/attachments/mime.go`. Also implement default filename generation from MIME type (FR-011, edge case: no filename).
- [ ] T010 Implement the `Service` struct in `internal/attachments/service.go` that composes `Store` (SQLite) + CAS engine + MIME detector. Constructor: `NewService(store Store, cas *CAS, logger *slog.Logger) *Service`. This is the main entry point for all attachment operations.
- [ ] T011 Write table-driven unit tests for CAS engine (write, read, exists, delete, zero-byte rejection, duplicate write dedup, partial cleanup) in `internal/attachments/cas_test.go`
- [ ] T012 [P] Write table-driven unit tests for SQLiteStore (insert metadata, get by hash, get by message, delete orphans) in `internal/attachments/sqlite_store_test.go`
- [ ] T013 [P] Write table-driven unit tests for MIME detection and default filename generation in `internal/attachments/mime_test.go`

**Checkpoint**: Foundation ready -- CAS engine writes/reads files, SQLiteStore persists metadata, MIME detection works. User story implementation can now begin in parallel.

---

## Phase 3: User Story 1 -- Agent Uploads and Attaches a File (Priority: P1) MVP

**Goal**: An agent can call `upload_attachment` via MCP to store a file and receive its SHA-256 hash. The file is stored in content-addressable storage with metadata in SQLite.

**Independent Test**: Call `upload_attachment` with a file payload via MCP, verify the returned hash matches SHA-256 of the content, the file exists on disk at the correct sharded path, and the metadata row is created in SQLite.

### Implementation for User Story 1

- [ ] T014 [US1] Implement `Upload(ctx, UploadRequest) (UploadResponse, error)` on the `Service` in `internal/attachments/service.go`. The method: validates size <= 50MB (FR-001), rejects zero-byte (FR-012), detects MIME type (FR-011), assigns default filename if missing, calls CAS.Write (FR-002, FR-003), calls Store.InsertMetadata (FR-004), logs the operation via slog (FR-014). Returns hash, size, mime_type, original_filename.
- [ ] T015 [US1] [P] Define `UploadRequest` and `UploadResponse` structs in `internal/attachments/model.go`. UploadRequest: `Content io.Reader`, `Filename string`, `MIMEType string`, `MessageID int64`, `UploadedBy string`. UploadResponse: `Hash string`, `Size int64`, `MIMEType string`, `Filename string`.
- [ ] T016 [US1] Register `upload_attachment` MCP tool in `internal/mcp/tools_attachments.go`. The tool accepts base64-encoded file content, original filename (optional), MIME type (optional), and message_id. It decodes the content, calls `Service.Upload`, and returns the hash and metadata as JSON. Include JSON Schema descriptions per Principle II.
- [ ] T017 [US1] Write integration test for upload flow (MCP call -> CAS file on disk + SQLite metadata row) in `internal/attachments/service_test.go`. Cover: successful upload, 50MB limit rejection, zero-byte rejection, dedup (same content yields same hash, no duplicate file on disk), missing filename defaults.

**Checkpoint**: User Story 1 is fully functional. An agent can upload files via MCP and receive hashes.

---

## Phase 4: User Story 2 -- Agent Downloads an Attachment by Hash (Priority: P1) MVP

**Goal**: An agent can call `download_attachment` with a SHA-256 hash to retrieve the file content, original filename, and MIME type.

**Independent Test**: Upload a file, then call `download_attachment` with the returned hash and verify byte-identical content, correct filename, and correct MIME type.

### Implementation for User Story 2

- [ ] T018 [US2] Implement `Download(ctx, hash string) (DownloadResponse, error)` on the `Service` in `internal/attachments/service.go`. The method: calls Store.GetByHash to retrieve metadata (returns first row's filename/mime_type), calls CAS.Read to get file content, logs the operation via slog (FR-014). Returns content reader, original_filename, mime_type, size. Returns `ErrNotFound` if hash not in metadata or file missing from disk.
- [ ] T019 [US2] [P] Define `DownloadResponse` struct in `internal/attachments/model.go`: `Content io.ReadCloser`, `Hash string`, `Filename string`, `MIMEType string`, `Size int64`.
- [ ] T020 [US2] Register `download_attachment` MCP tool in `internal/mcp/tools_attachments.go`. The tool accepts a SHA-256 hash string, calls `Service.Download`, base64-encodes the content, and returns JSON with content, original_filename, mime_type, and size. Returns clear "attachment not found" error for missing hashes (FR-006).
- [ ] T021 [US2] Write integration test for download flow in `internal/attachments/service_test.go`. Cover: successful download (byte-identical content), hash not found, file missing from disk (metadata exists but file deleted).

**Checkpoint**: User Stories 1 and 2 are both functional. Upload + Download form the minimum viable attachment feature.

---

## Phase 5: User Story 5 -- Deduplication Across Messages (Priority: P2)

**Goal**: Multiple agents uploading identical file content results in a single physical file on disk with multiple metadata rows, saving disk space.

**Independent Test**: Upload the same file content twice with different filenames, verify only one file exists on disk, and both metadata rows reference the same hash.

### Implementation for User Story 5

- [ ] T022 [US5] Verify and harden dedup logic in CAS.Write (`internal/attachments/cas.go`): if file at target path already exists, skip write (do not overwrite), return existing hash. Add file-level locking or atomic rename to handle concurrent uploads of the same content safely (SC-007). Log dedup events.
- [ ] T023 [US5] Write dedicated dedup integration tests in `internal/attachments/service_test.go`: upload same content with different filenames, verify single file on disk, two metadata rows with same hash but different original_filename. Verify disk usage does not grow with duplicate uploads.
- [ ] T024 [US5] [P] Write concurrent upload test in `internal/attachments/cas_test.go`: launch N goroutines uploading identical content simultaneously, verify no race conditions (use `-race` flag), single file on disk, no errors.

**Checkpoint**: Deduplication is verified and hardened. Content-addressable storage is fully production-ready.

---

## Phase 6: User Story 3 -- Human Views Attachments in Web UI (Priority: P2)

**Goal**: The Web UI renders inline image previews for supported image types and download cards for all other file types.

**Independent Test**: Upload image and non-image attachments, send messages referencing them, load the conversation in the Web UI, verify previews render for images and download links appear for other types.

### Implementation for User Story 3

- [ ] T025 [US3] Implement REST API endpoint `GET /api/attachments/{hash}` in `internal/api/attachments_handler.go`. The handler calls `Service.Download`, sets `Content-Type` from stored mime_type, sets `Content-Disposition: inline` for images and `attachment` for others, streams the file content to the response (FR-007). Return 404 for missing hashes.
- [ ] T026 [US3] [P] Implement REST API endpoint `GET /api/attachments/{hash}/meta` in `internal/api/attachments_handler.go` returning JSON metadata (hash, original_filename, size, mime_type) for the Web UI to decide rendering strategy without downloading full content.
- [ ] T027 [US3] Register attachment routes on the chi router in `internal/api/router.go` (or wherever routes are registered). Ensure routes are behind authentication middleware consistent with existing API patterns.
- [ ] T028 [US3] [P] Create Svelte attachment preview component in `web/src/lib/components/AttachmentPreview.svelte`. For image MIME types (image/jpeg, image/png, image/gif, image/webp, image/svg+xml): render inline `<img>` thumbnail (max 400px wide) with "download original" link (FR-008). For all other types: render a file card with icon, filename, human-readable size, and download button (FR-009).
- [ ] T029 [US3] Integrate `AttachmentPreview` component into the message view component (wherever messages are rendered in the Web UI). For each message, fetch attachment metadata and render previews/cards. Support multiple attachments per message displayed in order.
- [ ] T030 [US3] Write handler tests for `GET /api/attachments/{hash}` in `internal/api/attachments_handler_test.go`. Cover: successful image download (Content-Type set), successful non-image download (Content-Disposition: attachment), 404 for missing hash.

**Checkpoint**: Human owners can view attachments in the Web UI with inline previews for images and download cards for other file types.

---

## Phase 7: User Story 4 -- Garbage Collection of Orphaned Attachments (Priority: P3)

**Goal**: Orphaned attachment files (not referenced by any message) are identified and removed from both disk and SQLite, reclaiming storage space.

**Independent Test**: Upload an attachment, associate it with a message, delete the message, trigger GC, verify the orphaned file and metadata are removed.

### Implementation for User Story 4

- [ ] T031 [US4] Implement `Store.FindOrphanHashes(ctx) ([]string, error)` in `internal/attachments/sqlite_store.go`. Query returns hashes from the attachments table where no row references a valid message_id (message_id IS NULL or the referenced message no longer exists).
- [ ] T032 [US4] Implement `GarbageCollect(ctx) (GCResult, error)` on the `Service` in `internal/attachments/service.go`. The method: acquires a lock (mutex) to prevent concurrent GC/upload conflicts (edge case), calls Store.FindOrphanHashes, for each orphan hash calls CAS.Delete then Store.DeleteByHash, logs each deletion and the summary via slog (FR-010, FR-014). Returns `GCResult{FilesRemoved int, BytesReclaimed int64}`.
- [ ] T033 [US4] [P] Define `GCResult` struct in `internal/attachments/model.go`.
- [ ] T034 [US4] Register `gc_attachments` MCP tool (admin-only) in `internal/mcp/tools_attachments.go`. The tool calls `Service.GarbageCollect` and returns the GC summary as JSON. Consider also exposing as a CLI subcommand (`synapbus gc-attachments`) in `cmd/synapbus/`.
- [ ] T035 [US4] Write integration tests for GC in `internal/attachments/service_test.go`. Cover: orphan removed (file + metadata deleted), file referenced by one remaining message is NOT removed, empty state GC completes without error, concurrent GC + upload safety (edge case).

**Checkpoint**: Garbage collection works correctly. Orphaned files are cleaned up without affecting referenced attachments.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T036 [P] Add structured slog logging review across all attachment operations (upload, download, dedup, GC) -- verify log entries include agent name, hash, filename, size, duration, and error details per Principle VIII in `internal/attachments/service.go`
- [ ] T037 [P] Add trace entries for attachment operations (upload, download, GC) in the traces table via the existing trace infrastructure in `internal/trace/`. Ensure owners can see attachment activity for their agents.
- [ ] T038 [P] Verify the attachments directory tree (`{data_dir}/attachments/`) is auto-created on first use (SC-008, Principle I). Add initialization logic to CAS constructor or Service constructor in `internal/attachments/cas.go` if not already present.
- [ ] T039 [P] Handle edge case: attachment metadata exists but file missing from disk. `Download` should return a clear error ("attachment file missing") and optionally flag the metadata row for GC cleanup. Implement in `internal/attachments/service.go`.
- [ ] T040 [P] Handle edge case: disk full during upload. CAS.Write must clean up the temp file and return a descriptive error ("insufficient disk space"). Verify in `internal/attachments/cas.go`.
- [ ] T041 Add 50MB size limit enforcement at the MCP tool level (pre-check before reading full content into memory) in `internal/mcp/tools_attachments.go`. Consider streaming validation to avoid buffering the full 50MB.
- [ ] T042 [P] Run `make lint` and `make test` -- fix any linting errors, ensure all tests pass with `-race` flag
- [ ] T043 Run `make build` -- verify the binary compiles cleanly for `linux/amd64` and `darwin/arm64` with zero CGO per Principle III

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies -- can start immediately
- **Foundational (Phase 2)**: Depends on Phase 1 completion -- BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Phase 2 completion -- MVP upload
- **User Story 2 (Phase 4)**: Depends on Phase 2 completion -- MVP download (can run in parallel with Phase 3)
- **User Story 5 (Phase 5)**: Depends on Phase 3 completion -- hardens dedup on top of upload
- **User Story 3 (Phase 6)**: Depends on Phases 3 + 4 completion -- Web UI needs both upload and download working
- **User Story 4 (Phase 7)**: Depends on Phase 2 completion -- can run in parallel with other stories after foundation
- **Polish (Phase 8)**: Depends on all desired user stories being complete

### User Story Dependencies

- **US1 (Upload, P1)**: Can start after Phase 2 -- no dependencies on other stories
- **US2 (Download, P1)**: Can start after Phase 2 -- no dependencies on other stories (can parallel with US1)
- **US5 (Dedup, P2)**: Depends on US1 being implemented (hardens the upload dedup path)
- **US3 (Web UI, P2)**: Depends on US1 + US2 (needs working upload and download for REST endpoints)
- **US4 (GC, P3)**: Can start after Phase 2 -- independent of other stories conceptually, but best tested after US1

### Within Each User Story

- Models/types before service methods
- Service methods before MCP tools / API handlers
- Core implementation before integration tests
- Story complete before moving to next priority

### Parallel Opportunities

- All Phase 1 tasks marked [P] can run in parallel
- All Phase 2 tasks marked [P] can run in parallel (within Phase 2)
- US1 (Phase 3) and US2 (Phase 4) can run in parallel after Phase 2
- US4 (Phase 7) can start as soon as Phase 2 completes, in parallel with other stories
- All Polish tasks marked [P] can run in parallel

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL -- blocks all stories)
3. Complete Phase 3: User Story 1 (Upload)
4. Complete Phase 4: User Story 2 (Download)
5. **STOP and VALIDATE**: Test upload + download end-to-end via MCP
6. Deploy/demo if ready -- agents can share files

### Incremental Delivery

1. Setup + Foundational -> Foundation ready
2. US1 (Upload) + US2 (Download) -> MVP! Agents can share files
3. US5 (Dedup) -> Hardened storage, production-ready CAS
4. US3 (Web UI) -> Humans can view attachments inline
5. US4 (GC) -> Operational cleanup for long-running instances
6. Polish -> Logging, edge cases, cross-compilation verification

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- The `attachments` table already exists in `schema/001_initial.sql` -- no new migration is needed
- CAS path format: `{data_dir}/attachments/{hash[0:2]}/{hash[2:4]}/{hash}` (two levels of sharding)
- All code must be pure Go, zero CGO (Principle III)
- All agent-facing operations via MCP tools only; REST API is for Web UI only (Principle II)
- Structured logging via `slog` for all operations (Principle VIII)
- Commit after each task or logical group
