# Boundary Statement (v0.1)

1. Relay never stores DATA payload (memory forwarding only).
2. Relay never writes DATA payload to disk.
3. Relay logs only metadata (session_id, byte counts, errors), never payload body.
4. Relay only parses control messages: REGISTER, CONNECT, CONNECT_OK, SESSION_OPEN, CLOSE_SESSION, HEARTBEAT, ERROR.
5. Relay treats DATA payload as opaque bytes (plaintext/ciphertext both supported transparently).
6. Connector v0.1 does not implement busy/write_lock/concurrency interception.
7. Connector supports rich payload pass-through at event level (for example `attachments`, `mediaUrl`, `mediaUrls`), while Relay still treats DATA payload as opaque bytes.
8. No account system in v0.1. Access code is the only credential.
9. OpenClaw Gateway integration is Phase 2; this phase is stub-only in local implementation.
