# OpenClawBridge

OpenClawBridge 是一个 WebSocket 桥接系统，用于在**不暴露 OpenClaw Gateway 公网端口**的前提下，让用户侧应用安全访问远端 OpenClaw。

固定架构：

```text
用户侧 App/CLI -> Relay(中继服务器) -> Connector(OpenClaw 服务器) -> OpenClaw Gateway(127.0.0.1)
```

## 项目定位

- `relay`：中继服务，只做转发，不存 payload。
- `connector`：部署在 OpenClaw 服务器，连接 Relay 和本地 Gateway。
- `cli`：用户侧验收工具，用来验证链路。

## 设计边界（v0.1）

- 无账号体系，`access_code` 是唯一凭证。
- Relay 不保存消息内容，不落盘 payload，不记录 payload 日志。
- Relay 只解析控制帧和 DATA 头，不解析业务语义。
- 并发、排队、会话语义交给 OpenClaw，本项目不做写锁/并发拦截。

## Quick Start

以下步骤按角色分三段，适合你的生产架构（不是单机演示）。

### 0) 一键部署（推荐）

在对应机器执行（root 或 sudo）：

```bash
# 中继服务器
sudo ./deploy/install.sh --role relay

# OpenClaw 服务器（连接器）
sudo ./deploy/install.sh --role connector
```

可选参数：

- `--config /etc/openclaw-bridge/connector.json`：指定 connector 配置路径
- `--role both`：同机安装 relay + connector（仅测试场景）

### 1) 准备代码（两台服务器都执行）

```bash
git clone https://github.com/Atlas-SZ/OpenClawBridge.git
cd OpenClawBridge
go mod tidy
```

### 2) 中继服务器（Relay）

编译并启动：

```bash
go build -o /usr/local/bin/openclaw-relay ./relay
/usr/local/bin/openclaw-relay -addr :8080
```

生产建议：前置 Nginx 提供 TLS/WSS，反代到 Relay：

- `/tunnel` -> `http://127.0.0.1:8080/tunnel`
- `/client` -> `http://127.0.0.1:8080/client`

模板文件：`deploy/nginx/openclaw-bridge.conf`

### 3) OpenClaw 服务器（Connector）

编译：

```bash
go build -o /usr/local/bin/openclaw-connector ./connector
mkdir -p /etc/openclaw-bridge
```

创建配置 `/etc/openclaw-bridge/connector.json`（示例）：

```json
{
  "relay_url": "wss://YOUR_RELAY_DOMAIN/tunnel",
  "access_code": "A-123456",
  "generation": 1,
  "caps": { "e2ee": false },
  "reconnect_seconds": 2,
  "gateway": {
    "url": "ws://127.0.0.1:18789",
    "auth": { "token": "YOUR_GATEWAY_TOKEN" },
    "client": {
      "id": "cli",
      "displayName": "OpenClaw Bridge Connector",
      "version": "0.1.0",
      "platform": "linux",
      "mode": "cli"
    },
    "scopes": ["operator.read", "operator.write"],
    "send_method": "agent",
    "cancel_method": "chat.abort"
  }
}
```

启动：

```bash
/usr/local/bin/openclaw-connector -config /etc/openclaw-bridge/connector.json
```

### 4) 用户侧（CLI 验证）

```bash
go run ./cli -relay-url wss://YOUR_RELAY_DOMAIN/client -access-code A-123456 -response-timeout 30s
```

看到 `connected session=...` 后输入文本，能收到 `token/end` 即链路成功。
CLI 也支持 `json:` 前缀发送完整事件（可用于附件/媒体字段测试）。
可选参数：
- `-reconnect=true|false`（默认 `true`，断线自动重连）
- `-reconnect-delay 2s`（重连间隔）

### 5) 用户侧（Web 验收页）

仓库内提供单文件 Web 客户端：`web/client/index.html`。

启动本地静态服务（任选其一）：

```bash
cd web/client
python3 -m http.server 8787
```

浏览器打开 `http://127.0.0.1:8787`，手动输入：

- Relay WS URL（例如 `wss://YOUR_RELAY_DOMAIN/client`）
- Access Code / Token（用于 CONNECT 的 `access_code`）

页面支持：

- 文本 `user_message`
- 附件（浏览器内转 base64，走 `attachments` 字段）
- Raw JSON 事件发送（便于调试富媒体字段）

## Release 包内容

自动打包的压缩包结构（所有平台统一）：

- 第一层按功能拆分：`relay/`、`connector/`、`cli/`
- 每个功能目录内放：
  - 该功能对应的二进制
  - 启动与卸载脚本
  - `config/`（该功能相关配置与服务文件）

目录结构示例：

```text
openclaw-bridge-<platform>/
├── relay/
│   ├── openclaw-relay(.exe)
│   ├── start-relay.*
│   ├── uninstall-relay.*
│   └── config/
│       ├── nginx/openclaw-bridge.conf
│       └── systemd/openclaw-bridge-relay.service
├── connector/
│   ├── openclaw-connector(.exe)
│   ├── start-connector.*
│   ├── install-connector-service.sh (Linux)
│   ├── uninstall-connector.*
│   └── config/
│       ├── connector.json.example
│       └── systemd/openclaw-bridge-connector.service
└── cli/
    ├── openclaw-cli(.exe)
    ├── start-cli.*
    └── uninstall-cli.*
```

## 可选：systemd 常驻运行

仓库已提供模板：

- `deploy/systemd/openclaw-bridge-relay.service`
- `deploy/systemd/openclaw-bridge-connector.service`

安装方式（示例）：

```bash
cp deploy/systemd/openclaw-bridge-relay.service /etc/systemd/system/
cp deploy/systemd/openclaw-bridge-connector.service /etc/systemd/system/
systemctl daemon-reload
systemctl enable --now openclaw-bridge-relay
systemctl enable --now openclaw-bridge-connector
```

## 常见问题（最小版）

- `Gateway auth failed`：`gateway.auth.token` 与 Gateway 配置不一致。
- `missing scope: operator.admin`：Connector 会自动尝试补 admin scope；若仍失败，说明 token 本身无该权限。
- `unknown method ...`：`send_method` 拼写错误，推荐保持 `agent`。
- 发送后长时间无返回：用 `-response-timeout` 防止 CLI 无限制等待，并查看 Connector/Gateway 日志。

macOS 提示“二进制已损坏/不安全，无法打开”时：

1. 先校验下载包哈希是否与 Release 页 `SHA256SUMS.txt` 一致。
2. 去除隔离属性（quarantine）后再执行：

```bash
# 以下载目录为例
xattr -dr com.apple.quarantine /path/to/openclaw-bridge-darwin-*/
chmod +x /path/to/openclaw-bridge-darwin-*/relay/openclaw-relay
chmod +x /path/to/openclaw-bridge-darwin-*/connector/openclaw-connector
chmod +x /path/to/openclaw-bridge-darwin-*/cli/openclaw-cli
```

3. 若企业策略仍拦截，可做本地 ad-hoc 签名（仅本机信任）：

```bash
codesign --force --sign - /path/to/openclaw-bridge-darwin-*/relay/openclaw-relay
codesign --force --sign - /path/to/openclaw-bridge-darwin-*/connector/openclaw-connector
codesign --force --sign - /path/to/openclaw-bridge-darwin-*/cli/openclaw-cli
```

## 文档

- 协议：`docs/protocol.md`
- 边界：`docs/boundary.md`

## License

MIT License, see `LICENSE`.
