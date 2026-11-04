package types_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/legacy"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/legacy/legacytx"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	distrtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
)

func TestRepairM_645_AddressPrefixCache(t *testing.T) {
	config := sdk.GetConfig()
	cases := []struct {
		name         string
		address      sdk.Address
		getPrefix    func() string
		getPubPrefix func() string
		setPrefix    func(string, string)
		decode       func(string) ([]byte, error)
	}{
		{"account", sdk.AccAddress(bytes.Repeat([]byte{0x65}, 20)), config.GetBech32AccountAddrPrefix, config.GetBech32AccountPubPrefix, config.SetBech32PrefixForAccount, func(s string) ([]byte, error) { return sdk.AccAddressFromBech32(s) }},
		{"validator", sdk.ValAddress(bytes.Repeat([]byte{0x65}, 20)), config.GetBech32ValidatorAddrPrefix, config.GetBech32ValidatorPubPrefix, config.SetBech32PrefixForValidator, func(s string) ([]byte, error) { return sdk.ValAddressFromBech32(s) }},
		{"consensus", sdk.ConsAddress(bytes.Repeat([]byte{0x65}, 20)), config.GetBech32ConsensusAddrPrefix, config.GetBech32ConsensusPubPrefix, config.SetBech32PrefixForConsensusNode, func(s string) ([]byte, error) { return sdk.ConsAddressFromBech32(s) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldPrefix, oldPub := tc.getPrefix(), tc.getPubPrefix()
			t.Cleanup(func() { tc.setPrefix(oldPrefix, oldPub) })
			before := tc.address.String()
			decoded, err := tc.decode(before)
			require.NoError(t, err)
			require.Equal(t, tc.address.Bytes(), decoded)
			prefix := "repair" + tc.name
			tc.setPrefix(prefix, prefix+"pub")
			after := tc.address.String()
			require.NotEqual(t, before, after)
			require.Equal(t, sdk.MustBech32ifyAddressBytes(prefix, tc.address.Bytes()), after)
			require.Equal(t, after, tc.address.String(), "a cache hit must preserve the current prefix")
			decoded, err = tc.decode(after)
			require.NoError(t, err)
			require.Equal(t, tc.address.Bytes(), decoded)
			_, err = tc.decode(before)
			require.Error(t, err, "the decoder must enforce the new prefix")
			wire, err := tc.address.MarshalJSON()
			require.NoError(t, err)
			var encoded string
			require.NoError(t, json.Unmarshal(wire, &encoded))
			decoded, err = tc.decode(encoded)
			require.NoError(t, err)
			require.Equal(t, tc.address.Bytes(), decoded)
			tc.setPrefix(oldPrefix, oldPub)
			require.Equal(t, before, tc.address.String())
		})
	}
}

func TestRepairM_645_EncoderResultPropagation(t *testing.T) {
	s := new(configTestSuite)
	s.SetT(t)
	s.TestConfig_SetTxEncoder()

	config := sdk.NewConfig()
	tx := legacytx.NewStdTx(nil, legacytx.NewStdFee(10, sdk.NewCoins()), nil, "repair-m645")
	payload := []byte("encoder payload for repair-m645")
	sentinel := errors.New("repair-m645 encoder sentinel")
	config.SetTxEncoder(func(input sdk.Tx) ([]byte, error) {
		require.Same(t, &tx, input)
		return payload, sentinel
	})
	actual, err := config.GetTxEncoder()(&tx)
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, payload, actual)
	config.Seal()
	require.Panics(t, func() { config.SetTxEncoder(nil) })
}

