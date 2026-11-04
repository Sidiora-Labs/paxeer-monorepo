package main

import (
	"fmt"
	scriptops "github.com/Sidiora-Labs/Paxeer-X-Network/scripts"
	"os"
)

func main() {
	args := os.Args[1:]
	if len(args) >= 2 && args[0] == "--working-directory" {
		if err := os.Chdir(args[1]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		args = args[2:]
	}
	os.Exit(scriptops.RunMaintenance(args, os.Stdin, os.Stdout, os.Stderr))
}
