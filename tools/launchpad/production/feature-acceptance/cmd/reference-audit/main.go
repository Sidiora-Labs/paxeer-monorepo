package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	acceptance "github.com/Sidiora-Labs/Paxeer-X-Network/tools/launchpad/production/feature-acceptance"
)

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "provide materialized declaration paths relative to root")
		os.Exit(2)
	}
	reports := make([]acceptance.ReferenceReport, 0, flag.NArg())
	for _, document := range flag.Args() {
		report, err := acceptance.AuditReferenceDocument(os.DirFS(*root), document)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		reports = append(reports, report)
	}
	if err := json.NewEncoder(os.Stdout).Encode(reports); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