func TestRepairM_645_CachedContextEVMEvents(t *testing.T) {
	key := sdk.NewKVStoreKey("repair-m645")
	ctx := testutil.DefaultContext(key, sdk.NewTransientStoreKey("repair-m645-transient"))
	rootLog := &ethtypes.Log{Address: common.BytesToAddress([]byte{1}), Topics: []common.Hash{common.BytesToHash([]byte{1})}, Data: []byte("root")}
	childLog := &ethtypes.Log{Address: common.BytesToAddress([]byte{2}), Data: []byte("child")}
	nestedLog := &ethtypes.Log{Address: common.BytesToAddress([]byte{3}), Data: []byte("nested")}
	ctx.EVMEventManager().EmitEvents([]*ethtypes.Log{rootLog})
	ctx.EventManager().EmitEvent(sdk.NewEvent("root"))
	ctx.KVStore(key).Set([]byte("value"), []byte("root"))

	child, commit := ctx.CacheContext()
	require.NotSame(t, ctx.EVMEventManager(), child.EVMEventManager())
	require.Empty(t, child.EVMEventManager().Events())
	child.KVStore(key).Set([]byte("value"), []byte("child"))
	child.EVMEventManager().EmitEvents([]*ethtypes.Log{childLog})
	child.EventManager().EmitEvent(sdk.NewEvent("child"))

	nested, commitNested := child.CacheContext()
	nested.KVStore(key).Set([]byte("nested"), []byte("committed"))
	nested.EVMEventManager().EmitEvents([]*ethtypes.Log{nestedLog})
	commitNested()
	commitNested()
	require.Equal(t, []*ethtypes.Log{childLog, nestedLog}, child.EVMEventManager().Events())
	require.Equal(t, []*ethtypes.Log{rootLog}, ctx.EVMEventManager().Events())
	require.Nil(t, ctx.KVStore(key).Get([]byte("nested")))
	require.Equal(t, []byte("root"), ctx.KVStore(key).Get([]byte("value")))

	discarded, _ := child.CacheContext()
	discarded.KVStore(key).Set([]byte("discarded"), []byte("uncommitted"))
	discarded.EVMEventManager().EmitEvents([]*ethtypes.Log{{Data: []byte("discarded")}})
	discarded.EventManager().EmitEvent(sdk.NewEvent("discarded"))
	commit()
	commit()
	require.Equal(t, []*ethtypes.Log{rootLog, childLog, nestedLog}, ctx.EVMEventManager().Events())
	require.Equal(t, []byte("child"), ctx.KVStore(key).Get([]byte("value")))
	require.Equal(t, []byte("committed"), ctx.KVStore(key).Get([]byte("nested")))
	require.Nil(t, ctx.KVStore(key).Get([]byte("discarded")))
	require.Equal(t, sdk.Events{sdk.NewEvent("root")}, ctx.EventManager().Events())
	ctx.EventManager().EmitEvents(child.EventManager().Events())
	require.Equal(t, sdk.Events{sdk.NewEvent("root"), sdk.NewEvent("child")}, ctx.EventManager().Events())

	beforeLogs := append([]*ethtypes.Log(nil), ctx.EVMEventManager().Events()...)
	beforeEvents := append(sdk.Events(nil), ctx.EventManager().Events()...)
	failed, _ := ctx.CacheContext()
	failed.KVStore(key).Set([]byte("value"), []byte("failed"))
	failed.EVMEventManager().EmitEvents([]*ethtypes.Log{{Data: []byte("failed")}})
	failed.EventManager().EmitEvent(sdk.NewEvent("failed"))
	grandchild, commitGrandchild := failed.CacheContext()
	grandchild.KVStore(key).Set([]byte("failed-nested"), []byte("invisible"))
	grandchild.EVMEventManager().EmitEvents([]*ethtypes.Log{{Data: []byte("failed-nested")}})
	commitGrandchild()
	invalidBalance := banktypes.Balance{
		Address: sdk.AccAddress(bytes.Repeat([]byte{0x67}, 20)).String(),
		Coins:   sdk.Coins{{Denom: "aaa", Amount: sdk.NewInt(-1)}},
	}
	require.Error(t, invalidBalance.Validate(), "a failed branch validation must not publish state or logs")
	require.Equal(t, beforeLogs, ctx.EVMEventManager().Events(), "discarding an outer branch must discard committed inner logs")
	require.Equal(t, beforeEvents, ctx.EventManager().Events())
	require.Equal(t, []byte("child"), ctx.KVStore(key).Get([]byte("value")))
	require.Nil(t, ctx.KVStore(key).Get([]byte("failed-nested")))
}

func TestRepairM_645_DecCoinsZeroConversion(t *testing.T) {
	cases := []struct {
		name  string
		input sdk.Coins
		want  sdk.DecCoins
	}{
		{"zero_only", sdk.Coins{sdk.NewInt64Coin("aaa", 0)}, sdk.DecCoins{}},
		{"mixed", sdk.Coins{sdk.NewInt64Coin("aaa", 0), sdk.NewInt64Coin("bbb", 6), sdk.NewInt64Coin("ccc", 0)}, sdk.NewDecCoins(sdk.NewInt64DecCoin("bbb", 6))},
		{"sorted_multi_denom", sdk.Coins{sdk.NewInt64Coin("aaa", 2), sdk.NewInt64Coin("bbb", 4), sdk.NewInt64Coin("ccc", 6)}, sdk.NewDecCoins(sdk.NewInt64DecCoin("aaa", 2), sdk.NewInt64DecCoin("bbb", 4), sdk.NewInt64DecCoin("ccc", 6))},
		{"unsorted_with_zero", sdk.Coins{sdk.NewInt64Coin("ccc", 6), sdk.NewInt64Coin("aaa", 2), sdk.NewInt64Coin("bbb", 0)}, sdk.NewDecCoins(sdk.NewInt64DecCoin("aaa", 2), sdk.NewInt64DecCoin("ccc", 6))},
	}
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			converted := sdk.NewDecCoinsFromCoins(tc.input...)
			require.Equal(t, tc.want, converted)
			require.NoError(t, converted.Validate())
			require.Equal(t, tc.want.String(), converted.String())
			fee := legacytx.NewStdFee(2, tc.input)
			var decodedFee legacytx.StdFee
			require.NoError(t, legacy.Cdc.UnmarshalAsJSON(fee.Bytes(), &decodedFee))
			prices := decodedFee.GasPrices()
			require.NoError(t, prices.Validate())
			require.True(t, tc.want.QuoDec(sdk.NewDec(2)).IsEqual(prices))

			genesis := distrtypes.DefaultGenesisState()
			genesis.FeePool.CommunityPool = converted
			wire, err := cdc.Marshal(genesis)
			require.NoError(t, err)
			var recovered distrtypes.GenesisState
			require.NoError(t, cdc.Unmarshal(wire, &recovered))
			require.NoError(t, distrtypes.ValidateGenesis(&recovered))
			require.NoError(t, recovered.FeePool.CommunityPool.Validate())
			require.True(t, tc.want.IsEqual(recovered.FeePool.CommunityPool))
			withdrawn, change := recovered.FeePool.CommunityPool.TruncateDecimal()
			require.True(t, sdk.NewCoins(tc.input...).IsEqual(withdrawn))
			require.Empty(t, change)
			require.True(t, recovered.FeePool.CommunityPool.Sub(converted).IsZero())
		})
	}
}

