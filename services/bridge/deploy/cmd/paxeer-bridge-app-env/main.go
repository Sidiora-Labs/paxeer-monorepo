package main

import (
	"fmt"
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/appenv"
)

func main() {
	if err := appenv.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "paxeer-bridge-app-env: %v\n", err)
		os.Exit(1)
	}
}
