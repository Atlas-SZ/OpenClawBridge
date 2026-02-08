# openclaw-bridge

`openclaw-bridge` 是一个 WebSocket 桥接系统：
在不暴露 OpenClaw Gateway 公网端口的前提下，让用户客户端访问远端 OpenClaw。

## Architecture

固定架构（按角色分三段）：

1. OpenClaw 侧（远端服务器）：`OpenClaw Gateway + connector`
2. 中继侧（独立服务器）：`relay`
3. 用户侧：`app/cli`

链路：

```text
User App/CLI -> Relay -> Connector -> OpenClaw Gateway
```

## Components

- `relay`：中继服务（仅转发，不存 payload）
- `connector`：OpenClaw 侧桥接器（连接 Relay 与本地 Gateway）
- `cli`：命令行客户端（验收工具）

## Security & Privacy

- Relay 不保存聊天内容，不落盘 payload。
- Relay 不解析业务内容，只解析路由所需头部。
- OpenClaw Gateway 仅本地监听，不暴露公网端口。
- Connector 连接 Gateway 需要 token；鉴权失败直接退出。
- 本项目无账号体系，`access_code` 泄露即访问权限泄露。
- 本项目不替代 OpenClaw 自身安全（主机被入侵/Gateway 漏洞不在本项目防护范围）。

## Repository Layout

```text
.
├── relay/         # 中继服务
├── connector/     # OpenClaw 侧桥接器
├── cli/           # 用户侧验收客户端
├── deploy/        # 部署模板（Nginx）
├── shared/        # 共享协议
└── docs/          # 协议与边界文档
```

## Prerequisites

- Go 1.22+
- OpenClaw Gateway 运行在 OpenClaw 服务器（默认 `ws://127.0.0.1:18789`）

## Deployment

### A. OpenClaw 侧（远端服务器）

部署并运行：

1. OpenClaw Gateway（本地地址，如 `ws://127.0.0.1:18789`）
2. Connector

初始化：

```bash
go mod tidy
```

配置文件默认读取：`connector/config.example.json`

关键配置：

- `relay_url`：指向中继服务器的 `/tunnel`（生产建议 `wss://.../tunnel`）
- `access_code`：给用户客户端使用的授权码
- `gateway.url`：本地 Gateway 地址
- `gateway.auth.token`：OpenClaw operator token

启动 Connector：

```bash
go run ./connector
```

如果你的 Gateway 方法名不是默认值，调整：

- `gateway.send_method`（默认 `send`）
- `gateway.cancel_method`（默认 `cancel`）

### B. 中继侧（中继服务器）

启动 Relay：

```bash
go run ./relay -addr :8080
```

生产建议通过 Nginx 提供 TLS 与 WebSocket 反代。
仓库模板：`deploy/nginx/openclaw-bridge.conf`

最小启用步骤：

1. 复制模板到 `/etc/nginx/conf.d/openclaw-bridge.conf`
2. 替换 `DOMAIN`、`CERT_PATH`、`KEY_PATH`
3. `nginx -t && systemctl reload nginx`

对外端点：

- `/tunnel` -> Relay `/tunnel`
- `/client` -> Relay `/client`

### C. 用户侧（客户端设备）

当前客户端为 CLI（验收工具）：

```bash
go run ./cli -relay-url wss://YOUR_DOMAIN/client -access-code YOUR_ACCESS_CODE
```

要求：`-access-code` 必须与 Connector 配置中的 `access_code` 一致。

## Connector Config Example

`connector/config.example.json`：

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
- Connector 按设计会直接退出（fail-fast）

### `GATEWAY_NOT_READY`

- Gateway 未启动或地址错误
- Connector 会自动重连 Gateway

### 已连接但无 token

- Gateway 方法名可能与默认 `send/cancel` 不一致
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
