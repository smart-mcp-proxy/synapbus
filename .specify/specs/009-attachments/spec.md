# Feature Specification: Attachments

**Feature Branch**: `009-attachments`
**Created**: 2026-03-13
**Status**: Draft
**Input**: User description: "Upload files up to 50MB per message. Content-addressable storage with SHA-256 dedup. MCP tools for upload/download. Web UI inline preview. Garbage collection for orphaned files."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Agent Uploads and Attaches a File to a Message (Priority: P1)

An AI agent produces an artifact (e.g., a generated report, a CSV export, a log file) and needs to share it with another agent or a human owner. The agent calls `upload_attachment` via MCP with the file content and original filename. SynapBus computes the SHA-256 hash, stores the file in content-addressable storage, records metadata in SQLite, and returns the hash. The agent then includes the hash in a message sent via `send_message`. The recipient agent or human can later retrieve the file by hash.

**Why this priority**: Without upload capability, no other attachment feature is possible. This is the foundational building block that enables all downstream stories.

**Independent Test**: Can be fully tested by calling `upload_attachment` with a file payload via MCP and verifying the returned hash matches the SHA-256 of the content, the file exists on disk at the correct path, and the metadata row is created in SQLite.

**Acceptance Scenarios**:

1. **Given** an authenticated agent, **When** it calls `upload_attachment` with a 1MB PNG file named "chart.png", **Then** the tool returns a JSON response containing the SHA-256 hash, the file is stored at `{data_dir}/attachments/{hash[0:2]}/{hash[2:4]}/{hash}`, and a metadata row is inserted with `original_filename="chart.png"`, `mime_type="image/png"`, `size=1048576`.
2. **Given** an authenticated agent, **When** it calls `upload_attachment` with a file identical to one already stored (same SHA-256 hash), **Then** the tool returns the same hash, no duplicate file is written to disk, and a new metadata row is created linking this upload to the new `message_id`.
3. **Given** an authenticated agent, **When** it calls `upload_attachment` with a 60MB file, **Then** the tool returns an error indicating the file exceeds the 50MB limit, and no file is written to disk.

---

### User Story 2 - Agent Downloads an Attachment by Hash (Priority: P1)

An agent receives a message containing an attachment hash. It calls `download_attachment` with the hash to retrieve the file content. SynapBus looks up the hash in storage, verifies the file exists, and streams the content back to the agent along with the original filename and MIME type.

**Why this priority**: Download is the counterpart to upload; together they form the minimum viable attachment feature. An upload without download is useless.

**Independent Test**: Can be fully tested by first uploading a file, then calling `download_attachment` with the returned hash and verifying the content matches byte-for-byte, the original filename is returned, and the MIME type is correct.

**Acceptance Scenarios**:

1. **Given** a file previously uploaded with hash `abc123...`, **When** an authenticated agent calls `download_attachment` with that hash, **Then** the tool returns the file content with `original_filename` and `mime_type` metadata.
2. **Given** a hash that does not exist in storage, **When** an agent calls `download_attachment` with that hash, **Then** the tool returns a clear error: "attachment not found".
3. **Given** an agent that does not own and has not received a message with the attachment, **When** it calls `download_attachment`, **Then** access is permitted (attachments are accessible by hash to any authenticated agent, like a content-addressable CDN).

---

### User Story 3 - Human Views Attachments in Web UI (Priority: P2)

A human owner browses a conversation in the Web UI that contains messages with attachments. For image attachments (JPEG, PNG, GIF, WebP, SVG), the UI renders an inline preview thumbnail. For all other file types, the UI displays the original filename, file size, and MIME type with a download link. Clicking the download link fetches the file via the REST API.

**Why this priority**: The Web UI is a first-class citizen (Principle X), but this story depends on upload/download (P1 stories) being implemented first. Inline preview is a significant usability improvement for human operators.

**Independent Test**: Can be tested by uploading image and non-image attachments, sending messages referencing them, then loading the conversation in the Web UI and verifying previews render for images and download links appear for other types.

**Acceptance Scenarios**:

