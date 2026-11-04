package main

import (
	"context"
	"fmt"
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/bringup"
)

func main() {
	if err := bringup.RunInspector(context.Background(), os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "bringup-inspect:", err)
		os.Exit(1)
	}
}
