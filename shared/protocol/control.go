package protocol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

const (
	Version = 1

	TypeRegister     = "REGISTER"
	TypeConnect      = "CONNECT"
	TypeConnectOK    = "CONNECT_OK"
	TypeSessionOpen  = "SESSION_OPEN"
	TypeCloseSession = "CLOSE_SESSION"
	TypeHeartbeat    = "HEARTBEAT"
	TypeError        = "ERROR"
)

type Caps struct {
	E2EE bool `json:"e2ee"`
}

type ControlMessage struct {
	Type           string `json:"type"`
	V              int    `json:"v"`
	AccessCodeHash string `json:"access_code_hash,omitempty"`
	AccessCode     string `json:"access_code,omitempty"`
	Generation     int    `json:"generation,omitempty"`
	SessionID      string `json:"session_id,omitempty"`
	E2EE           bool   `json:"e2ee,omitempty"`
	Caps           *Caps  `json:"caps,omitempty"`
	Code           string `json:"code,omitempty"`
	Message        string `json:"message,omitempty"`
}

func DecodeControl(data []byte) (ControlMessage, error) {
	var msg ControlMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return ControlMessage{}, err
	}
	if msg.Type == "" {
		return ControlMessage{}, fmt.Errorf("missing type")
	}
	if msg.V == 0 {
		msg.V = Version
	}
	return msg, nil
}

func EncodeControl(msg ControlMessage) ([]byte, error) {
	if msg.V == 0 {
		msg.V = Version
	}
	return json.Marshal(msg)
}

func HashAccessCode(accessCode string) string {
	sum := sha256.Sum256([]byte(accessCode))
	return "sha256:" + hex.EncodeToString(sum[:])
}
