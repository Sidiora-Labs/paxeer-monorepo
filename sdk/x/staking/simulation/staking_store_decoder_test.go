package simulation

import (
	"bytes"
	"fmt"
	"math"
	"testing"
	"time"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	cryptocodec "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/keeper"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	paramskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/params/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

type stakingDecoderState struct {
	ctx        sdk.Context
	validator  types.Validator
	del        sdk.AccAddress
	dst        sdk.ValAddress
	delegation types.Delegation
	ubd        types.UnbondingDelegation
	red        types.Redelegation
	historical types.HistoricalInfo
}

type stakingDecoderFixture struct {
	cdc    codec.Codec
	keeper keeper.Keeper
	states [2]stakingDecoderState
}

func newStakingDecoderFixture(t *testing.T) stakingDecoderFixture {
	t.Helper()
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	types.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	ms := rootmulti.NewStore(dbm.NewMemDB())
	keys := sdk.NewKVStoreKeys("auth", "bank", "staking", "params")
	for _, key := range keys {
		ms.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	}
	transient := sdk.NewTransientStoreKey("transient_params")
	ms.MountStoreWithDB(transient, storetypes.StoreTypeTransient, nil)
	require.NoError(t, ms.LoadLatestVersion())
	t.Cleanup(func() { require.NoError(t, ms.Close()) })
	ctx := sdk.NewContext(ms, tmproto.Header{ChainID: "repair-m697", Height: 100, Time: time.Unix(1000, 0).UTC()}, false)
	pk := paramskeeper.NewKeeper(cdc, codec.NewLegacyAmino(), keys["params"], transient)
	ak := authkeeper.NewAccountKeeper(cdc, keys["auth"], pk.Subspace("auth"), authtypes.ProtoBaseAccount,
		map[string][]string{types.BondedPoolName: {authtypes.Staking}, types.NotBondedPoolName: {authtypes.Staking}})
	bk := bankkeeper.NewBaseKeeper(cdc, keys["bank"], ak, pk.Subspace("bank"), map[string]bool{})
	k := keeper.NewKeeper(cdc, keys["staking"], ak, bk, pk.Subspace("staking"))
	f := stakingDecoderFixture{cdc: cdc, keeper: k}
	for i := range f.states {
		branch, _ := ctx.CacheContext()
		branch = branch.WithBlockHeight(100 + int64(i))
		pubkey := ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("repair-m697-validator-%d", i))).PubKey()
		validator, err := types.NewValidator(sdk.ValAddress(pubkey.Address()), pubkey, types.Description{Moniker: fmt.Sprintf("validator-%d", i)})
		require.NoError(t, err)
		validator.Tokens = k.TokensFromConsensusPower(branch, 10+int64(i))
		validator.DelegatorShares = validator.Tokens.ToDec()
		validator.UnbondingTime = branch.BlockTime().Add(time.Hour)
		validator.UnbondingHeight = branch.BlockHeight()
		k.SetValidator(branch, validator)
		require.NoError(t, k.SetValidatorByConsAddr(branch, validator))
		k.SetValidatorByPowerIndex(branch, validator)
		k.SetLastValidatorPower(branch, validator.GetOperator(), 10+int64(i))
		k.SetLastTotalPower(branch, sdk.NewInt(10+int64(i)))
		del := sdk.AccAddress(bytes.Repeat([]byte{byte(i + 1)}, 20))
		dst := sdk.ValAddress(bytes.Repeat([]byte{byte(i + 3)}, 20))
		delegation := types.NewDelegation(del, validator.GetOperator(), sdk.NewDec(4+int64(i)))
		k.SetDelegation(branch, delegation)
		ubd := k.SetUnbondingDelegationEntry(branch, del, validator.GetOperator(), branch.BlockHeight(), validator.UnbondingTime, sdk.NewInt(2+int64(i)))
		k.InsertUBDQueue(branch, ubd, validator.UnbondingTime)
		red := k.SetRedelegationEntry(branch, del, validator.GetOperator(), dst, branch.BlockHeight(), validator.UnbondingTime, sdk.NewInt(3+int64(i)), sdk.OneDec(), sdk.NewDec(2+int64(i)))
		k.InsertRedelegationQueue(branch, red, validator.UnbondingTime)
		k.InsertUnbondingValidatorQueue(branch, validator)
		historical := types.NewHistoricalInfo(branch.BlockHeader(), []types.Validator{validator}, k.PowerReduction(branch))
		k.SetHistoricalInfo(branch, branch.BlockHeight(), &historical)
		f.states[i] = stakingDecoderState{branch, validator, del, dst, delegation, ubd, red, historical}
	}
	return f
}

func (f stakingDecoderFixture) pair(t *testing.T, state int, key []byte) kv.Pair {
	t.Helper()
	store := f.states[state].ctx.KVStore(f.keeper.GetStoreKey())
	require.True(t, store.Has(key), "keeper must have written key %X", key)
	return kv.Pair{Key: bytes.Clone(key), Value: bytes.Clone(store.Get(key))}
}

func TestStakingStoreDecoder_LastValidatorPowers(t *testing.T) {
	f := newStakingDecoderFixture(t)
	decode := NewDecodeStore(f.cdc)
	for _, powers := range [][2]int64{{0, 1}, {127, 128}, {1 << 40, math.MaxInt64}} {
		t.Run(fmt.Sprintf("%d_%d", powers[0], powers[1]), func(t *testing.T) {
			var pairs [2]kv.Pair
			for i, state := range f.states {
				f.keeper.SetLastValidatorPower(state.ctx, state.validator.GetOperator(), powers[i])
				pairs[i] = f.pair(t, i, types.GetLastValidatorPowerKey(state.validator.GetOperator()))
				require.Equal(t, powers[i], f.keeper.GetLastValidatorPower(state.ctx, state.validator.GetOperator()))
			}
			require.Equal(t, fmt.Sprintf("%d\n%d", powers[0], powers[1]), decode(pairs[0], pairs[1]))
		})
	}
}

