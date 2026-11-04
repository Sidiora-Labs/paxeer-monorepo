package keeper_test

import (
	"bytes"
	"context"
	"sort"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/exported"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/evidence/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

// Count calls while retaining the production codec's complete decoding path.
type repairM603DecodeCounter struct {
	codec.BinaryCodec
	decodes int
}

func (c *repairM603DecodeCounter) UnmarshalInterface(bz []byte, ptr interface{}) error {
	c.decodes++
	return c.BinaryCodec.UnmarshalInterface(bz, ptr)
}

func repairM603Evidence(height int64) exported.Evidence {
	return types.FromABCIEvidence(abci.Evidence{
		Type: abci.MisbehaviorType_DUPLICATE_VOTE,
		Validator: abci.Validator{
			Address: bytes.Repeat([]byte{0x63}, 20),
			Power:   100,
		},
		Height:           height,
		Time:             time.Unix(1_800_000_000, 0).UTC(),
		TotalVotingPower: 1000,
	})
}

func TestRepairM_603_Pagination(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	cdc := &repairM603DecodeCounter{BinaryCodec: codec.NewProtoCodec(registry)}
	key := sdk.NewKVStoreKey(types.StoreKey)
	db := dbm.NewMemDB()
	cms := store.NewCommitMultiStore(db)
	t.Cleanup(func() { require.NoError(t, cms.Close()) })
	cms.MountStoreWithDB(key, sdk.StoreTypeIAVL, db)
	require.NoError(t, cms.LoadLatestVersion())
	ctx := sdk.NewContext(cms, tmproto.Header{Height: 1}, false)
	k := keeper.NewKeeper(cdc, key, nil, nil)
	const collectionSize = 2048
	evidence := make([]exported.Evidence, collectionSize)
	for i := range evidence {
		evidence[i] = repairM603Evidence(int64(i + 1))
		require.NoError(t, evidence[i].ValidateBasic())
		k.SetEvidence(ctx, evidence[i])
	}
	cms.Commit(true)
	sort.Slice(evidence, func(i, j int) bool {
		return bytes.Compare(evidence[i].Hash(), evidence[j].Hash()) < 0
	})
	helper := baseapp.NewQueryServerTestHelper(ctx, registry)
	types.RegisterQueryServer(helper, k)
	client := types.NewQueryClient(helper)

	for _, tc := range []struct {
		name    string
		page    *query.PageRequest
		indices []int
		next    int
		total   uint64
	}{
		{"first", &query.PageRequest{Limit: 3}, []int{0, 1, 2}, 3, 0},
		{"continuation", &query.PageRequest{Key: evidence[3].Hash(), Limit: 3}, []int{3, 4, 5}, 6, 0},
		{"offset_and_total", &query.PageRequest{Offset: 7, Limit: 3, CountTotal: true}, []int{7, 8, 9}, 10, collectionSize},
		{"reverse", &query.PageRequest{Limit: 3, Reverse: true}, []int{2047, 2046, 2045}, 2044, 0},
		{"reverse_continuation", &query.PageRequest{Key: evidence[2044].Hash(), Limit: 3, Reverse: true}, []int{2044, 2043, 2042}, 2041, 0},
		{"last", &query.PageRequest{Offset: 2046, Limit: 3}, []int{2046, 2047}, -1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cdc.decodes = 0
			res, err := client.AllEvidence(context.Background(), types.NewQueryAllEvidenceRequest(tc.page))
			require.NoError(t, err)
			require.Equal(t, len(tc.indices), cdc.decodes, "only the returned page may be decoded")
			require.Len(t, res.Evidence, len(tc.indices))
			require.NotNil(t, res.Pagination)
			require.Equal(t, tc.total, res.Pagination.Total)
			if tc.next < 0 {
				require.Empty(t, res.Pagination.NextKey)
			} else {
				require.Equal(t, []byte(evidence[tc.next].Hash()), res.Pagination.NextKey)
			}
			for i, index := range tc.indices {
				want, err := codectypes.NewAnyWithValue(evidence[index])
				require.NoError(t, err)
				require.Equal(t, want.TypeUrl, res.Evidence[i].TypeUrl)
				require.Equal(t, want.Value, res.Evidence[i].Value)
				var decoded exported.Evidence
				require.NoError(t, registry.UnpackAny(res.Evidence[i], &decoded))
				require.Equal(t, evidence[index], decoded)
			}
		})
	}
}

func TestRepairM_603_NodeRouter(t *testing.T) {
	app := paxapp.Setup(t, true, false, false)
	t.Cleanup(func() { require.NoError(t, app.Close()) })
	ctx := app.NewUncachedContext(true, tmproto.Header{Height: 1, ChainID: "pax-test"})
	evidence := repairM603Evidence(1)
	msg, err := types.NewMsgSubmitEvidence(sdk.AccAddress(bytes.Repeat([]byte{0x60}, 20)), evidence)
	require.NoError(t, err)
	wire, err := app.AppCodec().Marshal(msg)
	require.NoError(t, err)
	var received types.MsgSubmitEvidence
	require.NoError(t, app.AppCodec().Unmarshal(wire, &received))
	require.NoError(t, received.ValidateBasic())
	require.Equal(t, evidence, received.GetEvidence())
	handler := app.MsgServiceRouter().Handler(&received)
	require.NotNil(t, handler, "use the Msg service registered by the actual node constructor")
	var result *sdk.Result
	require.NotPanics(t, func() { result, err = handler(ctx, &received) })
	require.Nil(t, result)
	require.ErrorIs(t, err, types.ErrNoEvidenceHandlerExists)
	space, code, _ := sdkerrors.ABCIInfo(err, false)
	require.Equal(t, types.ModuleName, space)
	require.Equal(t, uint32(2), code)
	stored, found := app.EvidenceKeeper.GetEvidence(ctx, evidence.Hash())
	require.False(t, found)
	require.Nil(t, stored)
	route, err := app.EvidenceKeeper.GetEvidenceHandler(evidence.Route())
	require.Nil(t, route)
	require.ErrorIs(t, err, types.ErrNoEvidenceHandlerExists)
	require.Panics(t, func() { app.EvidenceKeeper.SetRouter(types.NewRouter()) }, "node startup must explicitly configure its routing policy")
}

func TestRepairM_603_RouterPolicy(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	types.RegisterInterfaces(registry)
	k := keeper.NewKeeper(codec.NewProtoCodec(registry), sdk.NewKVStoreKey(types.StoreKey), nil, nil)
	handler, err := k.GetEvidenceHandler(types.RouteEquivocation)
	require.Nil(t, handler)
	require.ErrorIs(t, err, types.ErrNoEvidenceHandlerExists)
	require.Panics(t, func() { k.SetRouter(nil) })
	router := types.NewRouter()
	k.SetRouter(router)
	require.True(t, router.Sealed())
	require.Panics(t, func() { router.AddRoute(types.RouteEquivocation, nil) })
	require.Panics(t, func() { k.SetRouter(types.NewRouter()) })
	handler, err = k.GetEvidenceHandler(types.RouteEquivocation)
	require.Nil(t, handler)
	require.ErrorIs(t, err, types.ErrNoEvidenceHandlerExists)
}
