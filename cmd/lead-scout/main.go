package main

import (
	"context"
	"fmt"
	"os"

	"lead-scout/internal/app"
	"lead-scout/internal/config"
)

func main() {
	ctx := context.Background()

	config.LoadDotEnv(".env")
	cfg := config.Load()
	if err := app.Run(ctx, cfg, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