func TestStakingStoreDecoder_EmptyIndexes(t *testing.T) {
	f := newStakingDecoderFixture(t)
	decode := NewDecodeStore(f.cdc)
	for _, index := range []struct {
		name string
		key  func(stakingDecoderState) []byte
	}{
		{"unbonding_by_validator", func(s stakingDecoderState) []byte { return types.GetUBDByValIndexKey(s.del, s.validator.GetOperator()) }},
		{"redelegation_by_source", func(s stakingDecoderState) []byte {
			return types.GetREDByValSrcIndexKey(s.del, s.validator.GetOperator(), s.dst)
		}},
		{"redelegation_by_destination", func(s stakingDecoderState) []byte {
			return types.GetREDByValDstIndexKey(s.del, s.validator.GetOperator(), s.dst)
		}},
	} {
		t.Run(index.name, func(t *testing.T) {
			a := f.pair(t, 0, index.key(f.states[0]))
			b := f.pair(t, 1, index.key(f.states[1]))
			require.Empty(t, a.Value)
			require.Empty(t, b.Value)
			require.NotEqual(t, a.Key, b.Key)
			require.Equal(t, fmt.Sprintf("%X\n%X", a.Key, b.Key), decode(a, b))
		})
	}
}

func TestStakingStoreDecoder_QueuesHistoricalAndRecords(t *testing.T) {
	f := newStakingDecoderFixture(t)
	decode := NewDecodeStore(f.cdc)
	for _, record := range []struct {
		name  string
		key   func(stakingDecoderState) []byte
		value func(stakingDecoderState) interface{}
	}{
		{"total_power", func(s stakingDecoderState) []byte { return types.LastTotalPowerKey }, func(s stakingDecoderState) interface{} {
			return sdk.IntProto{Int: s.validator.Tokens.Quo(f.keeper.PowerReduction(s.ctx))}
		}},
		{"validator", func(s stakingDecoderState) []byte { return types.GetValidatorKey(s.validator.GetOperator()) }, func(s stakingDecoderState) interface{} { return s.validator }},
		{"consensus_index", func(s stakingDecoderState) []byte {
			addr, err := s.validator.GetConsAddr()
			require.NoError(t, err)
			return types.GetValidatorByConsAddrKey(addr)
		}, func(s stakingDecoderState) interface{} { return s.validator.GetOperator() }},
		{"power_index", func(s stakingDecoderState) []byte {
			return types.GetValidatorsByPowerIndexKey(s.validator, f.keeper.PowerReduction(s.ctx))
		}, func(s stakingDecoderState) interface{} { return s.validator.GetOperator() }},
		{"delegation", func(s stakingDecoderState) []byte { return types.GetDelegationKey(s.del, s.validator.GetOperator()) }, func(s stakingDecoderState) interface{} { return s.delegation }},
		{"unbonding", func(s stakingDecoderState) []byte { return types.GetUBDKey(s.del, s.validator.GetOperator()) }, func(s stakingDecoderState) interface{} { return s.ubd }},
		{"redelegation", func(s stakingDecoderState) []byte { return types.GetREDKey(s.del, s.validator.GetOperator(), s.dst) }, func(s stakingDecoderState) interface{} { return s.red }},
		{"unbonding_queue", func(s stakingDecoderState) []byte {
			return types.GetUnbondingDelegationTimeKey(s.validator.UnbondingTime)
		}, func(s stakingDecoderState) interface{} {
			return types.DVPairs{Pairs: []types.DVPair{{DelegatorAddress: s.del.String(), ValidatorAddress: s.validator.OperatorAddress}}}
		}},
		{"redelegation_queue", func(s stakingDecoderState) []byte { return types.GetRedelegationTimeKey(s.validator.UnbondingTime) }, func(s stakingDecoderState) interface{} {
			return types.DVVTriplets{Triplets: []types.DVVTriplet{{DelegatorAddress: s.del.String(), ValidatorSrcAddress: s.validator.OperatorAddress, ValidatorDstAddress: s.dst.String()}}}
		}},
		{"validator_queue", func(s stakingDecoderState) []byte {
			return types.GetValidatorQueueKey(s.validator.UnbondingTime, s.validator.UnbondingHeight)
		}, func(s stakingDecoderState) interface{} {
			return types.ValAddresses{Addresses: []string{s.validator.OperatorAddress}}
		}},
		{"historical", func(s stakingDecoderState) []byte { return types.GetHistoricalInfoKey(s.ctx.BlockHeight()) }, func(s stakingDecoderState) interface{} { return s.historical }},
	} {
		t.Run(record.name, func(t *testing.T) {
			a := f.pair(t, 0, record.key(f.states[0]))
			b := f.pair(t, 1, record.key(f.states[1]))
			expectedA, expectedB := record.value(f.states[0]), record.value(f.states[1])
			require.NotEqual(t, fmt.Sprint(expectedA), fmt.Sprint(expectedB), "both sides must carry distinguishable state")
			require.Equal(t, fmt.Sprintf("%v\n%v", expectedA, expectedB), decode(a, b))
			require.Equal(t, fmt.Sprintf("%v\n%v", expectedB, expectedA), decode(b, a))
		})
	}
}
