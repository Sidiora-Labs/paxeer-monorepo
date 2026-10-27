package main

import (
	"fmt"
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/appenv"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--help" || os.Args[1] == "-h") {
		fmt.Fprintln(os.Stdout, "usage: paxeer-bridge-app-env [--check <env-file>] <records-directory>")
		return
	}
	if err := appenv.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "paxeer-bridge-app-env: %v\n", err)
		os.Exit(1)
	}
}
