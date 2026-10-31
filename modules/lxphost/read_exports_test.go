//go:build cgo

package lxphost_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/exportcheck"
)

func TestReadBridgeOmitsIdempotentReceipt(t *testing.T) {
	const importPath = "github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	exports, err := exportcheck.PackageExports(importPath)
	if err != nil {
		t.Fatalf("PackageExports(%q): %v", importPath, err)
	}
	if !exports["Host.ReadBalance"] {
		t.Error("compiled lxphost package is missing Host.ReadBalance")
	}
	if exports["Host.ReadIdempotentReceipt"] {
		t.Error("compiled lxphost package still exports Host.ReadIdempotentReceipt")
	}
}

func TestReadBridgeOmitsPayerGrant(t *testing.T) {
	const importPath = "github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	exports, err := exportcheck.PackageExports(importPath)
	if err != nil {
		t.Fatalf("PackageExports(%q): %v", importPath, err)
	}
	if !exports["Host.ReadBalance"] {
		t.Error("compiled lxphost package is missing Host.ReadBalance")
	}
	if exports["Host.ReadPayerGrant"] {
		t.Error("compiled lxphost package still exports Host.ReadPayerGrant")
	}
	if exports["PayerGrantState"] {
		t.Error("compiled lxphost package still exports PayerGrantState")
	}
}
