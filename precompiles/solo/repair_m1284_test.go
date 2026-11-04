package solo_test

import (
	"compress/gzip"
	"fmt"
	"io"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/solo"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	wasmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

type repair1284Harness struct {
	ctx        sdk.Context
	key        cryptotypes.PrivKey
	sender     sdk.AccAddress
	caller     common.Address
	recipient  sdk.AccAddress
	precompile *pcommon.DynamicGasPrecompile
	wasm       *wasmkeeper.PermissionedKeeper
}

func newRepair1284Harness(t *testing.T) *repair1284Harness {
	t.Helper()
	app := testkeeper.EVMTestApp
	ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
	ctx = ctx.WithChainID("pax-test").WithBlockTime(time.Unix(1700000000, 0)).
		WithIsEVM(true).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	key := secp256k1.GenPrivKeyFromSecret([]byte("repair-m1284-sender"))
	sender := sdk.AccAddress(key.PubKey().Address())
	caller := common.HexToAddress("0x1284128412841284128412841284128412841284")
	recipient := app.EvmKeeper.GetPaxAddressOrDefault(ctx, caller)
	app.AccountKeeper.SetAccount(ctx, authtypes.NewBaseAccount(sender, key.PubKey(), 10, 0))
	app.AccountKeeper.SetAccount(ctx, authtypes.NewBaseAccount(recipient, nil, 11, 0))
	precompile, err := solo.NewPrecompile(app.GetPrecompileKeepers())
	require.NoError(t, err)
	return &repair1284Harness{ctx, key, sender, caller, recipient, precompile,
		wasmkeeper.NewDefaultPermissionKeeper(app.WasmKeeper)}
}

func (h *repair1284Harness) nft(t *testing.T, fixture string, count int) sdk.AccAddress {
	t.Helper()
	var code []byte
	var err error
	if fixture == "base" {
		code, err = os.ReadFile("../../contracts/wasm/cw721_base.wasm")
		require.NoError(t, err)
	} else {
		file, err := os.Open("testdata/cw721_" + fixture + ".wasm.gz")
		require.NoError(t, err)
		defer file.Close()
		reader, err := gzip.NewReader(file)
		require.NoError(t, err)
		defer reader.Close()
		code, err = io.ReadAll(reader)
		require.NoError(t, err)
	}
	codeID, err := h.wasm.Create(h.ctx, h.sender, code, nil)
	require.NoError(t, err)
	init := []byte(fmt.Sprintf(`{"name":"pagination","symbol":"PAGE","minter":%q}`, h.sender.String()))
	contract, _, err := h.wasm.Instantiate(h.ctx, codeID, h.sender, h.sender, init, "pagination", sdk.NewCoins())
	require.NoError(t, err)
	for i := 0; i < count; i++ {
		mint := []byte(fmt.Sprintf(`{"mint":{"token_id":"%04d","owner":%q}}`, i, h.sender.String()))
		_, err := h.wasm.Execute(h.ctx, contract, h.sender, mint, sdk.NewCoins())
		require.NoError(t, err)
	}
	return contract
}

func (h *repair1284Harness) payload(t *testing.T, assets ...*evmtypes.Asset) []byte {
	t.Helper()
	account := testkeeper.EVMTestApp.AccountKeeper.GetAccount(h.ctx, h.sender)
	return signClaimMsg(t, evmtypes.NewMsgClaimSpecific(h.sender, h.caller, assets...),
		h.sender, h.caller, account, h.key)
}

func (h *repair1284Harness) run(t *testing.T, payload []byte) ([]byte, uint64, error) {
	t.Helper()
	input, err := h.precompile.GetABI().Pack(solo.ClaimSpecificMethod, payload)
	require.NoError(t, err)
	db := state.NewDBImpl(h.ctx, &testkeeper.EVMTestApp.EvmKeeper, false)
	ret, remaining, err := h.precompile.RunAndCalculateGas(&vm.EVM{StateDB: db}, h.caller, h.caller,
		input, 100_000_000, big.NewInt(0), nil, false, false)
	h.ctx = db.Ctx().WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	if err != nil {
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		require.Error(t, db.GetPrecompileError())
		return ret, remaining, db.GetPrecompileError()
	}
	require.NoError(t, db.GetPrecompileError())
	want, err := h.precompile.GetABI().Methods[solo.ClaimSpecificMethod].Outputs.Pack(true)
	require.NoError(t, err)
	require.Equal(t, want, ret)
	require.Greater(t, remaining, uint64(0))
	return ret, remaining, nil
}

func repair1284NFTAsset(contract sdk.AccAddress, token string) *evmtypes.Asset {
	return &evmtypes.Asset{AssetType: evmtypes.AssetType_TYPECW721, ContractAddress: contract.String(), Denom: token}
}

func (h *repair1284Harness) owners(t *testing.T, contract sdk.AccAddress, count int, owner sdk.AccAddress) {
	t.Helper()
	for i := 0; i < count; i++ {
		require.Equal(t, owner.String(), queryCW721Owner(h.ctx, testkeeper.EVMTestApp.WasmKeeper,
			contract, fmt.Sprintf("%04d", i)), "token %04d", i)
	}
}

func (h *repair1284Harness) page(t *testing.T, contract sdk.AccAddress, cursor string) []string {
	t.Helper()
	response, err := testkeeper.EVMTestApp.WasmKeeper.QuerySmartSafe(h.ctx, contract,
		solo.CW721TokensQueryPayload(h.sender, cursor))
	require.NoError(t, err)
	tokens, err := solo.ParseCW721TokensQueryResponse(response)
	require.NoError(t, err)
	return tokens
}

func TestRepairM_1284_RepeatedPageAtomic(t *testing.T) {
	h := newRepair1284Harness(t)
	repeated := h.nft(t, "repeat", 15)
	honest := h.nft(t, "base", 2)
	cw20 := setupCW20Contract(h.ctx, h.key, *h.wasm)
	bank := testkeeper.EVMTestApp.BankKeeper
	coins := sdk.NewCoins(sdk.NewInt64Coin("repair1284", 7))
	require.NoError(t, bank.AddCoins(h.ctx, h.sender, coins, false))
	first := h.page(t, repeated, "")
	require.Len(t, first, 10)
	require.Equal(t, first, h.page(t, repeated, first[len(first)-1]), "real contract repeats its first page")
	payload := h.payload(t,
		&evmtypes.Asset{AssetType: evmtypes.AssetType_TYPENATIVE, Denom: "repair1284"},
		&evmtypes.Asset{AssetType: evmtypes.AssetType_TYPECW20, ContractAddress: cw20.String()},
		repair1284NFTAsset(honest, "0000"), repair1284NFTAsset(repeated, ""))
	var refusal string
	for attempt := 0; attempt < 2; attempt++ {
		ret, remaining, err := h.run(t, payload)
		require.EqualError(t, err, `CW721 claim-all contains duplicate token id "0000"`)
		if attempt == 0 {
			refusal = err.Error()
		} else {
			require.Equal(t, refusal, err.Error())
		}
		require.Empty(t, ret)
		require.Zero(t, remaining)
		require.Equal(t, coins, bank.GetAllBalances(h.ctx, h.sender))
		require.Empty(t, bank.GetAllBalances(h.ctx, h.recipient))
		require.Equal(t, sdk.NewInt(1000000000), queryCW20Balance(h.ctx, testkeeper.EVMTestApp.WasmKeeper, cw20, h.sender))
		require.Equal(t, sdk.ZeroInt(), queryCW20Balance(h.ctx, testkeeper.EVMTestApp.WasmKeeper, cw20, h.recipient))
		h.owners(t, honest, 2, h.sender)
		h.owners(t, repeated, 15, h.sender)
		require.Zero(t, testkeeper.EVMTestApp.AccountKeeper.GetAccount(h.ctx, h.sender).GetSequence())
		require.Empty(t, h.ctx.EventManager().Events())
	}
}

func TestRepairM_1284_MalformedPagination(t *testing.T) {
	for _, tc := range []struct{ fixture, refusal string }{
		{"duplicate", `CW721 claim-all contains duplicate token id "0000"`},
		{"backwards", `CW721 claim-all has nonadvancing token id "0000" after "0001"`},
		{"empty_id", `CW721 claim-all has nonadvancing token id "" after ""`},
		{"oversized", fmt.Sprintf("CW721 claim-all exceeds %d tokens; use claimSpecific with a token denom", solo.MaxCW721ClaimTokens)},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			h := newRepair1284Harness(t)
			count := 2
			if tc.fixture == "oversized" {
				count = solo.MaxCW721ClaimTokens + 1
			}
			contract := h.nft(t, tc.fixture, count)
			_, _, err := h.run(t, h.payload(t, repair1284NFTAsset(contract, "")))
			require.EqualError(t, err, tc.refusal)
			h.owners(t, contract, count, h.sender)
			require.Zero(t, testkeeper.EVMTestApp.AccountKeeper.GetAccount(h.ctx, h.sender).GetSequence())
		})
	}
}

