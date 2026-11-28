package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestMainRejectsMissingConfiguration(t *testing.T) {
	const argument = "pnld-missing-configuration"
	if os.Args[len(os.Args)-1] == argument {
		main()
		t.Fatal("command returned without rejecting missing configuration")
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run=^TestMainRejectsMissingConfiguration$", "--", argument)
	command.Env = []string{}
	if coverage := flag.Lookup("test.gocoverdir"); coverage != nil && coverage.Value.String() != "" {
		command.Env = append(command.Env, "GOCOVERDIR="+coverage.Value.String())
	}
	output, err := command.CombinedOutput()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 || !strings.Contains(string(output), "pnld: config:") || !strings.Contains(string(output), "WEBHOOK_HMAC_SECRET") {
		t.Fatalf("command error = %v, context = %v, output = %s", err, ctx.Err(), output)
	}
}
