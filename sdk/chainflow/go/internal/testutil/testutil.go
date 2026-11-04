package testutil

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

const testServerProbeTimeout = 2 * time.Second

func CheckTestServer(t *testing.T, url string) bool {
	t.Helper()
	if err := probeTestServer(url); err != nil {
		t.Errorf("Test server probe failed: %s", err)
		return false
	}
	return true
}

func probeTestServer(url string) error {
	client := &http.Client{Timeout: testServerProbeTimeout}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %q: %w", url, err)
	}
	if err := response.Body.Close(); err != nil {
		return fmt.Errorf("close response from %q: %w", url, err)
	}
	return nil
}
