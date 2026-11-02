// Proves: keeper.DefaultParams, Keeper.Schedule, Keeper.Reschedule, Keeper.ExportGenesis -> IAVL storage and types.GenesisState protobuf -> Keeper.InitGenesis, Keeper.EndBlocker and scheduler_test.executedEvents/failedEvents.
//
// The proof also crosses NewMsgServerImpl/MsgUpdateParams and Querier's exported
// Go surface using the real EVM application, bank escrow and typed event decoder.
// It closes and reopens the scheduler's committed disk store; the EVM application
// remains live. Repeated queries and event decodes must return detached results,
// and repeated end blocks must not execute a consumed job again.
//
// Repairs: keeper.go:GetParams previously accepted semantically invalid stored
// parameters; msg_server.go:UpdateParams previously omitted ValidateBasic on the
// exported Go entry point, accepting a malformed configured authority. Both now
// refuse malformed input before it can reach the scheduler.
package keeper_test

import (
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// proofAgentWire always serializes the actual producer result and decodes into
// a new consumer value; it does not substitute a message or transport codec.
func proofAgentWire(t *testing.T, producer, consumer proto.Message) []byte {
	t.Helper()
	encoded, err := proto.Marshal(producer)
	require.NoError(t, err)
	require.NoError(t, proto.Unmarshal(encoded, consumer))
	return encoded
}

func TestProofLayerxagentModule(t *testing.T) {
	t.Run("producer restart protobuf consumer and outcomes", func(t *testing.T) {
		h := newHarness(t)
		require.Equal(t, types.ModuleName, keeper.ModuleName)
		require.Equal(t, authtypes.NewModuleAddress(types.ModuleName), h.keeper.ModuleAddress())
		defaults := keeper.DefaultParams()
		require.Equal(t, types.DefaultParams(), defaults)
		require.Equal(t, uint64(8192), defaults.MaxCallDataBytes)
		require.Equal(t, uint64(30_000_000), defaults.MaxBlockGas)
		require.False(t, defaults.Paused)

		dir := t.TempDir()
		open := func() storetypes.CommitMultiStore {
			db, err := tmdb.NewGoLevelDB("layerxagent", dir)
			require.NoError(t, err)
			committed := store.NewCommitMultiStore(db)
			committed.MountStoreWithDB(h.key, sdk.StoreTypeIAVL, db)
			require.NoError(t, committed.LoadLatestVersion())
			h.ctx = withKVStore(h.ctx, h.key, committed.GetKVStore(h.key))
			h.keeper = keeper.NewKeeper(h.key, &testkeeper.EVMTestApp.EvmKeeper, precompiles.MergeUpgrade)
			return committed
		}
		committed := open()
		t.Cleanup(func() {
			if committed != nil {
				require.NoError(t, committed.Close())
			}
		})
		require.NoError(t, h.keeper.InitGenesis(h.ctx, *types.DefaultGenesis()))

		authority := authtypes.NewModuleAddress(govtypes.ModuleName).String()
		server := keeper.NewMsgServerImpl(h.keeper, authority)
		params := defaults
		params.MaxJobsPerCreator = 7
		params.MaxCallDataBytes = 16
		params.MaxJobsPerBlock = 1
		params.MaxBlockGas = params.MaxJobGasLimit
		update := &types.MsgUpdateParams{Authority: authority, Params: params}
		var decodedUpdate types.MsgUpdateParams
		proofAgentWire(t, update, &decodedUpdate)
		response, err := server.UpdateParams(sdk.WrapSDKContext(h.ctx), &decodedUpdate)
		require.NoError(t, err)
		require.NotNil(t, response)
		decodedUpdate.Authority = h.creatorAcc.String()
		response, err = server.UpdateParams(sdk.WrapSDKContext(h.ctx), &decodedUpdate)
		require.ErrorIs(t, err, sdkerrors.ErrUnauthorized)
		require.Nil(t, response)
		require.Equal(t, params, h.keeper.GetParams(h.ctx))

		// Each job increments an actual EVM storage slot, making duplicate
		// execution observable independently of the scheduler's own indexes.
		target := h.deploy([]byte{byte(vm.PUSH1), 0, byte(vm.SLOAD), byte(vm.PUSH1), 1,
			byte(vm.ADD), byte(vm.PUSH1), 0, byte(vm.SSTORE), byte(vm.STOP)})
		first, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, []byte{0xde, 0xad}, dueAt, gasLimit, deposit)
		require.NoError(t, err)
		require.Equal(t, uint64(1), first)
		second := h.schedule(target, dueAt)
		require.Equal(t, uint64(2), second)
		require.ErrorIs(t, h.keeper.Reschedule(h.ctx, target, first, later), agentcommon.ErrSchedulerUnauthorized)
		require.NoError(t, h.keeper.Reschedule(h.ctx, h.creator, first, later))
		require.Equal(t, []uint64{first, second}, h.keeper.Pending(h.ctx, h.creator))
		require.Equal(t, new(big.Int).Mul(deposit, big.NewInt(2)), h.balance(h.ctx, h.keeper.ModuleAddress()))

		before := h.keeper.ExportGenesis(h.ctx)
		require.NoError(t, before.Validate())
		require.Equal(t, params, before.Params)
		require.Equal(t, uint64(3), before.NextJobId)
		require.Len(t, before.Jobs, 2)
		require.Equal(t, later, before.Jobs[0].ExecuteAtBlock)
		require.Equal(t, []byte{0xde, 0xad}, before.Jobs[0].CallData)
		h.ctx.MultiStore().(storetypes.CacheMultiStore).Write()
		commitID := committed.Commit(true)
		require.NoError(t, committed.Close())
		committed = nil
		h.keeper = keeper.Keeper{}
		committed = open()
		require.Equal(t, commitID, committed.LastCommitID())
		require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))

		query := keeper.Querier{Keeper: h.keeper}
		for i := 0; i < 2; i++ {
			goCtx := sdk.WrapSDKContext(h.ctx)
			observed, err := query.Job(goCtx, &types.QueryJobRequest{Id: first})
			require.NoError(t, err)
			var consumer types.QueryJobResponse
			proofAgentWire(t, observed, &consumer)
			require.Equal(t, before.Jobs[0], consumer.Job)
			consumer.Job.CallData[0] ^= 0xff
			observed.Job.CallData[0] ^= 0xff
			pending, err := query.Pending(goCtx, &types.QueryPendingRequest{Creator: h.creator.Hex()})
			require.NoError(t, err)
			require.Equal(t, []uint64{first, second}, pending.JobIds)
			pending.JobIds[0] = 999
			observedParams, err := query.Params(goCtx, &types.QueryParamsRequest{})
			require.NoError(t, err)
			require.Equal(t, params, observedParams.Params)
			observedParams.Params.Paused = true
		}
		missing, err := query.Job(sdk.WrapSDKContext(h.ctx), &types.QueryJobRequest{Id: before.NextJobId})
		require.Equal(t, codes.NotFound, status.Code(err))
		require.Nil(t, missing)

		// Restore only the scheduler from its real exported protobuf. The
		// bank escrow is funded through the same bank path as genesis tests.
		var recovered types.GenesisState
		wire := proofAgentWire(t, h.keeper.ExportGenesis(h.ctx), &recovered)
		restored := newHarness(t)
		fundAgentGenesis(t, restored, h.balance(h.ctx, h.keeper.ModuleAddress()))
		require.NoError(t, restored.keeper.InitGenesis(restored.ctx, recovered))
		require.NoError(t, restored.keeper.InitGenesis(restored.ctx, recovered))
		require.Equal(t, before, restored.keeper.ExportGenesis(restored.ctx))
		require.Equal(t, []uint64{first, second}, restored.keeper.Pending(restored.ctx, h.creator))
		require.Equal(t, []uint64{second}, proofAgentDueIDs(restored, dueAt))
		require.Equal(t, []uint64{second, first}, proofAgentDueIDs(restored, later))
		require.Equal(t, h.balance(h.ctx, h.keeper.ModuleAddress()), restored.balance(restored.ctx, restored.keeper.ModuleAddress()))
		for _, corrupt := range []func(*types.GenesisState){
			func(gs *types.GenesisState) { gs.Jobs[0].Target = "0x12" },
			func(gs *types.GenesisState) { gs.Jobs[0].Deposit = "not-an-integer" },
			func(gs *types.GenesisState) { gs.NextJobId = gs.Jobs[1].Id },
		} {
			var malformed types.GenesisState
			require.NoError(t, proto.Unmarshal(wire, &malformed))
			corrupt(&malformed)
			require.Error(t, restored.keeper.InitGenesis(restored.ctx, malformed))
			require.Equal(t, before, restored.keeper.ExportGenesis(restored.ctx))
		}
		var truncated types.GenesisState
		require.Error(t, proto.Unmarshal(wire[:len(wire)-1], &truncated))

		paused := params
		paused.Paused = true
		_, err = keeper.NewMsgServerImpl(h.keeper, authority).UpdateParams(sdk.WrapSDKContext(h.ctx), &types.MsgUpdateParams{Authority: authority, Params: paused})
		require.NoError(t, err)
		_, err = h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, nil, later, gasLimit, deposit)
		require.Error(t, err)
		require.Error(t, h.keeper.Reschedule(h.ctx, h.creator, first, later+1))
		end := h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
		require.NoError(t, h.keeper.EndBlocker(end))
		require.Empty(t, executedEvents(t, end))
		require.Empty(t, failedEvents(t, end))
		require.Equal(t, []uint64{first, second}, h.keeper.Pending(end, h.creator))
		require.NoError(t, h.keeper.SetParams(end, params))

		collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
		for index, at := range []uint64{dueAt, later} {
			end = end.WithBlockHeight(int64(at)).WithEventManager(sdk.NewEventManager())
			creatorBefore := h.balance(end, h.creatorAcc)
			collectorBefore := h.balance(end, collector)
			require.NoError(t, h.keeper.EndBlocker(end))
			require.Empty(t, failedEvents(t, end))
			outcomes := executedEvents(t, end)
			require.Len(t, outcomes, 1)
			wantID := second
			if index == 1 {
				wantID = first
			}
			require.Equal(t, wantID, outcomes[0].JobId)
			require.Equal(t, h.creator.Hex(), outcomes[0].Creator)
			require.Greater(t, outcomes[0].GasUsed, intrinsic)
			require.LessOrEqual(t, outcomes[0].GasUsed, gasLimit)
			fee := new(big.Int).Mul(new(big.Int).SetUint64(outcomes[0].GasUsed), big.NewInt(baseFee))
			refund := new(big.Int).Sub(deposit, fee)
			require.Equal(t, refund.String(), outcomes[0].Refund)
			require.Equal(t, new(big.Int).Add(creatorBefore, refund), h.balance(end, h.creatorAcc))
			require.Equal(t, new(big.Int).Add(collectorBefore, fee), h.balance(end, collector))
			require.Equal(t, common.BigToHash(big.NewInt(int64(index+1))), testkeeper.EVMTestApp.EvmKeeper.GetState(end, target, common.Hash{}))
			// Re-decode the actual producer event twice; mutating an earlier
			// consumer value must not alter the next observation.
			for repeat := 0; repeat < 2; repeat++ {
				observed := executedEvents(t, end)
				require.Equal(t, outcomes, observed)
				observed[0].Refund = "changed by consumer"
			}
			proofAgentMalformedEvent(t, end)
			repeated := end.WithEventManager(sdk.NewEventManager())
			require.NoError(t, h.keeper.EndBlocker(repeated))
			require.Empty(t, executedEvents(t, repeated))
			require.Empty(t, failedEvents(t, repeated))
			require.Equal(t, h.balance(end, h.creatorAcc), h.balance(repeated, h.creatorAcc))
			require.Equal(t, common.BigToHash(big.NewInt(int64(index+1))), testkeeper.EVMTestApp.EvmKeeper.GetState(repeated, target, common.Hash{}))
		}
		require.Empty(t, h.keeper.Pending(end, h.creator))
		require.Empty(t, proofAgentDueIDs(&harness{ctx: end, keeper: h.keeper}, later))
		require.Zero(t, h.balance(end, h.keeper.ModuleAddress()).Sign())
		require.Equal(t, uint64(3), h.keeper.NextJobID(end))

		// The existing failure consumer also decodes an actual EVM refusal.
		h.ctx = end
		failing := h.deploy(reverter(t, 0, "proof refusal"))
		failedID := h.schedule(failing, later+1)
		refused := end.WithBlockHeight(int64(later + 1)).WithEventManager(sdk.NewEventManager())
		require.NoError(t, h.keeper.EndBlocker(refused))
		require.Empty(t, executedEvents(t, refused))
		for repeat := 0; repeat < 2; repeat++ {
			require.Equal(t, []*types.EventJobFailed{failed(failedID, h.creator, "proof refusal")}, failedEvents(t, refused))
		}
		require.Zero(t, h.balance(refused, h.keeper.ModuleAddress()).Sign())
	})

	t.Run("malformed stored parameters and message authority are refused", func(t *testing.T) {
		h := newHarness(t)
		before := h.keeper.ExportGenesis(h.ctx)
		server := keeper.NewMsgServerImpl(h.keeper, "malformed-authority")
		response, err := server.UpdateParams(sdk.WrapSDKContext(h.ctx), &types.MsgUpdateParams{Authority: "malformed-authority", Params: keeper.DefaultParams()})
		require.ErrorIs(t, err, sdkerrors.ErrInvalidAddress)
		require.Nil(t, response)
		require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
		for _, encoded := range []string{`{"max_jobs_per_creator":0}`, `{"max_block_gas":1}`, `{"paused":`} {
			cached, _ := h.ctx.CacheContext()
			cached.KVStore(h.key).Set(keeper.ParamsKey, []byte(encoded))
			require.Panics(t, func() {
				_, _ = (keeper.Querier{Keeper: h.keeper}).Params(sdk.WrapSDKContext(cached), &types.QueryParamsRequest{})
			})
			require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
		}
	})
}

func proofAgentDueIDs(h *harness, at uint64) []uint64 {
	var ids []uint64
	h.keeper.IterateDue(h.ctx, at, func(job keeper.Job) bool {
		ids = append(ids, job.ID)
		return false
	})
	return ids
}

func proofAgentMalformedEvent(t *testing.T, ctx sdk.Context) {
	t.Helper()
	for _, event := range ctx.EventManager().Events() {
		if event.Type != proto.MessageName(&types.EventJobExecuted{}) {
			continue
		}
		malformed := abci.Event{Type: event.Type, Attributes: append([]abci.EventAttribute(nil), event.Attributes...)}
		for i := range malformed.Attributes {
			if string(malformed.Attributes[i].Key) == "gas_used" {
				malformed.Attributes[i].Value = []byte(`"not-a-number"`)
				_, err := sdk.ParseTypedEvent(malformed)
				require.Error(t, err)
				return
			}
		}
	}
	t.Fatal("producer emitted no gas_used attribute to exercise the consumer decoder")
}
