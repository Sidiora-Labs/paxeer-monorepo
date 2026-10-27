package keeper_test

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVaultAttestationDocCarriesTheModuleVectors(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	moduleDoc, err := os.ReadFile(filepath.Join(root, "modules", "layerxbridge", "ATTESTATION.md"))
	require.NoError(t, err)
	vaultDoc, err := os.ReadFile(filepath.Join(root, "contracts", "bridge", "evm", "ATTESTATION.md"))
	require.NoError(t, err)
	moduleText, vaultText := string(moduleDoc), string(vaultDoc)

	vectors := regexp.MustCompile(`\b0x[0-9a-fA-F]{64}\b`).FindAllString(moduleText, -1)
	require.NotEmpty(t, vectors, "module attestation document must contain 32-byte vectors")
	for _, vector := range vectors {
		assert.Contains(t, vaultText, vector, "vault attestation document must carry module vector %s verbatim", vector)
	}

	domains := regexp.MustCompile("\\b(?:Domain|ASCII)\\s+`([^`]+)`").FindAllStringSubmatch(moduleText, -1)
	require.NotEmpty(t, domains, "module attestation document must contain domain strings")
	for _, domain := range domains {
		assert.Contains(t, vaultText, domain[1], "vault attestation document must carry module domain %s verbatim", domain[1])
	}
}
