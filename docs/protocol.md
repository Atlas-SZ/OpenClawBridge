# openclaw-bridge v0.1 Protocol

## Scope
- Transport: WebSocket only.
- Relay only parses control text frames and DATA frame header.
- DATA payload is opaque bytes; relay never inspects payload content.

## WebSocket Endpoints
- `GET /tunnel` for Connector.
- `GET /client` for CLI/Client.

## Control Messages (JSON text frame)
All control messages use `v=1`.

### REGISTER (Connector -> Relay)
```json
{"type":"REGISTER","v":1,"access_code_hash":"sha256:...","generation":1,"caps":{"e2ee":false}}
```

### CONNECT (Client -> Relay)
```json
{"type":"CONNECT","v":1,"access_code":"A-...","e2ee":false}
```

### CONNECT_OK (Relay -> Client)
```json
{"type":"CONNECT_OK","v":1,"session_id":"s_xxx","caps":{"e2ee":false}}
```

### SESSION_OPEN (Relay -> Connector)
```json
{"type":"SESSION_OPEN","v":1,"session_id":"s_xxx","e2ee":false}
```

### CLOSE_SESSION (Any side -> Relay or Relay -> Any side)
```json
{"type":"CLOSE_SESSION","v":1,"session_id":"s_xxx"}
```

### HEARTBEAT (Connector -> Relay)
```json
{"type":"HEARTBEAT","v":1}
```

### ERROR (Relay -> Any side)
```json
{"type":"ERROR","v":1,"code":"...","message":"..."}
```

## DATA Frame (binary)
Frame format:

| Field | Size | Notes |
|---|---:|---|
| `sid_len` | 1 byte | session id length (1..255) |
| `sid` | `sid_len` bytes | UTF-8 session id |
| `flags` | 1 byte | bit0 = e2ee |
| `payload` | remaining bytes | opaque payload |

Relay behavior:
- Parse `sid_len/sid/flags` only.
- Route by `sid` to opposite endpoint in session.
- Forward original binary frame unchanged.

## Unified Event Protocol (inside DATA payload)
v0.1 uses JSON event payload for text chat.

### user_message (Client -> Connector)
```json
{"type":"user_message","content":"hello"}
```

### control.stop (Client -> Connector)
```json
{"type":"control","action":"stop"}
```

### token (Connector -> Client)
```json
{"type":"token","content":"hel"}
```

### end (Connector -> Client)
```json
{"type":"end"}
```

### error (Connector -> Client)
```json
{"type":"error","code":"...","message":"..."}
```

## Connector <-> Gateway Mapping (Phase 2)
- Connector waits for `connect.challenge`, then sends `connect` as `role=operator`.
- `user_message` -> Gateway request (`gateway.send_method`, default `send`).
- `control.stop` -> Gateway request (`gateway.cancel_method`, default `cancel`).
- Gateway `token/chunk` events -> `token`.
- Gateway `completed/done` events -> `end`.
- Gateway `error/disconnect` events -> `error`.

## Session Rules
- Client sends CONNECT with access code.
- Relay hashes access code with SHA-256 and matches connector `access_code_hash`.
- Relay creates `session_id`, stores session map, sends CONNECT_OK and SESSION_OPEN.
- Close/session disconnect removes session map and informs peer with CLOSE_SESSION.
