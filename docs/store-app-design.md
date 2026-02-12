# fnOS Apps Store — 开发设计文档

**创建日期**: 2026-02-13
**状态**: 待开发

---

## 1. 项目目标

为 conversun/fnos-apps 仓库中的所有应用（Plex、Emby、Jellyfin、qBittorrent 等）提供一个运行在 fnOS 上的自动更新管理器。

**核心用户痛点**: 上游应用发布新版本后，用户必须手动到 GitHub Releases 下载 fpk，再到 fnOS 应用中心手动安装，流程繁琐。

**解决方案**: 一个独立的 fnOS 应用（fpk），提供 Web UI，自动检测更新并一键完成升级。

---

## 2. 技术可行性验证（已完成）

### 2.1 fnOS 平台能力

| 能力 | 验证结果 |
|------|----------|
| 编程式安装 fpk | `appcenter-cli install-fpk [fpk路径]` — 原生支持，支持升级 |
| 查询已安装应用 | `appcenter-cli list` — 返回 appname、version、status |
| 检查单个应用 | `appcenter-cli check [appname]` / `appcenter-cli status [appname]` |
| 启停应用 | `appcenter-cli start [appname]` / `appcenter-cli stop [appname]` |
| Root 权限 | fpk 应用可通过 privilege 配置以 root 运行 |
| 定时任务 | 支持 cron 和 systemd timers |
| 网络工具 | curl、wget、jq 均可用 |

### 2.2 `appcenter-cli` 完整参考

```
appcenter-cli [command]   # v1.0.1, Go 二进制, /usr/local/bin/appcenter-cli

install-fpk [fpk]         # 从 fpk 文件安装/升级应用
  -e, --env string        #   环境变量文件路径（对应 UI 安装向导的输入字段）
  -v, --volume int        #   安装卷索引（升级时忽略）

install [appname]         # 从官方应用中心安装
  -e, --env string
  -v, --volume int

uninstall [appname]       # 卸载应用
start [appname]           # 启动应用
stop [appname]            # 停止应用
check [appname]           # 检查是否已安装 → stdout: "Installed" / "Not installed"
status [appname]          # 获取运行状态 → stdout: "running" / "stopped"
list                      # 列出所有已安装应用（表格格式）
manual-install            # 查询/启用/禁用手动安装功能
default-volume            # 查询/设置 CLI 默认安装卷
```

> `appcenter-cli` 仅 root 可执行 (`-rwx------ root root`)。

### 2.3 fnOS 运行环境

| 项目 | 值 |
|------|-----|
| 操作系统 | Debian 12 (bookworm) |
| 架构 | x86_64 / aarch64 |
| 应用根目录 | `/var/apps/{appname}/` |
| 应用二进制 | `/var/apps/{appname}/target/` → symlink → `/vol1/@appcenter/{appname}` |
| 应用数据 | `/var/apps/{appname}/var/` → symlink → `/vol1/@appdata/{appname}` |
| 应用配置 | `/var/apps/{appname}/etc/` → symlink → `/vol1/@appconf/{appname}` |
| Manifest | `/var/apps/{appname}/manifest` — 包含 version、appname 等 |
| 可用工具 | bash, curl, wget, jq, cron, systemd, tar, ar |

### 2.4 Manifest 格式（从已安装的 Plex 读取）

```ini
appname         = plexmediaserver
version         = 1.43.0.10492
display_name    = Plex
platform        = x86
maintainer      = Plex Inc.
maintainer_url  = https://plex.tv
distributor     = conversun
distributor_url = https://github.com/conversun/fnos-apps
desktop_uidir   = ui
desktop_applaunchname = plexmediaserver.Application
service_port    = 32400
beta            = no
desc            = Plex Media Server是一款强大的媒体服务器软件...
source          = thirdparty
checksum        = 1b8eb150e100829442f4c0247839341e
```

---

## 3. 架构设计

### 3.1 总体架构

