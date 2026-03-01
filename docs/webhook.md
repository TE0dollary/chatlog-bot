# Webhook 使用文档

## 概述

Webhook 模块允许 chatlog-bot 在检测到新消息时，主动将消息数据推送（HTTP POST）到你指定的外部 URL。这是一种事件驱动的消息订阅机制，适合接入机器人、通知系统或自动化流水线。

**核心原理**：程序通过 `fsnotify` 监听微信解密后的 SQLite 数据库文件变化，一旦检测到消息库有写入（新消息），就从数据库中查询符合过滤条件的新消息，批量 POST 到配置的 URL。

---

## 配置文件结构

配置文件默认位于 `./data/config.yaml`（可通过 `CHATLOG_DIR` 环境变量或 `--config` 参数修改目录）。

Webhook 配置段：

```yaml
webhook:
  host: "127.0.0.1:5030"   # chatlog HTTP 服务地址，用于生成图片/语音的访问链接
  delay_ms: 500             # 触发 webhook 前的延迟毫秒数（等待数据库写入完成）
  items:
    - type: "message"       # 目前仅支持 "message"（可省略，默认即为 message）
      url: "http://your-server/webhook"  # 接收推送的目标 URL（必填）
      talker: ""            # 过滤：聊天对象（微信ID 或 群ID），留空表示不过滤
      sender: ""            # 过滤：发送者（微信ID），留空表示不过滤
      keyword: ""           # 过滤：消息关键词，留空表示不过滤
      disabled: false       # 是否禁用该条规则
```

### 字段说明

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `webhook.host` | string | 否 | chatlog 本身的 HTTP 地址。媒体消息（图片、语音）的 URL 会以此为前缀生成，例如 `http://127.0.0.1:5030/image/...` |
| `webhook.delay_ms` | int | 否 | 文件变更事件触发后的等待毫秒数。SQLite 写入非原子性，适当延迟可避免读取到不完整数据，建议设置 300~1000 |
| `items[].type` | string | 否 | 推送类型，目前只支持 `"message"`，省略时默认为 `"message"` |
| `items[].url` | string | 是 | 目标 webhook 地址，程序会向此 URL 发起 POST 请求 |
| `items[].talker` | string | 否 | 聊天对象过滤。支持微信 ID（如 `wxid_xxx`）、群 ID（如 `xxx@chatroom`），以及备注名/昵称。多个用逗号分隔 |
| `items[].sender` | string | 否 | 发送者过滤。支持微信 ID 或显示名。多个用逗号分隔 |
| `items[].keyword` | string | 否 | 关键词过滤，匹配消息内容包含该词的消息 |
| `items[].disabled` | bool | 否 | 设为 `true` 时跳过该条规则，方便临时停用 |

---

## 工作原理

```
微信写入 SQLite 数据库
        ↓
fsnotify 检测到 message_*.db 文件变更
        ↓
延迟 delay_ms 毫秒（等待写入完成）
        ↓
从数据库查询 lastTime 至 now+10min 之间的新消息
（按 talker / sender / keyword 过滤）
        ↓
对消息内容做处理：
  - 图片/语音生成带 host 的访问 URL
  - 内容转换为纯文本格式
        ↓
HTTP POST 到 items[].url
```

**注意**：每个 webhook item 独立维护 `lastTime`（上次推送的最后一条消息时间），程序启动时从当前时刻开始，不会重放历史消息。

---

## 推送数据格式

程序向目标 URL 发送 `Content-Type: application/json` 的 POST 请求，body 结构如下：

```json
{
  "talker": "xxx@chatroom",
  "sender": "",
  "keyword": "",
  "lastTime": "2026-03-01 12:00:01",
  "length": 2,
  "messages": [
    {
      "seq": 1234567890001,
      "time": "2026-03-01T12:00:00+08:00",
      "talker": "xxx@chatroom",
      "talkerName": "技术交流群",
      "isChatRoom": true,
      "sender": "wxid_abc123",
      "senderName": "张三",
      "isSelf": false,
      "type": 1,
      "subType": 0,
      "content": "你好，这里是消息内容",
      "contents": {
        "host": "127.0.0.1:5030"
      }
    }
  ]
}
```

### 顶层字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `talker` | string | 本次推送使用的 talker 过滤条件（来自配置） |
| `sender` | string | 本次推送使用的 sender 过滤条件（来自配置） |
| `keyword` | string | 本次推送使用的 keyword 过滤条件（来自配置） |
| `lastTime` | string | 本批消息中最后一条消息时间+1秒（下次从此时间开始查询） |
| `length` | int | 本次推送的消息数量 |
| `messages` | array | 消息列表 |

### Message 对象字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `seq` | int64 | 消息序号（10位时间戳 + 3位序号） |
| `time` | string | 消息发送时间（RFC3339 格式） |
| `talker` | string | 聊天对象 ID（私聊为对方微信ID，群聊为群ID如 `xxx@chatroom`） |
| `talkerName` | string | 聊天对象显示名称（群名或好友备注/昵称） |
| `isChatRoom` | bool | 是否为群聊消息 |
| `sender` | string | 发送者微信 ID |
| `senderName` | string | 发送者显示名称（群内昵称或备注名） |
| `isSelf` | bool | 是否为自己发送的消息 |
| `type` | int64 | 消息类型（见下表） |
| `subType` | int64 | 消息子类型（type=49 分享类消息时有效） |
| `content` | string | 消息纯文本内容（图片/语音等会转为带链接的文本） |
| `contents` | object | 额外内容（媒体 md5、URL 等，因消息类型而异） |

