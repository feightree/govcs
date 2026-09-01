package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	v16 "github.com/feightree/govcs/internal/engine/v16"
)

func main() {
	raw := os.Args[0]
	bin := filepath.Base(raw)

	if err := v16.Dispatch(bin, os.Args); err != nil {
		switch {
		case errors.Is(err, v16.ErrRootUsage):
			fmt.Printf("Error: %s", err.Error())
			v16.PrintRootUsage(bin)
		case errors.Is(err, v16.ErrCreateUsage):
			fmt.Printf("Error: %s", err.Error())
			v16.PrintCreateUsage(bin)
		case errors.Is(err, v16.ErrStartUsage):
			fmt.Printf("Error: %s", err.Error())
			v16.PrintStartUsage(bin)
		default:
			fmt.Printf("Error: %s\n", err.Error())
		}
	}
}
