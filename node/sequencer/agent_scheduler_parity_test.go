package sequencer_test

import (
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	agentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type EndBlocker func(sdk.Context) error

func TestAgentSchedulerSameInSealBatchAndProcessBlock(t *testing.T) {
	previousOCC := app.EnableOCC
	t.Cleanup(func() { app.EnableOCC = previousOCC })
	creator := utils.NewSigner()
	h := newSourceHarness(t, openHeight, func(ctx sdk.Context, a *app.App) {
		require.NoError(t, a.SequencerKeeper.SetActivationHeight(ctx, openHeight))
	}, creator).harness
	a := h.app
	base := h.committedCtx()
	key := sdk.NewKVStoreKey(agentkeeper.StoreKey)
	stores := map[storetypes.StoreKey]storetypes.CacheWrapper{}
	keys := map[string]storetypes.StoreKey{}
	base.MultiStore().SetKVStores(func(key storetypes.StoreKey, store storetypes.KVStore) storetypes.CacheWrap {
		stores[key] = store
		keys[key.Name()] = key
		return store.(storetypes.CacheWrap)
	})
	stores[key] = dbadapter.Store{DB: tmdb.NewMemDB()}
	keys[key.Name()] = key
	base = base.WithMultiStore(cachemulti.NewStore(tmdb.NewMemDB(), stores, keys, nil, nil, nil, 0))
	base = base.WithConsensusParams(a.GetConsensusParams(base))
	base = base.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(base))
	agent := agentkeeper.NewKeeper(key, &a.EvmKeeper, app.MergeUpgrade)
	a.EndBlockKeepers.AgentKeeper = &agent
	a.EndBlockKeepers.MergeLive = func() bool {
		return a.UpgradeKeeper.IsUpgradeActiveAtHeight(base, app.MergeUpgrade, openHeight)
	}
	require.True(t, a.EndBlockKeepers.MergeLive())
	require.True(t, a.Sequenced(base, openHeight))
	require.NoError(t, agent.Active(base.WithBlockHeight(openHeight)))
	require.NoError(t, agent.SetParams(base, agentkeeper.DefaultParams()))
	target := common.HexToAddress("0x5678")
	a.EvmKeeper.SetCode(base, target, []byte{
		byte(vm.PUSH1), 0, byte(vm.SLOAD), byte(vm.PUSH1), 1, byte(vm.ADD),
		byte(vm.PUSH1), 0, byte(vm.SSTORE), byte(vm.STOP),
	})
	const gasLimit = uint64(100_000)
	baseFee := new(big.Int).Set(a.EvmKeeper.GetBaseFee(base))
	require.Positive(t, baseFee.Sign())
	deposit := new(big.Int).Mul(new(big.Int).SetUint64(gasLimit*agent.GetParams(base).MinDepositFactor), baseFee)
	jobID, err := agent.Schedule(base, creator.AccountAddress, creator.EvmAddress, target, nil,
		uint64(openHeight), gasLimit, deposit)
	require.NoError(t, err)
	futureID, err := agent.Schedule(base, creator.AccountAddress, creator.EvmAddress, target, nil,
		uint64(openHeight+1), gasLimit, deposit)
	require.NoError(t, err)
	futureJob, found := agent.GetJob(base, futureID)
	require.True(t, found)
	creatorBefore := new(big.Int).Set(a.EvmKeeper.GetBalance(base, creator.AccountAddress))
	header := tmproto.Header{
		ChainID: testChainID, Height: openHeight,
		Time: time.UnixMilli(h.genesis.Add(time.Second).UnixMilli()).UTC(), ProposerAddress: h.proposer,
	}
	newBlockContext := func() sdk.Context {
		return base.WithMultiStore(base.MultiStore().CacheMultiStore()).WithBlockHeader(header).
			WithEventManager(sdk.NewEventManager())
	}
	sealCtx := newBlockContext()
	begin := a.BeginBlock(sealCtx, openHeight, nil, nil, true)
	batch := &sequencer.Batch{
		Height: openHeight, TimestampMs: header.Time.UnixMilli(), FromSeq: sourceLastSeq + 1,
		LastSeq: sourceLastSeq, PrevSeqHash: sourcePrevSeqHash, LastSeqHash: sourcePrevSeqHash,
		Ctx: sealCtx, Store: sealCtx.MultiStore().(sdk.CacheMultiStore), BeginBlockEvents: begin.Events,
	}
	_, found = agent.GetJob(batch.Ctx, jobID)
	require.True(t, found)
	require.Equal(t, common.Hash{}, a.EvmKeeper.GetState(batch.Ctx, target, common.Hash{}))
	for _, event := range begin.Events {
		require.NotEqual(t, proto.MessageName(&agenttypes.EventJobExecuted{}), event.Type)
		require.NotEqual(t, proto.MessageName(&agenttypes.EventJobFailed{}), event.Type)
	}
	sealed, tx0, err := sequencer.SealBatch(a, batch, signSealBatch(t, sequencer.BatchFields(batch), testKeySeed, testKeyID))
	require.NoError(t, err)
	require.Equal(t, [][]byte{tx0}, sealed.Txs)
	replay := newBlockContext()
	_, found = agent.GetJob(replay, jobID)
	require.True(t, found)
	require.Equal(t, common.Hash{}, a.EvmKeeper.GetState(replay, target, common.Hash{}))
	events, results, end, err := a.ProcessBlock(replay, sealed.Txs, &app.BlockProcessRequest{
		Height: openHeight, Time: header.Time,
	}, abci.CommitInfo{}, false, nil)
	require.NoError(t, err)
	assertExecuted := func(ctx sdk.Context, end abci.ResponseEndBlock) {
		t.Helper()
		var executed []*agenttypes.EventJobExecuted
		for _, event := range end.Events {
			require.NotEqual(t, proto.MessageName(&agenttypes.EventJobFailed{}), event.Type)
			if event.Type == proto.MessageName(&agenttypes.EventJobExecuted{}) {
				parsed, err := sdk.ParseTypedEvent(event)
				require.NoError(t, err)
				executed = append(executed, parsed.(*agenttypes.EventJobExecuted))
			}
		}
		require.Len(t, executed, 1)
		outcome := executed[0]
		require.Equal(t, jobID, outcome.JobId)
		require.Equal(t, creator.EvmAddress.Hex(), outcome.Creator)
		require.Positive(t, outcome.GasUsed)
		require.LessOrEqual(t, outcome.GasUsed, gasLimit)
		refund := new(big.Int).Sub(deposit, new(big.Int).Mul(new(big.Int).SetUint64(outcome.GasUsed), baseFee))
		require.Equal(t, refund.String(), outcome.Refund)
		require.Equal(t, new(big.Int).Add(creatorBefore, refund), a.EvmKeeper.GetBalance(ctx, creator.AccountAddress))
		require.Equal(t, common.HexToHash("0x1"), a.EvmKeeper.GetState(ctx, target, common.Hash{}))
		_, found := agent.GetJob(ctx, jobID)
		require.False(t, found)
		require.Equal(t, []uint64{futureID}, agent.Pending(ctx, creator.EvmAddress))
		future, found := agent.GetJob(ctx, futureID)
		require.True(t, found)
		require.Equal(t, futureJob, future)
		require.Equal(t, deposit, a.EvmKeeper.GetBalance(ctx, agent.ModuleAddress()))
		var due []uint64
		agent.IterateDue(ctx, uint64(openHeight), func(job agentkeeper.Job) bool {
			due = append(due, job.ID)
			return false
		})
		require.Empty(t, due)
		repeated, _ := ctx.CacheContext()
		endBlocker := EndBlocker(agent.EndBlocker)
		require.NoError(t, endBlocker(repeated))
		require.Empty(t, repeated.EventManager().Events())
		require.Equal(t, common.HexToHash("0x1"), a.EvmKeeper.GetState(repeated, target, common.Hash{}))
		utils.CompareStores(t, key, ctx.KVStore(key), repeated.KVStore(key), "scheduler after repeated end block")
	}
	assertExecuted(sealed.Context(), sealed.EndBlock)
	assertExecuted(replay, end)
	utils.CompareStores(t, key, sealed.Context().KVStore(key), replay.KVStore(key), "agent scheduler parity")
	requireSameBlock(t, a, processedBlock{ctx: replay, events: events, results: results, end: end}, sealed)
}
