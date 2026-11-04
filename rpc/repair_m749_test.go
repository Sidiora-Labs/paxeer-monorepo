package evmrpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

// accountless-old-host is proved by TestRepairM_749_ExplicitEndpoint,
// TestRepairM_749_MainnetBetaRoots and TestRepairM_749_LocalKeylessTransport.
// Wiring request (domain 16): eth/eth_accounts.spec.ts must replace its
// unreachable-node skip with required fixture readiness and chain-ID verification.
// Baseline verification was not run: the ordered task gates forbid a pre-change run.

func repairM749Runtime(t *testing.T, environment map[string]string) *goja.Runtime {
	t.Helper()
	source, err := os.ReadFile("../tests/integration/rpc_tests/config/endpoints.ts")
	require.NoError(t, err)
	// Erase only the module exports and TypeScript type annotations. The
	// configuration's executable statements run unchanged in the JS engine.
	javascript := strings.NewReplacer("export ", "", ": string", "", " as const", "").Replace(string(source))
	vm := goja.New()
	require.NoError(t, vm.Set("process", map[string]interface{}{"env": environment}))
	_, err = vm.RunScript("config/endpoints.ts", javascript)
	require.NoError(t, err)
	return vm
}

func TestRepairM_749_ExplicitEndpoint(t *testing.T) {
	for _, value := range []string{
		"", " ", "\n", "https://evm-rpc.pax-apis.com", "https://evm-rpc.pax-apis.com/",
		"http://api1.mainnet-beta.paxeer.network", "https://api15.mainnet-beta.paxeer.network",
		"https://api16.mainnet-beta.paxeer.network", "https://api0.mainnet-beta.paxeer.network",
		"https://api01.mainnet-beta.paxeer.network", "https://api1.paxeer.network",
		"https://api1.mainnet-beta.paxeer.network/evm", "https://api-mainnet-beta.paxeer.network/rpc",
		"https://api1.mainnet-beta.paxeer.network?key=secret", "https://api1.mainnet-beta.paxeer.network#fragment",
		"https://user:secret@api1.mainnet-beta.paxeer.network", "https://api1.mainnet-beta.paxeer.network.evil.test",
		"https://api1.mainnet-beta.paxeer.network:8443", "ws://127.0.0.1:8546",
		"http://127.0.0.1:0", "http://127.0.0.1:65536", "http://localhost:999999",
		"http://127.0.0.1:8545/rpc", "http://localhost.evil.test:8545", "http://192.0.2.1:8545",
		"http://127.0.0.1:8545/\n", " https://api1.mainnet-beta.paxeer.network",
	} {
		t.Run(fmt.Sprintf("reject_%q", value), func(t *testing.T) {
			vm := repairM749Runtime(t, map[string]string{"RPC_ACCOUNTLESS": value})
			_, err := vm.RunString("Endpoints.accountless")
			require.ErrorContains(t, err, "RPC_ACCOUNTLESS")
		})
	}
	vm := repairM749Runtime(t, map[string]string{})
	_, err := vm.RunString("Endpoints.accountless")
	require.ErrorContains(t, err, "verified keyless")
	// Accountless readiness must not change unrelated endpoint defaults or
	// force the bootstrap to contact an endpoint while importing this module.
	for expression, expected := range map[string]string{
		"Endpoints.pax.evmRpc":    "http://localhost:8545",
		"Endpoints.pax.evmWs":     "ws://localhost:8546",
		"Endpoints.pax.cosmosRpc": "http://localhost:26657",
		"Endpoints.pax.rest":      "http://localhost:1317",
		"Endpoints.eth.geth":      "http://127.0.0.1:9547",
		"Endpoints.eth.fork":      "http://127.0.0.1:9546",
		"AdminMnemonic":           "",
		"RuntimeStatePath":        "runtime/runtime.json",
	} {
		result, err := vm.RunString(expression)
		require.NoError(t, err)
		require.Equal(t, expected, result.String(), expression)
	}
}

func TestRepairM_749_MainnetBetaRoots(t *testing.T) {
	var endpoints []string
	for i := 1; i <= 14; i++ {
		endpoints = append(endpoints, fmt.Sprintf("https://api%d.mainnet-beta.paxeer.network", i))
	}
	endpoints = append(endpoints, "https://api-mainnet-beta.paxeer.network",
		"http://localhost:8545", "http://127.0.0.1:1", "https://127.0.0.1:65535", "http://[::1]:8545")
	for _, endpoint := range endpoints {
		for _, suffix := range []string{"", "/"} {
			value := endpoint + suffix
			t.Run(value, func(t *testing.T) {
				vm := repairM749Runtime(t, map[string]string{"RPC_ACCOUNTLESS": value})
				result, err := vm.RunString("Endpoints.accountless")
				require.NoError(t, err)
				require.Equal(t, value, result.String(), "preserve the operator's explicit endpoint")
			})
		}
	}
}

func TestRepairM_749_LocalKeylessTransport(t *testing.T) {
	// This is an isolated, real geth account manager with no wallet backends.
	// Register its production Accounts method as the local keyless fixture;
	// the RPC server produces the JSON envelope and empty-array encoding.
	manager := accounts.NewManager(nil)
	t.Cleanup(func() { require.NoError(t, manager.Close()) })
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", manager))
	fixture := httptest.NewServer(server)
	t.Cleanup(fixture.Close)

	vm := repairM749Runtime(t, map[string]string{"RPC_ACCOUNTLESS": fixture.URL})
	selected, err := vm.RunString("Endpoints.accountless")
	require.NoError(t, err)
	require.Equal(t, fixture.URL, selected.String())
	client, err := rpc.DialHTTP(selected.String())
	require.NoError(t, err)
	t.Cleanup(client.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for range 2 {
		var raw json.RawMessage
		require.NoError(t, client.CallContext(ctx, &raw, "eth_accounts"))
		require.JSONEq(t, "[]", string(raw), "keyless fixture must return an array, never null")
		var decoded []common.Address
		require.NoError(t, json.Unmarshal(raw, &decoded))
		require.NotNil(t, decoded)
		require.Empty(t, decoded)
	}
}
