package keeper_test

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	started := time.Now()
	code := m.Run()
	fmt.Fprintf(os.Stderr, "inventory_sweep_wall_time_seconds=%.6f exit=%d\n", time.Since(started).Seconds(), code)
	os.Exit(code)
}
