package config

import (
	"strings"
	"testing"
)

func TestRuntimeValidationRequiresLifecycleBindingAndHTTPArchive(t *testing.T) {
	setBaseEnv(t)
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	c.NodeEnv = "production"
	if err = c.Validate(); err == nil || !strings.Contains(err.Error(), "lifecycle document binding") {
		t.Fatalf("production lifecycle admission: %v", err)
	}
	c.NodeEnv = "development"
	for _, endpoint := range []string{"https://archive.example/rpc", "ftp://archive.example/rpc"} {
		c.HistoricalRPCURL = endpoint
		err = c.Validate()
		if strings.HasPrefix(endpoint, "https:") {
			if err != nil {
				t.Fatalf("HTTP archive: %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "INDEXER_HISTORICAL_RPC_URL") {
			t.Fatalf("archive URL admission: %v", err)
		}
	}
}
