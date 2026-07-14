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
| `MARSPI_CHECKPOINT_DB` | `.marspicli/checkpoints.db` | Supervisor 图检查点 SQLite 路径 |

持久化目录：`<cwd>/.marspicli/`（session、memory、loops、providers.json、checkpoints.db）

Supervisor（`/sv`）会把 graph Snapshot 写入 checkpoints.db。Esc 中断或 HITL 挂起后可用 `/sv resume <threadID>` 跨进程续跑（只恢复图状态，不恢复 worker 对话；见 marspi-graph ADR 0004）。`/sv list` 列出可续跑线程（含 mid-run 取消与 HITL）。

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

---

## 📁 项目结构

```
marspi-cli/
├── main.go                 # 入口
├── cmd/
│   ├── root.go             # App 装配 + TUI/Plain 双模式
│   ├── repl.go             # Bubble Tea TUI
│   ├── engine.go           # Loop Engineering
│   ├── flashext.go         # Flash-ext 代理
│   └── doctor.go           # 环境诊断
├── internal/
│   ├── agent/              # ReAct 主循环
│   ├── agentctx/           # 上下文管理 + 三级压缩
│   ├── config/             # 配置加载
│   ├── llm/                # LLM provider + Smart Routing
│   ├── flash/              # 结构化思考框架
│   ├── flashext/           # OpenAI 兼容代理服务
│   ├── tool/               # 12 个内置工具
│   ├── memory/             # 长期记忆 (Markdown)
│   ├── prompt/             # 系统提示词
│   ├── skill/              # 技能加载
│   ├── i18n/               # 国际化 (en/zh)
│   ├── ui/                 # TUI 事件 + Printer + Hooks
│   └── logx/               # 调试日志
└── .marspicli/             # 持久化目录
```

---

## 🛠 开发

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
