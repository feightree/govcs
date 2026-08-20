package main

import (
	"fmt"
	"log/slog"

	"github.com/feightree/govcs/internal/engine"
	"github.com/feightree/govcs/internal/transport"
)

func main() {
	engine, _ := engine.New16()
	engine.Start()

	slog.Info(fmt.Sprintf("ok: %+v", engine.ChargePoint))
	msg := transport.NewMessageID()
	slog.Info(fmt.Sprintf("%s, %d", msg, len(msg)))
}
