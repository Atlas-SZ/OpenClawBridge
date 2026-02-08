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

## GitHub Release

仓库包含自动发版工作流：`.github/workflows/release.yml`。

- 触发方式：推送 tag（例如 `v0.1.0`）
- 产物内容：Linux/macOS（amd64 + arm64）/Windows 的打包二进制（relay/connector/cli）+ `SHA256SUMS.txt`
- 发布位置：GitHub 仓库的 `Releases`

## 常见问题（最小版）

- `Gateway auth failed`：`gateway.auth.token` 与 Gateway 配置不一致。
- `missing scope: operator.admin`：Connector 会自动尝试补 admin scope；若仍失败，说明 token 本身无该权限。
- `unknown method ...`：`send_method` 拼写错误，推荐保持 `agent`。
- 发送后长时间无返回：用 `-response-timeout` 防止 CLI 无限制等待，并查看 Connector/Gateway 日志。

## 文档

- 协议：`docs/protocol.md`
- 边界：`docs/boundary.md`

## License

MIT License, see `LICENSE`.
