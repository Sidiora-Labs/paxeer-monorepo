package main

import (
	"fmt"
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tools/benchmark"
)

func main() {
	if err := benchmark.RunHome(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "benchmark home:", err)
		os.Exit(1)
	}
}
