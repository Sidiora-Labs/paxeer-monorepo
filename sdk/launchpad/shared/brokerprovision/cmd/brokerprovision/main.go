package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/brokerprovision"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	os.Exit(brokerprovision.Run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
