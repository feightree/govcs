package main

import (
	"github.com/feightree/govcs/internal/engine"
)

func main() {
	engine, _ := engine.New16()
	engine.Start()
}
