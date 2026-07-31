# Marspi CLI

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go)](https://go.dev)
[![CI](https://img.shields.io/github/actions/workflow/status/mars01pi/marspi-cli/ci.yml?branch=main&label=CI)](https://github.com/mars01pi/marspi-cli/actions)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue)](LICENSE)
[![Bubble Tea](https://img.shields.io/badge/TUI-Bubble%20Tea-ff69b4)](https://github.com/charmbracelet/bubbletea)

> Go 实现的终端 AI 编程助手：Bubble Tea TUI、三级上下文压缩、Smart Routing、Loop Engineering 与 DevFlow 流水线。
> TUI 层与核心逻辑解耦，按 Go 模块拆分：**marspi-cli**（CLI 层）/ **marspi-core**（核心逻辑）/ **marspi-graph**（图引擎）。

---

## 📑 目录

- [✨ 特性](#-特性)
- [🚀 快速开始](#-快速开始)
- [⌨️ TUI 快捷键](#-tui-快捷键)
- [🔧 环境变量](#-环境变量)
- [🧭 Smart Routing](#-smart-routing)
- [📟 内置命令](#-内置命令)
- [🧱 架构与项目结构](#-架构与项目结构)
- [🛠 开发](#-开发)
- [📄 License](#-license)

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
| 🧰 **12 个内置工具** | `read` / `write` / `edit` / `search` / `grep` / `bash` / `use_skill`<br>`search_memory` / `append_memory` / `web_search` / `view_image` / `attempt_completion` |
| 🔒 **安全防护** | 危险命令检测 + 路径越界校验 + 目录输出过滤 |

> 配置 `MARS_RAG_URL` 后额外注册 `search_knowledge`（知识库检索）工具；`MARS_MCP=on` 且配置 `mcp.json` 时注册 MCP 服务器工具。

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

# 查看版本
./marspi-cli -version

# 环境诊断
./marspi-cli -doctor
```

**Flash-ext 代理模式：**

```bash
./marspi-cli flash-ext [flags]
# --host 127.0.0.1   绑定地址（默认 127.0.0.1）
# --port 8080        服务端口（默认 8080）
# --token <token>    客户端 Bearer 认证
# --memory           启用自动记忆写入
# --web-search       启用联网搜索增强
# --debug            启用调试日志
```

> 💡 设置 `MARS_PLAIN=1` 退回旧版单行 REPL；非 TTY（管道/重定向）下自动回退 Plain 模式。

---

## ⌨️ TUI 快捷键

| 按键 | 作用 |
|------|------|
| `Enter` | 发送消息 |
| `Shift+Enter` / `Ctrl+J` / `Alt+Enter` | 换行（Cursor/VS Code 终端下 Shift+Enter 发 `\n`） |
| `PgUp` / `Ctrl+U` | 向上滚动历史 |
| `PgDn` / `Ctrl+D` | 向下滚动历史（滚轮亦可） |
| `Esc` / `Ctrl+C` | 中断当前 agent 任务（运行中） |
| `Ctrl+C` | 退出（空闲时） |
| `/stop` `/s` | 中断任务（命令方式） |

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
| `MARS_MCP` | `on` | `=off` 禁用 MCP 工具加载 |
| `MARS_RAG_URL` | — | 知识库检索端点；设置后注册 `search_knowledge` 工具 |
| `MARS_RAG_API_KEY` | — | RAG 端点 Bearer 认证（可选） |
| `MARS_RAG_MODE` | `tool` | RAG 注入模式：`tool` / `inject` / `hybrid` |
| `MARS_RAG_TOP_K` | `5` | RAG 检索条数 |
| `MARS_SEARCH_API_KEY` | — | 博查 Web Search API Key |
| `MARS_DEBUG` | — | `=1` 开启调试日志（TUI 内嵌显示） |
| `MARS_PLAIN` | — | `=1` 禁用 TUI，使用单行 REPL |
| `MARS_STREAM` | `1` | `=1`/`on` 启用 SSE 流式输出；`0`/`off` 回退非流式 |
| `MARSPI_CHECKPOINT_DB` | `.marspicli/checkpoints.db` | Supervisor / DevFlow 图检查点 SQLite 路径 |
| `MARSPI_DEVFLOW_ALLOW_PUSH` | `true` | `=0`/`false` 时 DevFlow 跳过 push（等同 `--no-push`） |

**持久化目录：** `<cwd>/.marspicli/`（session、memory、loops、skills、providers.json、mcp.json、checkpoints.db）

**检查点续跑：** Supervisor（`/sv`）与 DevFlow（`/df`）会把 graph Snapshot 写入 checkpoints.db。Esc 中断或 HITL 挂起后可用 `/sv resume` / `/df resume <threadID>` 跨进程续跑（只恢复图状态，不恢复 agent 对话；见 marspi-graph ADR 0004）。`/sv list` / `/df list` 列出可续跑线程。

**DevFlow：** `/df` / `/devflow` 加载内置 Design→Develop→Review→Test→Push Spec（或 `--workflow path.yaml`）。Push 前 Confirm；`--no-push` 只改代码不推送。

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

> ⚠️ `/goal` 已弃用，请改用 `/loop <goal>`。

---

## 🧱 架构与项目结构

项目按 Go 模块拆分三层：本仓库（CLI 层）、`marspi-core`（核心逻辑）、`marspi-graph`（图引擎），通过 `go.mod` 的 `replace` 指令关联（`../marspi-core`、`../marspi-graph`）。

```
marspi-cli/                      # 本仓库：CLI 层（TUI + 子命令）
├── main.go                      # 入口：-version / -doctor / flash-ext / REPL
├── cmd/
│   ├── root.go                  # App 装配 + 斜杠命令分发 + TUI/Plain 双模式
│   ├── repl.go                  # Bubble Tea TUI
│   ├── doctor.go                # 环境诊断
│   ├── flashext.go              # flash-ext 子命令解析
│   ├── engine.go                # Loop 引擎
│   ├── engine_supervisor.go     # Supervisor 引擎（+ _test.go）
│   ├── engine_graph.go          # Graph CodingLoop 引擎
│   ├── engine_devflow.go        # DevFlow 引擎（+ _test.go）
│   ├── agentsink.go             # Agent 事件 → UI sink
│   └── sink_console.go          # 控制台输出 sink
├── internal/
│   ├── i18n/                    # 国际化（en/zh）
│   ├── ui/                      # TUI 事件 + Printer + Hooks
│   └── flashext/                # OpenAI 兼容代理服务
├── providers.json.example       # Smart Routing 配置样例
├── .github/workflows/ci.yml     # CI：Go 1.22–1.24 矩阵构建 + 测试 + vet
└── .marspicli/                  # 持久化目录（运行时生成）

marspi-core/                     # 核心模块（replace → ../marspi-core）
├── agent/                       # ReAct 主循环
├── agentctx/                    # 上下文管理 + 三级压缩
├── config/                      # 配置加载（MARS_* 环境变量）
├── llm/                         # LLM provider + Smart Routing
├── flash/                       # 结构化思考框架
├── tool/                        # 12 个内置工具注册
├── memory/                      # 长期记忆 (Markdown)
├── prompt/                      # 系统提示词
├── skill/                       # 技能加载
├── mcp/                         # MCP 客户端
├── rag/                         # RAG 知识库检索
└── logx/                        # 调试日志

marspi-graph/                    # 图引擎（replace → ../marspi-graph）
├── graph/                       # 图运行时（节点、快照）
├── orchestrator/                # Supervisor / DevFlow 编排
├── checkpoint/                  # 检查点持久化（resume 能力）
├── workflow/                    # 流水线定义
└── agentspec/                   # 智能体规格
```

---

## 🛠 开发

```bash
make build    # 构建二进制
make install  # 安装到 $GOPATH/bin
make test     # 运行测试
make run      # 构建并运行
make doctor   # 构建并运行环境诊断
make clean    # 删除构建产物
```

**CI：** `.github/workflows/ci.yml` 在 Go 1.22 / 1.23 / 1.24 矩阵上执行 `go build`、`go test -race`（含覆盖率汇总）与 `go vet`。

**调试模式：**

```bash
export MARS_DEBUG=1
./marspi-cli
```

调试日志将在 TUI 中内嵌显示（plain 模式写 stderr），包含请求/响应摘要、工具调用等。

---

## 📄 License

[Apache License 2.0](LICENSE)
