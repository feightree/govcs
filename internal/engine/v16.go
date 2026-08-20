package engine

import (
	"errors"
	"flag"
	"log/slog"
	"os"
)

type ChargePoint struct {
	CSMS            string
	FirmwareVersion string
	ID              string
	Model           string
	SerialNumber    string
	StateDir        string
	Vendor          string
}

type Engine16 struct {
	ChargePoint
}

func New16() (*Engine16, error) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	engine := Engine16{}
	engine.ParseFlags()

	logger = logger.With("id", engine.ID)
	slog.SetDefault(logger)

	return &engine, nil
}

func (e *Engine16) ParseFlags() {
	fs := flag.NewFlagSet("engine16", flag.ContinueOnError)

	fs.StringVar(&e.ID, "id", "CS16-001", "ChargePoint Identifier")
	fs.StringVar(&e.CSMS, "csms", "ws://localhost:3000", "CSMS WebSocket URL")
	fs.StringVar(&e.Vendor, "vendor", "", "ChargePoint vendor name")
	fs.StringVar(&e.Model, "model", "", "ChargePoint model name")
	fs.StringVar(&e.SerialNumber, "serial-number", "", "ChargePoint serial number")
	fs.StringVar(&e.StateDir, "state-dir", "", "Directory for the state file")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}

		os.Exit(2)
	}
}

func (e *Engine16) Start() {
	slog.Info("Starting")
}
