package benchmark

import (
	"fmt"
	"io"
)

// RunCLI executes the standalone command and returns its process exit status.
func RunCLI(args []string, stderr io.Writer) int {
	if len(args) != 1 {
		_, _ = fmt.Fprintln(stderr, "Usage: run.sh <config-file-path>")
		return 1
	}
	engine, err := NewBenchmarkEngine(args[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Failed to create benchmark engine: %v\n", err)
		return 1
	}
	engine.Logger().Info("Configuration loaded", "path", args[0])
	engine.Logger().Info("Press Ctrl+C to stop the benchmark")
	if err := engine.Run(); err != nil {
		_, _ = fmt.Fprintf(stderr, "Benchmark failed: %v\n", err)
		return 1
	}
	engine.Logger().Info("Benchmark Terminated")
	return 0
}
