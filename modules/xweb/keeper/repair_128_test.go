package keeper_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/encoding"
	grpcproto "google.golang.org/grpc/encoding/proto"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// repair128Producer applies the paid program call through the real bank and
// keeper in the caller's transaction. Admission failure discards its fee leg.
func repair128Producer(t *testing.T, a *app.App, ctx sdk.Context, id uint64) (error, []byte) {
	t.Helper()
	principal := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	program, asset := types.Hash32{0x81}, types.Hash32{0x71}
	payload := []byte("https://example.test/repair128")
	event := binary.BigEndian.AppendUint64(nil, id)
	event = append(event, types.KindFetch)
	event = binary.BigEndian.AppendUint32(event, uint32(len(payload)))
	event = append(event, payload...)
	cached, write := ctx.CacheContext()
	coins := sdk.NewCoins(sdk.NewInt64Coin(types.ProgramFeeDenomUSDL, 7))
	require.NoError(t, a.BankKeeper.SendCoins(cached, principal, keeper.ProgramFeeAccount(asset), coins))
	call := &keeper.ProgramCall{ProgramID: program, Principal: principal, Sequence: id,
		Legs: []keeper.ProgramLeg{{From: principal, To: keeper.ProgramFeeAccount(asset), Asset: asset,
			Denom: types.ProgramFeeDenomUSDL, Amount: sdk.NewInt(7)}}}
	err := a.XWebKeeper.RequestFromProgram(cached, call, []byte(keeper.ProgramRequestTopic), event)
	if err == nil {
		write()
	}
	return err, event
}

func repair128Initialize(t *testing.T, a *app.App, ctx sdk.Context) []xwebtestutil.Attestor {
	t.Helper()
	keys := xwebtestutil.Attestors(3)
	genesis := types.DefaultGenesis()
	genesis.Paused = false
	genesis.Params.KernelNetworkId = 7
	genesis.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomUSDL}
	genesis.Attestors = xwebtestutil.Set(keys, 3)
	a.XWebKeeper.InitGenesis(ctx, *genesis)
	a.UpgradeKeeper.SetDone(ctx, keeper.ProgramOriginPlan)
	coins := sdk.NewCoins(sdk.NewInt64Coin(types.ProgramFeeDenomUSDL, 10000))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20)), coins))
	return keys
}

func repair128Observation(t *testing.T, id uint64, keys []xwebtestutil.Attestor) (types.Observation, []byte) {
	t.Helper()
	response := bytes.Repeat([]byte{byte(id), 0, 0xfe}, 800)
	observation := types.Observation{Origin: types.OriginProgram, NetworkID: 7,
		ProgramID: types.Hash32{0x81}, RequestID: id, Kind: types.KindFetch,
		PayloadHash:   types.Keccak([]byte("https://example.test/repair128")),
		ContentDigest: types.Keccak(response), FullLength: 4000, Response: response}
	return observation, repair128Signed(t, observation, keys)
}

func repair128Signed(t *testing.T, observation types.Observation, keys []xwebtestutil.Attestor) []byte {
	t.Helper()
	digest := types.Digest(types.Attestation{Origin: types.OriginProgram, NetworkID: big.NewInt(7),
		Requester: observation.ProgramID, RequestID: observation.RequestID, Kind: observation.Kind, PayloadHash: observation.PayloadHash,
		ContentDigest: observation.ContentDigest, ResponseHash: types.Keccak(observation.Response), FullLength: observation.FullLength})
	observation.Signatures = xwebtestutil.Sign(digest, keys...)
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	return raw
}

func repair128AssertAnswer(t *testing.T, observation types.Observation, answer *types.QueryProgramResultResponse) {
	t.Helper()
	require.Equal(t, observation.ContentDigest, answer.ContentDigest)
	require.Equal(t, observation.FullLength, answer.FullLength)
	require.Equal(t, uint32(len(observation.Response)), answer.ResponseLength)
	require.Equal(t, observation.Response, answer.Response)
}

func TestRepair_128_ProgramResultProof(t *testing.T) {
	TestProofGoApiProducerTransportConsumer(t)
}

