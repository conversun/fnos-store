# internal/api — HTTP API Layer

Handles all HTTP endpoints, SSE streaming, and operation orchestration. The `Server` struct is the central coordinator.

## STRUCTURE

```
api/
├── router.go       # Server struct, Config DI, route registration, SPA fallback
├── apps.go         # GET /api/apps — list apps with status
├── check.go        # POST /api/check — trigger registry refresh
├── install.go      # POST /api/apps/{appname}/install — SSE streaming install
├── update.go       # POST /api/apps/{appname}/update — SSE streaming update
├── uninstall.go    # POST /api/apps/{appname}/uninstall — SSE streaming uninstall
├── settings.go     # GET/PUT /api/settings — config CRUD
├── state.go        # RefreshRegistry, statusByApp helpers, shared state access
├── pipeline.go     # installPipeline: download → install-fpk → verify → refresh
├── queue.go        # OperationQueue: serializes CLI operations (one at a time)
├── sse.go          # sseStream: Server-Sent Events for progress/error streaming
├── helpers.go      # writeJSON(), writeAPIError(), formatTimestamp()
└── responses.go    # All JSON response struct definitions
```

## WHERE TO LOOK

| Task | File | Pattern |
|------|------|---------|
| Add new endpoint | `router.go` | Add to `routes()`, create handler function |
| Return JSON | `helpers.go` | `writeJSON(w, status, v)` or `writeAPIError(w, status, msg)` |
| Long-running operation | `install.go` | Copy pattern: TryStart queue → newSSEStream → pipeline → Finish |
| Stream progress | `sse.go` | `stream.sendProgress(progressPayload{...})` |
| Add response type | `responses.go` | Define struct with `json` tags |

## CONVENTIONS

- **Handler naming** — `handle<Verb><Resource>`: `handleListApps`, `handleInstall`, `handleGetSettings`.
- **Operation flow** — Every install/update/uninstall: (1) `queue.TryStart()`, (2) `defer queue.Finish()`, (3) create SSE stream, (4) launch goroutine with pipeline, (5) block on request context.
- **Error responses** — always via `writeAPIError(w, status, msg)`, never raw `http.Error()`.
- **State mutations** — always acquire `s.mu.Lock()` before modifying `statusByApp` or registry.
- **SSE event format** — `event: progress\ndata: {"step":"...","progress":N,"message":"..."}\n\n`.
- **Self-update detection** — if updating app == `s.storeApp`, uses `runSelfUpdate()` (no verify step, process dies).

## ANTI-PATTERNS

- **Don't call appcenter-cli directly** — always go through `pipeline` which uses `queue.WithCLI()`.
- **Don't skip `queue.TryStart()`** — returns `false` if another operation is running; respond 409 Conflict.
- **Don't add middleware** — no middleware chain exists; auth/CORS not needed (local service).
