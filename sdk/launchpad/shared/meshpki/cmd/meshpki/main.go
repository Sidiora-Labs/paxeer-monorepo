package main

import (
	"fmt"
	"os"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/meshpki"
)

func main() {
	if err := meshpki.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