func TestRepair_128_ProgramResultGRPC(t *testing.T) {
	home := filepath.Join(t.TempDir(), "node")
	a, ctx := proofGoAPIApp(t, home, true)
	defer func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	}()
	keys := repair128Initialize(t, a, ctx)
	err, _ := repair128Producer(t, a, ctx, 1)
	require.NoError(t, err)
	observation, raw := repair128Observation(t, 1, keys)
	require.NoError(t, a.XWebKeeper.FulfilProgram(ctx, raw))
	err, _ = repair128Producer(t, a, ctx, 2)
	require.NoError(t, err)
	empty := observation
	empty.RequestID = 2
	empty.ContentDigest = types.Keccak(nil)
	empty.FullLength, empty.Response = 0, nil
	require.NoError(t, a.XWebKeeper.FulfilProgram(ctx, repair128Signed(t, empty, keys)))
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, a.Close())
	a = nil
	a, ctx = proofGoAPIApp(t, home, false)

	// Both registered transports consume exactly the persisted producer result.
	request := &types.QueryProgramResultRequest{ProgramId: observation.ProgramID[:], RequestId: 1}
	router := baseapp.NewQueryServerTestHelper(ctx, a.InterfaceRegistry())
	xweb.NewAppModule(a.XWebKeeper).RegisterServices(module.NewConfigurator(a.AppCodec(), router, router))
	answer, err := types.NewQueryClient(router).ProgramResult(context.Background(), request)
	require.NoError(t, err)
	repair128AssertAnswer(t, observation, answer)
	client, stop := proofGoAPIQueries(t, a, ctx)
	defer stop()
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	answer, err = client.ProgramResult(queryCtx, request)
	require.NoError(t, err)
	repair128AssertAnswer(t, observation, answer)

	// Inspect bytes from the registered ABCI route with an independent wire
	// parser: digest is field 1, length-delimited bytes of exactly 32 bytes.
	data, err := request.Marshal()
	require.NoError(t, err)
	routed, err := router.Route("/paxprotocol.paxchain.xweb.Query/ProgramResult")(ctx, abci.RequestQuery{Data: data})
	require.NoError(t, err)
	number, wireType, size := protowire.ConsumeTag(routed.Value)
	require.Equal(t, protowire.Number(1), number)
	require.Equal(t, protowire.BytesType, wireType)
	require.Positive(t, size)
	digest, consumed := protowire.ConsumeBytes(routed.Value[size:])
	require.Positive(t, consumed)
	require.Equal(t, observation.ContentDigest[:], digest)
	generated, err := answer.Marshal()
	require.NoError(t, err)
	require.Equal(t, generated, routed.Value)
	codec := encoding.GetCodec(grpcproto.Name)
	emptyRequest := &types.QueryProgramResultRequest{ProgramId: observation.ProgramID[:], RequestId: 2}
	emptyData, err := emptyRequest.Marshal()
	require.NoError(t, err)
	emptyRouted, err := router.Route("/paxprotocol.paxchain.xweb.Query/ProgramResult")(ctx, abci.RequestQuery{Data: emptyData})
	require.NoError(t, err)
	// The same consumer object must lose the prior scalar and byte fields
	// when the next real response omits their zero values on the wire.
	require.NoError(t, codec.Unmarshal(emptyRouted.Value, answer))
	require.Equal(t, empty.ContentDigest, answer.ContentDigest)
	require.Zero(t, answer.FullLength)
	require.Zero(t, answer.ResponseLength)
	require.Empty(t, answer.Response)
	require.Error(t, codec.Unmarshal([]byte{0x0a, 0x20, 0x01}, &types.QueryProgramResultResponse{}))
	// A Google-generated message without SDK methods keeps its standard codec.
	modern := wrapperspb.Bytes([]byte{1, 2, 3})
	data, err = codec.Marshal(modern)
	require.NoError(t, err)
	var decoded wrapperspb.BytesValue
	require.NoError(t, codec.Unmarshal(data, &decoded))
	require.Equal(t, modern.Value, decoded.Value)
}

