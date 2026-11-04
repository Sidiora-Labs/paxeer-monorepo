package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/protocol/internal/qualification"
)

func run(args []string, out io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: protocol-policy clock|rejection [options]")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	root := flags.String("root", ".", "repository source root")
	evidence := flags.String("evidence", "", "retained node policy evidence JSON")
	revision := flags.String("revision", "", "expected verified source revision")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	switch args[0] {
	case "clock":
		if err := qualification.CheckClock(*root); err != nil {
			return err
		}
		fmt.Fprintln(out, "consensus clock scan passed")
	case "rejection":
		if *evidence == "" || *revision == "" {
			return fmt.Errorf("rejection requires --evidence and --revision")
		}
		file, err := os.Open(*evidence)
		if err != nil {
			return err
		}
		defer file.Close()
		checked, err := qualification.ReadPolicyEvidence(file, *revision)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "revision: %s\npolicy cases: %d\npaxeer x rejection policy passed\n", checked.Revision, len(checked.Cases))
	default:
		return fmt.Errorf("unknown policy %q", args[0])
	}
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
}
