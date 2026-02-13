# fnOS 应用商店

[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

面向 [飞牛 fnOS](https://www.fnnas.com/) 的第三方应用商店，提供友好的 Web 界面管理第三方应用。

> ⭐️ 如果觉得本项目对你有帮助，请右上角点个 Star！

## 功能特性

- 📦 浏览和安装 fnOS 第三方应用
- 🔄 自动检测应用更新
- 📊 实时查看安装进度（SSE 推送）
- 🎯 支持批量更新
- 🌙 暗黑模式支持

## 技术栈

- **后端**: Go 1.25+ (标准库 http)
- **前端**: React 19 + TypeScript + Tailwind CSS + shadcn/ui
- **构建**: Vite + fnpack

## 快速开始

### 开发环境

```bash
# 1. 启动后端 (端口 8011)
go run ./cmd/server/

# 2. 启动前端开发服务器 (端口 5173)
cd frontend
npm install
npm run dev
```

### 构建

```bash
./build.sh
```

构建产物：
- `fnos-apps-store_*.fpk` - 可安装到 fnOS 的商店包
- `store-server-*` - 独立服务器二进制

## 项目结构

```
fnos-store/
├── cmd/server/          # 服务端入口
├── internal/
│   ├── api/            # HTTP API 处理器
│   ├── core/           # 业务逻辑 (registry, downloader)
│   ├── platform/       # fnOS 平台抽象
│   ├── source/         # 远程应用源
│   ├── cache/          # 本地缓存
│   ├── config/         # 配置管理
│   └── scheduler/      # 定时任务
├── frontend/           # React 前端
│   └── src/
│       ├── components/
│       └── api/
├── fnos/              # fnOS 包清单和配置
└── web/               # 嵌入的前端构建产物
```

## API 文档

见 [API.md](docs/API.md)（如存在）或查看 `internal/api/` 源码。

## 配置

环境变量：
- `LISTEN_ADDR` - 监听地址 (默认 `:8011`)
- `APPS_DIR` - 已安装应用目录
- `DATA_DIR` - 数据存储目录
- `PROJECT_ROOT` - 项目根路径（开发模式）

## 参与贡献

如果觉得本项目对你有帮助，请给我们点个 ⭐️ Star！

## 许可证

MIT License
