package main

import (
	"fmt"
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/hosted/paxeer"
)

func main() {
	if err := paxeer.RunDeployment(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
