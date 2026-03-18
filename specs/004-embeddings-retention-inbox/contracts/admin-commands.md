# Admin Socket Command Contracts

All commands use the existing admin socket JSON-RPC protocol:
- Request: `{"command": "...", "args": {...}}\n`
- Response: `{"ok": true, "data": {...}}\n` or `{"ok": false, "error": "..."}\n`

## embeddings.status

**Args**: None

**Response data**:
```json
{
  "provider": "openai",
  "total_embedded": 1500,
  "pending_count": 23,
  "failed_count": 2,
  "index_size": 1498,
  "dimensions": 1536
}
```

If no provider configured: `provider` is empty string, all counts are from existing data.

## embeddings.reindex

**Args**: None

**Response data**:
```json
{
  "deleted_embeddings": 1500,
  "cleared_index": true,
  "enqueued_messages": 1523
}
```

Requires a running embedding pipeline (provider configured). Returns error if no provider.

## embeddings.clear

**Args**: None

**Response data**:
```json
{
  "deleted_embeddings": 1500,
  "cleared_index": true,
  "cleared_queue": true
}
```

## retention.status

**Args**: None

**Response data**:
```json
{
  "enabled": true,
  "retention_period": "8760h0m0s",
  "retention_period_human": "12 months",
  "warning_window": "720h0m0s",
  "cleanup_interval": "24h0m0s",
  "last_cleanup_at": "2026-03-14T00:00:00Z",
  "next_cleanup_at": "2026-03-15T00:00:00Z",
  "message_age_distribution": {
    "< 1 month": 500,
    "1-3 months": 300,
    "3-6 months": 200,
    "6-12 months": 100,
    "> 12 months": 15
  },
  "total_messages": 1115
}
```

## messages.purge

**Args**:
```json
{
  "older_than": "4320h",
  "agent": "bot-test",
  "channel": "test-channel"
}
```

At least one of `older_than`, `agent`, or `channel` must be specified.

**Response data**:
```json
{
  "deleted_messages": 150,
  "deleted_embeddings": 120,
  "deleted_attachments": 5,
  "cleaned_conversations": 3
}
```

## db.vacuum

**Args**: None

**Response data**:
```json
{
  "before_size_bytes": 104857600,
  "after_size_bytes": 52428800,
  "reclaimed_bytes": 52428800,
  "duration_ms": 3200
}
```

## CLI Command Mapping

| CLI Command | Admin Socket Command |
|------------|---------------------|
| `synapbus embeddings status` | `embeddings.status` |
| `synapbus embeddings reindex` | `embeddings.reindex` |
| `synapbus embeddings clear` | `embeddings.clear` |
| `synapbus retention status` | `retention.status` |
| `synapbus messages purge --older-than 6m --agent X --channel Y` | `messages.purge` |
| `synapbus db vacuum` | `db.vacuum` |
