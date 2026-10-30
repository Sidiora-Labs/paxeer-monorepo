package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func rebuildArguments(source string, args []string) []string {
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "..", ".."))
	return append([]string{"-C", filepath.Join(root, "services", "launchpad", "trading-charts"), "run", "./cmd/charts-rebuild"}, args...)
}

func main() {
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "operator launcher source path unavailable")
		os.Exit(1)
	}
	command := exec.Command("go", rebuildArguments(source, os.Args[1:])...)
	command.Stdin = os.Stdin
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "rebuild command unavailable")
		os.Exit(1)
	}
}
