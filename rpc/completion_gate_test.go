package evmrpc_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRpcCompletionRetiredSurfacesAbsent(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("locate repository root: %v", err)
	}

	t.Run("retired/api/buf.lock", func(t *testing.T) {
		path := "api/buf.lock"
		_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(path)))
		if err == nil {
			t.Fatalf("retired interface interface.7.fmt-api-buf-lock-j39 remains at %s", path)
		}
		if !os.IsNotExist(err) {
			t.Fatalf("check retired surface %s: %v", path, err)
		}
	})

	for _, path := range []string{
		"rpc/ethclient_pending_test.go",
		"rpc/error_codes_test.go",
		"api/buf_apidir_test.go",
		"api/gateway_mount_test.go",
		"api/buf_generate_test.go",
	} {
		t.Run("planned/"+path, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(path)))
			if err != nil {
				t.Fatalf("required planned path %s: %v", path, err)
			}
			if !info.Mode().IsRegular() {
				t.Fatalf("required planned path %s is not a regular file", path)
			}
		})
	}
}
