package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"openclaw-bridge/shared/protocol"
)

func main() {
	relayURL := flag.String("relay-url", "ws://127.0.0.1:8080/client", "relay client websocket url")
	accessCode := flag.String("access-code", "", "access code")
	responseTimeout := flag.Duration("response-timeout", 45*time.Second, "max wait per prompt before timing out")
	flag.Parse()

	if strings.TrimSpace(*accessCode) == "" {
		log.Fatal("-access-code is required")
	}

	conn, _, err := websocket.DefaultDialer.Dial(*relayURL, nil)
	if err != nil {
		log.Fatalf("connect relay error=%v", err)
	}
	defer conn.Close()

	connectData, err := protocol.EncodeControl(protocol.ControlMessage{
		Type:       protocol.TypeConnect,
		AccessCode: *accessCode,
		E2EE:       false,
	})
	if err != nil {
		log.Fatalf("encode connect error=%v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, connectData); err != nil {
		log.Fatalf("send connect error=%v", err)
	}

	sessionID, err := waitConnectOK(conn)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	fmt.Printf("connected session=%s\n", sessionID)

	events := make(chan protocol.Event, 16)
	errs := make(chan error, 1)
	go readLoop(conn, sessionID, events, errs)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("enter text and press Enter (Ctrl+D to quit)")
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		eventPayload, err := protocol.EncodeEvent(protocol.Event{Type: protocol.EventUserMessage, Content: line})
		if err != nil {
			log.Printf("encode event error=%v", err)
			continue
		}
		frame, err := protocol.BuildDataFrame(sessionID, 0, eventPayload)
		if err != nil {
			log.Printf("build frame error=%v", err)
			continue
		}
		if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
			log.Fatalf("send user_message error=%v", err)
		}

		for {
			select {
			case err := <-errs:
				log.Fatalf("read error=%v", err)
			case <-time.After(*responseTimeout):
				fmt.Printf("\nerror: RESPONSE_TIMEOUT no terminal event within %s\n", responseTimeout.String())
				goto nextInput
			case ev := <-events:
				switch ev.Type {
				case protocol.EventToken:
					fmt.Print(ev.Content)
				case protocol.EventEnd:
					fmt.Println()
					goto nextInput
				case protocol.EventError:
					fmt.Printf("\nerror: %s %s\n", ev.Code, ev.Message)
					goto nextInput
				}
			}
		}
	nextInput:
	}

	if err := scanner.Err(); err != nil {
		log.Printf("stdin error=%v", err)
	}

	closeData, _ := protocol.EncodeControl(protocol.ControlMessage{Type: protocol.TypeCloseSession, SessionID: sessionID})
	_ = conn.WriteMessage(websocket.TextMessage, closeData)
}

func waitConnectOK(conn *websocket.Conn) (string, error) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			return "", err
		}
		if msgType != websocket.TextMessage {
			continue
		}
		msg, err := protocol.DecodeControl(data)
		if err != nil {
			continue
		}
		switch msg.Type {
		case protocol.TypeConnectOK:
			if msg.SessionID == "" {
				return "", fmt.Errorf("missing session_id")
			}
			return msg.SessionID, nil
		case protocol.TypeError:
			return "", fmt.Errorf("%s: %s", msg.Code, msg.Message)
		}
	}
}

func readLoop(conn *websocket.Conn, sessionID string, out chan<- protocol.Event, errs chan<- error) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			errs <- err
			return
		}
		if msgType != websocket.BinaryMessage {
			continue
		}

		sid, _, payload, err := protocol.ParseDataFrame(data)
		if err != nil {
			continue
		}
		if sid != sessionID {
			continue
		}

		event, err := protocol.DecodeEvent(payload)
		if err != nil {
			continue
		}
		out <- event
	}
}
