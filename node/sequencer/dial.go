package sequencer

import (
	"context"
	"fmt"
	"net"
	"time"
)

// DialStandby returns a TCP dialer bounded by both timeout and the caller's context.
// The returned connection carries the WAL group and acknowledgement protocol.
func DialStandby(address string, timeout time.Duration) func(ctx context.Context) (net.Conn, error) {
	return func(ctx context.Context) (net.Conn, error) {
		if _, _, err := net.SplitHostPort(address); err != nil {
			return nil, fmt.Errorf("%w %q: %v", ErrConfigStandbyAddress, address, err)
		}
		if timeout <= 0 {
			return nil, fmt.Errorf("%w: must be positive, got %s", ErrConfigReplicationTimeout, timeout)
		}
		dialer := net.Dialer{Timeout: timeout}
		return dialer.DialContext(ctx, "tcp", address)
	}
}
