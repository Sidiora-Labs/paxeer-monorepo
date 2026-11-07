package keeper_test

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	flag.Parse()
	if err := flag.Set("test.v", "true"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	started := time.Now()
	code := m.Run()
	fmt.Fprintf(os.Stderr, "launchpad keeper inventory sweep wall time: %s\n", time.Since(started))
	os.Exit(code)
}
