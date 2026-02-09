# Boundary Statement (v2)

1. Relay never stores DATA payload (memory forwarding only).
2. Relay never writes DATA payload to disk.
3. Relay logs only metadata (session_id, byte counts, errors), never payload body.
4. Relay only parses control messages: REGISTER, CONNECT, CONNECT_OK, SESSION_OPEN, CLOSE_SESSION, HEARTBEAT, ERROR.
5. Relay treats DATA payload as opaque bytes (plaintext/ciphertext both supported transparently).
6. Connector does not implement busy/write_lock/concurrency interception.
7. Connector only accepts simplified user payload: `content` + optional `images`.
8. No account system. Access code is the only credential.
9. OpenClaw Gateway integration is connector-side only; relay remains protocol-agnostic for payload content.
