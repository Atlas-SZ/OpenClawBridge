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

## Download & Install

在需要部署的每台机器上先下载代码：

```bash
git clone https://github.com/Atlas-SZ/OpenClawBridge.git
cd OpenClawBridge
```

### 中继服务器安装（relay）

```bash
go mod tidy
go build -o /usr/local/bin/openclaw-relay ./relay
chmod +x /usr/local/bin/openclaw-relay
```

### OpenClaw 侧服务器安装（connector）

```bash
go mod tidy
go build -o /usr/local/bin/openclaw-connector ./connector
chmod +x /usr/local/bin/openclaw-connector
mkdir -p /etc/openclaw-bridge
cp connector/config.example.json /etc/openclaw-bridge/connector.json
```

然后编辑 `/etc/openclaw-bridge/connector.json`（至少填写 `relay_url`、`gateway.auth.token`、`access_code`）。

## Deployment

### A. OpenClaw 侧（远端服务器）

部署并运行：

1. OpenClaw Gateway（本地地址，如 `ws://127.0.0.1:18789`）
2. Connector

关键配置：

- `relay_url`：指向中继服务器的 `/tunnel`（生产建议 `wss://.../tunnel`）
- `access_code`：给用户客户端使用的授权码
- `gateway.url`：本地 Gateway 地址
- `gateway.auth.token`：OpenClaw operator token
- `gateway.min_protocol` / `gateway.max_protocol`：Gateway 协议版本（默认 `3/3`）
- `gateway.client.id`：默认 `cli`（部分 Gateway 版本会校验固定 id）
- `gateway.client.mode`：默认 `operator`（若 Gateway 校验不通过，Connector 会自动回退尝试 `cli/desktop`）
- `gateway.client.version` / `gateway.client.platform`：为必填字段
- `gateway.scopes`：默认 `["operator.read","operator.write"]`

启动 Connector：

```bash
/usr/local/bin/openclaw-connector -config /etc/openclaw-bridge/connector.json
```

如果你的 Gateway 方法名不是默认值，调整：

- `gateway.send_method`（默认 `send`）
- `gateway.send_to`（可选，默认 `remote`，仅当你的 Gateway 需要其他目标时再改）
- `gateway.cancel_method`（默认 `cancel`）

### B. 中继侧（中继服务器）

启动 Relay：

```bash
/usr/local/bin/openclaw-relay -addr :8080
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

#### 作为 systemd service 长期运行（推荐）

仓库模板：`deploy/systemd/openclaw-bridge-relay.service`

```bash
cp deploy/systemd/openclaw-bridge-relay.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now openclaw-bridge-relay
systemctl status openclaw-bridge-relay
```

### C. 用户侧（客户端设备）

当前客户端为 CLI（验收工具）：

```bash
go run ./cli -relay-url wss://YOUR_DOMAIN/client -access-code YOUR_ACCESS_CODE
```

要求：`-access-code` 必须与 Connector 配置中的 `access_code` 一致。

### OpenClaw 侧 Connector 作为 systemd service 长期运行（推荐）

仓库模板：`deploy/systemd/openclaw-bridge-connector.service`

```bash
cp deploy/systemd/openclaw-bridge-connector.service /etc/systemd/system/
mkdir -p /etc/openclaw-bridge
# 准备 /etc/openclaw-bridge/connector.json 后再启动
systemctl daemon-reload
systemctl enable --now openclaw-bridge-connector
systemctl status openclaw-bridge-connector
```

说明：仓库内 systemd 模板使用 `WorkingDirectory=/`，不依赖代码目录路径。

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
    },
    "send_to": "remote"
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

### `gateway connect failed: protocol mismatch`

- Gateway 协议版本不匹配
- 调整 `/etc/openclaw-bridge/connector.json` 中：
  - `gateway.min_protocol`
  - `gateway.max_protocol`

### `gateway connect failed: invalid connect params`

- 检查 `gateway.client` 是否包含：
  - `id`（推荐先用 `cli`）
  - `mode`（建议 `operator`，若 Gateway 严格校验会自动回退 `cli/desktop`）
  - `version`
  - `platform`
- 检查 `caps` 是否为数组（代码默认 `[]`）

### `GATEWAY_NOT_READY`

- Gateway 未启动或地址错误
- Connector 会自动重连 Gateway

### 已连接但无 token

- Gateway 方法名可能与默认 `send/cancel` 不一致
- 调整 `gateway.send_method` / `gateway.cancel_method`

### `invalid send params`（缺少 `to/message/idempotencyKey`）

- 你的 Gateway `send` 协议要求地址化参数
- Connector 已自动发送 `to/message/idempotencyKey`
- 如目标不是默认 `remote`，再在 `/etc/openclaw-bridge/connector.json` 覆盖 `gateway.send_to`

## Protocol & Boundaries

- 协议文档：`docs/protocol.md`
- 边界声明：`docs/boundary.md`

关键边界：

- Relay 不保存 payload，不记录 payload 日志
- Relay 仅解析控制帧和 DATA header
- v0.1 不做并发调度/写锁

## License

MIT，见 `LICENSE`。
