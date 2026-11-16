package meshpki

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
)

// Run implements the meshpki command. Mutations produce no stdout; describe
// emits exactly one JSON object with public values. Errors contain no material.
func Run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("action required: authority, issue, assertion, describe")
	}
	action := args[0]
	if action != "authority" && action != "issue" && action != "assertion" && action != "describe" {
		return errors.New("unknown action")
	}
	flags := flag.NewFlagSet(action, flag.ContinueOnError)
	// Flag errors may echo supplied values; keep those errors out of diagnostics.
	flags.SetOutput(io.Discard)
	dir := flags.String("dir", "", "operator-owned output directory outside Git working copies")
	var service, names *string
	var lifetime *time.Duration
	if action != "authority" {
		service = flags.String("service", "", "service token")
	}
	if action == "issue" {
		names = flags.String("names", "", "comma-separated DNS and IP names")
	}
	if action == "authority" || action == "issue" {
		lifetime = flags.Duration("lifetime", 0, "explicit certificate lifetime, e.g. 8760h")
	}
	if err := flags.Parse(args[1:]); err != nil {
		return errors.New("invalid command arguments")
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if *dir == "" {
		return errors.New("--dir is required")
	}
	if service != nil {
		if err := checkService(*service); err != nil {
			return err
		}
	}
	var err error
	switch action {
	case "authority":
		err = CreateAuthority(*dir, *lifetime)
	case "issue":
		err = IssueService(*dir, *service, strings.Split(*names, ","), *lifetime)
	case "assertion":
		err = CreateAssertionKey(*dir, *service)
	case "describe":
		var description Description
		description, err = DescribeService(*dir, *service)
		if err == nil {
			err = json.NewEncoder(stdout).Encode(description)
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", action, err)
	}
	return nil
}
