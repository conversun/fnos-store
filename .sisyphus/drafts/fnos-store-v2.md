# Draft: fnOS Apps Store — Architecture Redesign

## Requirements (confirmed)
- User does NOT want to depend on GitHub Releases API for version detection
- Wants mainstream third-party store features: install, update, etc.
- Needs architecture that supports future maintainers joining
- Technology: Go single binary (from original design doc)
- Platform: fnOS (Debian 12, x86_64 + aarch64)

## Key Context from Research

### Current fnos-apps Repo Structure
- 8 apps: plex, emby, jellyfin, qbittorrent, gopeed, nginx, ani-rss, audiobookshelf
- CI/CD: `reusable-build-app.yml` — check-version → build (x86+arm matrix) → GitHub Release
- Each app has: `scripts/apps/{app}/meta.env`, `build.sh`, `get-latest-version.sh`, `release-notes.tpl`
- fpk naming: `{file_prefix}_{version}_{platform}.fpk`
- Release tag format: `{app}/v{version}` (with `-rN` revision suffix)
- Manifest format: key=value, fixed-width column 16 alignment

### FnDepot Reference (501 stars)
- Uses `fnpack.json` in repo root as app registry/metadata
- Apps stored as fpk files in git repo or via GitHub Release for >100MB
- Client-side: closed-source fpk that pulls from user-added "sources"
- Source = any GitHub repo named "FnDepot" with `fnpack.json`
- Decentralized model: anyone creates a source

### User's Constraints
- NO GitHub Releases API dependency → needs self-hosted metadata source
- Wants: install (new apps), update (existing apps), and other mainstream features
- Architecture should accommodate future contributors

## Technical Decisions (confirmed)
- **Data source**: 仓库静态 JSON — fnos-apps repo 维护 apps.json, CI 自动更新, Store fetch raw file
- **Product scope**: 先单源 (conversun/fnos-apps), 架构层预留多源扩展接口
- **Features**: 安装 + 更新 + 卸载 — 完整商店体验
- **Frontend**: React SPA + Go embed.FS
- **Backend**: Go 单二进制

## Technical Decisions (confirmed - round 2)
- **fpk 下载**: GitHub Release 直链 + ghfast.top 镜像. Store 不调 API, 纯拼接 URL.
- **React 栈**: Vite + React + TypeScript + Tailwind CSS
- **UI 安全**: 依赖 fnOS 自身认证 (iframe 会话). 绑定 0.0.0.0.
- **安装卷**: 使用 `appcenter-cli default-volume` 获取系统默认卷

## Open Questions (remaining)
1. apps.json 的 CI 更新机制 — CI release 后自动 commit 更新 apps.json?
2. apps.json 具体字段设计 — 需要哪些元数据字段?
3. Store 自身是否也通过 apps.json 注册?

## Scope Boundaries
- INCLUDE: 浏览所有可用应用, 一键安装/更新/卸载, 版本检测, 更新进度 SSE, Store自更新, React SPA UI, 定时检测, 应用状态管理
- EXCLUDE: Docker 应用管理, 多源支持(架构预留但不实现), 通知推送, 自建认证系统
