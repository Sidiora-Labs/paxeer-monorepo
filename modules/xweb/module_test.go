package xweb_test

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func moduleOriginTwoState(t *testing.T) (*app.App, sdk.Context, *sdk.KVStoreKey, keeper.Keeper) {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(100).
		WithBlockTime(time.Unix(1_800_000_000, 0))
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx = xwebtestutil.WithXWebStore(ctx, key)
	testApp.UpgradeKeeper.SetDone(ctx, keeper.ProgramOriginPlan)
	k := keeper.NewKeeper(key, testApp.BankKeeper, &testApp.EvmKeeper)
	k.InitGenesis(ctx, *types.DefaultGenesis())
	params := k.GetParams(ctx)
	params.KernelNetworkId = 7
	params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX}
	require.NoError(t, k.SetParams(ctx, params))
	attestors := xwebtestutil.Attestors(3)
	for _, attestor := range attestors {
		registration := attestor.Registration()
		registration.PublicKey = crypto.CompressPubkey(&attestor.Key.PublicKey)
		require.NoError(t, k.RegisterAttestor(ctx, types.MsgRegisterAttestor{
			Authority: params.Authority, Attestor: registration,
		}))
	}
	require.NoError(t, k.Unpause(ctx, types.MsgUnpause{Authority: params.Authority}))
	principal := sdk.AccAddress(bytes.Repeat([]byte{0xa1}, 20))
	programID := types.Hash32{0x41, 0x42}
	asset := types.Hash32{9}
	payload := []byte("origin-2 genesis query")
	fee := sdk.NewInt(75)
	coins := sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, fee.MulRaw(2)))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, "evm", coins))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, "evm", principal, coins))
	for _, id := range []uint64{3, 5} {
		cached, write := ctx.CacheContext()
		feeAccount := keeper.ProgramFeeAccount(asset)
		require.NoError(t, testApp.BankKeeper.SendCoins(cached, principal, feeAccount,
			sdk.NewCoins(sdk.NewCoin(types.ProgramFeeDenomPAX, fee))))
		call := &keeper.ProgramCall{
			ProgramID: programID, Principal: principal, Sequence: id + 100,
			Legs: []keeper.ProgramLeg{{From: principal, To: feeAccount, Asset: asset,
				Denom: types.ProgramFeeDenomPAX, Amount: fee}},
		}
		data := binary.BigEndian.AppendUint64(nil, id)
		data = append(data, types.KindSearch)
		data = binary.BigEndian.AppendUint32(data, uint32(len(payload)))
		data = append(data, payload...)
		require.NoError(t, k.RequestFromProgram(cached, call, []byte(keeper.ProgramRequestTopic), data))
		write()
	}
	response := bytes.Repeat([]byte{0xab}, 2*keeper.ProgramAnswerChunkBytes+17)
	observation := types.Observation{
		Origin: types.OriginProgram, NetworkID: params.KernelNetworkId,
		ProgramID: programID, RequestID: 3, Kind: types.KindSearch,
		PayloadHash: types.Keccak(payload), ContentDigest: types.Keccak(response),
		FullLength: uint32(len(response) + 100), Response: response,
	}
	digest := types.Digest(types.Attestation{
		Origin: observation.Origin, NetworkID: new(big.Int).SetUint64(uint64(observation.NetworkID)),
		Requester: observation.ProgramID, RequestID: observation.RequestID, Kind: observation.Kind,
		PayloadHash: observation.PayloadHash, ContentDigest: observation.ContentDigest,
		ResponseHash: types.Keccak(response), FullLength: observation.FullLength,
	})
	observation.Signatures = xwebtestutil.Sign(digest, attestors[:2]...)
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	require.NoError(t, k.FulfilProgram(ctx, raw))
	require.Equal(t, uint64(1), k.ProgramPendingCount(ctx))
	require.Equal(t, uint64(1), k.ProgramCommittedCount(ctx))
	return testApp, ctx, key, k
}

func TestModuleGenesisJSONRoundTripsOriginTwoState(t *testing.T) {
	testApp, ctx, key, k := moduleOriginTwoState(t)
	am := xweb.NewAppModule(k)
	cdc := testApp.AppCodec()
	exported := am.ExportGenesis(ctx, cdc)
	require.NoError(t, (xweb.AppModuleBasic{}).ValidateGenesis(cdc, nil, exported))
	var genesis types.GenesisState
	require.NoError(t, json.Unmarshal(exported, &genesis))
	require.Equal(t, uint64(1), genesis.ProgramPendingCount)
	require.Equal(t, uint64(1), genesis.ProgramCommittedCount)
	require.Len(t, genesis.ProgramPending, 2)
	require.Len(t, genesis.ProgramAnswers, 4)
	require.Equal(t, []types.ProgramFeeDenomEntry{{Asset: types.Hash32{9}, Denom: types.ProgramFeeDenomPAX}},
		genesis.ProgramFeeDenoms)
	for _, entry := range genesis.ProgramPending {
		record, err := keeper.DecodeProgramPendingRecord(entry.Record)
		require.NoError(t, err)
		require.Equal(t, entry.RequestID == 3, record.Fulfilled)
	}
	fresh := xwebtestutil.WithXWebStore(ctx, key)
	iterator := fresh.KVStore(key).Iterator(nil, nil)
	require.False(t, iterator.Valid(), "the import must start with an empty xweb store")
	require.NoError(t, iterator.Close())
	require.Empty(t, am.InitGenesis(fresh, cdc, exported))
	require.Equal(t, exported, am.ExportGenesis(fresh, cdc))
}

func TestModuleValidateGenesisRefusesOverCapacityCounters(t *testing.T) {
	testApp, ctx, _, k := moduleOriginTwoState(t)
	cdc := testApp.AppCodec()
	exported := xweb.NewAppModule(k).ExportGenesis(ctx, cdc)
	basic := xweb.AppModuleBasic{}
	require.NoError(t, basic.ValidateGenesis(cdc, nil, exported))
	for _, tc := range []struct {
		field string
		value json.RawMessage
		want  string
	}{
		{"program_committed_count", json.RawMessage(`65`), "program committed count 65 exceeds capacity 64"},
		{"program_pending_count", json.RawMessage(`257`), "program pending count 257 exceeds capacity 256"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(exported, &fields))
			fields[tc.field] = tc.value
			invalid, err := json.Marshal(fields)
			require.NoError(t, err)
			err = basic.ValidateGenesis(cdc, nil, invalid)
			require.ErrorIs(t, err, types.ErrInvalidGenesis)
			require.ErrorContains(t, err, tc.want)
		})
	}
}
