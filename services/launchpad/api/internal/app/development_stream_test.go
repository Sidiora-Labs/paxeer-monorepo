package app_test

import "testing"

func TestDevelopmentStreamAndTransactionPaths(t *testing.T) {
	for _, tc := range []struct {
		name string
		run  func(*testing.T)
	}{
		{"known pool snapshot", TestDevelopmentStreamSnapshotKnownPoolWithoutData},
		{"registry read", TestDevelopmentStreamSnapshotRegistryFailureStaysRetryable},
		{"websocket and event stream", TestDevelopmentStreamSnapshotWebSocketAndSSE},
		{"terminal frames", TestDevelopmentStreamTerminalFramesCarryCloseCode},
		{"unknown pool and legacy", TestDevelopmentStreamUnknownPoolAndLegacy},
		{"unknown subscription", TestDevelopmentStreamUnknownPoolSubscribeCloses},
	} {
		t.Run(tc.name, tc.run)
	}
}