func TestRepair_128_CommittedCapacityLifetime(t *testing.T) {
	home := filepath.Join(t.TempDir(), "node")
	a, ctx := proofGoAPIApp(t, home, true)
	defer func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	}()
	keys := repair128Initialize(t, a, ctx)
	var firstRaw []byte
	for id := uint64(1); id <= 2*keeper.ProgramCommittedCapacity+1; id++ {
		err, _ := repair128Producer(t, a, ctx, id)
		require.NoError(t, err)
		if id > keeper.ProgramCommittedCapacity {
			require.Equal(t, uint64(63), a.XWebKeeper.ProgramCommittedCount(ctx), "admission must reserve hot capacity before settlement")
		}
		observation, raw := repair128Observation(t, id, keys)
		require.NoError(t, a.XWebKeeper.FulfilProgram(ctx, raw))
		if id == 1 {
			firstRaw = raw
		}
		result, err := a.XWebKeeper.GetProgramResult(ctx, observation.ProgramID, id)
		require.NoError(t, err)
		require.Equal(t, observation.Response, result.Response)
		require.LessOrEqual(t, a.XWebKeeper.ProgramCommittedCount(ctx), uint64(keeper.ProgramCommittedCapacity))
		require.Zero(t, a.XWebKeeper.ProgramPendingCount(ctx))
		message, broken := keeper.ProgramCountersInvariant(a.XWebKeeper)(ctx)
		require.False(t, broken, message)
		// Commit at saturation, stop the producer and reopen its disk stores before
		// the first request beyond the old lifetime bound.
		if id == keeper.ProgramCommittedCapacity {
			a.SetDeliverStateToCommit()
			_, err = a.Commit(context.Background())
			require.NoError(t, err)
			require.NoError(t, a.Close())
			a = nil
			a, ctx = proofGoAPIApp(t, home, false)
			// Subsequent writes use a real cache over the recovered multistore.
			ctx, _ = ctx.CacheContext()
		}
	}
	// Seal the recovered application's writable state for a second disk reopen.
	require.NoError(t, a.SetSealedStateToCommit(ctx.WithBlockHeight(2), ctx.MultiStore().(sdk.CacheMultiStore)))
	_, err := a.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, a.Close())
	a = nil
	a, ctx = proofGoAPIApp(t, home, false)
	require.Equal(t, uint64(64), a.XWebKeeper.ProgramCommittedCount(ctx))
	var headers uint64
	for _, entry := range a.XWebKeeper.ProgramAnswerEntries(ctx) {
		if entry.Part == 0 {
			headers++
		}
	}
	require.Equal(t, uint64(64), headers)
	client, stop := proofGoAPIQueries(t, a, ctx)
	defer stop()
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for id := uint64(1); id <= 2*keeper.ProgramCommittedCapacity+1; id++ {
		observation, _ := repair128Observation(t, id, keys)
		answer, err := client.ProgramResult(queryCtx, &types.QueryProgramResultRequest{ProgramId: observation.ProgramID[:], RequestId: id})
		require.NoError(t, err)
		repair128AssertAnswer(t, observation, answer)
	}
	principal := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	asset, program := types.Hash32{0x71}, types.Hash32{0x81}
	require.True(t, a.BankKeeper.GetBalance(ctx, keeper.ProgramFeeAccount(asset), types.ProgramFeeDenomUSDL).IsZero())
	for index, key := range keys {
		share := int64(2)
		if index == 0 {
			share = 3
		}
		require.Equal(t, sdk.NewInt(share*129), a.BankKeeper.GetBalance(ctx, key.Payout, types.ProgramFeeDenomUSDL).Amount)
	}
	before := a.BankKeeper.GetBalance(ctx, principal, types.ProgramFeeDenomUSDL)
	require.ErrorContains(t, a.XWebKeeper.FulfilProgram(ctx, firstRaw), "LXP_SEQUENCE_REUSED")
	err, _ = repair128Producer(t, a, ctx, 1)
	require.ErrorContains(t, err, "LXP_SEQUENCE_REUSED")
	require.Equal(t, before, a.BankKeeper.GetBalance(ctx, principal, types.ProgramFeeDenomUSDL))
	record, found, err := a.XWebKeeper.GetProgramPendingRecord(ctx, program, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, record.Fulfilled)
	damaged, _ := ctx.CacheContext()
	archivedChunk := append([]byte("web/archive/"), program[:]...)
	archivedChunk = binary.BigEndian.AppendUint64(archivedChunk, 1)
	archivedChunk = append(archivedChunk, 1)
	damaged.KVStore(a.GetKey(types.StoreKey)).Delete(archivedChunk)
	_, err = a.XWebKeeper.GetProgramResult(damaged, program, 1)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	message, broken := keeper.ProgramCountersInvariant(a.XWebKeeper)(damaged)
	require.True(t, broken, message)
	message, broken = keeper.ProgramCountersInvariant(a.XWebKeeper)(ctx)
	require.False(t, broken, message)
}