func TestRepairM_645_DecCoinsUnionOrder(t *testing.T) {
	left := sdk.NewDecCoins(sdk.NewDecCoinFromDec("aaa", sdk.MustNewDecFromStr("1.25")), sdk.NewInt64DecCoin("ccc", 8), sdk.NewInt64DecCoin("eee", 2))
	right := sdk.NewDecCoins(sdk.NewInt64DecCoin("bbb", 3), sdk.NewInt64DecCoin("ccc", 6), sdk.NewDecCoinFromDec("eee", sdk.MustNewDecFromStr("2.5")), sdk.NewInt64DecCoin("zzz", 9))
	want := sdk.NewDecCoins(left[0], right[0], left[1], right[2], right[3])
	require.NoError(t, left.Validate())
	require.NoError(t, right.Validate())
	for i := 0; i < 128; i++ {
		for _, result := range []sdk.DecCoins{left.UnionMax(right), right.UnionMax(left)} {
			require.Equal(t, want, result)
			require.True(t, result.IsValid())
			for j, coin := range want {
				if j > 0 {
					require.Less(t, result[j-1].Denom, result[j].Denom)
				}
				require.True(t, coin.Amount.Equal(result.AmountOf(coin.Denom)))
			}
			require.True(t, result.AmountOf("missing").IsZero())
			require.True(t, result.Sub(want).IsZero())
		}
	}
	require.Equal(t, "1.250000000000000000aaa,8.000000000000000000ccc,2.000000000000000000eee", left.String())
	require.Equal(t, "3.000000000000000000bbb,6.000000000000000000ccc,2.500000000000000000eee,9.000000000000000000zzz", right.String())
}

func TestRepairM_645_UnionTestResult(t *testing.T) {
	s := new(decCoinTestSuite)
	s.SetT(t)
	s.TestUnionMax()
}

func TestRepairM_645_CeilOverflowBoundary(t *testing.T) {
	TestCeilOverflow(t)
}

func TestRepairM_645_NormalizationGenesis(t *testing.T) {
	sdk.Repair293RegisterDenoms(t)
	cases := []struct {
		input  string
		amount int64
	}{
		{"1atom,1matom,1uatom", 1001001000},
		{"0.1atom,0.1matom,0.1uatom", 100100100},
		{"0.0000000006atom,0.0000006matom,0.0006uatom", 1},
		{"1.0000000009atom,0.0000009matom", 1000000001},
		{"0atom,0matom,0uatom", 0},
		{"0.0000000001atom,0.0000001matom", 0},
	}
	cdc := codec.NewProtoCodec(codectypes.NewInterfaceRegistry())
	address := sdk.AccAddress(bytes.Repeat([]byte{0x66}, 20)).String()
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			coins, err := sdk.ParseCoinsNormalized(tc.input)
			require.NoError(t, err)
			require.Equal(t, sdk.NewCoins(sdk.NewInt64Coin("natom", tc.amount)), coins)
			require.NoError(t, coins.Validate())
			balance := banktypes.Balance{Address: address, Coins: coins}
			require.NoError(t, balance.Validate())
			genesis := banktypes.NewGenesisState(banktypes.DefaultParams(), []banktypes.Balance{balance}, coins, nil, nil)
			wire, err := cdc.MarshalAsJSON(genesis)
			require.NoError(t, err)
			recovered := banktypes.GetGenesisStateFromAppState(cdc, map[string]json.RawMessage{banktypes.ModuleName: wire})
			require.NoError(t, recovered.Validate())
			require.Equal(t, coins.String(), recovered.Balances[0].Coins.String())
			require.Equal(t, coins.String(), recovered.Supply.String())
		})
	}
	for _, input := range []string{"1atom,2atom", "-1atom", "1!atom", "0.1234567890123456789atom"} {
		_, err := sdk.ParseCoinsNormalized(input)
		require.Error(t, err, input)
	}
	invalid := banktypes.Balance{Address: address, Coins: sdk.Coins{sdk.NewInt64Coin("natom", 1), sdk.NewInt64Coin("natom", 2)}}
	require.Error(t, invalid.Validate(), "the consumer must continue rejecting duplicate-denomination balances")
}
