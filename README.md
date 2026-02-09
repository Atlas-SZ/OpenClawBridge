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
- `--web-root /var/www/openclaw-bridge-web`：指定 Web 静态文件目录（relay）
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
- `/` -> Nginx 静态页面（Web 验收页）

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
  "gateway": {
    "url": "ws://127.0.0.1:18789",
    "auth": { "token": "YOUR_GATEWAY_TOKEN" },
    "scopes": ["operator.read", "operator.write"],
    "min_protocol": 3,
    "max_protocol": 3
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
CLI 也支持 `json:` 前缀发送完整事件（可用于 `images` 字段测试）。
可选参数：
- `-reconnect=true|false`（默认 `true`，断线自动重连）
- `-reconnect-delay 2s`（重连间隔）

### 5) 用户侧（Web 验收页，Nginx 静态）

仓库内提供单文件 Web 客户端：`web/client/index.html`。

在 Relay 服务器发布静态文件：

```bash
sudo ./deploy/relay/linux/publish-web-static.sh
```

然后用 `deploy/nginx/openclaw-bridge.conf` 配置 Nginx（`root /var/www/openclaw-bridge-web;`），浏览器打开：

```text
https://YOUR_RELAY_DOMAIN/
```

页面里手动输入：

- Relay WS URL（例如 `wss://YOUR_RELAY_DOMAIN/client`）
- Access Code / Token（用于 CONNECT 的 `access_code`）

页面支持：

- 文本 `user_message`
- 图片（浏览器内转 base64，走 `images` 字段）
- Raw JSON 事件发送（便于调试 `images` 字段）

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

## 线上编译更新与检查

以下命令用于服务器上“拉代码 -> 编译 -> 重启生效 -> 检查状态”。

### Relay（中继服务）

```bash
cd /opt/code/OpenClawBridge
git pull
go build -o /usr/local/bin/openclaw-relay ./relay
systemctl restart openclaw-bridge-relay
```

### Connector（OpenClaw 侧桥接器）

```bash
cd /opt/code/OpenClawBridge
git pull
go build -o /usr/local/bin/openclaw-connector ./connector
systemctl restart openclaw-bridge-connector
```

### 检查指令（状态 / 日志 / 健康）

```bash
systemctl status openclaw-bridge-relay --no-pager
systemctl status openclaw-bridge-connector --no-pager
journalctl -u openclaw-bridge-relay -n 80 --no-pager
journalctl -u openclaw-bridge-connector -n 80 --no-pager
curl -v http://127.0.0.1:8080/healthz
```

如果 OpenClaw Gateway 也是 systemd user service，可额外检查：

```bash
systemctl --user status openclaw-gateway.service --no-pager
journalctl --user -u openclaw-gateway.service -n 80 --no-pager
```

## 常见问题（最小版）

- `Gateway auth failed`：`gateway.auth.token` 与 Gateway 配置不一致。
- `missing scope ...`：检查 `gateway.scopes` 是否与 token 权限匹配（v2 不再自动 scope 回退）。
- `unknown method ...`：当前 Connector 固定调用 `agent` 与 `chat.abort`，请确认 Gateway 版本支持。
- `websocket: close 1009 (message too big)`：请求包超过 Nginx 或 Gateway 限制；请在 Nginx/Gateway 侧调整允许的消息大小。
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
