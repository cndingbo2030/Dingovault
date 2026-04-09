# Dingovault（中文）

English | [中文](README_zh.md)

[![Release](https://github.com/cndingbo2030/dingovault/actions/workflows/release.yml/badge.svg)](https://github.com/cndingbo2030/dingovault/actions)
[![Go mod](https://img.shields.io/github/go-mod/go-version/cndingbo2030/dingovault/main?label=go)](https://github.com/cndingbo2030/dingovault/blob/main/go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**基于 Go 的高性能、本地优先大纲笔记系统，支持 SaaS 同步。**

从源码安装 CLI / 服务端二进制：

```bash
go install github.com/cndingbo2030/dingovault/cmd/dingovault@latest
```

Go 模块路径：**`github.com/cndingbo2030/dingovault`**

Dingovault 以 Markdown 块（block）为核心，提供 FTS5 全文搜索、双链、YAML Frontmatter 与桌面端体验。通过统一的 `storage.Provider` 抽象，同一套业务逻辑可运行在本地 SQLite 或远程 SaaS API。

## 核心亮点

- **性能（Go 优势）**：常见基准中，**FTS 查询 p50 约 1ms**，**页面加载 p50 约 0.2ms**（与硬件/缓存相关，建议本机执行 `make benchmark`）。
- **安全能力**：支持 `DINGO_MASTER_KEY` 开启 **AES-256-GCM** 数据加密；SaaS 模式使用 **JWT** 鉴权（`Authorization: Bearer ...`）。
- **可扩展插件系统**：后端支持 `before:block:save`、`after:block:indexed` 事件钩子；前端支持插件按钮与侧栏插槽。

## 性能与安全

- 基准命令：`make benchmark`
- 加密压力与完整性校验：`make benchmark-encrypted`（自动启用 `DINGO_MASTER_KEY` 并执行 `-verify`）
- 加密说明：启用后块内容以 AES-256-GCM 存储；若密钥丢失或变更，历史加密数据无法解密。

## 插件开发速览

- **后端（Go）**：
  - 订阅 `after:block:indexed`，处理重建索引后的业务逻辑。
  - 使用 `storage.Provider` 访问与更新数据，避免直接耦合底层数据库实现。
  - 参考实现：`internal/plugins/summarizer`（检测 `#summarize` 并自动追加摘要子块）。
- **前端（Svelte）**：
  - `window.__DINGOVAULT__.registerToolbarButton(...)`
  - `window.__DINGOVAULT__.registerSidebarSection(...)`

## 快速开始

```bash
make dev
```

或：

```bash
wails dev
```

更多部署、SaaS、API 与发布流程，请阅读主文档 [README.md](README.md)。

## 维护者

- Maintainer: **cndingbo2030**
- Email: **[cndingbo@outlook.com](mailto:cndingbo@outlook.com)**
- Repo: [github.com/cndingbo2030/dingovault](https://github.com/cndingbo2030/dingovault)