func TestRepair_128_AdmissionReservesCapacity(t *testing.T) {
	s := newProgramSuite(t)
	for id := uint64(1); id <= keeper.ProgramPendingCapacity; id++ {
		require.NoError(t, s.run(newCall(feeLeg(programPrincipal, paxAsset, sdk.NewInt(7))), webRequest(id)))
	}
	before := s.balanceOf(programPrincipal, paxAsset)
	feeBefore := s.balanceOf(keeper.ProgramFeeAccount(paxAsset.id), paxAsset)
	err := s.run(newCall(feeLeg(programPrincipal, paxAsset, sdk.NewInt(7))), webRequest(keeper.ProgramPendingCapacity+1))
	require.EqualError(t, err, "LXP_ARENA_EXHAUSTED")
	s.requireNoRecord(keeper.ProgramPendingCapacity + 1)
	require.Equal(t, before, s.balanceOf(programPrincipal, paxAsset))
	require.Equal(t, feeBefore, s.balanceOf(keeper.ProgramFeeAccount(paxAsset.id), paxAsset))
	require.Equal(t, uint64(256), s.k.ProgramPendingCount(s.ctx))
	counters, err := keeper.NewQueryServer(s.k).ProgramCounters(sdk.WrapSDKContext(s.ctx), &types.QueryProgramCountersRequest{})
	require.NoError(t, err)
	require.Equal(t, uint64(256), counters.PendingCapacity)
	require.Equal(t, uint64(64), counters.CommittedCapacity)
}

func TestRepair_128_AdmissionReclamationAtomic(t *testing.T) {
	home := filepath.Join(t.TempDir(), "node")
	a, ctx := proofGoAPIApp(t, home, true)
	defer func() { require.NoError(t, a.Close()) }()
	keys := repair128Initialize(t, a, ctx)
	for id := uint64(1); id <= keeper.ProgramCommittedCapacity; id++ {
		err, _ := repair128Producer(t, a, ctx, id)
		require.NoError(t, err)
		_, raw := repair128Observation(t, id, keys)
		require.NoError(t, a.XWebKeeper.FulfilProgram(ctx, raw))
	}
	program, asset := types.Hash32{0x81}, types.Hash32{0x71}
	principal := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	store := ctx.KVStore(a.GetKey(types.StoreKey))
	// A damaged completed answer cannot be reclaimed or silently discarded.
	chunkKey := keeper.ProgramAnswerKey(program, 1, 1)
	chunk := bytes.Clone(store.Get(chunkKey))
	store.Delete(chunkKey)
	header := bytes.Clone(store.Get(keeper.ProgramAnswerKey(program, 1, 0)))
	balance := a.BankKeeper.GetBalance(ctx, principal, types.ProgramFeeDenomUSDL)
	feeBalance := a.BankKeeper.GetBalance(ctx, keeper.ProgramFeeAccount(asset), types.ProgramFeeDenomUSDL)
	err, _ := repair128Producer(t, a, ctx, 65)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	require.Equal(t, balance, a.BankKeeper.GetBalance(ctx, principal, types.ProgramFeeDenomUSDL))
	require.Equal(t, feeBalance, a.BankKeeper.GetBalance(ctx, keeper.ProgramFeeAccount(asset), types.ProgramFeeDenomUSDL))
	require.Equal(t, header, store.Get(keeper.ProgramAnswerKey(program, 1, 0)))
	require.Equal(t, uint64(64), a.XWebKeeper.ProgramCommittedCount(ctx))
	require.Zero(t, a.XWebKeeper.ProgramPendingCount(ctx))
	_, found, err := a.XWebKeeper.GetProgramPendingRecord(ctx, program, 65)
	require.NoError(t, err)
	require.False(t, found)
	store.Set(chunkKey, chunk)
	err, _ = repair128Producer(t, a, ctx, 65)
	require.NoError(t, err)
	require.Equal(t, uint64(63), a.XWebKeeper.ProgramCommittedCount(ctx))
	require.Equal(t, uint64(1), a.XWebKeeper.ProgramPendingCount(ctx))
	first, err := a.XWebKeeper.GetProgramResult(ctx, program, 1)
	require.NoError(t, err)
	observation, _ := repair128Observation(t, 1, keys)
	require.Equal(t, observation.Response, first.Response)
	_, raw := repair128Observation(t, 65, keys)
	require.NoError(t, a.XWebKeeper.FulfilProgram(ctx, raw))
	message, broken := keeper.ProgramCountersInvariant(a.XWebKeeper)(ctx)
	require.False(t, broken, message)
}

