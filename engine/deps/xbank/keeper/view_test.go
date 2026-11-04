package keeper_test

import (
	"encoding/hex"
	"encoding/json"
	"testing"

	gigastore "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	"github.com/stretchr/testify/require"

	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/xbank/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/apptesting"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/testdata"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (suite *IntegrationTestSuite) TestViewKeeperStoreTrace() {
	app, ctx := suite.app, suite.ctx
	_, _, addr := testdata.KeyTestPubAddr()
	origCoins := sdk.NewCoins(newFooCoin(50), newBarCoin(30))
	acc := app.AccountKeeper.NewAccountWithAddress(ctx, addr)

	app.AccountKeeper.SetAccount(ctx, acc)
	suite.Require().NoError(apptesting.FundAccount(app.BankKeeper, ctx, acc.GetAddress(), origCoins))

	ctx = ctx.WithIsTracing(true)
	app.BankKeeper.GetBalance(ctx, addr, fooDenom)
	app.BankKeeper.GetAllBalances(ctx, addr)

	trace := ctx.StoreTracer().DerivePrestateToJson()
	typedTrace := &sdk.StoreTraceDump{}
	suite.Require().NoError(json.Unmarshal(trace, &typedTrace))
	suite.Require().Len(typedTrace.Modules, 1)
	suite.Require().Contains(typedTrace.Modules, "bank")

	bankDump := typedTrace.Modules["bank"]
	fooKey := append(banktypes.CreateAccountBalancesPrefix(addr), []byte(fooDenom)...)
	barKey := append(banktypes.CreateAccountBalancesPrefix(addr), []byte(barDenom)...)
	suite.Require().Len(bankDump.Iterators, 1)
	iterKeys := bankDump.Iterators[0].Keys
	suite.Require().Len(iterKeys, 2)
	suite.Require().Contains(iterKeys, hex.EncodeToString(fooKey))
	suite.Require().Contains(iterKeys, hex.EncodeToString(barKey))
	suite.Require().Len(bankDump.Reads, 2)
	suite.Require().Contains(bankDump.Reads, hex.EncodeToString(fooKey))
	suite.Require().Contains(bankDump.Reads, hex.EncodeToString(barKey))
	fooCoin, barCoin := sdk.Coin{}, sdk.Coin{}
	fooBz, err := hex.DecodeString(bankDump.Reads[hex.EncodeToString(fooKey)])
	suite.Require().NoError(err)
	barBz, err := hex.DecodeString(bankDump.Reads[hex.EncodeToString(barKey)])
	suite.Require().NoError(err)
	suite.Require().NoError(app.BankKeeper.GetCdc().Unmarshal(fooBz, &fooCoin))
	suite.Require().NoError(app.BankKeeper.GetCdc().Unmarshal(barBz, &barCoin))
	suite.Require().Equal(sdk.NewInt(50), fooCoin.Amount)
	suite.Require().Equal(sdk.NewInt(30), barCoin.Amount)
}

func TestGigaBankKeeperViewAndTrace(t *testing.T) {
	for _, regular := range []bool{false, true} {
		name := "giga_store"
		if regular {
			name = "regular_store"
		}
		t.Run(name, func(t *testing.T) {
			f := repairM1125Open(t, t.TempDir())
			repairM1125Seed(t, f)
			k := f.giga
			k.UseRegularStore = regular
			ctx := f.ctx.WithMultiStore(f.cache)
			from, to := repairM1125Addresses()
			_, isGiga := k.GetKVStore(ctx).(*gigastore.Store)
			require.Equal(t, !regular, isGiga)
			for _, addr := range []sdk.AccAddress{from, to} {
				for _, denom := range []string{"foo", "bar", "missing", sdk.MustGetBaseDenom()} {
					require.Equal(t, f.bank.GetBalance(f.ctx, addr, denom), k.GetBalance(ctx, addr, denom))
				}
				require.Equal(t, f.bank.GetWeiBalance(f.ctx, addr), k.GetWeiBalance(ctx, addr))
				require.Empty(t, k.LockedCoins(ctx, addr))
				require.Equal(t, sdk.NewCoins(f.bank.GetBalance(f.ctx, addr, "uhpx")), k.SpendableCoins(ctx, addr))
				require.Equal(t, f.bank.SpendableCoins(f.ctx, addr).AmountOf("uhpx"), k.SpendableCoins(ctx, addr).AmountOf("uhpx"))
			}
			require.True(t, k.HasBalance(ctx, from, sdk.NewInt64Coin("foo", 50)))
			require.False(t, k.HasBalance(ctx, from, sdk.NewInt64Coin("foo", 51)))
			require.False(t, k.HasBalance(ctx, from, sdk.NewInt64Coin("missing", 1)))

			traced := ctx.WithIsTracing(true)
			require.Equal(t, sdk.NewInt64Coin("foo", 50), k.GetBalance(traced, from, "foo"))
			require.Equal(t, sdk.NewInt64Coin("bar", 30), k.GetBalance(traced, from, "bar"))
			require.Equal(t, sdk.NewInt64Coin("missing", 0), k.GetBalance(traced, from, "missing"))
			require.Equal(t, sdk.OneInt(), k.GetWeiBalance(traced, from))
			var trace sdk.StoreTraceDump
			require.NoError(t, json.Unmarshal(traced.StoreTracer().DerivePrestateToJson(), &trace))
			if regular {
				require.Len(t, trace.Modules, 1)
				dump := trace.Modules["bank"]
				require.Empty(t, dump.Iterators)
				require.Len(t, dump.Reads, 4)
				for _, coin := range []sdk.Coin{sdk.NewInt64Coin("foo", 50), sdk.NewInt64Coin("bar", 30)} {
					key := banktypes.CreatePrefixedAccountStoreKey(from, []byte(coin.Denom))
					encoded, exists := dump.Reads[hex.EncodeToString(key)]
					require.True(t, exists)
					data, err := hex.DecodeString(encoded)
					require.NoError(t, err)
					var actual sdk.Coin
					require.NoError(t, f.cdc.Unmarshal(data, &actual))
					require.Equal(t, coin, actual)
				}
				missingKey := banktypes.CreatePrefixedAccountStoreKey(from, []byte("missing"))
				require.Contains(t, dump.Reads, hex.EncodeToString(missingKey))
				require.Empty(t, dump.Reads[hex.EncodeToString(missingKey)])
				weiKey := append(append([]byte(nil), banktypes.WeiBalancesPrefix...), from...)
				data, err := hex.DecodeString(dump.Reads[hex.EncodeToString(weiKey)])
				require.NoError(t, err)
				var wei sdk.Int
				require.NoError(t, wei.Unmarshal(data))
				require.Equal(t, sdk.OneInt(), wei)
			} else {
				require.Empty(t, trace.Modules, "direct Giga reads bypass the SDK gas/tracing wrapper")
				accountStore := prefix.NewStore(k.GetKVStore(traced), banktypes.CreateAccountBalancesPrefix(from))
				require.PanicsWithError(t, gigastore.ErrIteratorUnsupported.Error(), func() { accountStore.Iterator(nil, nil) })
				require.PanicsWithError(t, gigastore.ErrIteratorUnsupported.Error(), func() { accountStore.ReverseIterator(nil, nil) })
			}

			// Exercise the actual Giga writer and cache publication into the canonical store.
			require.NoError(t, k.AddCoins(ctx, from, sdk.NewCoins(sdk.NewInt64Coin("foo", 5)), true))
			require.NoError(t, k.AddWei(ctx, from, sdk.NewInt(3)))
			require.Equal(t, sdk.NewInt64Coin("foo", 55), k.GetBalance(ctx, from, "foo"))
			require.Equal(t, sdk.NewInt(4), k.GetWeiBalance(ctx, from))
			require.Equal(t, sdk.NewInt64Coin("foo", 50), f.bank.GetBalance(f.ctx, from, "foo"))
			require.Equal(t, sdk.OneInt(), f.bank.GetWeiBalance(f.ctx, from))
			if !regular {
				f.cache.WriteGiga()
			}
			f.cache.Write()
			require.Equal(t, sdk.NewInt64Coin("foo", 55), f.bank.GetBalance(f.ctx, from, "foo"))
			require.Equal(t, sdk.NewInt(4), f.bank.GetWeiBalance(f.ctx, from))
		})
	}
}

func TestCanonicalBankKeeperViewAndTrace(t *testing.T) {
	f := repairM1125Open(t, t.TempDir())
	repairM1125Seed(t, f)
	from, _ := repairM1125Addresses()
	ctx := f.ctx.WithMultiStore(f.cache).WithIsTracing(true)
	require.Equal(t, sdk.NewInt64Coin("foo", 50), f.bank.GetBalance(ctx, from, "foo"))
	require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("bar", 30), sdk.NewInt64Coin("foo", 50), sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 10)), f.bank.GetAllBalances(ctx, from))
	var trace sdk.StoreTraceDump
	require.NoError(t, json.Unmarshal(ctx.StoreTracer().DerivePrestateToJson(), &trace))
	require.Len(t, trace.Modules, 1)
	dump := trace.Modules["bank"]
	require.Len(t, dump.Iterators, 1)
	require.Len(t, dump.Iterators[0].Keys, 3)
	require.Len(t, dump.Reads, 3)
	for _, coin := range []sdk.Coin{sdk.NewInt64Coin("bar", 30), sdk.NewInt64Coin("foo", 50), sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 10)} {
		key := hex.EncodeToString(banktypes.CreatePrefixedAccountStoreKey(from, []byte(coin.Denom)))
		require.Contains(t, dump.Iterators[0].Keys, key)
		data, err := hex.DecodeString(dump.Reads[key])
		require.NoError(t, err)
		var actual sdk.Coin
		require.NoError(t, f.cdc.Unmarshal(data, &actual))
		require.Equal(t, coin, actual)
	}
}
