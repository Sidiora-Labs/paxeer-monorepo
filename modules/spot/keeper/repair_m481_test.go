package keeper_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/artifacts"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestRepairM_481_MessageServerListingLifecycle(t *testing.T) {
	testUpdateParamsRegistersNewToken(t)
}

func TestRepairM_481_GovernanceListingLifecycle(t *testing.T) {
	e := newRepair17Env(t)
	vault := common.HexToAddress(types.PECORVaultAddress)
	tokens := []common.Address{e.stable, e.native}
	for _, token := range tokens {
		_, _, exists := e.a.EvmKeeper.GetERC20NativePointer(e.ctx, e.a.SpotKeeper.ShareDenom(token))
		require.False(t, exists, "the fixture must not provision pool shares")
	}
	before := len(e.ctx.EVMEventManager().Events())
	require.NoError(t, e.govern(e.listing()))
	require.Equal(t, tokens, e.view(vault, e.vault, "getRegisteredTokens")[0])
	for _, token := range tokens {
		pointer, _, exists := e.a.EvmKeeper.GetERC20NativePointer(e.ctx, e.a.SpotKeeper.ShareDenom(token))
		require.True(t, exists)
		created := 0
		for _, log := range logsSince(e.ctx, before) {
			if len(log.Topics) > 1 && log.Topics[0] == e.vault.Events["ShareTokenCreated"].ID &&
				log.Topics[1] == common.BytesToHash(token.Bytes()) {
				requireLog(t, log, vault, []common.Hash{e.vault.Events["ShareTokenCreated"].ID,
					common.BytesToHash(token.Bytes()), common.BytesToHash(pointer.Bytes())})
				created++
			}
		}
		require.Equal(t, 1, created)
	}
	repairM481PoolLifecycle(e, tokens)
}

// Consume only the governance-created pointers, through the production vault
// and ERC20 ABI, then close and reopen every application store before redeeming.
func repairM481PoolLifecycle(e *repair17Env, tokens []common.Address) {
	t := e.t
	t.Helper()
	vault := common.HexToAddress(types.PECORVaultAddress)
	shareABI := *artifacts.GetParsedABI("native")
	const deposit, redeem = int64(100_000_000_000), int64(10_000_000_000)
	pointers := make(map[common.Address]common.Address)
	balances := make(map[common.Address]*big.Int)
	underlyingBalance := func(token common.Address) *big.Int {
		t.Helper()
		return e.view(token, shareABI, "balanceOf", e.user)[0].(*big.Int)
	}
	assertPool := func(token common.Address, reserves, shares int64) {
		t.Helper()
		denom := e.a.SpotKeeper.ShareDenom(token)
		pointer, _, exists := e.a.EvmKeeper.GetERC20NativePointer(e.ctx, denom)
		require.True(t, exists)
		require.Equal(t, pointers[token], pointer)
		require.NotEqual(t, common.Address{}, pointer)
		require.NotEmpty(t, e.a.EvmKeeper.GetCode(e.ctx, pointer))
		require.Equal(t, pointer, e.view(vault, e.vault, "shareTokens", token)[0])
		require.Zero(t, big.NewInt(shares).Cmp(e.view(pointer, shareABI, "balanceOf", e.user)[0].(*big.Int)))
		require.Equal(t, sdk.NewInt(shares), e.a.BankKeeper.GetBalance(e.ctx,
			e.a.EvmKeeper.GetPaxAddressOrDefault(e.ctx, e.user), denom).Amount)
		locked := int64(0)
		if reserves > 0 {
			locked = keeper.MinimumShares
		}
		require.Zero(t, big.NewInt(locked).Cmp(e.view(pointer, shareABI, "balanceOf",
			e.a.SpotKeeper.CustodyEVMAddress(e.ctx))[0].(*big.Int)))
		require.Zero(t, big.NewInt(shares+locked).Cmp(e.view(pointer, shareABI, "totalSupply")[0].(*big.Int)))
		require.Equal(t, sdk.NewInt(shares+locked), e.a.BankKeeper.GetSupply(e.ctx, denom).Amount)
		book, err := e.a.SpotKeeper.GetBook(e.ctx, token)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(reserves), book.Reserves)
		require.Equal(t, reserves > 0, book.MinimumSharesLocked)
		_, custody, err := e.a.SpotKeeper.CustodyBalance(e.ctx, token)
		require.NoError(t, err)
		require.Zero(t, big.NewInt(reserves).Cmp(custody.BigInt()), "custody must equal the exact reserve amount")
	}
	for _, token := range tokens {
		denom := e.a.SpotKeeper.ShareDenom(token)
		pointer, _, exists := e.a.EvmKeeper.GetERC20NativePointer(e.ctx, denom)
		require.True(t, exists)
		pointers[token] = pointer
		metadata, found := e.a.BankKeeper.GetDenomMetaData(e.ctx, denom)
		require.True(t, found)
		require.NoError(t, metadata.Validate())
		require.Equal(t, uint8(6), e.view(pointer, shareABI, "decimals")[0])
		assertPool(token, 0, 0)
		balances[token] = underlyingBalance(token)
		e.call(e.user, token, shareABI, "approve", vault, big.NewInt(deposit))
		require.Empty(t, e.call(e.user, vault, e.vault, "deposit", token, big.NewInt(deposit)))
		assertPool(token, deposit, deposit-keeper.MinimumShares)
		require.Zero(t, new(big.Int).Sub(balances[token], big.NewInt(deposit)).Cmp(underlyingBalance(token)))
	}
	want := e.a.SpotKeeper.ExportGenesis(e.ctx)
	e.restart()
	repair17RequireGenesisEqual(t, want, e.a.SpotKeeper.ExportGenesis(e.ctx))
	for _, token := range tokens {
		assertPool(token, deposit, deposit-keeper.MinimumShares)
		paid := e.call(e.user, vault, e.vault, "withdraw", token, big.NewInt(redeem))
		require.Len(t, paid, 1)
		require.Zero(t, big.NewInt(redeem).Cmp(paid[0].(*big.Int)))
		assertPool(token, deposit-redeem, deposit-redeem-keeper.MinimumShares)
		require.Zero(t, new(big.Int).Sub(balances[token], big.NewInt(deposit-redeem)).Cmp(underlyingBalance(token)))
		book, err := e.a.SpotKeeper.GetBook(e.ctx, token)
		require.NoError(t, err)
		require.Equal(t, sdk.NewInt(deposit), book.TotalDeposited)
		require.Equal(t, sdk.NewInt(redeem), book.TotalWithdrawn)
	}
	want = e.a.SpotKeeper.ExportGenesis(e.ctx)
	e.restart()
	repair17RequireGenesisEqual(t, want, e.a.SpotKeeper.ExportGenesis(e.ctx))
	for _, token := range tokens {
		assertPool(token, deposit-redeem, deposit-redeem-keeper.MinimumShares)
		require.Zero(t, new(big.Int).Sub(balances[token], big.NewInt(deposit-redeem)).Cmp(underlyingBalance(token)))
	}
}
