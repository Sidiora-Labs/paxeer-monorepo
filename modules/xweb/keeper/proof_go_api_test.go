// Proves: keeper.Keeper.FulfilProgram and ExportGenesis -> committed application stores and AppModule.RegisterServices -> QueryClient, xweb.Precompile and teeattestor.TeeAttestorPrecompile.
//
// The producers include InitGenesis, ErrProgramModuleDisabled, GetProgramResult,
// NewQueryServer, ProgramAnswerEntries and ProgramCountersInvariant. The test
// closes the application and its stores before constructing fresh consumers.
// It repairs ProgramCountersInvariant in invariants.go, which accepted missing
// answer chunks, and App.xwebLive in node/upgrades.go, which served an upgraded
// module even when its genesis parameters were absent. Genesis JSON is produced
// by AppModule.ExportGenesis and replayed by AppModule.InitGenesis.
//
// Successful custom-type query responses cross the registered protobuf gRPC
// transport after disk reopen, using the same generated query client as callers.
package keeper_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"math"
	"math/big"
	"net"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/teeattestor"
	pxweb "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// proofGoAPIApp opens the repository's application with its production store
// implementation. Both the application metadata and SC/SS stores use this home.
func proofGoAPIApp(t *testing.T, home string, initialize bool) (*app.App, sdk.Context) {
	t.Helper()
	db, err := dbm.NewGoLevelDB("application", home)
	require.NoError(t, err, "node/app.go: application metadata producer")
	a := app.New(db, nil, true, map[int64]bool{}, home, 1, true,
		config.TestConfig(), app.MakeEncodingConfig(), nil, app.TestAppOpts{}, app.EmptyWasmOpts, nil)
	header := tmproto.Header{Height: 1, ChainID: "pax-test", Time: time.Unix(1800000000, 0).UTC()}
	if initialize {
		raw, err := json.Marshal(app.NewDefaultGenesisState(a.AppCodec()))
		require.NoError(t, err)
		_, err = a.InitChain(context.Background(), &abci.RequestInitChain{
			Time: header.Time, ChainId: header.ChainID, ConsensusParams: app.DefaultConsensusParams, AppStateBytes: raw,
		})
		require.NoError(t, err)
		return a, a.NewContext(false, header).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
	}
	return a, a.NewUncachedContext(false, header).WithGasMeter(sdk.NewInfiniteGasMeter(1, 1))
}

// The server only supplies the same SDK context that the application query
// router supplies. Protobuf encoding, registration and dispatch are real gRPC.
func proofGoAPIQueries(t *testing.T, a *app.App, ctx sdk.Context) (types.QueryClient, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "modules/xweb/module.go: query listener producer")
	server := grpc.NewServer(grpc.UnaryInterceptor(func(_ context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		return handler(sdk.WrapSDKContext(ctx), req)
	}))
	xweb.NewAppModule(a.XWebKeeper).RegisterServices(module.NewConfigurator(a.AppCodec(), server, server))
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	dialCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	require.NoError(t, err)
	return types.NewQueryClient(conn), func() {
		require.NoError(t, conn.Close())
		server.Stop()
		require.NoError(t, <-served)
	}
}

