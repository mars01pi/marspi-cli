# Marspi CLI

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue)](LICENSE)
[![Bubble Tea](https://img.shields.io/badge/TUI-Bubble%20Tea-ff69b4)](https://github.com/charmbracelet/bubbletea)

> Go 实现的终端 AI 编程助手 — 零第三方依赖核心，Bubble Tea TUI 交互

---

## ✨ 特性

| 特性 | 说明 |
|------|------|
| 🖥 **TUI 交互** | Bubble Tea 多行输入、可滚动历史、分区着色、任务可中断（`Esc`） |
| 🧠 **三级上下文压缩** | micro → session → full compact，自动管理 token 上限 |
| 🧭 **Smart Routing** | 关键词 + LLM 混合评分，自动在 low / medium / high 三级模型间路由 |
| 🔄 **Loop Engineering** | `/loop <goal>` 三智能体协作循环 |
| 🛠 **DevFlow** | `/df <goal>` 可配置 Design→…→Push 流水线（Spec / `--workflow`） |
| 🔌 **Flash-ext 代理** | OpenAI 兼容 HTTP 服务，自动注入结构化思考框架 |
| 💾 **长期记忆** | `search_memory` / `append_memory` 跨会话持久化 |
| 📦 **技能系统** | SKILL.md 技能加载，`use_skill` 按需调用 |
| 🛠 **12 个内置工具** | read / write / edit / search / grep / bash / web_search / view_image … |
| 🔒 **安全防护** | 危险命令检测 + 路径越界校验 + 目录输出过滤 |

---

## 🚀 快速开始

```bash
# 1. 构建
make build

# 2. 配置
export MARS_KEY=sk-your-key
export MARS_API_URL=https://api.deepseek.com
export MARS_MODEL=deepseek-v4-flash

# 3. 启动交互模式（默认 TUI）
./marspi-cli

# 环境诊断
./marspi-cli -doctor

# Flash-ext 代理模式
./marspi-cli flash-ext --port 8080
```

> 💡 设置 `MARS_PLAIN=1` 退回旧版单行 REPL（用于管道/非 TTY 场景）

---

## ⌨️ TUI 快捷键

| 按键 | 作用 |
|------|------|
| `Enter` | 发送消息 |
| `Shift+Enter` / `Ctrl+J` | 换行（Cursor/VS Code 终端下 Shift+Enter 发 `\n`） |
| `Alt+Enter` | 换行（备用） |
| `PgUp` / `PgDn` | 滚动历史（滚轮亦可） |
| `Esc` | 中断当前 agent 任务 |
| `/stop` / `/s` | 同上（命令方式） |
| `Ctrl+C` | 退出（无任务时） |

历史区域可滚动；工具调用、思考、输出分区着色显示。

---

## 🔧 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `MARS_KEY` | — | API Key **（必填）** |
| `MARS_API_URL` | `https://api.deepseek.com` | API 地址 |
| `MARS_MODEL` | `deepseek-v4-flash` | 模型名 |
| `MARS_MAX_CONTEXT` | `1000000` | 上下文 token 上限 |
| `MARS_MAX_ITER` | `100` | 单轮最大工具迭代 |
| `MARS_LANG` | `en` | 界面语言：`en` / `zh` |
| `MARS_ROUTING` | `off` | Smart Routing：`on` / `off` |
| `MARS_SEARCH_API_KEY` | — | 博查 Web Search API Key |
| `MARS_DEBUG` | — | `=1` 开启调试日志（TUI 内嵌显示） |
| `MARS_PLAIN` | — | `=1` 禁用 TUI，使用单行 REPL |
| `MARS_STREAM` | `1` | `=1`/`on` 启用 SSE 流式输出；`0`/`off` 回退非流式 |
| `MARSPI_CHECKPOINT_DB` | `.marspicli/checkpoints.db` | Supervisor / DevFlow 图检查点 SQLite 路径 |
| `MARSPI_DEVFLOW_ALLOW_PUSH` | `true` | `=0`/`false` 时 DevFlow 跳过 push（等同 `--no-push`） |

持久化目录：`<cwd>/.marspicli/`（session、memory、loops、providers.json、checkpoints.db）

CLI 负责读取 `MARS_*` 配置、装配系统提示词和管理会话文件。普通模式、TUI 和 `/loop` 都调用 Core 的 `agent.Run`，运行结束（包括取消）后由 CLI 保存一次会话。`session.json` 沿用原有消息数组格式，并支持从旧 `.tmp` 文件恢复；`/new` 备份后立即保存新会话，`/compact` 成功后立即保存。提示词只为当前已注册的工具生成使用指引。

CLI 在 `internal/tools` 中显式装配文件、Shell、搜索、记忆、技能、RAG、图片和完成工具；Core 提供工具协议、schema、注册表、执行、取消和 provider 生命周期。

Supervisor（`/sv`）与 DevFlow（`/df`）会把 graph Snapshot 写入 checkpoints.db。Esc 中断或 HITL 挂起后可用 `/sv resume` / `/df resume <threadID>` 跨进程续跑（只恢复图状态，不恢复 agent 对话；见 marspi-graph ADR 0004）。`/sv list` / `/df list` 列出可续跑线程。

DevFlow（`/df` / `/devflow`）加载内置 Design→Develop→Review→Test→Push Spec（或 `--workflow path.yaml`）。Push 前 Confirm；`--no-push` 只改代码不推送。

---

## 🧭 Smart Routing

按任务复杂度自动分配模型层级，无需手动切换。

```bash
mkdir -p .marspicli
cp providers.json.example .marspicli/providers.json
# 编辑 api_key
export MARS_ROUTING=on
./marspi-cli
```

**路由策略：**

```
用户输入 → 关键词评分 (30%) → 低/高复杂度直接判定
                               └→ 中等复杂度 → LLM 二次评分 (70%) → 加权混合 → 选择 tier
```

| 层级 | 适用场景 | 示例模型 |
|------|---------|---------|
| 🟢 `low` | 读文件、搜索、简单问答 | `deepseek-v4-flash` |
| 🟡 `medium` | 多文件编辑、调试 | `gpt-4o-mini` |
| 🔴 `high` | 架构设计、大型重构 | `gpt-4o` / `claude-opus` |

---

## 📟 内置命令

| 命令 | 说明 |
|------|------|
| `/q` `/quit` | 退出 |
| `/stop` `/s` | 中断当前任务（TUI 中也可用 `Esc`） |
| `/c` `/compact` | 手动 full compact |
| `/n` `/new` | 新建会话 |
| `/h` `/help` | 帮助 |
| `/l` `/loop <goal>` | Loop Engineering |
| `/lg` `/loopg <goal>` | Graph CodingLoop（实验） |
| `/sv` `/supervise <goal>` | Supervisor（实验；`/sv resume` `/sv list`） |
| `/df` `/devflow <goal>` | DevFlow 流水线（`--workflow` / `--no-push` / `resume` / `list`） |

---

## 📁 项目结构

```
marspi-cli/
├── main.go                 # 入口
├── cmd/
│   ├── root.go             # App 装配 + TUI/Plain 双模式
│   ├── repl.go             # Bubble Tea TUI
│   ├── session.go          # 会话加载与单轮运行后的保存
│   ├── engine.go           # Loop Engineering
│   ├── flashext.go         # Flash-ext 代理
│   └── doctor.go           # 环境诊断
├── internal/
│   ├── config/             # 配置加载
│   ├── flashext/           # OpenAI 兼容代理服务
│   ├── prompt/             # 系统提示词
│   ├── session/            # JSON 会话存储、恢复与备份
│   ├── tools/              # CLI 内置工具与显式工具装配
│   ├── i18n/               # 国际化 (en/zh)
│   └── ui/                 # TUI 事件 + Printer + Hooks
└── .marspicli/             # 持久化目录
```

---

## 🛠 开发

构建需要 Go 1.25 或更新版本。`marspi-core` 和 `marspi-graph` 使用 `go.mod` 中锁定的 GitHub 版本，无需克隆到相邻目录。

```bash
make test    # 运行测试
make build   # 构建二进制
go install   # 安装到 $GOPATH/bin
```

**调试模式：**

```bash
export MARS_DEBUG=1
./marspi-cli
```

调试日志将在 TUI 中内嵌显示（plain 模式写 stderr），包含请求/响应摘要、工具调用等。

---

## 📄 License

[Apache License 2.0](LICENSE)
