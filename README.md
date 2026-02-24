<div align="center">

<img width="1704" height="1210" alt="image" src="https://github.com/user-attachments/assets/f51bbc4d-c789-4f11-b037-6c2fa3cf9cfb" />

<br/>

```
 ██████╗██╗  ██╗ █████╗ ████████╗██╗      ██████╗  ██████╗      ██████╗  ██████╗ ████████╗
██╔════╝██║  ██║██╔══██╗╚══██╔══╝██║     ██╔═══██╗██╔════╝      ██╔══██╗██╔═══██╗╚══██╔══╝
██║     ███████║███████║   ██║   ██║     ██║   ██║██║  ███╗█████╗██████╔╝██║   ██║   ██║
██║     ██╔══██║██╔══██║   ██║   ██║     ██║   ██║██║   ██║╚════╝██╔══██╗██║   ██║   ██║
╚██████╗██║  ██║██║  ██║   ██║   ███████╗╚██████╔╝╚██████╔╝      ██████╔╝╚██████╔╝   ██║
 ╚═════╝╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   ╚══════╝ ╚═════╝  ╚═════╝       ╚═════╝  ╚═════╝    ╚═╝
```

<br/>

[![Platform](https://img.shields.io/badge/Platform-macOS_Only-000000?style=for-the-badge&logo=apple&logoColor=white)](https://github.com/TE0dollary/chatlog-bot)
[![WeChat](https://img.shields.io/badge/WeChat-v4_✓-07C160?style=for-the-badge&logo=wechat&logoColor=white)](https://github.com/TE0dollary/chatlog-bot)
[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://github.com/TE0dollary/chatlog-bot)
[![License](https://img.shields.io/badge/Apache-2.0-ff4757?style=for-the-badge)](./LICENSE)
[![MCP](https://img.shields.io/badge/MCP-Ready-7c3aed?style=for-the-badge)](https://modelcontextprotocol.io)
[![Docker](https://img.shields.io/badge/Docker-Supported-2496ED?style=for-the-badge&logo=docker&logoColor=white)](https://hub.docker.com/r/TE0dollary/chatlog-bot)

<br/>

**`// 从本地内存中提取密钥 · 解密数据库 · 以 API 形式暴露你的聊天数据`**

_基于 [@sjzar](https://github.com/sjzar) 的 [chatlog](https://github.com/sjzar/chatlog) 二次开发 · 专注 macOS + 微信 v4 深度适配_

</div>

---

## ◆ Features

| 模块 | 功能 | 状态 |
|------|------|:----:|
| 🔑 **密钥提取** | 从微信进程内存（`vmmap` + `glance`）动态提取加密密钥 | ✅ |
| 🔓 **数据库解密** | AES-256-CBC 解密本地 SQLite 数据库文件 | ✅ |
| 🌐 **HTTP API** | RESTful API，查询聊天记录 / 联系人 / 群聊 / 会话 | ✅ |
| 🤖 **MCP 协议** | Streamable HTTP，与 Claude / ChatWise 等 AI 助手无缝集成 | ✅ |
| 🪝 **Webhook** | 实时监听新消息，推送到任意 HTTP 端点（n8n 等） | ✅ |
| 🔄 **自动解密** | `fsnotify` 监控数据库变更，自动触发解密流程 | ✅ |
| 👥 **多账号** | 支持多账号管理，账号间自由切换 | ✅ |
| 🐳 **Docker** | 容器化部署，适配 NAS / 服务器等无头环境 | ✅ |
| 🖥️ **Terminal UI** | 基于 `tview` 的交互式终端界面 | ✅ |
| 🖼️ **图片解密** | 图片密钥获取 + `.dat` / `wxgf` 格式解析 | 🚧 |

---

## ◆ Quick Start

### 基本流程

```
[1] 安装 chatlog  →  [2] 临时关闭 SIP  →  [3] 运行程序  →  [4] 解密数据  →  [5] 开启服务
```

```bash
# Step 1 · 从源码安装
go install github.com/TE0dollary/chatlog-bot@latest

# Step 2 · 启动 Terminal UI
chatlog

# Step 3 · 无 UI 模式直接启动 HTTP 服务
chatlog server
```

> 💡 聊天记录不全？先[从手机迁移数据](#-从手机迁移聊天记录)，再解密
> ⚠️ macOS 用户需提前[临时关闭 SIP](#-macos--sip)才能提取密钥

---

## ◆ 安装

### 从源码编译

```bash
# 依赖 CGO（go-sqlite3）
CGO_ENABLED=1 go build -trimpath -o bin/chatlog main.go

# 或使用 Makefile（推荐）
make build
```

> ⚠️ `go-sqlite3` 必须在 `CGO_ENABLED=1` 下编译。macOS 需先安装 Xcode Command Line Tools：
> ```bash
> xcode-select --install
> ```

### 下载预编译版本

访问 [**Releases**](https://github.com/TE0dollary/chatlog-bot/releases) 页面，下载对应系统的预编译二进制文件。

---

## ◆ 使用指南

### Terminal UI 模式

```bash
chatlog
```

| 按键 | 操作 |
|------|------|
| `↑` `↓` | 选择菜单项 |
| `Enter` | 确认选择 |
| `Esc` | 返回上级菜单 |
| `Ctrl+C` | 退出程序 |

### 命令行模式

```bash
# 提取微信数据密钥
chatlog key

# 解密数据库文件
chatlog decrypt

# 仅启动 HTTP 服务（无 TUI）
chatlog server
```

---

## ◆ Docker 部署

适合 NAS 等无 GUI 的设备，需提前在本机提取密钥。

**0. 提取密钥**

```shell
$ chatlog key
Data Key:  [c0163e***ac3dc6]
Image Key: [38636***653361]
```

**1. 拉取镜像**

```shell
# Docker Hub
docker pull TE0dollary/chatlog-bot:latest

# GitHub Container Registry
docker pull ghcr.io/TE0dollary/chatlog-bot:latest
```

> - Docker Hub: https://hub.docker.com/r/TE0dollary/chatlog-bot
> - GHCR: https://ghcr.io/TE0dollary/chatlog-bot

**2. 运行容器**

```shell
docker run -d \
  --name chatlog \
  -p 5030:5030 \
  -v /path/to/your/wechat/data:/app/data \
  TE0dollary/chatlog-bot:latest
```

详细指南：[Docker 部署文档](docs/docker.md)

---

## ◆ 从手机迁移聊天记录

1. 手机微信 → `我` → `设置` → `通用` → `聊天记录迁移与备份`
2. 选择 `迁移` → `迁移到电脑`，按提示操作
3. 迁移完成后，重新运行 `chatlog` 提取密钥并解密

> 此操作不影响手机端聊天记录，仅是数据复制

---

## ◆ macOS · SIP

获取密钥前需临时关闭 SIP（System Integrity Protection）：

```shell
# Intel Mac:  重启时按住 Command + R 进入恢复模式
# Apple Silicon: 重启时长按电源键进入恢复模式

# 在恢复模式终端中执行
csrutil disable

# 重启系统后即可提取密钥
# 提取完成后可重新启用
csrutil enable
```

```shell
# 安装 Xcode Command Line Tools（首次使用需要）
xcode-select --install
```

> ⚠️ Apple Silicon 用户：确保微信、chatlog 和终端均未在 Rosetta 模式下运行

---

## ◆ HTTP API

服务默认监听 `http://127.0.0.1:5030`

### 聊天记录

```http
GET /api/v1/chatlog?time=2024-01-01&talker=wxid_xxx&limit=50&format=json
```

| 参数 | 说明 |
|------|------|
| `time` | 时间范围：`YYYY-MM-DD` 或 `YYYY-MM-DD~YYYY-MM-DD` |
| `talker` | 聊天对象（支持 wxid、群聊 ID、备注名、昵称等） |
| `limit` | 返回记录数量 |
| `offset` | 分页偏移量 |
| `format` | 输出格式：`json` / `csv` / 纯文本 |

### 其他接口

```http
GET /api/v1/contact    # 联系人列表
GET /api/v1/chatroom   # 群聊列表
GET /api/v1/session    # 最近会话
```

### 多媒体内容

```http
GET /image/<id>   # 图片（302 跳转，支持 .dat 实时解密）
GET /voice/<id>   # 语音（直接返回 MP3，SILK 格式实时转码）
GET /video/<id>   # 视频（302 跳转）
GET /file/<id>    # 文件（302 跳转）
GET /data/<path>  # 基于数据目录的相对路径访问
```

---

## ◆ Webhook

开启自动解密后，新消息到达时通过 HTTP POST 推送到指定 URL。

> ⏱️ 延迟参考：本地服务 ~13 秒 · 远程同步 ~45 秒

### 配置方式 1 · JSON 配置文件

`$HOME/.chatlog/chatlog.json`:

```json
{
  "webhook": {
    "host": "localhost:5030",
    "items": [
      {
        "url": "http://localhost:8080/webhook",
        "talker": "wxid_123",
        "sender": "",
        "keyword": ""
      }
    ]
  }
}
```

### 配置方式 2 · 环境变量（Server 模式）

```shell
# 方案 A · 完整 JSON
CHATLOG_WEBHOOK='{"host":"localhost:5030","items":[{"url":"http://localhost:8080/proxy","talker":"wxid_123"}]}'

# 方案 B · 分离配置
CHATLOG_WEBHOOK_HOST="localhost:5030"
CHATLOG_WEBHOOK_ITEMS='[{"url":"http://localhost:8080/proxy","talker":"wxid_123"}]'
```

### Payload 示例

```http
POST /webhook HTTP/1.1
Host: localhost:8080
Content-Type: application/json

{
  "talker": "wxid_123",
  "length": 1,
  "lastTime": "2025-08-27 00:00:00",
  "messages": [
    {
      "seq": 1756225000000,
      "time": "2025-08-27T00:00:00+08:00",
      "talker": "wxid_123",
      "sender": "wxid_123",
      "senderName": "Name",
      "isSelf": false,
      "isChatRoom": false,
      "type": 1,
      "subType": 0,
      "content": "测试消息"
    }
  ]
}
```

---

## ◆ MCP 集成

Chatlog 实现了 [MCP (Model Context Protocol)](https://modelcontextprotocol.io) 协议，让 AI 助手可以直接查询你的聊天记录。

```
MCP Endpoint: http://127.0.0.1:5030/mcp
```

| 客户端 | 接入方式 |
|--------|----------|
| **ChatWise** | 工具设置 → 添加 `http://127.0.0.1:5030/mcp` |
| **Cherry Studio** | MCP 服务器设置 → 添加 `http://127.0.0.1:5030/mcp` |
| **Claude Desktop** | 通过 [mcp-proxy](https://github.com/sparfenyuk/mcp-proxy) 转发 |
| **Monica Code** | 通过 mcp-proxy + VSCode 插件配置 |

详细配置步骤：[MCP 集成指南](docs/mcp.md)

---

## ◆ Prompt 示例

查看 [Prompt 指南](docs/prompt.md) 获取与 AI 助手协作查询聊天记录的示例 prompt。

欢迎通过 [Discussions](https://github.com/TE0dollary/chatlog-bot/discussions) 分享你的使用经验和 prompt！

---

## ◆ 免责声明

> ⚠️ **使用本项目前，请务必阅读完整的 [免责声明](./DISCLAIMER.md)**

- 仅限处理您自己合法拥有或已获授权的聊天数据
- 严禁用于未经授权获取、查看或分析他人聊天记录
- 开发者不对使用本工具导致的任何损失承担责任
- 使用第三方 LLM 服务时，须遵守相应服务的使用条款

**本项目完全免费开源，任何以本项目名义收费的行为均与本项目无关。**

---

## ◆ License

本项目基于 [Apache-2.0 许可证](./LICENSE) 开源。
本项目不收集任何用户数据，所有数据处理均在用户本地设备上进行。

---

## ◆ 致谢

本项目在以下优秀开源项目的基础上构建：

| 项目 | 贡献 |
|------|------|
| [@sjzar](https://github.com/sjzar) · [chatlog](https://github.com/sjzar/chatlog) | 原项目，本项目基于此二次开发 |
| [@0xlane](https://github.com/0xlane) · [wechat-dump-rs](https://github.com/0xlane/wechat-dump-rs) | 密钥提取思路参考 |
| [@xaoyaoo](https://github.com/xaoyaoo) · [PyWxDump](https://github.com/xaoyaoo/PyWxDump) | 数据库解析参考 |
| [@git-jiadong](https://github.com/git-jiadong) · [go-lame](https://github.com/git-jiadong/go-lame) / [go-silk](https://github.com/git-jiadong/go-silk) | SILK 音频转码 |
| [Anthropic](https://www.anthropic.com/) · [MCP](https://github.com/modelcontextprotocol) | Model Context Protocol |
