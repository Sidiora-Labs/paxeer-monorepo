//go:build release
// +build release

// The code in here is comprehensive as an integration
// test and is long, hence is selected explicitly with the release tag in CI.

package client

import "testing"

func TestWSClientReconnectWithJitter(t *testing.T) {
	t.Run("configured_retries", func(t *testing.T) {
		repairM1102ReconnectWithJitter(t, 8, 3)
	})
	t.Run("first_retry_exhausted", func(t *testing.T) {
		repairM1102ReconnectWithJitter(t, 8, 0)
	})
}
