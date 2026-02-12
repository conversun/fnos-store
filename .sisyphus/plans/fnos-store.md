# fnOS Apps Store — Full Development Plan

## TL;DR

> **Quick Summary**: Build a complete third-party app store for fnOS NAS that can browse, install, update, and uninstall applications from the conversun/fnos-apps repository. Replaces the original GitHub-API-dependent "auto updater" with a self-hosted metadata architecture (static JSON in repo).
> 
> **Deliverables**:
> - `apps.json` schema + CI generation script in fnos-apps repo
> - Go backend server (single binary, cross-compiled x86+arm)
> - React SPA frontend (Vite + TypeScript + Tailwind CSS, embedded via Go embed.FS)
> - fpk package for fnOS distribution
> - Mock development environment for macOS
> 
> **Estimated Effort**: Large
> **Parallel Execution**: YES - 3 waves
> **Critical Path**: Task 1 (apps.json) → Task 3 (Go core) → Task 5 (API) → Task 7 (React frontend) → Task 9 (fpk packaging)

---

## Context

### Original Request
Build an fnOS Apps Store with mainstream third-party store features (install, update, uninstall). Must NOT depend on GitHub Releases API. Architecture should accommodate future maintainers.

### Interview Summary
**Key Discussions**:
- **Data source**: Static `apps.json` in fnos-apps repo, CI auto-generates after releases. No GitHub API calls.
- **Product scope**: Single-source now (conversun/fnos-apps), architecture abstracts "source" interface for future multi-source.
- **Features**: Full store — browse, install, update, uninstall. Plus scheduled checks, SSE progress, self-update.
- **Frontend**: React SPA (Vite + TypeScript + Tailwind CSS), embedded in Go binary.
- **fpk downloads**: GitHub Release direct links + ghfast.top mirror. URLs constructed, never fetched from API.
- **Security**: Rely on fnOS native authentication (iframe session). Bind 0.0.0.0.
- **Install volume**: Use `appcenter-cli default-volume` for system default.
- **Tests**: No unit tests. Agent-Executed QA Scenarios only.

### Research Findings
- fnos-apps has 8 apps with **naming inconsistencies** (CI slug ≠ manifest appname ≠ file prefix in 4/8 apps)
- FnDepot (501 stars) validates market need; uses fnpack.json approach
- `appcenter-cli` provides all necessary operations: install-fpk, uninstall, start, stop, check, status, list, default-volume
- Revision system (`-rN` suffix) creates invisible updates since manifest stores only clean upstream version

### Metis Review
**Identified Gaps** (addressed):
- Naming mapping table (slug/appname/file_prefix/tag_prefix) — captured in apps.json schema
- Revision (-rN) detection gap — solved by including `release_tag` and `fpk_version` in apps.json
- Store port assignment — need to designate (auto-resolved to 8011)
- App icon delivery for uninstalled apps — icon URLs in apps.json via raw.githubusercontent.com
- Self-update SSE drop — frontend reconnect logic with "updating" overlay
- Mock development on macOS — mock appcenter-cli script included as task
- appcenter-cli concurrency — serialize via Go mutex
- Version comparison across different segment counts — segment-by-segment numeric comparison

---

## Work Objectives

### Core Objective
Deliver a fully functional fnOS third-party app store that allows users to discover, install, update, and uninstall applications from the conversun/fnos-apps repository without any GitHub API dependency.

### Concrete Deliverables
- `apps.json` — Static metadata file in fnos-apps repo root
- `scripts/ci/generate-apps-json.sh` — CI script to auto-generate apps.json
- `fnos-store/` — Complete Go + React application
- `fnos-apps-store_1.0.0_{x86,arm}.fpk` — Distributable packages

### Definition of Done
- [ ] `apps.json` exists in fnos-apps repo with all 8 apps' metadata
- [ ] Store server runs, serves React UI, exposes REST API
- [ ] Can browse all available apps (installed or not)
- [ ] Can install a new app via UI
- [ ] Can update an installed app via UI with progress feedback
- [ ] Can uninstall an app via UI
- [ ] Scheduled version checks work
- [ ] Store can update itself
- [ ] Zero calls to `api.github.com` in entire codebase
- [ ] fpk packages build for both x86 and arm

### Must Have
- apps.json as sole metadata source (no GitHub API)
- Browse all available apps with install/update/uninstall actions
- Real-time progress feedback via SSE during install/update
- Platform detection (x86/arm) for correct fpk selection
- Offline-first: work with cached data when network unavailable
- Store self-update capability
- Chinese-only UI

