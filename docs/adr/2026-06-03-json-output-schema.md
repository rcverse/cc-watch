# ADR: JSON Output Schema

Date: 2026-06-03

## Status

Accepted

## Context

`cc-watch --json` is a public machine-readable interface for scripts and future clients. It needs a stable schema before implementation so parser, config, refresh, Reminder, KeepAlive, and CLI behavior all encode state consistently.

JSON output must never emit ANSI escape sequences.

## Decision

Use `schema_version: 1`.

Success output uses one top-level object:

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-03T12:00:00Z",
  "query": {"id": null, "limit": 5},
  "refresh": {
    "mode": "snapshot",
    "watcher": {"status": "not_started", "degraded": false, "messages": []},
    "safety_refresh_active": false,
    "last_refresh_at": "2026-06-03T12:00:00Z"
  },
  "notifications": {"status": "not_started", "degraded": false, "recent": []},
  "sessions": [],
  "selected_session": null,
  "error": null
}
```

Each full session object includes:

```json
{
  "session_id": "11111111-1111-1111-1111-111111111111",
  "short_id": "11111111",
  "project": "tmp-cc-watch",
  "jsonl_path": "/tmp/home/.claude/projects/-tmp-cc-watch/11111111-1111-1111-1111-111111111111.jsonl",
  "file_modified_at": "2026-06-03T12:00:00Z",
  "cache_window": {"tier": "1h", "label": "1h", "ttl_seconds": 3600, "known": true, "evidence": ["ephemeral_1h_input_tokens"]},
  "status": {"state": "active", "last_message_at": "2026-06-03T11:55:00Z", "remaining_seconds": 3300, "expired_seconds": null, "percent_elapsed": 8.33},
  "messages": {"first_user_excerpt": "start", "last_user_excerpt": "continue"},
  "token_stats": {"cache_writes": 100, "cache_reads": 900, "output_tokens": 50, "hit_rate": 90.0},
  "gaps": {"count": 0, "reset_count": 0, "latest": []},
  "warnings": [],
  "reminder": {"available": false, "enabled": null, "thresholds": [20, 10], "fired": []},
  "keep_alive": {"available": false, "enabled": null, "auto_send": null, "state": "unavailable", "scope": null, "last_result": null}
}
```

Error output uses the same top-level shape with `sessions` containing safe candidate summaries when helpful and `error` set:

No-match partial ID:

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-03T12:00:00Z",
  "query": {"id": "999", "limit": 5},
  "refresh": {"mode": "snapshot", "watcher": {"status": "not_started", "degraded": false, "messages": []}, "safety_refresh_active": false, "last_refresh_at": "2026-06-03T12:00:00Z"},
  "notifications": {"status": "not_started", "degraded": false, "recent": []},
  "sessions": [],
  "selected_session": null,
  "error": {"code": "session_not_found", "message": "partial id did not match any session", "query": "999"}
}
```

Ambiguous partial ID:

```json
{
  "schema_version": 1,
  "generated_at": "2026-06-03T12:00:00Z",
  "query": {"id": "111", "limit": 5},
  "refresh": {"mode": "snapshot", "watcher": {"status": "not_started", "degraded": false, "messages": []}, "safety_refresh_active": false, "last_refresh_at": "2026-06-03T12:00:00Z"},
  "notifications": {"status": "not_started", "degraded": false, "recent": []},
  "sessions": [{"session_id": "11111111-1111-1111-1111-111111111111", "short_id": "11111111", "project": "tmp-cc-watch"}],
  "selected_session": null,
  "error": {"code": "ambiguous_session_id", "message": "partial id matched multiple sessions", "query": "111"}
}
```

Allowed error codes are:

- `projects_dir_missing`
- `no_sessions_found`
- `session_not_found`
- `ambiguous_session_id`
- `parse_error`
- `config_error`

Error commands exit non-zero except first-run empty list states, which may exit zero with `sessions: []` and `error: null`.

List output sets `selected_session` to `null`. Single-session output sets `selected_session` to the full session object and may also include the surrounding `sessions` list when useful for candidate/error context.

Reminder and KeepAlive fields are present in the session object. Before those runtime systems are available, they encode `available: false`; once available, they encode enabled state, configured thresholds/scope, fired thresholds, auto-send state, current state, and last result.

Degraded refresh and notification state is encoded under the top-level `refresh` and `notifications` objects. Degraded state is data, not ANSI styling.

## Consequences

Scripts can rely on a versioned top-level object and stable field names. Future schema changes require an intentional schema version update or backwards-compatible additions.
