package main

import (
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/benchmark"
)

func main() {
	os.Exit(benchmark.RunCLI(os.Args[1:], os.Stderr))
}