1. **Given** a message with a PNG attachment in a conversation, **When** a human owner views the conversation in the Web UI, **Then** the image is rendered inline as a thumbnail (max 400px wide) with a "download original" link.
2. **Given** a message with a PDF attachment (report.pdf, 2.3MB), **When** a human owner views the conversation, **Then** the UI shows a file card with icon, filename "report.pdf", size "2.3 MB", and a download button.
3. **Given** a message with multiple attachments (2 images + 1 CSV), **When** viewing in the Web UI, **Then** all attachments are displayed: images as inline previews, CSV as a download card, in the order they were attached.

---

### User Story 4 - Garbage Collection of Orphaned Attachments (Priority: P3)

Over time, messages may be deleted or expire, leaving attachment files on disk that no message references. An administrator (or automated background process) runs garbage collection. SynapBus identifies attachment hashes present on disk but not referenced by any message's metadata, and removes them from both the filesystem and the SQLite metadata table.

**Why this priority**: This is an operational concern that only matters after the system has been running for a while with active attachment usage. It is not needed for the feature to be functional.

**Independent Test**: Can be tested by uploading an attachment, associating it with a message, deleting the message, then triggering garbage collection and verifying the orphaned file and metadata are removed.

**Acceptance Scenarios**:

1. **Given** an attachment file on disk whose hash is not referenced by any message in SQLite, **When** garbage collection runs, **Then** the file is deleted from disk and its metadata row is removed from SQLite.
2. **Given** an attachment file referenced by two messages and one message is deleted, **When** garbage collection runs, **Then** the file is NOT deleted because it is still referenced by the remaining message.
3. **Given** an empty attachments directory with no orphans, **When** garbage collection runs, **Then** it completes without errors and reports zero files removed.

---

### User Story 5 - Deduplication Across Messages (Priority: P2)

Multiple agents independently upload the same file (identical content, possibly different filenames). SynapBus detects that the SHA-256 hash already exists on disk and skips writing a duplicate. Each upload still creates its own metadata row (with its own `original_filename` and `message_id`), but all point to the same physical file. This saves disk space and speeds up uploads of previously-seen content.

**Why this priority**: Deduplication is a core architectural property of content-addressable storage and should be implemented alongside the basic upload flow, but it is not strictly required for a first working prototype.

**Independent Test**: Can be tested by uploading the same file content twice with different filenames, verifying only one file exists on disk, and both metadata rows reference the same hash.

**Acceptance Scenarios**:

1. **Given** an agent uploads "results_v1.csv" (hash: `aabbcc...`), **When** a different agent uploads "final_results.csv" with identical content, **Then** only one file exists at `{data_dir}/attachments/aa/bb/aabbcc...`, and two metadata rows exist with different `original_filename` values but the same hash.
2. **Given** 10 agents upload the same 5MB image, **When** checking disk usage, **Then** approximately 5MB is used (not 50MB), confirming deduplication.

---

### Edge Cases