```
┌─────────────────────────────────────────────────────┐
│  用户浏览器                                           │
│  http://<NAS-IP>:<PORT>                              │
│                                                      │
│  ┌────────────────────────────────────────────────┐  │
│  │  fnOS Apps Store                               │  │
│  │                                                │  │
│  │  Plex       v1.43.0.10492 → v1.44.0.xxxx [更新]│  │
│  │  Emby       v4.9.3.0        ✅ 最新            │  │
│  │  Jellyfin   v10.11.6      → v10.11.7     [更新]│  │
│  │  qBittorrent v5.1.4         ✅ 最新            │  │
│  │  Gopeed     v1.9.1          ✅ 最新            │  │
│  │                                                │  │
│  │  上次检查: 2026-02-13 08:00  [立即检查]         │  │
│  └────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────┘
         │
         ▼
┌──────────────────────────────────────┐
│  store-server (root)                 │
│                                      │
│  定时检测 ──→ GitHub Releases API    │
│       │                              │
│       ├─→ 读 /var/apps/*/manifest    │ 获取本地已安装版本
│       │                              │
│       ├─→ 比对版本                    │ 判断是否有更新
│       │                              │
│       └─→ 写入状态缓存               │ JSON 文件
│                                      │
│  一键更新 ──→ 下载 fpk 到 /tmp       │
│       │                              │
│       └─→ appcenter-cli install-fpk  │ fnOS 原生升级
│                                      │
└──────────────────────────────────────┘
```

### 3.2 核心更新流程

```
检测阶段:
  1. 遍历已知应用列表（hardcoded: plex, emby, jellyfin, qbittorrent, ...）
  2. 对每个应用:
     a. 读取 /var/apps/{appname}/manifest → 提取 version + platform
     b. 调用 GitHub API:
        GET https://api.github.com/repos/conversun/fnos-apps/releases
        过滤 tag_name 前缀匹配 "{app}/"
        取最新 release → 提取版本号
     c. 比较版本，记录结果

更新阶段:
  1. 根据 platform (x86/arm) 选择对应的 fpk asset
  2. 下载: curl -L -o /tmp/{appname}.fpk <asset_download_url>
  3. 安装: appcenter-cli install-fpk /tmp/{appname}.fpk
  4. 清理: rm /tmp/{appname}.fpk
  5. 验证: appcenter-cli status {appname}
```

### 3.3 应用注册表

Store 需要知道哪些应用属于 conversun/fnos-apps 管理范围。建议在 store 内部维护一个静态映射：

```json
{
  "apps": [
    {
      "appname": "plexmediaserver",
      "tag_prefix": "plex",
      "fpk_prefix": "plexmediaserver",
      "display_name": "Plex",
      "github_repo": "conversun/fnos-apps"
    },
    {
      "appname": "embyserver",
      "tag_prefix": "emby",
      "fpk_prefix": "embyserver",
      "display_name": "Emby",
      "github_repo": "conversun/fnos-apps"
    },
    {
      "appname": "jellyfin",
      "tag_prefix": "jellyfin",
      "fpk_prefix": "jellyfin",
      "display_name": "Jellyfin",
      "github_repo": "conversun/fnos-apps"
    },
    {
      "appname": "qBittorrent",
      "tag_prefix": "qbittorrent",
      "fpk_prefix": "qbittorrent",
      "display_name": "qBittorrent",
      "github_repo": "conversun/fnos-apps"
    },
    {
      "appname": "gopeed",
      "tag_prefix": "gopeed",
      "fpk_prefix": "gopeed",
      "display_name": "Gopeed",
      "github_repo": "conversun/fnos-apps"
    },
    {
      "appname": "nginxserver",
      "tag_prefix": "nginx",
      "fpk_prefix": "nginxserver",
      "display_name": "Nginx",
      "github_repo": "conversun/fnos-apps"
    }
  ]
}
```

> 后续可改为远程拉取（从 repo 的某个 JSON 文件），实现新增应用自动发现。

---

## 4. 技术选型

