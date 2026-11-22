// Command rolecheckgen renews SQL inventory locations from the service source.
package main

import (
	"flag"
	"log"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/rolecheck"
)

func main() {
	root := flag.String("root", ".", "repository root")
	selection := flag.String("select", "", "comma-separated file:function selections for new SELECT calls")
	flag.Parse()
	var selections []string
	if *selection != "" {
		selections = strings.Split(*selection, ",")
	}
	if err := rolecheck.WriteStatementLocations(*root, selections...); err != nil {
		log.Fatal(err)
	}
}