func TestRepairM_1284_TokenBoundAndSpecificRoute(t *testing.T) {
	h := newRepair1284Harness(t)
	count := solo.MaxCW721ClaimTokens + 1
	contract := h.nft(t, "base", count)
	_, _, err := h.run(t, h.payload(t, repair1284NFTAsset(contract, "")))
	require.EqualError(t, err, fmt.Sprintf("CW721 claim-all exceeds %d tokens; use claimSpecific with a token denom", solo.MaxCW721ClaimTokens))
	h.owners(t, contract, count, h.sender)
	require.Zero(t, testkeeper.EVMTestApp.AccountKeeper.GetAccount(h.ctx, h.sender).GetSequence())
	token := fmt.Sprintf("%04d", count-1)
	payload := h.payload(t, repair1284NFTAsset(contract, token))
	_, _, err = h.run(t, payload)
	require.NoError(t, err)
	h.owners(t, contract, count-1, h.sender)
	require.Equal(t, h.recipient.String(), queryCW721Owner(h.ctx, testkeeper.EVMTestApp.WasmKeeper, contract, token))
	require.Equal(t, uint64(1), testkeeper.EVMTestApp.AccountKeeper.GetAccount(h.ctx, h.sender).GetSequence())
	_, _, err = h.run(t, payload)
	require.ErrorContains(t, err, "account sequence mismatch")
}

