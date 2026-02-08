# openclaw-bridge

`openclaw-bridge` 是一个 WebSocket 桥接系统，用于在**不暴露 OpenClaw Gateway 公网端口**的前提下，让客户端访问你的 OpenClaw。

本仓库当前提供：

- `relay`：云端中继服务
- `connector`：用户侧桥接器（连接 Relay 与本地 OpenClaw Gateway）
- `cli`：验收客户端（命令行）

## Features

- 全链路 WebSocket
- Relay 仅做会话路由与转发（不存 payload）
- 授权码为唯一凭证（无账号体系）
- Connector 支持 OpenClaw Gateway `operator` 握手
- Gateway 鉴权失败直接退出（不 silent retry）
- Gateway 断线指数退避重连

## Architecture

```text
Client (CLI / App)
  -> Relay (cloud)
  -> Connector (user machine)
  -> OpenClaw Gateway (local, ws://127.0.0.1:18789)
```

## Repository Layout

```text
.
├── relay/         # 云端中继服务
├── connector/     # 用户侧桥接器
├── cli/           # 验收客户端
├── shared/        # 共享协议定义
└── docs/          # 协议与边界文档
```

## Prerequisites

- Go 1.22+
- OpenClaw Gateway 已在 Connector 机器上运行（默认 `ws://127.0.0.1:18789`）

## Quick Start (Local)

### 1) 初始化

```bash
go mod tidy
```

### 2) 配置 Connector

默认直接读取 `connector/config.example.json`（可用 `-config` 指定其他路径）。

编辑 `connector/config.example.json`，至少确认：

- `access_code`：客户端连接用授权码
- `relay_url`：Relay 的 `/tunnel` 地址
- `gateway.url`：本地 Gateway 地址
- `gateway.auth.token`：OpenClaw operator token

如果 Gateway 业务方法名不是默认值，调整：

- `gateway.send_method`（默认 `send`）
- `gateway.cancel_method`（默认 `cancel`）

### 3) 启动（3 个终端）

终端 A（Relay）：

```bash
go run ./relay -addr :8080
```

终端 B（Connector）：

```bash
go run ./connector
```

终端 C（CLI）：

```bash
go run ./cli -relay-url ws://127.0.0.1:8080/client -access-code A-123456
```

注意：CLI 的 `-access-code` 必须与当前 Connector 配置文件中的 `access_code` 一致。

### 4) 验收

在 CLI 中输入文本并回车，预期：

- 流式收到 `token`
- 最终收到 `end`

## Deployment Guide

### OpenClaw Side (用户侧)

在**同一台机器（或同内网）**部署：

1. OpenClaw Gateway（本地监听）
2. Connector（连接 Gateway + Relay）

### Server Side (云端)

部署 Relay 到云服务器，并用 Nginx 做 TLS 终止与 WS 反代：

- `/tunnel` -> `http://127.0.0.1:8080/tunnel`
- `/client` -> `http://127.0.0.1:8080/client`

生产环境客户端应使用 `wss://...`。

### Client Side

当前客户端是 `cli`（验收工具），可运行在任意可访问 Relay 的设备。
后续可替换为 Web/桌面/移动 UI。

## Connector Config Example

见 `connector/config.example.json`。

核心结构：

```json
{
  "relay_url": "ws://127.0.0.1:8080/tunnel",
  "access_code": "A-123456",
  "gateway": {
    "url": "ws://127.0.0.1:18789",
    "auth": { "token": "GATEWAY_AUTH_TOKEN" },
    "client": {
      "id": "bridge-connector",
      "displayName": "OpenClaw Bridge Connector"
    }
  }
}
```

## Build

```bash
go build -o /tmp/openclaw-relay ./relay
go build -o /tmp/openclaw-connector ./connector
go build -o /tmp/openclaw-cli ./cli
```

## Troubleshooting

### `Gateway auth failed`

- `gateway.auth.token` 无效或为空
- 这是 fail-fast 行为，Connector 会退出

### `GATEWAY_NOT_READY`

- Gateway 未启动或地址错误
- Connector 会自动重连 Gateway

### 已连接但无 token

- Gateway 业务方法名不匹配默认值
- 调整 `gateway.send_method` / `gateway.cancel_method`

## Protocol & Boundaries

- 协议文档：`docs/protocol.md`
- 边界声明：`docs/boundary.md`

关键边界：

- Relay 不保存 payload，不记录 payload 日志
- Relay 仅解析控制帧和 DATA header
- v0.1 不做并发调度/写锁

## License

MIT，见 `LICENSE`。