### 4.1 推荐: Go 单二进制

| 考量 | 说明 |
|------|------|
| 无运行时依赖 | fnOS 是 Debian，无需安装 Node/Python 等 |
| 交叉编译 | `GOOS=linux GOARCH=amd64` / `GOARCH=arm64` |
| 内嵌静态资源 | `embed.FS` 打包 HTML/CSS/JS |
| HTTP server | 标准库 `net/http` 即可 |
| JSON 解析 | 标准库 `encoding/json` |
| 子进程调用 | `os/exec` 调用 `appcenter-cli` |
| 体积 | 预计 5-10MB，与 appcenter-cli 本身相当 |

### 4.2 备选: Bash + 轻量 HTTP

如果想保持项目 100% bash 风格，可以用 `busybox httpd` 或 `socat` 做极简 HTTP。但 Web UI 交互体验受限，不推荐。

---

## 5. 项目结构（建议）

```
apps/fnos-apps-store/
├── fnos/                          # fnOS 包定义
│   ├── manifest                   # appname=fnos-apps-store, service_port=XXXX
│   ├── cmd/
│   │   ├── service-setup          # SERVICE_COMMAND 指向 store-server 二进制
│   │   └── (继承 shared/cmd/*)
│   ├── config/
│   │   ├── privilege              # {"defaults":{"run-as":"root"}}  ← 关键
│   │   └── resource
│   ├── wizard/
│   │   └── install                # 安装向导（可选：配置 GitHub token）
│   ├── ui/
│   │   └── config                 # fnOS 桌面入口配置
│   ├── ICON.PNG
│   └── ICON_256.PNG
├── server/                        # Go 源码
│   ├── main.go                    # 入口
│   ├── api/                       # HTTP API handlers
│   │   ├── apps.go                # GET /api/apps — 列出应用及更新状态
│   │   ├── update.go              # POST /api/apps/{name}/update — 触发更新
│   │   └── check.go               # POST /api/check — 手动触发检测
│   ├── core/
│   │   ├── registry.go            # 应用注册表（上面的 JSON 映射）
│   │   ├── detector.go            # 版本检测逻辑
│   │   ├── updater.go             # 下载 + appcenter-cli install-fpk
│   │   └── manifest.go            # 解析 fnOS manifest 文件
│   ├── scheduler/
│   │   └── cron.go                # 定时检测调度
│   ├── web/                       # 前端静态文件（embed 打包）
│   │   ├── index.html
│   │   ├── style.css
│   │   └── app.js
│   └── go.mod
├── update_fnos-apps-store.sh      # 本地构建脚本
├── Makefile                       # go build 交叉编译
└── README.md
```

---

## 6. API 设计

### 6.1 获取应用列表及更新状态

```
GET /api/apps

Response:
{
  "apps": [
    {
      "appname": "plexmediaserver",
      "display_name": "Plex",
      "installed": true,
      "installed_version": "1.43.0.10492",
      "latest_version": "1.44.0.xxxxx",
      "has_update": true,
      "platform": "x86",
      "release_url": "https://github.com/conversun/fnos-apps/releases/tag/plex/v1.44.0.xxxxx",
      "release_notes": "...",
      "status": "running"
    },
    ...
  ],
  "last_check": "2026-02-13T08:00:00+08:00"
}
```

### 6.2 触发单个应用更新

```
POST /api/apps/{appname}/update

Response (SSE stream 或 WebSocket，实时反馈进度):
{"step": "downloading", "progress": 45}
{"step": "downloading", "progress": 100}
{"step": "installing", "message": "appcenter-cli install-fpk ..."}
{"step": "verifying", "message": "checking status..."}
{"step": "done", "new_version": "1.44.0.xxxxx"}
```

### 6.3 手动触发版本检测

```
POST /api/check

Response:
{"status": "ok", "checked": 6, "updates_available": 2}
```

### 6.4 全部更新

