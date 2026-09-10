package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/S1933/agentic-workflow/internal/cli"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	stat, err := os.Stdin.Stat()
	interactive := err == nil && stat.Mode()&os.ModeCharDevice != 0
	app := cli.App{Interactive: interactive}
	if err := app.Run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "agentic:", err)
		os.Exit(1)
	}
}
