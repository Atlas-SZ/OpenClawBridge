package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"openclaw-bridge/connector/pkg/bridge"
	"openclaw-bridge/connector/pkg/config"
	"openclaw-bridge/connector/pkg/gatewayclient"
	"openclaw-bridge/connector/pkg/relayclient"
	"openclaw-bridge/shared/protocol"
)

func main() {
	configPath := flag.String("config", "connector/config.example.json", "config file path")
	flag.Parse()

	logger := log.New(os.Stdout, "[connector] ", log.LstdFlags|log.Lmicroseconds)

	cfg, err := config.Load(*configPath)
	if err != nil {
		logger.Fatalf("load config error=%v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var bridgeHandler *bridge.GatewayBridge

	relay := relayclient.New(cfg, logger,
		func(msg protocol.ControlMessage) {
			switch msg.Type {
			case protocol.TypeSessionOpen:
				bridgeHandler.OpenSession(msg.SessionID)
				logger.Printf("session open sid=%s", msg.SessionID)
			case protocol.TypeCloseSession:
				bridgeHandler.CloseSession(msg.SessionID)
				logger.Printf("session close sid=%s", msg.SessionID)
			case protocol.TypeError:
				logger.Printf("relay error code=%s message=%s", msg.Code, msg.Message)
			}
		},
		func(sessionID string, flags byte, payload []byte) {
			bridgeHandler.HandleData(sessionID, flags, payload)
		},
	)
	bridgeHandler = bridge.NewGatewayBridge(logger, relay)

	gateway := gatewayclient.New(cfg.Gateway, logger, gatewayclient.Handlers{
		OnEvent: func(sessionID string, event protocol.Event) {
			bridgeHandler.HandleGatewayEvent(sessionID, event)
		},
		OnDisconnected: func(err error) {
			bridgeHandler.HandleGatewayDisconnected(err)
		},
		OnReady: func() {
			logger.Printf("gateway connected and ready")
		},
	})
	bridgeHandler.BindGateway(gateway)

	logger.Printf(
		"start relay_url=%s access_code_hash=%s gateway_url=%s",
		cfg.RelayURL,
		cfg.AccessCodeHash,
		cfg.Gateway.URL,
	)

	errCh := make(chan error, 2)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- relay.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		errCh <- gateway.Run(ctx)
	}()

	var runErr error
	for i := 0; i < 2; i++ {
		if err := <-errCh; err != nil {
			runErr = err
			cancel()
			break
		}
	}
	wg.Wait()
	close(errCh)

	if runErr != nil {
		if errors.Is(runErr, gatewayclient.ErrGatewayAuthFailed) {
			logger.Fatalf("Gateway auth failed: %v", runErr)
		}
		logger.Fatalf("connector exited err=%v", runErr)
	}
}
