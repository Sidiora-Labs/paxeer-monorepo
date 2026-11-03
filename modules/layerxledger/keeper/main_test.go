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
	fmt.Fprintf(os.Stderr, "keeper_wall_time_seconds=%.6f\n", time.Since(started).Seconds())
	os.Exit(code)
}
