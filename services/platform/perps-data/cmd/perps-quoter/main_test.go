package main

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestConfigurationRefusesBeforeOpeningChain(t *testing.T) {
	t.Setenv("PERPS_CHAIN_ID", "125")
	t.Setenv("PERPS_RPC_URLS", "https://user:password@rpc.example/private-token?key=secret")
	t.Setenv("PERPS_QUOTER_KEY_FILE", "/private/account-key")
	t.Setenv("PERPS_QUOTER_MODE", "send-everything")
	var logs bytes.Buffer
	err := run(slog.New(slog.NewJSONHandler(&logs, nil)))
	if err == nil || !strings.Contains(err.Error(), "MODE must be dry or live") {
		t.Fatalf("configuration error = %v", err)
	}
	for _, s := range []string{"password", "private-token", "secret", "account-key"} {
		if strings.Contains(logs.String(), s) || strings.Contains(err.Error(), s) {
			t.Fatalf("startup discloses credential %q", s)
		}
	}
}
