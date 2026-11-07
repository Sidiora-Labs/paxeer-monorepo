package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/paxeer-network/paxlog"

	noderuntime "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/node/runtime"
)

var logger = paxlog.NewLogger("tendermint", "test", "e2e", "node")

// main is the binary entrypoint.
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) != 2 {
		fmt.Printf("Usage: %v <configfile>", os.Args[0])
		return
	}
	if err := run(ctx, os.Args[1]); err != nil {
		os.Exit(1)
	}
	<-ctx.Done()
	logger.Info("Shutting down...")
}

// run runs the application - basically like main() with error handling.
func run(ctx context.Context, configFile string) error {
	return noderuntime.Run(ctx, configFile, os.Getenv("TMHOME"))
}