func TestRepairM_1284_PageBound(t *testing.T) {
	h := newRepair1284Harness(t)
	contract := h.nft(t, "single", solo.MaxCW721ClaimPages)
	require.Equal(t, []string{"0000"}, h.page(t, contract, ""))
	require.Equal(t, []string{"0001"}, h.page(t, contract, "0000"))
	_, _, err := h.run(t, h.payload(t, repair1284NFTAsset(contract, "")))
	require.EqualError(t, err, fmt.Sprintf("CW721 claim-all exceeds %d query pages; use claimSpecific with a token denom", solo.MaxCW721ClaimPages))
	h.owners(t, contract, solo.MaxCW721ClaimPages, h.sender)
	require.Zero(t, testkeeper.EVMTestApp.AccountKeeper.GetAccount(h.ctx, h.sender).GetSequence())
	_, _, err = h.run(t, h.payload(t, repair1284NFTAsset(contract, "0000")))
	require.NoError(t, err)
	require.Equal(t, h.recipient.String(), queryCW721Owner(h.ctx, testkeeper.EVMTestApp.WasmKeeper, contract, "0000"))
}

func TestRepairM_1284_FiniteEnumeration(t *testing.T) {
	for _, tc := range []struct {
		name, fixture string
		count         int
	}{
		{"empty", "base", 0},
		{"multiple_pages", "base", 15},
		{"exact_token_bound", "base", solo.MaxCW721ClaimTokens},
		{"terminal_page_at_bound", "single", solo.MaxCW721ClaimPages - 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newRepair1284Harness(t)
			contract := h.nft(t, tc.fixture, tc.count)
			_, _, err := h.run(t, h.payload(t, repair1284NFTAsset(contract, "")))
			require.NoError(t, err)
			h.owners(t, contract, tc.count, h.recipient)
			require.Empty(t, h.page(t, contract, ""))
			require.Equal(t, uint64(1), testkeeper.EVMTestApp.AccountKeeper.GetAccount(h.ctx, h.sender).GetSequence())
			if tc.count > 0 {
				require.NotEmpty(t, h.ctx.EventManager().Events(), "successful transfers publish their real events")
			}
		})
	}
}