func TestRepair_128_LegacyAcceptedRecovery(t *testing.T) {
	home := filepath.Join(t.TempDir(), "node")
	a, ctx := proofGoAPIApp(t, home, true)
	defer func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	}()
	keys := repair128Initialize(t, a, ctx)
	for id := uint64(1); id <= keeper.ProgramCommittedCapacity; id++ {
		err, _ := repair128Producer(t, a, ctx, id)
		require.NoError(t, err)
		_, raw := repair128Observation(t, id, keys)
		require.NoError(t, a.XWebKeeper.FulfilProgram(ctx, raw))
	}
	// Recover the pre-upgrade layout: 64 hot answers and a paid pending
	// request. Its canonical record comes from the real admission producer;
	// only the old absence of a hot-slot reservation is reconstructed.
	legacy := a.XWebKeeper.ExportGenesis(ctx)
	branch, _ := ctx.CacheContext()
	err, _ := repair128Producer(t, a, branch, 65)
	require.NoError(t, err)
	program, asset := types.Hash32{0x81}, types.Hash32{0x71}
	record, found, err := a.XWebKeeper.GetProgramPendingRecord(branch, program, 65)
	require.NoError(t, err)
	require.True(t, found)
	encoded, err := record.Bytes()
	require.NoError(t, err)
	legacy.ProgramPending = append(legacy.ProgramPending, types.ProgramPendingEntry{ProgramID: program, RequestID: 65, Record: encoded})
	legacy.ProgramPendingCount = 1
	require.NoError(t, legacy.Validate())
	a.XWebKeeper.InitGenesis(ctx, legacy)
	principal := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	coins := sdk.NewCoins(sdk.NewInt64Coin(types.ProgramFeeDenomUSDL, 7))
	require.NoError(t, a.BankKeeper.SendCoins(ctx, principal, keeper.ProgramFeeAccount(asset), coins))
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.NoError(t, a.Close())
	a = nil
	a, ctx = proofGoAPIApp(t, home, false)
	ctx, _ = ctx.CacheContext()
	require.Equal(t, uint64(64), a.XWebKeeper.ProgramCommittedCount(ctx))
	require.Equal(t, uint64(1), a.XWebKeeper.ProgramPendingCount(ctx))
	_, raw := repair128Observation(t, 65, keys)
	// A genuine settlement failure must also roll back archive reclamation.
	require.Equal(t, sdk.NewInt(7), a.BankKeeper.GetBalance(ctx, keeper.ProgramFeeAccount(asset), types.ProgramFeeDenomUSDL).Amount)
	require.NoError(t, a.BankKeeper.SendCoins(ctx, keeper.ProgramFeeAccount(asset), principal, coins))
	require.True(t, a.BankKeeper.GetBalance(ctx, keeper.ProgramFeeAccount(asset), types.ProgramFeeDenomUSDL).IsZero())
	store := ctx.KVStore(a.GetKey(types.StoreKey))
	firstHeader := bytes.Clone(store.Get(keeper.ProgramAnswerKey(program, 1, 0)))
	require.Error(t, a.XWebKeeper.FulfilProgram(ctx, raw))
	require.Equal(t, firstHeader, store.Get(keeper.ProgramAnswerKey(program, 1, 0)))
	require.Equal(t, uint64(64), a.XWebKeeper.ProgramCommittedCount(ctx))
	require.Equal(t, uint64(1), a.XWebKeeper.ProgramPendingCount(ctx))
	require.NoError(t, a.BankKeeper.SendCoins(ctx, principal, keeper.ProgramFeeAccount(asset), coins))
	require.NoError(t, a.XWebKeeper.FulfilProgram(ctx, raw))
	require.Zero(t, a.XWebKeeper.ProgramPendingCount(ctx))
	require.Equal(t, uint64(64), a.XWebKeeper.ProgramCommittedCount(ctx))
	require.True(t, a.BankKeeper.GetBalance(ctx, keeper.ProgramFeeAccount(asset), types.ProgramFeeDenomUSDL).IsZero())
	for _, id := range []uint64{1, 65} {
		observation, _ := repair128Observation(t, id, keys)
		result, err := a.XWebKeeper.GetProgramResult(ctx, program, id)
		require.NoError(t, err)
		require.Equal(t, observation.Response, result.Response)
	}
	require.ErrorContains(t, a.XWebKeeper.FulfilProgram(ctx, raw), "LXP_SEQUENCE_REUSED")
	message, broken := keeper.ProgramCountersInvariant(a.XWebKeeper)(ctx)
	require.False(t, broken, message)
}
