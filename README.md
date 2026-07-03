# Marspi CLI

Go 实现的终端 AI 编程助手

## 特性

- 交互式 REPL：默认 Bubble Tea TUI（多行输入、可滚动历史、任务可中断）
- 会话持久化与三级上下文压缩（micro / session / full compact）
- Smart Provider Routing（`MARS_ROUTING=on` + `providers.json`）
- Loop Engineering（`/loop <goal>` 三智能体协作）
- Flash-ext 子命令：OpenAI 兼容代理，注入结构化思考框架
- 长期记忆（`search_memory` / `append_memory`）与 SKILL.md 技能加载
- TUI 基于 [Bubble Tea](https://github.com/charmbracelet/bubbletea)；`MARS_PLAIN=1` 可退回单行模式

## 快速开始

```bash
# 构建
make build

# 配置 API Key
export MARS_KEY=sk-your-key
export MARS_API_URL=https://api.deepseek.com
export MARS_MODEL=deepseek-v4-flash

# 启动交互模式
./marspi-cli

# 环境诊断
./marspi-cli -doctor

# Flash-ext 代理模式
./marspi-cli flash-ext --port 8080
```

## 环境变量


| 变量                    | 默认值                        | 说明                         |
| --------------------- | -------------------------- | -------------------------- |
| `MARS_KEY`            | —                          | API Key（必填）                |
| `MARS_API_URL`        | `https://api.deepseek.com` | API 地址                     |
| `MARS_MODEL`          | `deepseek-v4-flash`        | 模型名                        |
| `MARS_MAX_CONTEXT`    | `1000000`                  | 上下文 token 上限               |
| `MARS_MAX_ITER`       | `100`                      | 单轮最大工具迭代                   |
| `MARS_LANG`           | `en`                       | 界面语言 `en` / `zh`           |
| `MARS_ROUTING`        | `off`                      | Smart Routing：`on` / `off` |
| `MARS_SEARCH_API_KEY` | —                          | 博查 Web Search API Key      |
| `MARS_DEBUG`          | —                          | 设为 `1` 开启调试日志（TUI 内嵌显示，plain 模式写 stderr）  |
| `MARS_PLAIN`          | —                          | 设为 `1` 使用旧版单行 REPL（禁用 TUI）   |


持久化目录：`<cwd>/.marspicli/`（session、memory、loops、providers.json）。

## TUI 快捷键（默认交互模式）

| 按键 | 作用 |
| --- | --- |
| `Enter` | 发送消息 |
| `Shift+Enter` | 换行（多行输入） |
| `Esc` | 中断当前 agent 任务 |
| `/stop` | 同上（任务运行中） |
| `Ctrl+C` | 退出（无任务时） |

历史区域可滚动；工具调用、思考、输出分区着色显示。

## Smart Routing

```bash
mkdir -p .marspicli
cp providers.json.example .marspicli/providers.json
# 编辑 api_key 后：
export MARS_ROUTING=on
./marspi-cli
```

## 内置命令


| 命令                  | 说明               |
| ------------------- | ---------------- |
| `/q` `/quit`        | 退出               |
| `/stop` `/s`        | 中断当前任务（TUI 中也可用 Esc） |
| `/c` `/compact`     | 手动 full compact  |
| `/n` `/new`         | 新建会话             |
| `/h` `/help`        | 帮助               |
| `/l` `/loop <goal>` | Loop Engineering |


## 开发

```bash
make test    # 运行测试
make build   # 构建二进制
go install   # 安装到 $GOPATH/bin
```

调试（查看请求/响应摘要、工具调用等）：

```bash
export MARS_DEBUG=1
./marspi-cli
```

## License

Apache License 2.0