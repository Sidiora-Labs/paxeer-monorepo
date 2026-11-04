package configuration

import (
	"context"
	"errors"
	"flag"
	"io"
	"path/filepath"
)

// Run is the shared command path used by every launcher and the native tests.
func Run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) == 0 {
		return errors.New("expected serve, send or restore")
	}
	flags := flag.NewFlagSet(args[0], flag.ContinueOnError)
	flags.SetOutput(output)
	config := flags.String("config", ".", "fixture configuration directory")
	listen := flags.String("listen", "0.0.0.0:8000", "HTTP listen address")
	archive := flags.String("archive", "", "pinned snapshot archive")
	home := flags.String("home", "/root/.simapp", "empty or absent node home")
	archiveOutput := flags.String("output", "/tmp/data.tar.gz", "reproduced archive")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	switch args[0] {
	case "send":
		if flags.NArg() != 1 {
			return errors.New("send requires one Cosmos address")
		}
		f, err := LoadFixture(*config)
		if err != nil {
			return err
		}
		return (BankProcess{Binary: "simd"}).Fund(ctx, f, flags.Arg(0))
	case "serve":
		if flags.NArg() != 0 {
			return errors.New("unexpected serve arguments")
		}
		f, err := LoadFixture(*config)
		if err != nil {
			return err
		}
		return Serve(ctx, *listen, f)
	case "restore":
		if flags.NArg() != 0 {
			return errors.New("unexpected restore arguments")
		}
		if *archive == "" {
			*archive = filepath.Join(*config, "../node/data.tar.gz")
		}
		return RestoreFixture(*config, *archive, *home, *archiveOutput)
	default:
		return errors.New("expected serve, send or restore")
	}
}
