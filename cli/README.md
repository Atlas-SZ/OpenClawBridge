# CLI Script Configuration

如果你使用发布包里的 `start-cli.sh`，推荐这样配置默认参数：

- 保留“命令行参数优先，脚本默认值兜底”
- 不要把敏感 token 提交到 GitHub（本地改即可）

示例（`start-cli.sh`）：

```bash
RELAY_URL="${1:-wss://bridge.claw.qinfei.top/client}"
ACCESS_CODE="${2:-A-123456}"
RESPONSE_TIMEOUT="${3:-30s}"
```

这样你直接 `./start-cli.sh` 就能跑，也支持临时覆盖参数，例如：

```bash
./start-cli.sh wss://bridge.example.com/client A-999999 45s
```
