package abi_test

import (
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	spotabi "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/spot/abi"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func repair714Read(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("../../..", path))
	require.NoError(t, err, path)
	return string(raw)
}

// Follow every registry reference to these normative pages, including fragments.
// Removing a page or its public anchor must fail even if another page still loads.
func TestRepairM_714_RegistryContracts(t *testing.T) {
	registry := repair714Read(t, "spec/layerx-merge/interfaces.kvx")
	references := regexp.MustCompile(`(?m)^contract\s*=\s*"(contracts/(?:markets|perps-data|perps-client)\.md)(?:#([^"]+))?"`).FindAllStringSubmatch(registry, -1)
	require.NotEmpty(t, references)
	seen := map[string]bool{}
	for _, ref := range references {
		page := repair714Read(t, ref[1])
		require.NotEmpty(t, page)
		seen[ref[1]] = true
		if ref[2] == "" {
			continue
		}
		anchors := map[string]bool{}
		for _, match := range regexp.MustCompile(`<a id="([^"]+)"`).FindAllStringSubmatch(page, -1) {
			anchors[match[1]] = true
		}
		for _, match := range regexp.MustCompile(`(?m)^#{1,6} (.+)$`).FindAllStringSubmatch(page, -1) {
			slug := strings.ToLower(match[1])
			slug = regexp.MustCompile(`[^\p{L}\p{N}_ -]`).ReplaceAllString(slug, "")
			anchors[strings.ReplaceAll(slug, " ", "-")] = true
		}
		require.True(t, anchors[ref[2]], "broken normative contract: %s#%s", ref[1], ref[2])
	}
	require.Equal(t, map[string]bool{
		"contracts/markets.md": true, "contracts/perps-data.md": true, "contracts/perps-client.md": true,
	}, seen)
}

