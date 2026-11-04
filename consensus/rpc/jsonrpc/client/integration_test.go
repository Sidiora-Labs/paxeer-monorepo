//go:build release
// +build release

// The code in here is comprehensive as an integration
// test and is long, hence is only run before releases.

package client

import "testing"

func TestWSClientReconnectWithJitter(t *testing.T) {
	repairM1102ReconnectWithJitter(t, 8, 3)
}