### 消息类型（type）

| type | 说明 | content 示例 |
|------|------|-------------|
| 1 | 文本 | `"你好"` |
| 3 | 图片 | `"![图片](http://host/image/md5)"` |
| 34 | 语音 | `"[语音](http://host/voice/serverID)"` |
| 42 | 名片 | `"[名片]"` |
| 43 | 视频 | `"![视频](http://host/video/md5)"` |
| 47 | 动画表情 | `"![动画表情](cdnUrl)"` |
| 48 | 位置 | `"[位置\|label\|city\|x\|y]"` |
| 49 | 分享/链接/文件/小程序等 | 依 subType 而定 |
| 50 | 语音通话 | `"[语音通话]"` |
| 10000 | 系统消息 | `"xxx加入了群聊"` |

---

## 使用场景示例

### 场景：监听指定群的消息

**需求**：有一个或多个特定的群，想在每次有新消息时，推送到自己的 Bot 服务处理。

**可行性**：完全支持。通过 `talker` 字段填写群 ID（或群名），即可只推送该群的消息。每个群可配置不同的 webhook URL，也可以共享同一个 URL。

**第一步：获取群 ID**

通过 chatlog 的 HTTP API 查询群聊列表，找到目标群的 `name` 字段（格式为 `xxxxx@chatroom`）：

```bash
curl "http://127.0.0.1:5030/api/v1/chatroom?key=群名关键词"
```

返回示例：
```json
{
  "items": [
    {
      "name": "12345678@chatroom",
      "nickName": "技术交流群",
      "remark": ""
    }
  ]
}
```

**第二步：配置 webhook**

监听单个群：

```yaml
webhook:
  host: "127.0.0.1:5030"
  delay_ms: 500
  items:
    - url: "http://your-bot-server/webhook"
      talker: "12345678@chatroom"
```

监听多个群（推送到同一 URL）：

```yaml
webhook:
  host: "127.0.0.1:5030"
  delay_ms: 500
  items:
    - url: "http://your-bot-server/webhook"
      talker: "12345678@chatroom,87654321@chatroom"
```

监听多个群（各自推送到不同 URL）：

```yaml
webhook:
  host: "127.0.0.1:5030"
  delay_ms: 500
  items:
    - url: "http://your-bot-server/webhook/group-a"
      talker: "12345678@chatroom"
    - url: "http://your-bot-server/webhook/group-b"
      talker: "87654321@chatroom"
```

监听所有群的消息（不过滤 talker，只过滤群聊）：

> 目前配置层面没有专门的"只看群聊"开关，但可以在你的 webhook 服务端通过 `isChatRoom: true` 字段过滤。若 `talker` 留空，所有私聊和群聊消息均会推送。

```yaml
webhook:
  host: "127.0.0.1:5030"
  delay_ms: 500
  items:
    - url: "http://your-bot-server/webhook/all"
      talker: ""   # 留空 = 不过滤，接收全部消息
```

**第三步：实现接收端**

你的服务需要监听对应端口，处理 POST 请求：

```python
from flask import Flask, request

app = Flask(__name__)

@app.route('/webhook', methods=['POST'])
def handle_webhook():
    data = request.json
    messages = data.get('messages', [])
    for msg in messages:
        if not msg.get('isChatRoom'):
            continue  # 若未指定 talker，可在此过滤只处理群聊
        talker_name = msg.get('talkerName', msg.get('talker'))
        sender_name = msg.get('senderName', msg.get('sender'))
        content = msg.get('content', '')
        print(f"[{talker_name}] {sender_name}: {content}")
    return 'ok', 200

if __name__ == '__main__':
    app.run(port=8080)
```

---

## 环境变量支持

所有配置项均可通过环境变量覆盖，前缀为 `CHATLOG_`，`.` 替换为 `_`。

常用环境变量：

| 环境变量 | 对应配置 | 示例 |
|---------|---------|------|
| `CHATLOG_WEBHOOK_HOST` | `webhook.host` | `127.0.0.1:5030` |

> 注意：`items` 是数组结构，较难通过单一环境变量完整表达，建议通过配置文件管理。

---

## 注意事项

1. **Webhook 仅在数据库有新文件写入时触发**，轮询间隔由微信写入 SQLite 的频率决定，不是实时毫秒级。
2. **程序启动时不会重放历史消息**，只推送启动后产生的新消息。
3. **推送失败不会重试**，若目标 URL 返回非 200 状态码或网络超时（默认 10 秒），该批消息会丢失，请确保接收端稳定。
4. **`talker` 支持模糊匹配**：填写备注名或昵称时，系统会尝试从联系人缓存中解析为真实的微信 ID。
5. **需要先完成数据库解密**，webhook 依赖解密后的 SQLite 文件，请确保自动解密已开启（`auto_decrypt: true`）或手动解密过。
