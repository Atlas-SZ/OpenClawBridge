package main

import (
	"bufio"
	"encoding/json"
	"errors"
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
	reconnect := flag.Bool("reconnect", true, "auto reconnect when relay connection is lost")
	reconnectDelay := flag.Duration("reconnect-delay", 2*time.Second, "delay between reconnect attempts")
	flag.Parse()

	if strings.TrimSpace(*accessCode) == "" {
		log.Fatal("-access-code is required")
	}

	conn, sessionID, err := connectSession(*relayURL, *accessCode)
	if err != nil {
		log.Fatalf("connect failed: %v", err)
	}
	defer conn.Close()
	fmt.Printf("connected session=%s\n", sessionID)

	events := make(chan protocol.Event, 16)
	errs := make(chan error, 1)
	go readLoop(conn, sessionID, events, errs)

	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("enter text and press Enter (Ctrl+D to quit)")
	fmt.Println("tip: prefix with json: to send a full event payload (images field)")
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		outboundEvent := protocol.Event{Type: protocol.EventUserMessage, Content: line}
		if strings.HasPrefix(strings.TrimSpace(line), "json:") {
			raw := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "json:"))
			if raw == "" {
				fmt.Println("error: empty json payload after json:")
				continue
			}
			if err := json.Unmarshal([]byte(raw), &outboundEvent); err != nil {
				fmt.Printf("error: invalid json event: %v\n", err)
				continue
			}
			if outboundEvent.Type == "" {
				outboundEvent.Type = protocol.EventUserMessage
			}
		}

		eventPayload, err := protocol.EncodeEvent(outboundEvent)
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
			if !*reconnect {
				log.Fatalf("send user_message error=%v", err)
			}
			fmt.Printf("connection lost, reconnecting... err=%v\n", err)
			conn, sessionID, events, errs, err = reconnectSession(conn, *relayURL, *accessCode, *reconnectDelay)
			if err != nil {
				log.Fatalf("reconnect failed: %v", err)
			}
			frame, err = protocol.BuildDataFrame(sessionID, 0, eventPayload)
			if err != nil {
				log.Fatalf("build frame after reconnect error=%v", err)
			}
			if err := conn.WriteMessage(websocket.BinaryMessage, frame); err != nil {
				log.Fatalf("send user_message after reconnect error=%v", err)
			}
		}

		for {
			select {
			case err := <-errs:
				if !*reconnect {
					log.Fatalf("read error=%v", err)
				}
				fmt.Printf("\nconnection lost, reconnecting... err=%v\n", err)
				conn, sessionID, events, errs, err = reconnectSession(conn, *relayURL, *accessCode, *reconnectDelay)
				if err != nil {
					log.Fatalf("reconnect failed: %v", err)
				}
				fmt.Printf("request interrupted, please resend your message\n")
				goto nextInput
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

func reconnectSession(oldConn *websocket.Conn, relayURL, accessCode string, delay time.Duration) (*websocket.Conn, string, chan protocol.Event, chan error, error) {
	if oldConn != nil {
		_ = oldConn.Close()
	}
	for {
		conn, sessionID, err := connectSession(relayURL, accessCode)
		if err == nil {
			fmt.Printf("reconnected session=%s\n", sessionID)
			events := make(chan protocol.Event, 16)
			errs := make(chan error, 1)
			go readLoop(conn, sessionID, events, errs)
			return conn, sessionID, events, errs, nil
		}
		fmt.Printf("reconnect attempt failed: %v\n", err)
		time.Sleep(delay)
	}
}

func connectSession(relayURL, accessCode string) (*websocket.Conn, string, error) {
	conn, _, err := websocket.DefaultDialer.Dial(relayURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("connect relay: %w", err)
	}

	connectData, err := protocol.EncodeControl(protocol.ControlMessage{
		Type:       protocol.TypeConnect,
		AccessCode: accessCode,
		E2EE:       false,
	})
	if err != nil {
		_ = conn.Close()
		return nil, "", fmt.Errorf("encode connect: %w", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, connectData); err != nil {
		_ = conn.Close()
		return nil, "", fmt.Errorf("send connect: %w", err)
	}

	sessionID, err := waitConnectOK(conn)
	if err != nil {
		_ = conn.Close()
		return nil, "", err
	}
	return conn, sessionID, nil
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
			return "", fmt.Errorf("connect error %s: %s", msg.Code, msg.Message)
		}
	}
}

func readLoop(conn *websocket.Conn, sessionID string, out chan<- protocol.Event, errs chan<- error) {
	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			select {
			case errs <- err:
			default:
			}
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
		select {
		case out <- event:
		default:
			select {
			case errs <- errors.New("event queue overflow"):
			default:
			}
			return
		}
	}
}