```
POST /api/apps/update-all

Response (SSE):
{"app": "plexmediaserver", "step": "downloading", ...}
{"app": "plexmediaserver", "step": "done", ...}
{"app": "jellyfin", "step": "downloading", ...}
...
```

---

## 7. 关键实现细节

### 7.1 版本检测逻辑

```go
// 从 GitHub Releases 获取最新版本
// GET https://api.github.com/repos/conversun/fnos-apps/releases
// 过滤: tag_name 以 "{tag_prefix}/v" 开头
// 排序: 取最新的 release
// 提取版本: tag_name = "plex/v1.43.0.10492" → version = "1.43.0.10492"
```

**注意事项**:
- GitHub API 未认证限制 60 次/小时，认证后 5000 次/小时
- 建议支持可选的 GitHub Token 配置（安装向导或设置页面）
- 缓存检测结果，避免重复请求

### 7.2 fpk 下载链接解析

Release assets 命名规则（从现有 CI 推断）：

```
{fpk_prefix}_{version}_{platform}.fpk

示例:
plexmediaserver_1.43.0.10492_x86.fpk
plexmediaserver_1.43.0.10492_arm.fpk
embyserver_4.9.3.0_x86.fpk
```

需要根据本机 `platform`（从 manifest 读取，或 `uname -m` 判断）选择正确的 asset。

### 7.3 Manifest 解析

```go
// manifest 格式: "key         = value"（固定宽度对齐，列 16）
// 关键字段:
//   appname  — 用于匹配注册表
//   version  — 用于版本比较
//   platform — 用于选择正确的 fpk asset（"x86" 或 "arm"）
```

### 7.4 更新执行

```bash
# 伪代码
fpk_path="/tmp/${appname}_${version}_${platform}.fpk"
curl -L -f -o "$fpk_path" "$download_url"
appcenter-cli install-fpk "$fpk_path"
rm -f "$fpk_path"
appcenter-cli status "$appname"  # 验证
```

**注意**: `install-fpk` 会自动处理：
- 停止旧服务
- 执行 `upgrade_init`（调用 `service_preupgrade`, `service_save`）
- 解包新文件
- 执行 `upgrade_callback`（调用 `service_restore`, `service_postupgrade`）
- 启动新服务

### 7.5 错误处理

| 场景 | 处理 |
|------|------|
| GitHub API 不可达 | 返回缓存的上次检测结果，标记检测失败 |
| fpk 下载失败 | 不执行安装，报告下载错误 |
| `install-fpk` 失败 | 捕获 stderr，报告安装错误（fnOS 自身有回滚机制） |
| 磁盘空间不足 | 下载前检查 `/tmp` 可用空间 |

### 7.6 自更新

Store 应用自身也需要能更新。方案：
- Store 自身也注册在应用列表中
- 更新自己时：下载新 fpk → `appcenter-cli install-fpk` → 进程会被替换重启
- 这是安全的，因为 `install-fpk` 由 fnOS 框架管理，不受应用进程影响

---

## 8. 安全考量

| 风险 | 缓解措施 |
|------|----------|
| Root 权限运行 | 仅调用 `appcenter-cli`，不直接操作文件系统 |
| 中间人攻击 | fpk 下载使用 HTTPS；manifest 有 checksum 字段，fnOS 安装时验证 |
| GitHub Token 泄露 | Token 存储在 `/vol1/@appconf/` 下，权限 600 |
| Web UI 未授权访问 | 建议绑定 127.0.0.1 或加基础认证 |

---

## 9. 同类项目参考

### FnDepot

- **仓库**: https://github.com/EWEDLCM/FnDepot
- **定位**: 通用第三方应用商店（去中心化，任何人可创建源）
- **技术**: Docker 容器应用为主，闭源客户端 fpk (v0.0.3)
- **源格式**: `fnpack.json` — 手动维护版本和元数据
- **与本项目区别**: FnDepot 面向 Docker 应用生态；本项目专注于原生 fpk 应用的自动更新
- **参考价值**: 它验证了 fnOS 第三方应用商店的用户需求（501 stars），以及 `appcenter-cli` 的可编程性

