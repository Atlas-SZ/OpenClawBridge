package protocol

import "fmt"

const FlagE2EE byte = 1 << 0

func BuildDataFrame(sessionID string, flags byte, payload []byte) ([]byte, error) {
	if len(sessionID) == 0 {
		return nil, fmt.Errorf("session_id required")
	}
	if len(sessionID) > 255 {
		return nil, fmt.Errorf("session_id too long")
	}

	frame := make([]byte, 0, 1+len(sessionID)+1+len(payload))
	frame = append(frame, byte(len(sessionID)))
	frame = append(frame, []byte(sessionID)...)
	frame = append(frame, flags)
	frame = append(frame, payload...)
	return frame, nil
}

func ParseDataFrame(frame []byte) (sessionID string, flags byte, payload []byte, err error) {
	if len(frame) < 3 {
		return "", 0, nil, fmt.Errorf("frame too short")
	}
	sidLen := int(frame[0])
	if sidLen == 0 {
		return "", 0, nil, fmt.Errorf("sid_len must be > 0")
	}
	if len(frame) < 1+sidLen+1 {
		return "", 0, nil, fmt.Errorf("invalid frame header")
	}

	sessionID = string(frame[1 : 1+sidLen])
	flags = frame[1+sidLen]
	payload = frame[1+sidLen+1:]
	return sessionID, flags, payload, nil
}