func proofGoAPIVM(t *testing.T, a *app.App, ctx sdk.Context) (*vm.EVM, *pxweb.Precompile, *teeattestor.TeeAttestorPrecompile) {
	t.Helper()
	web, err := pxweb.NewPrecompile(a.GetPrecompileKeepers())
	require.NoError(t, err)
	tee, err := teeattestor.NewPrecompile(&a.XWebKeeper, &a.EvmKeeper, &a.UpgradeKeeper, keeper.ProgramOriginPlan)
	require.NoError(t, err)
	block, err := a.EvmKeeper.GetVMBlockContext(ctx, core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(a.EvmKeeper.ChainID(ctx))
	evm := vm.NewEVM(*block, state.NewDBImpl(ctx, &a.EvmKeeper, false), chain, vm.Config{},
		map[common.Address]vm.PrecompiledContract{web.Address(): web, tee.Address(): tee})
	return evm, web, tee
}

func proofGoAPIView(t *testing.T, evm *vm.EVM, addr common.Address, contract abi.ABI, name string, args ...interface{}) []interface{} {
	t.Helper()
	input, err := contract.Pack(name, args...)
	require.NoError(t, err)
	output, _, err := evm.StaticCall(common.HexToAddress("0xa111"), addr, input, 2000000)
	require.NoError(t, err, name)
	decoded, err := contract.Unpack(name, output)
	require.NoError(t, err)
	return decoded
}

func TestProofGoApiProducerTransportConsumer(t *testing.T) {
	home := filepath.Join(t.TempDir(), "node")
	a, ctx := proofGoAPIApp(t, home, true)
	// A closure closes the currently live instance even if an assertion fails.
	defer func() {
		if a != nil {
			require.NoError(t, a.Close())
		}
	}()
	keys := xwebtestutil.Attestors(5)
	genesis := types.DefaultGenesis()
	genesis.Params.KernelNetworkId = 7
	genesis.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomUSDL}
	genesis.Attestors = xwebtestutil.Set(keys, 3)
	genesis.Paused = false
	rawGenesis, err := json.Marshal(genesis)
	require.NoError(t, err)
	am := xweb.NewAppModule(a.XWebKeeper)
	require.Empty(t, am.InitGenesis(ctx, a.AppCodec(), rawGenesis))
	require.True(t, ctx.KVStore(a.GetKey(types.StoreKey)).Has(types.ParamsKey))
	require.ErrorIs(t, a.XWebKeeper.FulfilProgram(ctx, nil), keeper.ErrProgramModuleDisabled)
	require.False(t, a.XWebKeeper.ProgramOriginActive(ctx))
	a.UpgradeKeeper.SetDone(ctx, keeper.ProgramOriginPlan)
	a.UpgradeKeeper.SetDone(ctx, precompiles.XWebUpgrade)
	require.True(t, a.XWebKeeper.ProgramOriginActive(ctx))

	asset, program := types.Hash32{0x71}, types.Hash32{0x81}
	principal := sdk.AccAddress(bytes.Repeat([]byte{0x42}, 20))
	coins := sdk.NewCoins(sdk.NewInt64Coin(types.ProgramFeeDenomUSDL, 76))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", coins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", principal, coins))
	feeAccount := keeper.ProgramFeeAccount(asset)
	require.NoError(t, a.BankKeeper.SendCoins(ctx, principal, feeAccount, coins))
	payload := []byte("https://example.test/proof")
	const requestID uint64 = 0x0102030405060708
	event := binary.BigEndian.AppendUint64(nil, requestID)
	event = append(event, types.KindFetch)
	event = binary.BigEndian.AppendUint32(event, uint32(len(payload)))
	event = append(event, payload...)
	call := &keeper.ProgramCall{ProgramID: program, Principal: principal, Sequence: 17,
		Legs: []keeper.ProgramLeg{{From: principal, To: feeAccount, Asset: asset, Denom: types.ProgramFeeDenomUSDL, Amount: sdk.NewInt(76)}}}
	require.NoError(t, a.XWebKeeper.RequestFromProgram(ctx, call, []byte(keeper.ProgramRequestTopic), event))
	require.Equal(t, uint64(1), a.XWebKeeper.ProgramPendingCount(ctx))
	response := bytes.Repeat([]byte{0x41, 0, 0xfe}, 800)
	observation := types.Observation{Origin: types.OriginProgram, NetworkID: 7, ProgramID: program,
		RequestID: requestID, Kind: types.KindFetch, PayloadHash: types.Keccak(payload),
		ContentDigest: types.Keccak(response), FullLength: 4000, Response: response}
	digest := types.Digest(types.Attestation{Origin: types.OriginProgram, NetworkID: big.NewInt(7),
		Requester: program, RequestID: requestID, Kind: observation.Kind, PayloadHash: observation.PayloadHash,
		ContentDigest: observation.ContentDigest, ResponseHash: types.Keccak(response), FullLength: observation.FullLength})
	observation.Signatures = xwebtestutil.Sign(digest, keys[:3]...)
	raw, err := types.EncodeObservation(observation)
	require.NoError(t, err)
	// The first write crosses the very same ABI entry used by the EVM consumer.
	evm, web, _ := proofGoAPIVM(t, a, ctx)
	input, err := web.GetABI().Pack(pxweb.FulfilProgramMethod, raw)
	require.NoError(t, err)
	_, _, err = web.RunAndCalculateGas(evm, common.HexToAddress("0xa111"), web.Address(), input, 2000000, nil, nil, false, false)
	require.NoError(t, err)
	ctx = state.GetDBImpl(evm.StateDB).Ctx()
	for i, share := range []int64{26, 25, 25, 0, 0} {
		require.Equal(t, sdk.NewInt(share), a.BankKeeper.GetBalance(ctx, keys[i].Payout, types.ProgramFeeDenomUSDL).Amount)
	}
	require.True(t, a.BankKeeper.GetBalance(ctx, feeAccount, types.ProgramFeeDenomUSDL).IsZero())

	originFee := genesis.Params.Fee
	originPayer := a.EvmKeeper.GetPaxAddressOrDefault(ctx, requester)
	originCoins := sdk.NewCoins(sdk.NewCoin(sdk.MustGetBaseDenom(), originFee.AddRaw(1)))
	require.NoError(t, a.BankKeeper.MintCoins(ctx, "mint", originCoins))
	require.NoError(t, a.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", originPayer, originCoins))
	originID, err := a.XWebKeeper.Request(ctx, requester, types.KindFetch, payload, callbackGas, originFee)
	require.NoError(t, err)
	originRequest, found := a.XWebKeeper.GetRequest(ctx, originID)
	require.True(t, found)
	require.NoError(t, originRequest.Validate())
	originResponse := []byte{0, 0xff, 0x42, 0, 0xfe}
	originDigest := types.Keccak(originResponse)
	originAttestation := types.Attestation{Origin: types.OriginEVM, NetworkID: a.EvmKeeper.ChainID(ctx),
		Requester: types.EVMRequester(types.Address20(requester)), RequestID: originID,
		Kind: types.KindFetch, PayloadHash: types.Keccak(payload), ContentDigest: originDigest,
		ResponseHash: types.Keccak(originResponse), FullLength: 9000}
	originRequest, originResult, err := a.XWebKeeper.Fulfil(ctx, originID, originResponse, originDigest, 9000,
		xwebtestutil.Sign(types.Digest(originAttestation), keys[:3]...))
	require.NoError(t, err)
	require.NoError(t, a.XWebKeeper.RecordCallback(ctx, originID, types.CallbackDelivered, 21_000))
	originResult.Callback, originResult.CallbackGasUsed = types.CallbackDelivered, 21_000
	require.Equal(t, sdk.OneInt(), a.BankKeeper.GetBalance(ctx, originPayer, sdk.MustGetBaseDenom()).Amount)
	require.True(t, a.BankKeeper.GetBalance(ctx, a.XWebKeeper.ModuleAddress(), sdk.MustGetBaseDenom()).IsZero())
	for i, share := range []int64{333334, 333333, 333333, 0, 0} {
		require.Equal(t, sdk.NewInt(share), a.BankKeeper.GetBalance(ctx, keys[i].Payout, sdk.MustGetBaseDenom()).Amount)
	}
	exported := a.XWebKeeper.ExportGenesis(ctx)
	require.NoError(t, exported.Validate())
	require.Len(t, a.XWebKeeper.ProgramAnswerEntries(ctx), 4)
	require.Equal(t, exported.ProgramAnswers, a.XWebKeeper.ProgramAnswerEntries(ctx))
	result, err := a.XWebKeeper.GetProgramResult(ctx, program, requestID)
	require.NoError(t, err)
	require.Equal(t, keeper.ProgramResult{ContentDigest: observation.ContentDigest, FullLength: 4000,
		ResponseLength: uint32(len(response)), Response: response}, result)
	owned := a.XWebKeeper.ProgramAnswerEntries(ctx)
	owned[0].Value[0] ^= 0xff
	require.Equal(t, exported.ProgramAnswers, a.XWebKeeper.ProgramAnswerEntries(ctx))
	moduleGenesis := am.ExportGenesis(ctx, a.AppCodec())
	var decoded types.GenesisState
	require.NoError(t, json.Unmarshal(moduleGenesis, &decoded))
	require.Equal(t, exported, decoded)
	message, broken := keeper.ProgramCountersInvariant(a.XWebKeeper)(ctx)
	require.False(t, broken, message)

	// Commit the actual application state, stop every producer, then reopen the
	// same home without InitChain. No genesis replay supplies the restart state.
	_, err = state.GetDBImpl(evm.StateDB).Finalize()
	require.NoError(t, err)
	a.SetDeliverStateToCommit()
	_, err = a.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(1), a.LastCommitID().Version)
	require.NoError(t, a.Close())
	a = nil
	a, ctx = proofGoAPIApp(t, home, false)
	require.Equal(t, exported, a.XWebKeeper.ExportGenesis(ctx))
	require.True(t, a.XWebKeeper.ProgramOriginActive(ctx))
	require.ErrorContains(t, a.XWebKeeper.FulfilProgram(ctx, raw), "LXP_SEQUENCE_REUSED")
	_, _, err = a.XWebKeeper.Fulfil(ctx, originID, originResponse, originDigest, 9000,
		xwebtestutil.Sign(types.Digest(originAttestation), keys[:3]...))
	require.ErrorIs(t, err, types.ErrAlreadyFulfilled)
	repeated := keeper.ProgramCall{ProgramID: program, Principal: principal, Sequence: call.Sequence, Legs: call.Legs}
	require.ErrorContains(t, a.XWebKeeper.RequestFromProgram(ctx, &repeated, []byte(keeper.ProgramRequestTopic), event), "LXP_SEQUENCE_REUSED")
	require.Equal(t, exported, a.XWebKeeper.ExportGenesis(ctx))
	for i, share := range []int64{26, 25, 25, 0, 0} {
		require.Equal(t, sdk.NewInt(share), a.BankKeeper.GetBalance(ctx, keys[i].Payout, types.ProgramFeeDenomUSDL).Amount)
	}
	require.Equal(t, sdk.OneInt(), a.BankKeeper.GetBalance(ctx, originPayer, sdk.MustGetBaseDenom()).Amount)
	require.True(t, a.BankKeeper.GetBalance(ctx, a.XWebKeeper.ModuleAddress(), sdk.MustGetBaseDenom()).IsZero())
	for i, share := range []int64{333334, 333333, 333333, 0, 0} {
		require.Equal(t, sdk.NewInt(share), a.BankKeeper.GetBalance(ctx, keys[i].Payout, sdk.MustGetBaseDenom()).Amount)
	}

	// Node activation consumes the genesis parameter and its registered store key.
	a.RefreshPrecompiles(ctx)
	require.Contains(t, a.EvmKeeper.CustomPrecompiles(ctx), common.HexToAddress(types.PrecompileAddress))
	absent, _ := ctx.CacheContext()
	absent.KVStore(a.GetKey(types.StoreKey)).Delete(types.ParamsKey)
	a.RefreshPrecompiles(absent)
	require.NotContains(t, a.EvmKeeper.CustomPrecompiles(absent), common.HexToAddress(types.PrecompileAddress))
	a.RefreshPrecompiles(ctx)

	client, stop := proofGoAPIQueries(t, a, ctx)
	queryCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	answer, err := client.ProgramResult(queryCtx, &types.QueryProgramResultRequest{ProgramId: program[:], RequestId: requestID})
	require.NoError(t, err)
	require.Equal(t, observation.ContentDigest, answer.ContentDigest)
	require.Equal(t, response, answer.Response)
	require.Equal(t, uint32(4000), answer.FullLength)
	require.Equal(t, uint32(len(response)), answer.ResponseLength)
	request, err := client.ProgramRequest(queryCtx, &types.QueryProgramRequestRequest{ProgramId: program[:], RequestId: requestID})
	require.NoError(t, err)
	pending, err := keeper.DecodeProgramPendingRecord(request.Record)
	require.NoError(t, err)
	require.True(t, pending.Fulfilled)
	require.Equal(t, asset, pending.FeeAsset)
	require.Equal(t, types.ProgramFeeDenomUSDL, request.FeeDenom)
	counters, err := client.ProgramCounters(queryCtx, &types.QueryProgramCountersRequest{})
	require.NoError(t, err)
	require.Zero(t, counters.Pending)
	require.Equal(t, uint64(1), counters.Committed)
	params, err := client.Params(queryCtx, &types.QueryParamsRequest{})
	require.NoError(t, err)
	require.Equal(t, genesis.Params, params.Params)
	attestors, err := client.Attestors(queryCtx, &types.QueryAttestorsRequest{})
	require.NoError(t, err)
	require.Equal(t, genesis.Attestors.Attestors, attestors.Attestors)
	require.Equal(t, uint32(3), attestors.Threshold)
	originRequestAnswer, err := client.Request(queryCtx, &types.QueryRequestRequest{RequestId: originID})
	require.NoError(t, err)
	require.Equal(t, originRequest, originRequestAnswer.Request)
	require.NoError(t, originRequestAnswer.Request.Validate())
	originResultAnswer, err := client.Result(queryCtx, &types.QueryResultRequest{RequestId: originID})
	require.NoError(t, err)
	require.Equal(t, originResult, originResultAnswer.Result)
	require.Equal(t, originResponse, originResultAnswer.Result.Response)
	require.Equal(t, originDigest, originResultAnswer.Result.ContentDigest)
	require.Equal(t, types.CallbackDelivered, originResultAnswer.Result.Callback)
	require.Equal(t, uint64(21_000), originResultAnswer.Result.CallbackGasUsed)
	_, err = client.ProgramResult(queryCtx, &types.QueryProgramResultRequest{ProgramId: program[:], RequestId: requestID + 1})
	require.Equal(t, codes.NotFound, status.Code(err))
	_, err = client.ProgramResult(queryCtx, &types.QueryProgramResultRequest{ProgramId: []byte{1}, RequestId: requestID})
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	stop()

	evm, web, tee := proofGoAPIVM(t, a, ctx)
	require.Equal(t, []interface{}{[32]byte(observation.ContentDigest), uint32(4000), uint32(len(response)), response},
		proofGoAPIView(t, evm, web.Address(), web.GetABI(), pxweb.GetProgramResultMethod, [32]byte(program), requestID))
	require.Equal(t, []interface{}{uint32(7)}, proofGoAPIView(t, evm, web.Address(), web.GetABI(), pxweb.GetKernelNetworkIdMethod))
	require.Equal(t, []interface{}{big.NewInt(5)}, proofGoAPIView(t, evm, tee.Address(), tee.GetABI(), teeattestor.RootCountMethod, uint8(0)))
	require.Equal(t, []interface{}{keys[0].Signer[:]}, proofGoAPIView(t, evm, tee.Address(), tee.GetABI(), teeattestor.RootOfMethod, uint8(0), big.NewInt(0)))
	input, err = web.GetABI().Pack(pxweb.GetProgramResultMethod, [32]byte(program), requestID+1)
	require.NoError(t, err)
	revert, _, err := evm.StaticCall(common.HexToAddress("0xa111"), web.Address(), input, 2000000)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	reason, err := abi.UnpackRevert(revert)
	require.NoError(t, err)
	require.Contains(t, reason, "LXP_UNKNOWN_FIELD")

	// A damaged persisted chunk must fail both the public reader and invariant.
	damaged, _ := ctx.CacheContext()
	damaged.KVStore(a.GetKey(types.StoreKey)).Delete(keeper.ProgramAnswerKey(program, requestID, 2))
	_, err = a.XWebKeeper.GetProgramResult(damaged, program, requestID)
	require.EqualError(t, err, "LXP_NON_CANONICAL")
	message, broken = keeper.ProgramCountersInvariant(a.XWebKeeper)(damaged)
	require.True(t, broken, message)
	require.Contains(t, message, "invalid program answer")
	require.Equal(t, exported, a.XWebKeeper.ExportGenesis(ctx))

	// The module's own consumer replays its own serialized export in a clean
	// store branch, independently of the disk-restart assertion above.
	replay, _ := ctx.CacheContext()
	store := replay.KVStore(a.GetKey(types.StoreKey))
	iterator := store.Iterator(nil, nil)
	var stale [][]byte
	for ; iterator.Valid(); iterator.Next() {
		stale = append(stale, bytes.Clone(iterator.Key()))
	}
	require.NoError(t, iterator.Close())
	for _, key := range stale {
		store.Delete(key)
	}
	replayModule := xweb.NewAppModule(a.XWebKeeper)
	require.Empty(t, replayModule.InitGenesis(replay, a.AppCodec(), moduleGenesis))
	require.Equal(t, exported, a.XWebKeeper.ExportGenesis(replay))
	require.Empty(t, replayModule.InitGenesis(replay, a.AppCodec(), moduleGenesis))
	require.Equal(t, exported, a.XWebKeeper.ExportGenesis(replay))
}