### fnpackup (by snltty)

- **仓库**: https://github.com/snltty/fnpackup
- **定位**: fpk 可视化打包工具
- **参考价值**: 其 Docker 命令暴露了 `appcenter-cli` 和 `fnpack` 的存在路径

---

## 10. 里程碑

### M1: MVP — 版本检测 + 手动更新

- [ ] Go 项目初始化，基础 HTTP server
- [ ] Manifest 解析器
- [ ] GitHub Releases 版本检测
- [ ] 单个应用更新（下载 + install-fpk）
- [ ] 极简 Web UI（应用列表 + 更新按钮）
- [ ] fpk 打包 + 本地构建脚本

### M2: 完善体验

- [ ] 更新进度实时反馈（SSE）
- [ ] 定时自动检测（可配置间隔）
- [ ] GitHub Token 配置页面
- [ ] 全部更新
- [ ] 更新历史记录

### M3: 进阶功能

- [ ] Store 自更新
- [ ] Release Notes 展示
- [ ] 中国镜像加速支持（ghfast.top）
- [ ] 新应用发现（检测 repo 中新增的、本地未安装的应用）
- [ ] 通知机制（可选）

---

## 附录 A: fnOS 已安装应用目录结构

```
/var/apps/plexmediaserver/
├── cmd/                    # 生命周期脚本
│   ├── common              # 共享框架（daemon管理、install/upgrade钩子）
│   ├── main                # start|stop|status 入口
│   ├── service-setup       # 应用特定配置（SERVICE_COMMAND 等）
│   ├── installer           # 安装/升级脚本加载器
│   ├── install_init        # 安装前钩子
│   ├── install_callback    # 安装后钩子
│   ├── upgrade_init        # 升级前钩子
│   ├── upgrade_callback    # 升级后钩子
│   ├── uninstall_init
│   ├── uninstall_callback
│   ├── config_init
│   └── config_callback
├── config/
│   ├── privilege           # 运行权限 {"defaults":{"run-as":"package"}, ...}
│   └── resource            # 端口、共享目录、systemd 配置
├── manifest                # 应用元数据（version、appname、checksum 等）
├── wizard/
│   └── uninstall           # 卸载向导 JSON
├── ICON.PNG
├── ICON_256.PNG
├── shares/
├── target/  → /vol1/@appcenter/plexmediaserver   # 应用二进制
├── var/     → /vol1/@appdata/plexmediaserver      # 运行时数据
├── etc/     → /vol1/@appconf/plexmediaserver      # 配置文件
├── home/    → /vol1/@apphome/plexmediaserver      # 用户数据
├── tmp/     → /vol1/@apptemp/plexmediaserver      # 临时文件
└── meta/    → /vol1/@appmeta/plexmediaserver      # 元数据（当前为空）
```

## 附录 B: privilege 配置 — root 运行

```json
{
    "defaults": {
        "run-as": "root"
    }
}
```

> 对比普通应用（如 Plex）的 `"run-as": "package"`，Store 需要 root 来调用 `appcenter-cli`。

## 附录 C: GitHub Releases API 参考

```bash
# 列出所有 releases（分页，默认 30 条）
curl -sL "https://api.github.com/repos/conversun/fnos-apps/releases?per_page=100"

# 单个 release 的 asset 结构
{
  "tag_name": "plex/v1.43.0.10492",
  "assets": [
    {
      "name": "plexmediaserver_1.43.0.10492_x86.fpk",
      "browser_download_url": "https://github.com/.../plexmediaserver_1.43.0.10492_x86.fpk"
    },
    {
      "name": "plexmediaserver_1.43.0.10492_arm.fpk",
      "browser_download_url": "https://github.com/.../plexmediaserver_1.43.0.10492_arm.fpk"
    }
  ]
}

# 带认证（提升限额到 5000/h）
curl -sL -H "Authorization: token ghp_xxxx" "https://api.github.com/..."
```