- What happens when the disk is full during an upload? The system MUST return a clear error ("insufficient disk space") and not leave partial files on disk. Any partially written file MUST be cleaned up.
- What happens when an attachment file exists in the metadata table but is missing from disk (e.g., manual deletion)? `download_attachment` MUST return an error ("attachment file missing") rather than panicking, and the metadata row SHOULD be flagged for cleanup.
- What happens when the SHA-256 hash collides (astronomically unlikely)? The system MUST overwrite with the new content since SHA-256 collision implies identical content for practical purposes. No special handling is required.
- What happens when an upload request has no filename? The system MUST accept the upload and assign a default filename based on the MIME type (e.g., "untitled.bin" for `application/octet-stream`).
- What happens when the MIME type cannot be detected? The system MUST fall back to `application/octet-stream` and store the file normally.
- What happens when `upload_attachment` is called with zero-byte content? The system MUST reject the upload with an error ("empty file not allowed").
- What happens when the data directory's `attachments/` subdirectory does not exist at startup? The system MUST create the directory tree automatically on first use.
- What happens when garbage collection runs concurrently with an upload? The system MUST use locking or transaction isolation to prevent deleting a file that is actively being uploaded and linked to a message.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST accept file uploads up to 50MB per attachment via the `upload_attachment` MCP tool. Files exceeding this limit MUST be rejected with a descriptive error.
- **FR-002**: System MUST compute the SHA-256 hash of uploaded file content and use it as the storage key (content-addressable storage).
- **FR-003**: System MUST store attachment files on the local filesystem at `{data_dir}/attachments/{hash[0:2]}/{hash[2:4]}/{hash}`, creating intermediate directories as needed.
- **FR-004**: System MUST store attachment metadata in SQLite with at minimum: `hash` (TEXT, primary key component), `original_filename` (TEXT), `size` (INTEGER, bytes), `mime_type` (TEXT), `message_id` (TEXT, foreign key to messages), and `created_at` (DATETIME).
- **FR-005**: System MUST deduplicate files on disk: if a file with the same SHA-256 hash already exists, the upload MUST skip writing the file and only create a new metadata row.
- **FR-006**: System MUST expose a `download_attachment` MCP tool that accepts a SHA-256 hash and returns the file content along with `original_filename` and `mime_type` from the metadata.
- **FR-007**: System MUST expose a REST API endpoint (e.g., `GET /api/attachments/{hash}`) for the Web UI to fetch attachment content, with the `Content-Type` header set from the stored `mime_type`.
- **FR-008**: Web UI MUST render inline preview thumbnails for image MIME types (`image/jpeg`, `image/png`, `image/gif`, `image/webp`, `image/svg+xml`) within message views.
- **FR-009**: Web UI MUST display a download card (filename, size, download link) for non-image attachments.
- **FR-010**: System MUST implement garbage collection that identifies and removes attachment files not referenced by any message, cleaning up both the filesystem and SQLite metadata.
- **FR-011**: System MUST detect MIME type from file content (magic bytes) when not explicitly provided by the caller, falling back to `application/octet-stream` if detection fails.
- **FR-012**: System MUST reject zero-byte uploads with a descriptive error.
- **FR-013**: System MUST clean up partially written files if an upload fails mid-write (e.g., disk full, connection dropped).
- **FR-014**: System MUST log all attachment operations (upload, download, garbage collection) via `slog` structured logging per Principle VIII.

### Key Entities *(include if feature involves data)*

- **Attachment**: Represents a stored file. Key attributes: `hash` (SHA-256 hex string, identifies the physical file), `original_filename` (user-provided name), `size` (bytes), `mime_type` (detected or provided), `message_id` (the message this attachment belongs to), `created_at` (upload timestamp). A single physical file (hash) may be referenced by multiple Attachment metadata rows (deduplication). Relationship: many-to-one with Message.
- **AttachmentFile**: The physical file on disk at `{data_dir}/attachments/{hash[0:2]}/{hash[2:4]}/{hash}`. This is an implicit entity -- it has no SQLite row of its own beyond the Attachment metadata rows that reference it. Its existence is determined by whether at least one Attachment row references its hash.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: An agent can upload a 50MB file via `upload_attachment` and receive a hash response within 10 seconds on standard hardware (SSD, 8GB RAM).
- **SC-002**: An agent can download a previously uploaded file via `download_attachment` and receive byte-identical content with correct `original_filename` and `mime_type`.
- **SC-003**: Uploading the same file content N times results in exactly 1 file on disk and N metadata rows in SQLite, confirming deduplication works correctly.
- **SC-004**: Garbage collection correctly removes 100% of orphaned files (those not referenced by any message) and 0% of referenced files.
- **SC-005**: The Web UI renders inline image previews for all supported image MIME types and download cards for non-image types without JavaScript errors.
- **SC-006**: All attachment operations (upload, download, delete via GC) produce structured log entries viewable via `slog` output.
- **SC-007**: The system handles concurrent uploads of the same file without data corruption, race conditions, or duplicate file writes.
- **SC-008**: The attachment storage directory structure is created automatically on first upload -- no manual setup required, consistent with Principle I (single binary, zero setup).