### Must NOT Have (Guardrails)
- **NO** GitHub API calls — anywhere, ever, not even as fallback
- **NO** i18n/l10n infrastructure — Chinese strings only, hardcoded
- **NO** source management UI — single hardcoded source, interface in Go only
- **NO** authentication system — rely on fnOS iframe session
- **NO** Docker app support — fpk native apps only
- **NO** notification/push system
- **NO** app categories, tags, or search (8 apps don't need it)
- **NO** install wizard UI — use system default volume
- **NO** rollback/downgrade features
- **NO** settings page beyond minimal check-interval config
- **NO** GitHub token configuration
- **NO** concurrent appcenter-cli calls — serialize all operations via mutex

---

## Verification Strategy

> **UNIVERSAL RULE: ZERO HUMAN INTERVENTION**
>
> ALL tasks are verified by agent-executed commands and Playwright scenarios.
> No "user manually tests..." or "user visually confirms..." allowed.

### Test Decision
- **Infrastructure exists**: NO (greenfield project)
- **Automated tests**: None
- **Framework**: N/A

### Agent-Executed QA Scenarios (MANDATORY — ALL tasks)

Every task includes ultra-detailed QA scenarios. The executing agent directly verifies:
- **Frontend/UI**: Playwright opens browser, navigates, fills forms, clicks, asserts DOM, screenshots
- **API/Backend**: curl sends requests, parses JSON, asserts fields and status codes
- **Build**: `go build`, `file`, `tar -tzf` verify artifacts
- **CLI integration**: mock appcenter-cli validates command construction

---

## Critical Data: App Naming Map

This table is THE source of truth. Every naming inconsistency is a potential bug:

| CI Slug | manifest `appname` | `FILE_PREFIX` | Tag Prefix | display_name | service_port |
|---------|-------------------|---------------|------------|--------------|--------------|
| plex | plexmediaserver | plexmediaserver | plex | Plex | 32400 |
| emby | embyserver | embyserver | emby | Emby | 8096 |
| jellyfin | jellyfin | jellyfin | jellyfin | Jellyfin | 8097 |
| qbittorrent | qBittorrent | qBittorrent | qbittorrent | qBittorrent | 8085 |
| gopeed | gopeed | gopeed | gopeed | Gopeed | 9999 |
| nginx | nginxserver | nginxserver | nginx | Nginx | 8888 |
| ani-rss | ani-rss | ani-rss | ani-rss | ANI-RSS | 7789 |
| audiobookshelf | audiobookshelf | audiobookshelf | audiobookshelf | Audiobookshelf | 13378 |

---

## Execution Strategy

### Parallel Execution Waves

```
Wave 1 (Start Immediately):
├── Task 1: apps.json schema + generation script (fnos-apps repo)
├── Task 2: Go project scaffold + mock dev environment (fnos-store repo)
└── (independent, no dependencies)

Wave 2 (After Wave 1):
├── Task 3: Go core modules (manifest parser, version comparator, source interface)
├── Task 4: React project scaffold + routing + layout
└── (3 depends on 2; 4 depends on none but benefits from 1 for data shape)

Wave 3 (After Wave 2):
├── Task 5: Go API layer (HTTP handlers, SSE, operation queue)
├── Task 6: React app list + install/update/uninstall UI
└── (5 depends on 3; 6 depends on 4,5)

Wave 4 (After Wave 3):
├── Task 7: Scheduled checks + Store self-update logic
├── Task 8: React settings + self-update UX
└── (7 depends on 5; 8 depends on 6,7)

Wave 5 (Final):
└── Task 9: fpk packaging + build scripts + CI integration
    (depends on all previous)

Critical Path: Task 1 → Task 3 → Task 5 → Task 6 → Task 9
```

### Dependency Matrix

| Task | Depends On | Blocks | Can Parallelize With |
|------|------------|--------|---------------------|
| 1 | None | 3, 5 | 2 |
| 2 | None | 3, 4 | 1 |
| 3 | 1, 2 | 5, 7 | 4 |
| 4 | 2 | 6, 8 | 3 |
| 5 | 3 | 6, 7 | 4 |
| 6 | 4, 5 | 9 | 7 |
| 7 | 5 | 8, 9 | 6 |
| 8 | 6, 7 | 9 | None |
| 9 | ALL | None | None (final) |

### Agent Dispatch Summary

| Wave | Tasks | Recommended Agents |
|------|-------|-------------------|
| 1 | 1, 2 | task(category="unspecified-high") — shell scripting + Go project init |
| 2 | 3, 4 | task(category="deep") for Go core; task(category="visual-engineering", load_skills=["frontend-ui-ux"]) for React |
| 3 | 5, 6 | task(category="deep") for Go API; task(category="visual-engineering", load_skills=["frontend-ui-ux"]) for React UI |
| 4 | 7, 8 | task(category="unspecified-high") for backend; task(category="visual-engineering", load_skills=["frontend-ui-ux"]) for frontend |
| 5 | 9 | task(category="unspecified-high") for packaging + CI |

---

## TODOs

---

- [ ] 1. apps.json Schema Design + CI Generation Script

  **What to do**:
  
  **1a. Design and create `apps.json` in fnos-apps repo root** with this exact schema:
  ```json
  {
    "schema_version": 1,
    "generated_at": "2026-02-13T08:00:00Z",
    "source": {
      "name": "conversun/fnos-apps",
      "url": "https://github.com/conversun/fnos-apps"
    },
    "apps": [
      {
        "slug": "plex",
        "appname": "plexmediaserver",
        "file_prefix": "plexmediaserver",
        "display_name": "Plex",
        "description": "Plex Media Server是一款强大的媒体服务器软件...",
        "version": "1.43.0.10492",
        "fpk_version": "1.43.0.10492",
        "release_tag": "plex/v1.43.0.10492",
        "service_port": 32400,
        "homepage_url": "https://www.plex.tv/media-server-downloads/",
        "icon_url": "https://raw.githubusercontent.com/conversun/fnos-apps/main/apps/plex/fnos/ICON_256.PNG",
        "platforms": ["x86", "arm"],
        "updated_at": "2026-02-13T08:00:00Z"
      }
    ]
  }
  ```
  
  **1b. Create `scripts/ci/generate-apps-json.sh`** that:
  - Iterates over all `scripts/apps/*/meta.env` directories
  - For each app: reads `meta.env` (FILE_PREFIX, RELEASE_TITLE, HOMEPAGE_URL, DEFAULT_PORT), reads `apps/{slug}/fnos/manifest` (appname, display_name, desc, service_port)
  - Queries existing GitHub releases via `gh release list` to get the LATEST release tag and fpk_version for each app
  - Constructs the icon_url using raw.githubusercontent.com path
  - Outputs valid JSON to `apps.json`
  - NOTE: This script runs in CI ONLY. The Store app itself NEVER calls this script or GitHub API.
  
  **1c. Manually create the initial `apps.json`** with data for all 8 current apps by reading existing manifests and latest release tags. This serves as the seed data. Future CI runs will auto-update it.
  
  **1d. Add a CI step to `reusable-build-app.yml`** (or a new workflow triggered by release creation) that:
  - Runs `generate-apps-json.sh`
  - Commits and pushes updated `apps.json` with message `chore: update apps.json [skip ci]`
  - The `[skip ci]` prevents infinite loops

  **Must NOT do**:
  - Do NOT modify any existing build scripts or workflows (only ADD new step/workflow)
  - Do NOT use `api.github.com` endpoints — use `gh` CLI which handles auth natively
  - Do NOT include the store itself in apps.json yet (that comes in Task 9)
  - Do NOT add icon files to a separate location — use existing `apps/*/fnos/ICON_256.PNG` paths

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Shell scripting + JSON generation + CI workflow modification. Not a standard category.
  - **Skills**: []
    - No specialized skills needed — bash scripting and GitHub Actions expertise.
  - **Skills Evaluated but Omitted**:
    - `git-master`: Not needed — we're creating files, not doing complex git operations.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 2)
  - **Blocks**: Tasks 3, 5
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `scripts/apps/plex/meta.env` — meta.env format: `FILE_PREFIX=plexmediaserver`, `RELEASE_TITLE="Plex"`, `HOMEPAGE_URL="..."`, `DEFAULT_PORT=32400`
  - `apps/plex/fnos/manifest` — Manifest format: fixed-width key=value at column 16. Fields: appname, version, display_name, desc, service_port, source
  - `scripts/ci/resolve-release-tag.sh` — How release tags are resolved. Shows `{slug}/v{version}` and `-rN` revision pattern
  - `.github/workflows/reusable-build-app.yml:146-194` — Release job: where to add the apps.json update step. Shows `gh release create` pattern

  **API/Type References**:
  - All 8 `scripts/apps/*/meta.env` files — Each has FILE_PREFIX, RELEASE_TITLE, HOMEPAGE_URL, DEFAULT_PORT
  - All 8 `apps/*/fnos/manifest` files — Each has appname, version, display_name, desc, service_port

  **Critical Data**:
  - The naming map table from this plan's "Critical Data" section — ALL four name columns must be captured independently

  **Acceptance Criteria**:

  - [ ] `apps.json` exists at fnos-apps repo root
  - [ ] `apps.json` is valid JSON (verified: `jq . apps.json`)
  - [ ] Contains exactly 8 apps (verified: `jq '.apps | length' apps.json` → 8)
  - [ ] Every app has all required fields: slug, appname, file_prefix, display_name, description, version, fpk_version, release_tag, service_port, homepage_url, icon_url, platforms, updated_at
  - [ ] `scripts/ci/generate-apps-json.sh` runs successfully: `bash scripts/ci/generate-apps-json.sh && jq '.apps | length' apps.json`
  - [ ] The qBittorrent entry preserves case: `jq '.apps[] | select(.slug=="qbittorrent") | .appname' apps.json` → `"qBittorrent"`
  - [ ] All 4 naming mismatches correctly captured (plex→plexmediaserver, emby→embyserver, qbittorrent→qBittorrent, nginx→nginxserver)
  - [ ] Icon URLs resolve: `curl -sf -o /dev/null -w "%{http_code}" "$(jq -r '.apps[0].icon_url' apps.json)"` → `200`

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: apps.json schema validation
    Tool: Bash (jq)
    Preconditions: apps.json exists at repo root
    Steps:
      1. jq '.schema_version' apps.json → Assert: 1
      2. jq '.apps | length' apps.json → Assert: 8
      3. jq '.apps[] | select(.slug=="plex") | .appname' apps.json → Assert: "plexmediaserver"
      4. jq '.apps[] | select(.slug=="qbittorrent") | .appname' apps.json → Assert: "qBittorrent"
      5. jq '.apps[] | select(.slug=="emby") | .appname' apps.json → Assert: "embyserver"
      6. jq '.apps[] | select(.slug=="nginx") | .appname' apps.json → Assert: "nginxserver"
      7. jq '[.apps[] | keys] | flatten | unique | sort' apps.json → Assert: includes all required fields
    Expected Result: All 8 apps with correct naming, all required fields present
    Evidence: Terminal output captured

  Scenario: generate script produces valid output
    Tool: Bash
    Preconditions: In fnos-apps repo root, gh CLI authenticated
    Steps:
      1. bash scripts/ci/generate-apps-json.sh
      2. Assert: exit code 0
      3. jq . apps.json → Assert: valid JSON
      4. diff <(jq -S . apps.json) <(jq -S . apps.json.expected) or manual field verification
    Expected Result: Script generates valid apps.json matching expected schema
    Evidence: Terminal output captured

  Scenario: icon URLs are accessible
    Tool: Bash (curl)
    Preconditions: apps.json exists with icon_url fields
    Steps:
      1. for url in $(jq -r '.apps[].icon_url' apps.json); do curl -sf -o /dev/null -w "%{http_code}\n" "$url"; done
      2. Assert: all responses are 200
    Expected Result: All 8 icon URLs return HTTP 200
    Evidence: Response codes captured
  ```

  **Commit**: YES
  - Message: `feat(store): add apps.json schema and CI generation script`
  - Files: `apps.json`, `scripts/ci/generate-apps-json.sh`
  - Pre-commit: `jq . apps.json`

---

- [ ] 2. Go Project Scaffold + Mock Development Environment

  **What to do**:
  
  **2a. Initialize Go project** in `fnos-store/`:
  ```
  fnos-store/
  ├── cmd/
  │   └── server/
  │       └── main.go          # Entry point
  ├── internal/
  │   ├── platform/
  │   │   ├── appcenter.go     # Interface for appcenter-cli operations
  │   │   ├── appcenter_linux.go  # Real implementation (calls appcenter-cli)
  │   │   └── appcenter_mock.go   # Mock for development (build tag: !linux)
  │   ├── source/
  │   │   └── source.go        # Source interface (for future multi-source)
  │   ├── core/
  │   │   ├── manifest.go      # Manifest parser
  │   │   ├── version.go       # Version comparison
  │   │   └── registry.go      # App registry (combines local + remote state)
  │   ├── api/
  │   │   ├── router.go        # HTTP router setup
  │   │   ├── apps.go          # GET /api/apps
  │   │   ├── install.go       # POST /api/apps/{appname}/install
  │   │   ├── update.go        # POST /api/apps/{appname}/update
  │   │   ├── uninstall.go     # POST /api/apps/{appname}/uninstall
  │   │   ├── check.go         # POST /api/check
  │   │   └── sse.go           # SSE helper
  │   └── scheduler/
  │       └── cron.go          # Scheduled version checks
  ├── web/                     # React build output (embedded)
  │   └── .gitkeep
  ├── go.mod
  └── go.sum
  ```
  
  **2b. Create mock `appcenter-cli`** at `fnos-store/dev/mock-appcenter-cli.sh`:
  - Simulates: `list`, `check`, `status`, `install-fpk`, `uninstall`, `start`, `stop`, `default-volume`
  - Uses a mock data directory (e.g., `/tmp/fnos-store-dev/apps/`) with sample manifests
  - `list` → outputs table format like real CLI
  - `check [appname]` → "Installed" or "Not installed"
  - `status [appname]` → "running" or "stopped"
  - `install-fpk [path]` → simulates install (extract fpk name, create mock manifest)
  - `default-volume` → "1"
  
  **2c. Create sample manifest files** at `fnos-store/dev/mock-apps/` for at least 3 apps (plex, qbittorrent, jellyfin) to exercise different naming patterns.
  
  **2d. Create `Makefile`** with targets:
  - `make dev` — run Go server in development mode (uses mock appcenter-cli)
  - `make build-linux-x86` — `GOOS=linux GOARCH=amd64 go build`
  - `make build-linux-arm` — `GOOS=linux GOARCH=arm64 go build`
  - `make build-frontend` — `cd frontend && npm run build && cp -r dist/ ../web/`
  - `make build-all` — frontend + both Go binaries
  
  **2e. Implement the `platform.AppCenter` interface** with method signatures:
  ```go
  type AppCenter interface {
      List() ([]InstalledApp, error)
      Check(appname string) (bool, error)
      Status(appname string) (string, error)
      InstallFpk(fpkPath string, volume int) error
      Uninstall(appname string) error
      Start(appname string) error
      Stop(appname string) error
      DefaultVolume() (int, error)
  }
  ```
  
  **2f. Implement the `source.Source` interface**:
  ```go
  type Source interface {
      Name() string
      FetchApps(ctx context.Context) ([]RemoteApp, error)
  }
  ```

  **Must NOT do**:
  - Do NOT implement actual business logic yet — just interfaces, scaffolding, and mocks
  - Do NOT set up React project here (that's Task 4)
  - Do NOT write to `web/` directory — leave it with `.gitkeep`
  - Do NOT add any third-party Go dependencies beyond standard library for now

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Go project scaffolding with interface design and mock system. Needs clean architecture decisions.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: Not relevant — this is pure Go backend scaffolding.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 1 (with Task 1)
  - **Blocks**: Tasks 3, 4
  - **Blocked By**: None (can start immediately)

  **References**:

  **Pattern References**:
  - `apps/plex/fnos/manifest` — Manifest format the parser must handle (fixed-width key=value at column 16)
  - `apps/qbittorrent/fnos/manifest` — qBittorrent manifest (case-sensitive appname: `qBittorrent`)
  - `apps/plex/fnos/cmd/service-setup` — Service setup pattern the store needs to follow
  - `apps/plex/fnos/config/privilege` — Privilege config (`"run-as":"root"` for the store)

  **Documentation References**:
  - `fnos-store/docs/store-app-design.md:20-53` — appcenter-cli full command reference
  - `fnos-store/docs/store-app-design.md:59-68` — fnOS runtime environment details

  **Acceptance Criteria**:

  - [ ] `go build ./cmd/server/` compiles on macOS without errors
  - [ ] `GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/server/` cross-compiles
  - [ ] `GOOS=linux GOARCH=arm64 go build -o /dev/null ./cmd/server/` cross-compiles
  - [ ] Mock appcenter-cli responds to all commands: `bash dev/mock-appcenter-cli.sh list`, `check plexmediaserver`, `status plexmediaserver`, `default-volume`
  - [ ] `platform.AppCenter` interface defined with all methods
  - [ ] `source.Source` interface defined
  - [ ] `Makefile` targets work: `make dev` starts server (can exit immediately)

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Go project compiles for all targets
    Tool: Bash
    Preconditions: Go installed, in fnos-store/ directory
    Steps:
      1. go build ./cmd/server/ → Assert: exit code 0
      2. GOOS=linux GOARCH=amd64 go build -o /tmp/store-x86 ./cmd/server/ → Assert: exit code 0
      3. GOOS=linux GOARCH=arm64 go build -o /tmp/store-arm ./cmd/server/ → Assert: exit code 0
      4. file /tmp/store-x86 → Assert: contains "ELF 64-bit LSB" and "x86-64"
      5. file /tmp/store-arm → Assert: contains "ELF 64-bit LSB" and "aarch64"
    Expected Result: All 3 builds succeed, correct architectures
    Evidence: file command output captured

  Scenario: Mock appcenter-cli works
    Tool: Bash
    Preconditions: dev/mock-appcenter-cli.sh exists, dev/mock-apps/ populated
    Steps:
      1. bash dev/mock-appcenter-cli.sh list → Assert: shows at least 3 apps
      2. bash dev/mock-appcenter-cli.sh check plexmediaserver → Assert: "Installed"
      3. bash dev/mock-appcenter-cli.sh check nonexistent → Assert: "Not installed"
      4. bash dev/mock-appcenter-cli.sh status plexmediaserver → Assert: "running"
      5. bash dev/mock-appcenter-cli.sh default-volume → Assert: numeric output
    Expected Result: Mock CLI simulates all required operations
    Evidence: Terminal output captured
  ```

  **Commit**: YES
  - Message: `feat(store): initialize Go project with platform abstraction and mock dev environment`
  - Files: `fnos-store/cmd/`, `fnos-store/internal/`, `fnos-store/dev/`, `fnos-store/Makefile`, `fnos-store/go.mod`
  - Pre-commit: `go build ./cmd/server/`

---

- [ ] 3. Go Core Modules — Manifest Parser, Version Comparator, Source Implementation

  **What to do**:
  
  **3a. Implement manifest parser** (`internal/core/manifest.go`):
  - Parse fnOS manifest format: `key         = value` (fixed-width, column 16 alignment)
  - Handle leading/trailing spaces in values
  - Extract required fields: appname, version, platform, display_name, distributor, service_port
  - Read from file path: `ParseManifest(path string) (*Manifest, error)`
  - Scan all installed apps: `ScanInstalled(appsDir string) ([]Manifest, error)` — iterates `/var/apps/*/manifest`
  - **CRITICAL**: Check `distributor` field — only include apps where `distributor = conversun`
  
  **3b. Implement version comparator** (`internal/core/version.go`):
  - Segment-by-segment numeric comparison (NOT semver library)
  - Handle 3-segment (5.1.4) and 4-segment (1.43.0.10492) versions
  - `CompareVersions(a, b string) int` — returns -1, 0, +1
  - Handle edge cases: empty string, non-numeric segments
  
  **3c. Implement fnos-apps source** (`internal/source/fnos_apps.go`):
  - Implements `source.Source` interface
  - `FetchApps(ctx)` fetches `apps.json` from raw.githubusercontent.com
  - URL: `https://raw.githubusercontent.com/conversun/fnos-apps/main/apps.json`
  - Parse JSON into `[]RemoteApp` structs matching apps.json schema
  - Cache fetched data locally (JSON file in store's data dir)
  - Return cached data when network unavailable (offline-first)
  - Construct download URLs:
    - Direct: `https://github.com/conversun/fnos-apps/releases/download/{release_tag}/{file_prefix}_{fpk_version}_{platform}.fpk`
    - Mirror: `https://ghfast.top/` + direct URL
  
  **3d. Implement app registry** (`internal/core/registry.go`):
  - Combines local state (manifest scan) with remote state (source fetch)
  - For each remote app, determine: not_installed / installed_up_to_date / update_available
  - Compare installed `version` against remote `version` using the comparator
  - Also detect revision updates: compare constructed `{slug}/v{installed_version}` against remote `release_tag`
  - Output unified list: `[]AppInfo` with all metadata + computed status
  
  **3e. Implement platform detection** (`internal/platform/detect.go`):
  - `DetectPlatform() string` — maps `runtime.GOARCH` to manifest platform:
    - `amd64` → `x86`
    - `arm64` → `arm`
  - Use this EVERYWHERE when constructing fpk filenames or download URLs
  - ONE place, one function, no duplication

  **Must NOT do**:
  - Do NOT use any semver library — segment-by-segment comparison only
  - Do NOT call `api.github.com` — only `raw.githubusercontent.com` for apps.json
  - Do NOT add HTTP server code — that's Task 5
  - Do NOT implement the actual download/install operations — that's Task 5

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Core domain logic with edge cases (naming inconsistencies, version formats, offline caching). Needs thorough understanding before action.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: Not relevant — pure Go backend logic.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 4)
  - **Blocks**: Tasks 5, 7
  - **Blocked By**: Tasks 1, 2

  **References**:

  **Pattern References**:
  - `fnos-store/internal/platform/appcenter.go` — AppCenter interface to depend on (from Task 2)
  - `fnos-store/internal/source/source.go` — Source interface to implement (from Task 2)
  - `apps/plex/fnos/manifest` — Manifest format (fixed-width at column 16)
  - `apps/qbittorrent/fnos/manifest` — qBittorrent case sensitivity test case

  **API/Type References**:
  - `apps.json` schema (from Task 1) — defines RemoteApp structure
  - This plan's "Critical Data" naming map table — all 8 apps with their 4 name columns

  **Documentation References**:
  - `fnos-store/docs/store-app-design.md:73-88` — Manifest format reference
  - `fnos-store/docs/store-app-design.md:342-348` — Version detection logic

  **Acceptance Criteria**:

  - [ ] Manifest parser correctly extracts all fields from `dev/mock-apps/plexmediaserver/manifest`
  - [ ] Manifest parser handles fixed-width format (spaces around `=`)
  - [ ] Version comparator: `CompareVersions("1.43.0.10492", "1.44.0.10000")` → -1
  - [ ] Version comparator: `CompareVersions("10.11.6", "10.11.6")` → 0
  - [ ] Version comparator: `CompareVersions("5.1.4", "5.1.3")` → 1
  - [ ] Version comparator: `CompareVersions("2.5.2", "2.5.10")` → -1 (numeric, not lexicographic)
  - [ ] Source fetches and parses apps.json (can be tested with local file)
  - [ ] Registry correctly merges local (3 mock apps) with remote (8 apps from apps.json)
  - [ ] `DetectPlatform()` returns "x86" on amd64, "arm" on arm64
  - [ ] `go build ./...` compiles without errors

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Manifest parsing handles all format variations
    Tool: Bash (go run test program)
    Preconditions: Mock manifests in dev/mock-apps/
    Steps:
      1. Create a small Go test program that calls ParseManifest on each mock manifest
      2. Assert: plexmediaserver manifest → appname="plexmediaserver", version="1.43.0.10492"
      3. Assert: qBittorrent manifest → appname="qBittorrent" (case preserved)
      4. Assert: jellyfin manifest → appname="jellyfin"
      5. Run with: go run ./dev/test-manifest/main.go
    Expected Result: All manifests parsed correctly with exact field values
    Evidence: Terminal output captured

  Scenario: Version comparison handles all app version formats
    Tool: Bash (go run test program)
    Preconditions: Go project compiles
    Steps:
      1. Create test program exercising CompareVersions with real app versions:
         - "1.43.0.10492" vs "1.44.0.10000" → -1 (Plex)
         - "4.9.3.0" vs "4.9.3.0" → 0 (Emby)
         - "10.11.6" vs "10.11.7" → -1 (Jellyfin)
         - "5.1.4" vs "5.1.3" → 1 (qBittorrent)
         - "2.5.2" vs "2.5.10" → -1 (ANI-RSS, numeric not lexicographic)
      2. Assert: all comparisons return expected values
    Expected Result: All version comparisons correct
    Evidence: Terminal output captured

  Scenario: Offline-first source caching
    Tool: Bash
    Preconditions: Store server with mock data
    Steps:
      1. First fetch: source fetches apps.json (from local test server or file)
      2. Assert: cache file written to data directory
      3. Simulate network failure (use invalid URL)
      4. Second fetch: source returns cached data instead of error
      5. Assert: cached data contains all 8 apps
    Expected Result: Source works offline with cached data
    Evidence: Terminal output and cache file contents captured
  ```

  **Commit**: YES
  - Message: `feat(store): implement manifest parser, version comparator, and fnos-apps source`
  - Files: `fnos-store/internal/core/`, `fnos-store/internal/source/fnos_apps.go`
  - Pre-commit: `go build ./...`

---

- [ ] 4. React Project Scaffold + Layout + Routing

  **What to do**:
  
  **4a. Initialize React project** at `fnos-store/frontend/`:
  ```bash
  npm create vite@latest frontend -- --template react-ts
  cd frontend
  npm install
  npm install -D tailwindcss @tailwindcss/vite
  ```
  
  **4b. Configure Tailwind CSS** — use `@import "tailwindcss"` in CSS, add Vite plugin.
  
  **4c. Configure Vite** for production build:
  - Output to `../web/` directory (for Go embed)
  - Base path: `.` (relative, not absolute — important for fnOS iframe embedding)
  - Proxy `/api` to Go backend during development (e.g., `localhost:8011`)
  
  **4d. Create basic layout components**:
  - `App.tsx` — Main layout: header + content area
  - Header: "fnOS Apps Store" title + last check time + "立即检查" button
  - Content: scrollable app list container
  - Footer: minimal status bar
  
  **4e. Create placeholder components**:
  - `AppList.tsx` — will show all apps (placeholder with mock data)
  - `AppCard.tsx` — individual app card: icon, name, version, status badge, action button
  - `ProgressOverlay.tsx` — overlay for install/update progress (SSE consumer)
  - `SettingsDialog.tsx` — minimal settings (check interval only)
  
  **4f. Set up API client** (`src/api/client.ts`):
  - Type definitions matching Go API response shapes
  - `fetchApps()`, `installApp()`, `updateApp()`, `uninstallApp()`, `triggerCheck()`
  - SSE connection helper for progress streams
  
  **4g. Chinese-only strings** — all UI text in Chinese. NO i18n setup.
  
  **4h. Set up development workflow**:
  - `npm run dev` starts Vite dev server with API proxy to Go backend
  - `npm run build` outputs to `../web/` for embedding

  **Must NOT do**:
  - Do NOT install a routing library — single-page app with no routes needed (8 apps = flat list)
  - Do NOT install state management library (Redux, Zustand) — React useState + context is sufficient
  - Do NOT implement actual API calls yet — use hardcoded mock data for layout verification
  - Do NOT add i18n framework — hardcode Chinese strings
  - Do NOT add dark mode or theme switching

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Frontend scaffold with UI layout and component structure. Visual engineering fits.
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Component design, layout structure, Tailwind styling.
  - **Skills Evaluated but Omitted**:
    - `playwright`: Not needed yet — no verification against running UI at this stage.

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 2 (with Task 3)
  - **Blocks**: Tasks 6, 8
  - **Blocked By**: Task 2 (needs project structure)

  **References**:

  **Pattern References**:
  - `fnos-store/docs/store-app-design.md:96-131` — UI wireframe showing app list layout
  - `fnos-store/docs/store-app-design.md:275-334` — API response shapes for type definitions

  **Documentation References**:
  - Vite React+TS template: https://vite.dev/guide/#scaffolding-your-first-vite-project
  - Tailwind CSS v4: https://tailwindcss.com/docs/installation/vite

  **Acceptance Criteria**:

  - [ ] `npm run build` succeeds with zero errors (exit code 0)
  - [ ] Build output exists in `fnos-store/web/index.html`
  - [ ] `npm run dev` starts dev server
  - [ ] All placeholder components render without errors
  - [ ] Chinese text visible in UI (not English placeholders)

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: React builds and outputs to correct directory
    Tool: Bash
    Preconditions: Node.js installed, in fnos-store/frontend/
    Steps:
      1. npm install → Assert: exit code 0
      2. npm run build → Assert: exit code 0
      3. ls ../web/index.html → Assert: file exists
      4. ls ../web/assets/ → Assert: directory with .js and .css files
    Expected Result: Frontend builds to ../web/ for Go embedding
    Evidence: ls output captured

  Scenario: Dev server starts with API proxy
    Tool: Bash
    Preconditions: npm install completed
    Steps:
      1. npm run dev &
      2. sleep 3
      3. curl -sf http://localhost:5173/ → Assert: returns HTML with React mount point
      4. kill %1
    Expected Result: Dev server serves the React app
    Evidence: curl output captured
  ```

  **Commit**: YES
  - Message: `feat(store): scaffold React frontend with Vite + TypeScript + Tailwind`
  - Files: `fnos-store/frontend/`
  - Pre-commit: `npm run build`

---

- [ ] 5. Go API Layer — HTTP Handlers, SSE Progress, Operation Queue

  **What to do**:
  
  **5a. Implement HTTP router** (`internal/api/router.go`):
  - Use `net/http` standard library (no frameworks)
  - Serve React SPA from `embed.FS` at `/`
  - API routes under `/api/`:
    - `GET /api/apps` — list all apps with status
    - `POST /api/apps/{appname}/install` — install app (SSE response)
    - `POST /api/apps/{appname}/update` — update app (SSE response)
    - `POST /api/apps/{appname}/uninstall` — uninstall app
    - `POST /api/check` — trigger version check
    - `GET /api/status` — server status (version, last check, platform)
  - SPA fallback: any non-API, non-asset path → serve `index.html`
  
  **5b. Implement SSE helper** (`internal/api/sse.go`):
  - Standard SSE format: `event: progress\ndata: {"step":"downloading","progress":45}\n\n`
  - Steps: `downloading` → `installing` → `verifying` → `done` (or `error`)
  - Flush after each event
  - Handle client disconnect (context cancellation)
  
  **5c. Implement operation queue** (`internal/api/queue.go`):
  - **Mutex**: Only one appcenter-cli operation at a time
  - **Queue**: If install/update/uninstall requested while another is running, queue it
  - Return 409 Conflict if same app operation already queued/running
  - Expose queue status via `/api/status`
  
  **5d. Implement install handler** (`internal/api/install.go`):
  - Accept appname, look up in registry
  - Download fpk to temp file with `.tmp` suffix (atomic rename on completion)
  - During download: send SSE progress events
  - After download: call `AppCenter.InstallFpk(fpkPath, volume)`
  - Cleanup: delete fpk from temp dir
  - Verify: call `AppCenter.Check(appname)` → "Installed"
  - Refresh local app state after successful install
  
  **5e. Implement update handler** (`internal/api/update.go`):
  - Same as install but targets already-installed apps
  - Use fpk_version from apps.json (handles -rN revisions)
  - No `--volume` flag needed for upgrades
  
  **5f. Implement uninstall handler** (`internal/api/uninstall.go`):
  - Stop app first: `AppCenter.Stop(appname)` (ignore error if already stopped)
  - Then: `AppCenter.Uninstall(appname)`
  - Refresh local app state
  
  **5g. Implement check handler** (`internal/api/check.go`):
  - Re-fetch apps.json from source
  - Re-scan local manifests
  - Rebuild registry
  - Return: `{"status":"ok","checked":8,"updates_available":2}`
  
  **5h. Implement fpk downloader** (`internal/core/downloader.go`):
  - Download fpk via HTTP with progress tracking
  - Try ghfast.top mirror first, fallback to GitHub direct
  - Atomic write: download to `{name}.fpk.tmp`, rename to `{name}.fpk` on completion
  - Check `/tmp` available space before download
  - Progress callback for SSE integration
  - Cleanup stale `.tmp` files on startup

  **5i. Implement Go embed for static files**:
  - `//go:embed web/*` in main.go or dedicated embed.go
  - Serve with `http.FileServer(http.FS(...))`
  - SPA fallback for non-file paths

  **Must NOT do**:
  - Do NOT use any HTTP framework (gin, echo, fiber) — `net/http` only
  - Do NOT implement WebSocket — SSE is simpler and sufficient
  - Do NOT allow concurrent appcenter-cli calls — mutex is mandatory
  - Do NOT call `api.github.com` for anything — download URLs are constructed from apps.json data
  - Do NOT implement batch "update all" as parallel operations — serialize them

  **Recommended Agent Profile**:
  - **Category**: `deep`
    - Reason: Complex integration — HTTP server, SSE streaming, operation queue, file downloads, CLI calls. Many edge cases.
  - **Skills**: []
  - **Skills Evaluated but Omitted**:
    - `frontend-ui-ux`: Not relevant — pure Go backend.

  **Parallelization**:
  - **Can Run In Parallel**: NO (depends on Task 3 being complete)
  - **Parallel Group**: Wave 3 (with Task 6 partially, but 6 depends on 5)
  - **Blocks**: Tasks 6, 7
  - **Blocked By**: Task 3

  **References**:

  **Pattern References**:
  - `fnos-store/internal/platform/appcenter.go` — AppCenter interface (from Task 2)
  - `fnos-store/internal/source/source.go` — Source interface (from Task 2)
  - `fnos-store/internal/core/registry.go` — Registry providing app data (from Task 3)

  **API/Type References**:
  - `fnos-store/docs/store-app-design.md:275-334` — API design: GET /api/apps response shape, POST /api/apps/{name}/update SSE format, POST /api/check response

  **Documentation References**:
  - `fnos-store/docs/store-app-design.md:383-413` — Error handling: GitHub API不可达→返回缓存, fpk下载失败→报告错误, install-fpk失败→捕获stderr, 磁盘空间不足→预检

  **Acceptance Criteria**:

  - [ ] Server starts and serves embedded React UI at `http://localhost:8011/`
  - [ ] `GET /api/apps` returns JSON with all apps and correct statuses
  - [ ] `POST /api/check` triggers re-fetch and returns update count
  - [ ] `POST /api/apps/{appname}/install` returns SSE stream with progress events
  - [ ] `POST /api/apps/{appname}/uninstall` succeeds (with mock CLI)
  - [ ] Second concurrent install returns 409 Conflict
  - [ ] `.fpk.tmp` files cleaned up on startup
  - [ ] Server compiles: `go build ./...`

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: API serves app list with correct statuses
    Tool: Bash (curl + jq)
    Preconditions: Server running on localhost:8011 with mock appcenter-cli
    Steps:
      1. curl -sf http://localhost:8011/api/apps | jq '.apps | length' → Assert: >= 3
      2. curl -sf http://localhost:8011/api/apps | jq '.apps[] | select(.appname=="plexmediaserver") | .installed' → Assert: true
      3. curl -sf http://localhost:8011/api/apps | jq '.apps[] | select(.appname=="plexmediaserver") | .status' → Assert: "running"
      4. curl -sf http://localhost:8011/api/apps | jq '.last_check' → Assert: valid ISO8601 timestamp
    Expected Result: All apps listed with correct install/status/version info
    Evidence: Full JSON response captured

  Scenario: SSE progress stream during install
    Tool: Bash (curl)
    Preconditions: Server running, mock appcenter-cli configured
    Steps:
      1. curl -N -H "Accept: text/event-stream" -X POST http://localhost:8011/api/apps/gopeed/install &
      2. Wait 5s, capture output
      3. Assert: output contains "event: progress"
      4. Assert: output contains "downloading" step
      5. Assert: output contains "done" step
      6. kill background curl
    Expected Result: SSE stream sends progress events from start to completion
    Evidence: SSE output captured

  Scenario: Concurrent operations return 409
    Tool: Bash (curl)
    Preconditions: Server running
    Steps:
      1. Start long-running install: curl -N -X POST http://localhost:8011/api/apps/plexmediaserver/install &
      2. Immediately: curl -s -o /dev/null -w "%{http_code}" -X POST http://localhost:8011/api/apps/embyserver/install
      3. Assert: second request returns 409
      4. kill background curl
    Expected Result: Server rejects concurrent operations
    Evidence: HTTP status codes captured
  ```

  **Commit**: YES
  - Message: `feat(store): implement HTTP API, SSE progress, and operation queue`
  - Files: `fnos-store/internal/api/`, `fnos-store/internal/core/downloader.go`, `fnos-store/cmd/server/main.go`
  - Pre-commit: `go build ./...`

---

- [ ] 6. React UI — App List, Install/Update/Uninstall Actions, Progress

  **What to do**:
  
  **6a. Implement `AppList` component** — fetches `GET /api/apps` on mount, renders list of `AppCard` components.
  
  **6b. Implement `AppCard` component** showing:
  - App icon (from icon_url in API response)
  - Display name
  - Version info: installed version → latest version (if update available)
  - Status badge: "已安装" (green), "有更新" (orange), "未安装" (gray), "运行中"/"已停止"
  - Action button:
    - Not installed → "安装" (primary button)
    - Update available → "更新" (warning button)
    - Up to date → "已是最新" (disabled)
    - Each card also has "卸载" option (for installed apps)
  - Service port display
  - Homepage link
  
  **6c. Implement `ProgressOverlay` component**:
  - Full-screen semi-transparent overlay
  - Shows during install/update operations
  - Connects to SSE endpoint
  - Displays: current step (下载中.../安装中.../验证中...), progress bar for download
  - Auto-closes on "done" event
  - Shows error message on "error" event with retry option
  
  **6d. Implement store self-update detection in frontend**:
  - When user clicks "更新" on the store itself:
    - Show special overlay: "商店正在更新，请稍候..."
    - After SSE connection drops (expected during self-update):
      - Show reconnecting spinner
      - Poll `GET /api/status` every 2 seconds with exponential backoff
      - When server responds: reload the page
  
  **6e. Implement manual check trigger**:
  - "立即检查" button in header
  - Calls `POST /api/check`
  - Shows loading spinner during check
  - Refreshes app list on completion
  
  **6f. Implement uninstall confirmation**:
  - Click "卸载" → confirmation dialog: "确定要卸载 {display_name} 吗？应用数据将被保留。"
  - On confirm → call `POST /api/apps/{appname}/uninstall`
  - Refresh list on completion
  
  **6g. Connect all API calls** — replace mock data with real API client calls from Task 4's `src/api/client.ts`.
  
  **6h. Responsive design** — the UI will be displayed in fnOS desktop's iframe. Design for minimum 800px width. Mobile responsiveness is nice-to-have but not required.

  **Must NOT do**:
  - Do NOT add app detail/info pages — flat list only
  - Do NOT add search or filter — 8 apps don't need it
  - Do NOT add app categories or tags
  - Do NOT implement "Update All" button (serialize individual updates instead)
  - Do NOT add animations beyond basic transitions
  - Do NOT add dark mode

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: React UI implementation with interactive components, SSE integration, and visual polish.
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Component design, Tailwind styling, interaction patterns.
  - **Skills Evaluated but Omitted**:
    - `playwright`: Will be used for QA verification but not as a skill for implementation.

  **Parallelization**:
  - **Can Run In Parallel**: NO (needs Task 5 API to be running)
  - **Parallel Group**: Wave 3 (starts after Task 5, can overlap with Task 7)
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 4, 5

  **References**:

  **Pattern References**:
  - `fnos-store/frontend/src/api/client.ts` — API client types and functions (from Task 4)
  - `fnos-store/frontend/src/components/` — Placeholder components (from Task 4)

  **API/Type References**:
  - `fnos-store/docs/store-app-design.md:275-334` — API response shapes
  - `fnos-store/internal/api/` — Go API handlers defining exact response formats (from Task 5)

  **Documentation References**:
  - `fnos-store/docs/store-app-design.md:96-131` — UI wireframe showing desired layout
  - SSE EventSource API: https://developer.mozilla.org/en-US/docs/Web/API/EventSource

  **Acceptance Criteria**:

  - [ ] `npm run build` succeeds with zero errors
  - [ ] App list displays all apps from API
  - [ ] Each app card shows icon, name, version, status, action button
  - [ ] "安装" button triggers install with SSE progress overlay
  - [ ] "更新" button triggers update with SSE progress overlay
  - [ ] "卸载" button shows confirmation then triggers uninstall
  - [ ] "立即检查" refreshes app list
  - [ ] All UI text is Chinese

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: App list displays all apps with correct status
    Tool: Playwright (playwright skill)
    Preconditions: Go server running on localhost:8011 with mock data
    Steps:
      1. Navigate to: http://localhost:8011/
      2. Wait for: app cards to render (timeout: 5s)
      3. Assert: at least 3 app cards visible
      4. Assert: card with text "Plex" contains version info
      5. Assert: card with text "Plex" contains status badge
      6. Assert: at least one card has "安装" button (uninstalled app)
      7. Assert: at least one card has "已是最新" or "更新" status
      8. Screenshot: .sisyphus/evidence/task-6-app-list.png
    Expected Result: All apps displayed with correct Chinese text and status
    Evidence: .sisyphus/evidence/task-6-app-list.png

  Scenario: Install flow with SSE progress
    Tool: Playwright (playwright skill)
    Preconditions: Go server running, mock app available for install
    Steps:
      1. Navigate to: http://localhost:8011/
      2. Find app card with "安装" button
      3. Click "安装" button
      4. Wait for: progress overlay visible (timeout: 3s)
      5. Assert: overlay contains progress text (下载中 or 安装中)
      6. Wait for: overlay disappears or shows "完成" (timeout: 30s)
      7. Assert: app card now shows "已安装" status
      8. Screenshot: .sisyphus/evidence/task-6-install-flow.png
    Expected Result: Full install flow with progress feedback
    Evidence: .sisyphus/evidence/task-6-install-flow.png

  Scenario: Uninstall with confirmation dialog
    Tool: Playwright (playwright skill)
    Preconditions: Go server running, at least one mock app installed
    Steps:
      1. Navigate to: http://localhost:8011/
      2. Find installed app card
      3. Click "卸载" button
      4. Wait for: confirmation dialog (timeout: 3s)
      5. Assert: dialog contains "确定要卸载" text
      6. Click confirm button
      7. Wait for: dialog closes (timeout: 10s)
      8. Assert: app card now shows "未安装" status
      9. Screenshot: .sisyphus/evidence/task-6-uninstall-confirm.png
    Expected Result: Uninstall with Chinese confirmation dialog
    Evidence: .sisyphus/evidence/task-6-uninstall-confirm.png
  ```

  **Commit**: YES
  - Message: `feat(store): implement React UI with app list, install/update/uninstall actions`
  - Files: `fnos-store/frontend/src/`
  - Pre-commit: `npm run build`

---

- [ ] 7. Scheduled Checks + Store Self-Update Logic

  **What to do**:
  
  **7a. Implement scheduled version checks** (`internal/scheduler/cron.go`):
  - Run version check on configurable interval (default: every 6 hours)
  - On startup: load last check time from cache; if stale, trigger immediate check
  - Check = re-fetch apps.json + re-scan local manifests + rebuild registry
  - Store check results and timestamp
  - Configurable interval: read from config file at store's data dir
  
  **7b. Implement configuration persistence**:
  - Config file: `{store_data_dir}/config.json`
  - Fields: `{"check_interval_hours": 6}`
  - API: `GET /api/settings`, `PUT /api/settings`
  - Store data dir: `/var/apps/fnos-apps-store/var/` (following fnOS convention) or mock path for dev
  
  **7c. Implement store self-update logic**:
  - The store itself is listed in apps.json (added in Task 9)
  - When updating the store: same download + install-fpk flow
  - **Special handling**: After calling `AppCenter.InstallFpk()` for the store's own fpk:
    - The current process WILL be terminated by fnOS (install-fpk stops the service)
    - fnOS will restart the new version automatically
    - The old process should: send a final SSE event `{"step":"self_update","message":"商店正在重启..."}` BEFORE calling install-fpk
    - Then proceed with install-fpk (process will be killed)
  - Frontend handles reconnection (implemented in Task 6d)
  
  **7d. Implement cache management**:
  - Cache directory: `{store_data_dir}/cache/`
  - `apps.json` cache for offline-first behavior
  - Last check timestamp
  - Cleanup old cache files on startup

  **Must NOT do**:
  - Do NOT implement push notifications or email alerts
  - Do NOT add complex scheduling (cron expressions) — simple interval is enough
  - Do NOT try to keep the process alive during self-update — let fnOS handle it

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Backend logic with scheduling, persistence, and the tricky self-update edge case.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: YES
  - **Parallel Group**: Wave 4 (with Task 8)
  - **Blocks**: Tasks 8, 9
  - **Blocked By**: Task 5

  **References**:

  **Pattern References**:
  - `fnos-store/internal/api/` — API handlers to extend with settings endpoints (from Task 5)
  - `fnos-store/internal/core/registry.go` — Registry rebuild logic (from Task 3)

  **Documentation References**:
  - `fnos-store/docs/store-app-design.md:406-413` — Self-update description: "Store 自身也注册在应用列表中... 进程会被替换重启"
  - `fnos-store/docs/store-app-design.md:393-395` — install-fpk lifecycle: stops old service, executes upgrade hooks, starts new service

  **Acceptance Criteria**:

  - [ ] Scheduled check runs on interval (verify with short interval in dev)
  - [ ] Config persists between restarts: `cat {data_dir}/config.json` shows saved interval
  - [ ] `GET /api/settings` returns current config
  - [ ] `PUT /api/settings {"check_interval_hours": 12}` updates config
  - [ ] Self-update sends final SSE event before install-fpk
  - [ ] `go build ./...` compiles

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Scheduled check triggers automatically
    Tool: Bash (curl + wait)
    Preconditions: Server running with check_interval set to very short (e.g., 10 seconds for testing)
    Steps:
      1. Start server with short interval config
      2. curl -sf http://localhost:8011/api/status | jq '.last_check'
      3. Wait 15 seconds
      4. curl -sf http://localhost:8011/api/status | jq '.last_check'
      5. Assert: second timestamp is later than first
    Expected Result: Automatic check updates last_check timestamp
    Evidence: Two timestamps captured

  Scenario: Settings persist across restarts
    Tool: Bash
    Preconditions: Server running
    Steps:
      1. curl -sf -X PUT http://localhost:8011/api/settings -d '{"check_interval_hours":12}'
      2. Assert: HTTP 200
      3. Restart server (kill + restart)
      4. curl -sf http://localhost:8011/api/settings | jq '.check_interval_hours'
      5. Assert: 12
    Expected Result: Config survives restart
    Evidence: API responses captured
  ```

  **Commit**: YES
  - Message: `feat(store): add scheduled checks, settings persistence, and self-update logic`
  - Files: `fnos-store/internal/scheduler/`, `fnos-store/internal/api/settings.go`
  - Pre-commit: `go build ./...`

---

- [ ] 8. React Settings + Self-Update UX

  **What to do**:
  
  **8a. Implement settings dialog** (`SettingsDialog.tsx`):
  - Trigger: gear icon in header
  - Content: "检查间隔" dropdown — 1小时, 6小时, 12小时, 24小时
  - Calls `PUT /api/settings` on change
  - Shows store version and server platform info
  
  **8b. Implement self-update UX**:
  - When user clicks "更新" on the store's own card:
    - Normal progress overlay starts
    - When SSE receives `self_update` event:
      - Overlay changes to: "商店正在更新，请稍候..."
      - Progress bar becomes indeterminate (spinning)
    - When SSE connection drops (expected):
      - Overlay shows: "正在重启..." with reconnection spinner
      - Start polling `GET /api/status` every 2 seconds (max 30 retries)
      - On success: check if version changed → reload page
      - On timeout (60s): show error "重启超时，请手动刷新页面"
  
  **8c. Implement "更新全部" button** (if any updates available):
  - Shows in header when updates_available > 0: "更新全部 (N)"
  - Serializes updates one by one (NOT parallel)
  - Shows progress for each app sequentially
  - If store update is among them, always execute it LAST

  **Must NOT do**:
  - Do NOT add theme/dark mode settings
  - Do NOT add mirror URL configuration
  - Do NOT add source management
  - Do NOT parallelize "更新全部" — serialize all operations

  **Recommended Agent Profile**:
  - **Category**: `visual-engineering`
    - Reason: Frontend UI components with specific interaction patterns.
  - **Skills**: [`frontend-ui-ux`]
    - `frontend-ui-ux`: Dialog design, state management, UX patterns.

  **Parallelization**:
  - **Can Run In Parallel**: NO
  - **Parallel Group**: Wave 4 (after Tasks 6, 7)
  - **Blocks**: Task 9
  - **Blocked By**: Tasks 6, 7

  **References**:

  **Pattern References**:
  - `fnos-store/frontend/src/components/ProgressOverlay.tsx` — Progress overlay to extend (from Task 6)
  - `fnos-store/frontend/src/api/client.ts` — API client to extend with settings calls (from Task 4)

  **Acceptance Criteria**:

  - [ ] Settings dialog opens and displays check interval options
  - [ ] Changing interval calls API and persists
  - [ ] Self-update overlay shows "正在重启..." after SSE drops
  - [ ] Reconnection polling works (verify with server restart)
  - [ ] "更新全部" serializes updates correctly
  - [ ] `npm run build` succeeds

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: Settings dialog saves check interval
    Tool: Playwright (playwright skill)
    Preconditions: Server running on localhost:8011
    Steps:
      1. Navigate to: http://localhost:8011/
      2. Click: gear icon / settings button
      3. Wait for: settings dialog visible
      4. Select: "12小时" from interval dropdown
      5. Click: save/confirm button
      6. Wait for: dialog closes
      7. Reopen settings dialog
      8. Assert: interval shows "12小时" (persisted)
      9. Screenshot: .sisyphus/evidence/task-8-settings.png
    Expected Result: Settings saved and restored
    Evidence: .sisyphus/evidence/task-8-settings.png

  Scenario: Self-update reconnection after server restart
    Tool: Playwright (playwright skill)
    Preconditions: Server running, simulate self-update by killing server
    Steps:
      1. Navigate to: http://localhost:8011/
      2. Trigger a mock self-update (or simulate by stopping server)
      3. Wait for: "正在重启" overlay (timeout: 5s)
      4. Restart server in background
      5. Wait for: page reloads or overlay disappears (timeout: 30s)
      6. Assert: app list visible again
      7. Screenshot: .sisyphus/evidence/task-8-self-update.png
    Expected Result: Frontend recovers after server restart
    Evidence: .sisyphus/evidence/task-8-self-update.png
  ```

  **Commit**: YES
  - Message: `feat(store): add settings dialog, self-update UX, and batch update`
  - Files: `fnos-store/frontend/src/components/SettingsDialog.tsx`, `fnos-store/frontend/src/`
  - Pre-commit: `npm run build`

---

- [ ] 9. fpk Packaging + Build Scripts + CI Integration

  **What to do**:
  
  **9a. Create fnOS app structure** for the store at `fnos-store/fnos/`:
  ```
  fnos-store/fnos/
  ├── manifest           # appname=fnos-apps-store, version=1.0.0, service_port=8011
  ├── cmd/
  │   ├── service-setup  # SERVICE_COMMAND points to store binary
  │   └── (inherit shared/cmd/* from fnos-apps repo)
  ├── config/
  │   ├── privilege      # {"defaults":{"run-as":"root"}}
  │   └── resource       # Port forwarding config
  ├── wizard/
  │   └── uninstall      # Uninstall wizard JSON
  ├── ui/
  │   ├── config         # Desktop entry config
  │   └── images/
  ├── fnos-apps-store.sc # Firewall port rule for 8011
  ├── ICON.PNG           # App icon (small)
  └── ICON_256.PNG       # App icon (large, 256x256)
  ```
  
  **9b. Create `manifest`** file:
  ```ini
  appname         = fnos-apps-store
  version         = 1.0.0
  display_name    = fnOS Apps Store
  platform        = x86
  maintainer      = conversun
  maintainer_url  = https://github.com/conversun
  distributor     = conversun
  distributor_url = https://github.com/conversun/fnos-store
  desktop_uidir   = ui
  desktop_applaunchname = fnos-apps-store.Application
  service_port    = 8011
  beta            = no
  desc            = fnOS第三方应用商店，支持一键安装、更新和卸载来自conversun/fnos-apps的所有应用。
  source          = thirdparty
  checksum        = 
  ```
  
  **9c. Create `cmd/service-setup`**:
  ```bash
  #!/bin/bash
  SERVICE_COMMAND="/var/apps/fnos-apps-store/target/store-server"
  SERVICE_PID_FILE="/var/apps/fnos-apps-store/var/store-server.pid"
  SERVICE_LOG="/var/apps/fnos-apps-store/var/store-server.log"
  ```
  
  **9d. Create `config/privilege`**:
  ```json
  {"defaults":{"run-as":"root"}}
  ```
  
  **9e. Create `ui/config`** following the pattern from existing apps:
  ```json
  {
    "desktop_applaunchname": "fnos-apps-store.Application",
    "url": "http://localhost:8011/",
    "port": 8011,
    "icon": "ICON.PNG",
    "icon_256": "ui/images/256.png"
  }
  ```
  
  **9f. Create build script** `fnos-store/build.sh`:
  - Install Go toolchain
  - Install Node.js
  - `cd frontend && npm install && npm run build` → outputs to `web/`
  - `GOOS=linux GOARCH=$ARCH go build -o store-server ./cmd/server/`
  - Create `app.tgz` containing the Go binary
  - Use `scripts/build-fpk.sh` from fnos-apps to package
  
  **9g. Create Makefile target** `make fpk` that builds fpk locally.
  
  **9h. Create app icons** — ICON.PNG (small) and ICON_256.PNG (256x256). Can use a simple placeholder for now.
  
  **9i. Add the store to apps.json** — update the generation script to include fnos-apps-store entry.
  
  **9j. Validate fpk structure**:
  - `tar -tzf *.fpk` must contain: manifest, app.tgz, cmd/, config/, ICON.PNG, ICON_256.PNG, ui/

  **Must NOT do**:
  - Do NOT create a separate CI workflow for the store yet — focus on local build first
  - Do NOT use `"run-as":"package"` — the store needs root for appcenter-cli
  - Do NOT change the port to something that conflicts with existing apps

  **Recommended Agent Profile**:
  - **Category**: `unspecified-high`
    - Reason: Build system integration, fpk packaging, fnOS manifest creation. Cross-cutting concerns.
  - **Skills**: []

  **Parallelization**:
  - **Can Run In Parallel**: NO (final integration task)
  - **Parallel Group**: Wave 5 (sequential, depends on all previous)
  - **Blocks**: None (final task)
  - **Blocked By**: ALL previous tasks

  **References**:

  **Pattern References**:
  - `apps/plex/fnos/manifest` — Manifest format to follow exactly (fixed-width at column 16)
  - `apps/plex/fnos/cmd/service-setup` — Service setup pattern (SERVICE_COMMAND, PID_FILE, LOG)
  - `apps/plex/fnos/config/privilege` — Privilege config pattern (JSON format)
  - `apps/plex/fnos/ui/config` — UI desktop config pattern
  - `apps/plex/fnos/*.sc` — Port forwarding rule pattern
  - `scripts/build-fpk.sh` — Generic fpk packager (validates structure, merges shared + app-specific)
  - `apps/plex/update_plex.sh` — Local build script pattern

  **Documentation References**:
  - `fnos-store/docs/store-app-design.md:233-269` — Proposed project structure
  - `fnos-store/docs/store-app-design.md:474-507` — fnOS app directory structure reference

  **Acceptance Criteria**:

  - [ ] `fnos-store/fnos/manifest` exists with all required fields
  - [ ] `fnos-store/fnos/config/privilege` has `"run-as":"root"`
  - [ ] `bash build.sh` produces `fnos-apps-store_1.0.0_x86.fpk` (on x86 system)
  - [ ] `tar -tzf fnos-apps-store_1.0.0_x86.fpk` contains: manifest, app.tgz, cmd/, config/, ICON.PNG, ICON_256.PNG, ui/
  - [ ] `tar -xzf fnos-apps-store_1.0.0_x86.fpk manifest && grep "service_port" manifest` → `service_port    = 8011`
  - [ ] `tar -xzf fnos-apps-store_1.0.0_x86.fpk manifest && grep "run-as" config/privilege` → contains `root`
  - [ ] Go binary inside app.tgz is correct architecture: `file store-server` → "ELF 64-bit LSB"

  **Agent-Executed QA Scenarios**:

  ```
  Scenario: fpk builds and has correct structure
    Tool: Bash
    Preconditions: Go + Node installed, all previous tasks complete
    Steps:
      1. bash build.sh → Assert: exit code 0
      2. ls fnos-apps-store_1.0.0_x86.fpk → Assert: file exists
      3. tar -tzf fnos-apps-store_1.0.0_x86.fpk → Assert: contains manifest, app.tgz, cmd/, config/, ICON.PNG, ICON_256.PNG, ui/
      4. tar -xzf fnos-apps-store_1.0.0_x86.fpk manifest -O | grep "appname" → Assert: "fnos-apps-store"
      5. tar -xzf fnos-apps-store_1.0.0_x86.fpk manifest -O | grep "service_port" → Assert: "8011"
      6. Extract and check binary: tar -xzf fnos-apps-store_1.0.0_x86.fpk app.tgz && tar -xzf app.tgz && file store-server
      7. Assert: "ELF 64-bit LSB executable, x86-64"
    Expected Result: Valid fpk with correct manifest and binary
    Evidence: tar listing and file output captured

  Scenario: Manifest follows fnOS format conventions
    Tool: Bash
    Preconditions: fnos/manifest exists
    Steps:
      1. cat fnos-store/fnos/manifest
      2. Assert: all keys aligned at column 16 (key + spaces + "= " + value)
      3. Assert: appname = fnos-apps-store
      4. Assert: source = thirdparty
      5. Assert: distributor = conversun
    Expected Result: Manifest matches fnOS conventions exactly
    Evidence: File content captured
  ```

  **Commit**: YES
  - Message: `feat(store): add fpk packaging, fnOS manifest, and build scripts`
  - Files: `fnos-store/fnos/`, `fnos-store/build.sh`
  - Pre-commit: `tar -tzf *.fpk`

---

## Commit Strategy

| After Task | Message | Key Files | Verification |
|------------|---------|-----------|--------------|
| 1 | `feat(store): add apps.json schema and CI generation script` | apps.json, scripts/ci/generate-apps-json.sh | `jq . apps.json` |
| 2 | `feat(store): initialize Go project with platform abstraction and mock dev environment` | fnos-store/ scaffold | `go build ./cmd/server/` |
| 3 | `feat(store): implement manifest parser, version comparator, and fnos-apps source` | internal/core/, internal/source/ | `go build ./...` |
| 4 | `feat(store): scaffold React frontend with Vite + TypeScript + Tailwind` | fnos-store/frontend/ | `npm run build` |
| 5 | `feat(store): implement HTTP API, SSE progress, and operation queue` | internal/api/ | `go build ./...` + curl tests |
| 6 | `feat(store): implement React UI with app list, install/update/uninstall actions` | frontend/src/ | `npm run build` + Playwright |
| 7 | `feat(store): add scheduled checks, settings persistence, and self-update logic` | internal/scheduler/ | `go build ./...` |
| 8 | `feat(store): add settings dialog, self-update UX, and batch update` | frontend/src/ | `npm run build` + Playwright |
| 9 | `feat(store): add fpk packaging, fnOS manifest, and build scripts` | fnos/, build.sh | `tar -tzf *.fpk` |

---

## Success Criteria

### Verification Commands
```bash
# Build verification
cd fnos-store && go build ./cmd/server/              # Go compiles
cd fnos-store/frontend && npm run build               # React builds
bash fnos-store/build.sh                              # fpk packages

# Cross-compilation
GOOS=linux GOARCH=amd64 go build -o /dev/null ./cmd/server/
GOOS=linux GOARCH=arm64 go build -o /dev/null ./cmd/server/

# apps.json validation
jq '.apps | length' apps.json                         # Expected: 8 (or 9 with store)
jq '.apps[] | .appname' apps.json                     # All appnames present

# API verification (with mock)
curl -sf http://localhost:8011/api/apps | jq '.apps | length'  # >= 8
curl -sf -X POST http://localhost:8011/api/check | jq '.status'  # "ok"

# fpk verification
tar -tzf fnos-apps-store_1.0.0_x86.fpk | grep manifest  # manifest exists

# Zero GitHub API calls
grep -r "api.github.com" fnos-store/ --include="*.go" --include="*.ts" --include="*.tsx"
# Expected: no matches
```

### Final Checklist
- [ ] All "Must Have" present (browse, install, update, uninstall, SSE, scheduled checks, self-update)
- [ ] All "Must NOT Have" absent (no GitHub API, no i18n, no Docker, no notifications, no auth system)
- [ ] Both Go targets compile (linux/amd64, linux/arm64)
- [ ] React builds to web/ without errors
- [ ] fpk packages with correct structure
- [ ] All Chinese UI text renders correctly
- [ ] Mock development environment works on macOS
- [ ] Zero calls to api.github.com in entire codebase
