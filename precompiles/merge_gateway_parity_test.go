package precompiles_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

const UNSERVED_PRECOMPILES = `(?s)export const UNSERVED_PRECOMPILES\b[^=]*=\s*new Set(?:<[^>]*>)?\(\s*(?:\[(.*?)\])?\s*\);`

type gatewayParityChecks struct{}

func (gatewayParityChecks) MissingMergeConstructors() map[common.Address]struct{} {
	missing := make(map[common.Address]struct{})
	for _, entry := range precompiles.MissingMergeConstructors() {
		missing[entry.Address] = struct{}{}
	}
	return missing
}

func TestGatewayUnservedMatchesMissingConstructors(t *testing.T) {
	source, err := os.ReadFile("../services/wallet/gateway/src/precompiles.ts")
	require.NoError(t, err)

	addressTables := regexp.MustCompile(`(?s)export const PRECOMPILE_ADDRESSES\s*=\s*\{(.*?)\}\s*as const`).FindAllStringSubmatch(string(source), -1)
	require.Len(t, addressTables, 1, "expected one gateway address table")
	addressPattern := regexp.MustCompile(`(?m)^\s+(\w+): '(0x[0-9a-fA-F]{40})',\s*$`)
	routes := make(map[string]common.Address)
	for _, match := range addressPattern.FindAllStringSubmatch(addressTables[0][1], -1) {
		require.NotContains(t, routes, match[1], "duplicate gateway route")
		routes[match[1]] = common.HexToAddress(match[2])
	}
	require.NotEmpty(t, routes, "expected gateway precompile addresses")
	require.Empty(t, strings.TrimSpace(addressPattern.ReplaceAllString(addressTables[0][1], "")), "unsupported gateway address entry")

	unservedDeclarations := regexp.MustCompile(UNSERVED_PRECOMPILES).FindAllStringSubmatch(string(source), -1)
	require.Len(t, unservedDeclarations, 1, "expected one literal gateway unserved set")
	unserved := make(map[common.Address]struct{})
	if body := strings.TrimSpace(unservedDeclarations[0][1]); body != "" {
		keys := strings.Split(body, ",")
		keyPattern := regexp.MustCompile(`^\s*(?:'(\w+)'|"(\w+)")\s*$`)
		for i, key := range keys {
			if i == len(keys)-1 && strings.TrimSpace(key) == "" {
				continue
			}
			match := keyPattern.FindStringSubmatch(key)
			require.Len(t, match, 3, "unsupported gateway unserved entry: %q", key)
			name := match[1] + match[2]
			address, found := routes[name]
			require.True(t, found, "unserved gateway key %q has no precompile address", name)
			unserved[address] = struct{}{}
		}
	}

	require.Equal(t, gatewayParityChecks{}.MissingMergeConstructors(), unserved, "gateway unserved addresses must match missing constructors after node registration")
}
