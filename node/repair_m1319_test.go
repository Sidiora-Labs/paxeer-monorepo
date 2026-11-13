package app_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"testing"
)

func TestRepairM_1319_RestoredBoundaryManifest(t *testing.T) {
	const manifest = "../services/platform/hosted/paxeer/deployment.yaml"
	// Commit 9e17ec8e4d (task 18.1790.1) produced the pinned manifest by
	// restoring network-policy connectivity between the gateway and its hosted peers.
	// A task changing the manifest re-pins this digest in the same commit.
	const want = "ba84aff353486b744f3b9759fb158219b9de90558366485b483e4793c60dc9a2"
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
		t.Fatalf("boundary manifest differs from task 18.1790.1 network-policy restoration: SHA-256 %s, want %s", got, want)
	}
}
