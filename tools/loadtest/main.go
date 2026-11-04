package main

import (
	"fmt"
	"log"
	"os"

	"github.com/paxeer-network/pax-load/config"
	"github.com/paxeer-network/pax-load/config/cli"
	"github.com/paxeer-network/pax-load/stats"
	"github.com/spf13/cobra"
)

var (
	rootCmd              *cobra.Command
	ErrPerformanceBounds = cli.ErrPerformanceBounds
)

func init() {
	var err error
	rootCmd, err = cli.NewCommand()
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		if _, err := fmt.Fprintf(os.Stderr, "Error: %v\n", err); err != nil {
			log.Fatal(err)
		}
		os.Exit(1)
	}
}

func enforceBounds(s config.Settings, fs *stats.FinalStats) error {
	return cli.EnforceBounds(s, fs)
}
