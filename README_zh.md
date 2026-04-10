# Dingovault（中文）

English | [中文](README_zh.md)

[![Release](https://img.shields.io/github/v/release/cndingbo2030/dingovault?v=1.4.2)](https://github.com/cndingbo2030/dingovault/releases)
[![Test](https://github.com/cndingbo2030/dingovault/actions/workflows/test.yml/badge.svg?v=1.4.2)](https://github.com/cndingbo2030/dingovault/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/cndingbo2030/dingovault?v=1.4.2)](https://goreportcard.com/report/github.com/cndingbo2030/dingovault)
[![Go mod](https://img.shields.io/github/go-mod/go-version/cndingbo2030/dingovault/main?label=go)](https://github.com/cndingbo2030/dingovault/blob/main/go.mod)
[![License](https://img.shields.io/github/license/cndingbo2030/dingovault?v=1.4.2)](https://github.com/cndingbo2030/dingovault/blob/main/LICENSE)
[![Stars](https://img.shields.io/github/stars/cndingbo2030/dingovault?v=1.4.2)](https://github.com/cndingbo2030/dingovault/stargazers)
[![Forks](https://img.shields.io/github/forks/cndingbo2030/dingovault?v=1.4.2)](https://github.com/cndingbo2030/dingovault/forks)
<!-- badge-refresh-2026-04-09 -->

**基于 Go 的高性能、本地优先大纲笔记系统，支持 SaaS 同步。**

从源码安装 CLI / 服务端二进制：

```bash
go install github.com/cndingbo2030/dingovault/cmd/dingovault@latest
```

Go 模块路径：**`github.com/cndingbo2030/dingovault`**

Dingovault 以 Markdown 块（block）为核心，提供 FTS5 全文搜索、双链、YAML Frontmatter 与桌面端体验。通过统一的 `storage.Provider` 抽象，同一套业务逻辑可运行在本地 SQLite 或远程 SaaS API。

## 为什么选择 Dingovault？

**快到像没有等待。** Dingovault 为「住在笔记里」的人优化：本地热路径下 **FTS 全文检索 p50 常见约 1ms 量级**，页面加载同样追求 **亚毫秒级体感**（因硬件而异，请用 `make benchmark` 自测）。找一条子弹不应先看完进度条。

**数据在磁盘上也要「军事级」保密。** 通过 **`DINGO_MASTER_KEY`** 可启用 **AES-256-GCM** 静态加密，磁盘丢失不等于明文泄露；自托管 SaaS 时配合 **JWT** 鉴权，多用户场景也能收紧边界。

**AI 不必把大脑上传到云端。** 优先对接 **Ollama** 等本地端点：行内辅助、仓库感知对话、可选向量嵌入都可以留在 **本机 GPU/CPU** 上完成，让「RAG」不必等于「把每一段笔记发给第三方模型」。

## v1.4.2 — AGPL-3.0、GHCR 镜像与 npm SDK 占位包

详见 **[CHANGELOG.md](CHANGELOG.md)**。

- **许可证：** **AGPL-3.0**；**GHCR** 按标签发布 **`ghcr.io/cndingbo2030/dingovault`**；**`@cndingbo2030/dingovault-sdk`** 发布至 GitHub Packages。

## v1.4.1 — 工作流修复与语义化发行文件名

详见 **[CHANGELOG.md](CHANGELOG.md)**。

- **Android CI：** 稳定的 **NDK/SDK 环境变量**、`ndk-bundle` 软链、**gomobile init** 顺序。
- **发行包：** 文件名自解释（**Apple-Silicon-M1-M2-M3**、**Intel-Processor**、**Windows-64bit-Installer**、**Linux-Desktop-amd64**、**Android-Mobile-Phone-Tablet** 等）。
- **UI：** **安全区**与平板横屏 / 手机竖屏下的 **顶栏**适配。

## v1.4.0 — Android gomobile 与响应式界面

详见 **[CHANGELOG.md](CHANGELOG.md)**。

- **gomobile `.aar`**（`cmd/dingovault-android/mobile`）与极简 **Gradle 壳**，在版本标签的 CI 中产出 **APK / AAB**。
- **分区存储路径**辅助函数，便于将仓库根目录放在应用专属外部目录下。
- **响应式 UI：**`dvh`、**48px** 触控、手机 **底部导航**、宽屏 **三栏**（大纲 / 语义相关 / 侧栏）。

## v1.3.2 — WebDAV、S3 兼容同步与局域网发现

详见 **[CHANGELOG.md](CHANGELOG.md)**。

- **WebDAV 双向同步** `.md` 笔记（时间戳 + 大小策略）；冲突时保留 `*.conflict.md`。
- **S3 兼容对象存储** 同步，冲突策略与 WebDAV 一致（AWS S3、MinIO 等）。
- **mDNS 发现** 与 **四位 PIN 配对**，便于在同一 Wi‑Fi 下交换同步配置（Wails 桥接 API 已就绪）。

## v1.3.0 — AI 写作与智能关联

用户向更新说明见 **[CHANGELOG.md](CHANGELOG.md)**（英文为主，含体验描述）。

- 行内 AI 流式输出；AI 服务中断时可恢复编辑并提示连接问题。
- 侧栏 AI 对话结合当前页与语义相关的历史笔记块。
- 「语义相关」与图谱中的语义边，帮助发现未手动链接的相关内容。
- 基于语义的标签推荐。

## v1.2.0 Master Release

- 全新高级视觉身份：应用图标与品牌样式全面升级。
- 100% Clean Code 架构推进：高复杂度路径完成重构。
- macOS Gatekeeper 指引增强：未签名应用安装步骤更清晰。

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

## macOS 安装提示（Gatekeeper）

如果 macOS 提示应用来自未识别开发者或“可能包含恶意软件”：

1. 对 `Dingovault.app` 右键，选择 **打开** 并确认。
2. 如仍受阻，可执行：

```bash
xattr -cr /Applications/Dingovault.app
```

这是未进行 Apple 开发者签名的开源应用常见现象。

## 容器镜像（GitHub Container Registry）

每个 **`v*`** 标签会构建并推送 **SaaS 服务端** 镜像：

```bash
docker pull ghcr.io/cndingbo2030/dingovault:v1.4.2
docker run --rm -p 12030:12030 -e DINGO_JWT_SECRET='至少16字符的密钥' -v dingovault-data:/data ghcr.io/cndingbo2030/dingovault:latest
```

详见仓库根目录 [`Dockerfile`](Dockerfile)。

## 插件 SDK 占位包（GitHub Packages / npm）

作用域包 **`@cndingbo2030/dingovault-sdk`** 在每次发版标签时从 [`sdk/`](sdk/) 发布（当前为占位，便于后续扩展类型与协议）。安装说明见 [`sdk/README.md`](sdk/README.md)。

## 许可证

本项目以 **GNU Affero General Public License v3.0（AGPL-3.0）** 授权 — 见 [`LICENSE`](LICENSE)。若您对 **网络服务** 提供修改后的版本，AGPL 要求向通过网络与您交互的用户提供对应源代码。

## 维护者

- Maintainer: **cndingbo2030**
- Email: **[cndingbo@outlook.com](mailto:cndingbo@outlook.com)**
- Repo: [github.com/cndingbo2030/dingovault](https://github.com/cndingbo2030/dingovault)
