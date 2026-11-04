package app_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

func TestRepairM_1319_RestoredBoundaryManifest(t *testing.T) {
	const manifest = "../services/platform/hosted/paxeer/deployment.yaml"
	const want = "7f681de19c1c05401b1b0dfb98fb4ee3611e6e22f3bd3815b0fb1877108d9066"
	info, err := os.Lstat(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !info.Mode().IsRegular() {
		t.Fatalf("restored boundary manifest is not a regular file: %v", info.Mode())
	}
	raw, err := os.ReadFile(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != want {
		t.Fatalf("boundary manifest differs from decision 2533 restoration: SHA-256 %s, want %s", got, want)
	}
}
