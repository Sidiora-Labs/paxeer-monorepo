// Command release validates build inputs and verifies native release artifacts.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/sei-protocol/seictl/internal/releaseproof"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: release version|verify [flags]")
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	manifest := fs.String("manifest", "version.json", "release manifest")
	expected := fs.String("expected", "", "expected v-prefixed release tag")
	binary := fs.String("binary", "", "native artifact executable")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	version, err := releaseproof.ReadVersion(*manifest, *expected)
	if err != nil {
		return err
	}
	switch os.Args[1] {
	case "version":
		fmt.Println(version)
		return nil
	case "verify":
		if *binary == "" {
			return fmt.Errorf("--binary is required")
		}
		path, err := filepath.Abs(*binary)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return releaseproof.Verify(ctx, version, func(ctx context.Context, args []string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, path, args...)
			cmd.Stderr = os.Stderr
			return cmd.Output()
		})
	default:
		return fmt.Errorf("unknown release operation %q", os.Args[1])
	}
}