// The existing fixture opens real chain stores and deploys the keeper's native
// pointer into the production EVM. Decode its actual return/log/error bytes with
// the public ABI consumed by indexers, rather than constructing response fixtures.
func TestRepairM_714_NativeShareContract(t *testing.T) {
	e := newRepair19Env(t)
	markets := repair714Read(t, "contracts/markets.md")
	data := repair714Read(t, "contracts/perps-data.md")
	require.Contains(t, markets, "NativePaxTokensERC20")
	require.Contains(t, markets, "share-deployment.kvx")
	require.Contains(t, data, "deployed `Share()` ABI")
	require.Contains(t, data, "perps.spot_token_registry")
	require.Contains(t, data, "share_lookup")
	for name := range e.contract.Methods {
		require.Contains(t, markets, "`"+name+"`", "undocumented native method")
	}
	token := repair19Address(t.Name() + " underlying")
	require.Equal(t, "spot/share/"+strings.ToLower(token.Hex()), e.denom)
	pointer, _, exists := e.app.EvmKeeper.GetERC20NativePointer(e.ctx, e.denom)
	require.True(t, exists)
	require.Equal(t, e.pointer, pointer)
	_, _, missing := e.app.EvmKeeper.GetERC20NativePointer(e.ctx, e.keeper.ShareDenom(repair19Address(t.Name()+" unknown")))
	require.False(t, missing)
	require.Equal(t, "Spot Vault Share USDL", e.view("name"))
	require.Equal(t, "sUSDL", e.view("symbol"))
	require.Equal(t, uint8(6), e.view("decimals"))
	require.Equal(t, e.denom, e.view("denom"))
	e.requireTransferLog(e.mint(100), common.Address{}, e.holder, 100)
	receiver := repair19Address(t.Name() + " receiver")
	ret, err := e.call(e.contract, e.holder, "transfer", receiver, big.NewInt(30))
	require.NoError(t, err)
	values, err := e.contract.Unpack("transfer", ret)
	require.NoError(t, err)
	require.Equal(t, []interface{}{true}, values)
	require.Equal(t, big.NewInt(70), e.view("balanceOf", e.holder))
	require.Equal(t, big.NewInt(30), e.view("balanceOf", receiver))
	ret, err = e.call(e.contract, e.holder, "transfer", receiver, big.NewInt(71))
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	insufficient := e.contract.Errors["ERC20InsufficientBalance"]
	require.Equal(t, "e450d38c", common.Bytes2Hex(insufficient.ID[:4]))
	require.Contains(t, markets, "`0xe450d38c`")
	decoded, err := insufficient.Unpack(ret)
	require.NoError(t, err)
	require.Equal(t, []interface{}{e.holder, big.NewInt(70), big.NewInt(71)}, decoded)
	require.Equal(t, big.NewInt(70), e.view("balanceOf", e.holder))
	require.Equal(t, big.NewInt(30), e.view("balanceOf", receiver))
	require.Equal(t, big.NewInt(100), e.view("totalSupply"))

	historical, err := spotabi.HistoricalSolidityShare()
	require.NoError(t, err)
	for _, call := range []struct {
		method string
		args   []interface{}
	}{
		{"mint", []interface{}{e.holder, big.NewInt(1)}},
		{"burn", []interface{}{e.holder, big.NewInt(1)}},
		{"owner", nil}, {"renounceOwnership", nil},
		{"transferOwnership", []interface{}{receiver}},
	} {
		require.Contains(t, markets, historical.Methods[call.method].Sig)
		require.NotContains(t, e.contract.Methods, call.method)
		for _, caller := range []common.Address{e.holder, common.HexToAddress(types.PECORVaultAddress), types.GovernanceEVMAddress()} {
			ret, err := e.call(historical, caller, call.method, call.args...)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			require.Empty(t, ret)
		}
	}
	require.Len(t, e.contract.Events, 2)
	require.Contains(t, e.contract.Events, "Transfer")
	require.Contains(t, e.contract.Events, "Approval")
	logs := e.run(func(ctx sdk.Context, evm *vm.EVM) error {
		return e.keeper.BurnShares(ctx, evm, e.denom, e.holder, sdk.NewInt(20))
	})
	e.requireTransferLog(logs, e.holder, common.Address{}, 20)
	require.Equal(t, big.NewInt(50), e.view("balanceOf", e.holder))
	require.Equal(t, big.NewInt(80), e.view("totalSupply"))
	require.Equal(t, big.NewInt(50), e.app.BankKeeper.GetBalance(e.ctx,
		e.app.EvmKeeper.GetPaxAddressOrDefault(e.ctx, e.holder), e.denom).Amount.BigInt())
}

// Hook execution belongs to the client gate. Here pin the published return
// descriptions to the committed consumers without invoking a JS toolchain.
func TestRepairM_714_HookContracts(t *testing.T) {
	page := repair714Read(t, "contracts/perps-client.md")
	for _, pair := range []struct {
		path          string
		source        string
		documentation string
	}{
		{"hooks/perps/usePerpsMarkets.ts", "export function usePerpsMarkets()", "`{ catalog, tradeable, isLoading, error, refetch }`"},
		{"hooks/perps/usePerpsMarkets.ts", "return { catalog, tradeable };", "`loadPerpsMarkets()` returns `{ catalog, tradeable }`"},
		{"hooks/perps/usePerpsMarkets.ts", "const data = query.isError ? undefined : query.data;", "A failed query\nexposes `error` and empty arrays"},
		{"hooks/perps/useFeeTier.ts", "view: query.isError ? undefined : query.data,", "`{ view, isLoading, error }`"},
		{"hooks/perps/useMarketConfig.ts", "return { config, tiers };", "`{ config, tiers }`"},
		{"hooks/spot/useSpotTokenState.ts", "state: query.isError ? null : (query.data ?? null),", "`{ state: SpotTokenState | null, error: string | null }`"},
	} {
		require.Contains(t, repair714Read(t, "apps/perps/src/"+pair.path), pair.source)
		require.Contains(t, page, pair.documentation)
		require.Contains(t, page, "../apps/perps/src/"+pair.path)
	}
}
