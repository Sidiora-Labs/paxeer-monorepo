package integration_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"math"
	"math/big"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/gogo/protobuf/proto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

const endBlockJobGas = uint64(100_000)

type agentEndBlockTest struct {
	*app.TestWrapper
	key     ed25519.PrivateKey
	keyID   [seqtypes.KeyIDLength]byte
	creator common.Address
	target  common.Address
}

func newAgentEndBlockTest(t *testing.T) *agentEndBlockTest {
	t.Helper()
	w := app.NewTestWrapper(t, time.Unix(1_800_000_000, 0).UTC(), secp256k1.GenPrivKey().PubKey(), true,
		func(ba *baseapp.BaseApp) { ba.SetOccEnabled(false) })
	t.Cleanup(func() { require.NoError(t, w.App.Close()) })
	h := &agentEndBlockTest{
		TestWrapper: w,
		key:         ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x38}, ed25519.SeedSize)),
		creator:     common.HexToAddress("0xa2383"),
		target:      common.HexToAddress("0xc2383"),
	}
	h.keyID[0] = 1
	var publicKey, leader [32]byte
	copy(publicKey[:], h.key.Public().(ed25519.PublicKey))
	leader[0] = 1
	require.NoError(t, w.App.SequencerKeeper.InitGenesis(w.Ctx, seqtypes.GenesisState{
		Leaders: [][32]byte{leader},
		ReceiptKeys: []seqtypes.ReceiptKey{{
			KeyId: h.keyID, PublicKey: publicKey, FirstHeight: 1, LastHeight: math.MaxUint64,
		}},
		ActivationHeight: 2,
	}))
	w.App.UpgradeKeeper.SetDone(w.Ctx, app.MergeUpgrade)
	require.False(t, w.App.Sequenced(w.Ctx, 1))
	require.True(t, w.App.Sequenced(w.Ctx, 2))
	require.NotNil(t, w.App.EndBlockKeepers.AgentKeeper)

	consensus := w.App.GetConsensusParams(w.Ctx)
	consensus.Block.MaxGas = 1_000_000
	consensus.Block.MaxGasWanted = 1_000_000
	w.App.StoreConsensusParams(w.Ctx, consensus)
	w.Ctx = w.Ctx.WithConsensusParams(consensus)
	params := w.App.LayerXAgentKeeper.GetParams(w.Ctx)
	params.MaxBlockGas = 2 * endBlockJobGas
	params.MaxJobGasLimit = endBlockJobGas
	require.Greater(t, params.MaxJobsPerBlock, uint64(3))
	require.NoError(t, w.App.LayerXAgentKeeper.SetParams(w.Ctx, params))
	fees := w.App.EvmKeeper.GetParams(w.Ctx)
	fees.MinimumFeePerGas = sdk.OneDec()
	fees.MaximumFeePerGas = sdk.NewDec(1_000_000_000_000)
	fees.TargetGasUsedPerBlock = 100_000
	fees.MaxDynamicBaseFeeDownwardAdjustment = sdk.NewDecWithPrec(5, 1)
	fees.MaxDynamicBaseFeeUpwardAdjustment = sdk.NewDecWithPrec(5, 1)
	w.App.EvmKeeper.SetParams(w.Ctx, fees)
	w.App.EvmKeeper.SetNextBaseFeePerGas(w.Ctx, sdk.NewDec(1_000_000_000))
	w.FundAcc(w.App.EvmKeeper.GetPaxAddressOrDefault(w.Ctx, h.creator),
		sdk.NewCoins(sdk.NewInt64Coin(sdk.MustGetBaseDenom(), 1_000_000)))
	w.App.EvmKeeper.SetCode(w.Ctx, h.target, []byte{
		byte(vm.PUSH1), 0, byte(vm.SLOAD), byte(vm.PUSH1), 1, byte(vm.ADD),
		byte(vm.PUSH1), 0, byte(vm.SSTORE), byte(vm.STOP),
	})
	return h
}

func (h *agentEndBlockTest) schedule(t *testing.T, height uint64) uint64 {
	t.Helper()
	address := common.HexToAddress(scheduler.SchedulerAddress)
	served := h.App.EvmKeeper.CustomPrecompiles(h.Ctx)
	require.Contains(t, served, address)
	contract, ok := served[address].(*scheduler.SchedulerPrecompile)
	require.True(t, ok)
	input, err := contract.GetABI().Pack("schedule", h.target, []byte{1}, height, endBlockJobGas)
	require.NoError(t, err)
	db := state.NewDBImpl(h.Ctx, &h.App.EvmKeeper, false)
	block, err := h.App.EvmKeeper.GetVMBlockContext(db.Ctx(), core.GasPool(math.MaxUint64))
	require.NoError(t, err)
	chain := evmtypes.DefaultChainConfig().EthereumConfig(h.App.EvmKeeper.ChainID(db.Ctx()))
	evm := vm.NewEVM(*block, db, chain, vm.Config{}, served)
	deposit := big.NewInt(1_000_000_000_000_000)
	value, overflow := uint256.FromBig(deposit)
	require.False(t, overflow)
	output, _, err := evm.Call(h.creator, address, input, 2_000_000, value)
	require.NoError(t, err)
	require.NoError(t, db.Err())
	_, err = db.Finalize()
	require.NoError(t, err)
	values, err := contract.GetABI().Methods["schedule"].Outputs.Unpack(output)
	require.NoError(t, err)
	require.Len(t, values, 1)
	id := values[0].(*big.Int).Uint64()
	job, found := h.App.LayerXAgentKeeper.GetJob(h.Ctx, id)
	require.True(t, found)
	require.True(t, job.Active)
	require.Equal(t, height, job.ExecuteAtBlock)
	require.Equal(t, endBlockJobGas, job.GasLimit)
	require.Equal(t, h.creator, job.Creator)
	require.Equal(t, h.target, job.Target)
	require.Equal(t, deposit, job.Deposit.BigInt())
	return id
}

func (h *agentEndBlockTest) finalize(t *testing.T, height int64) []*agenttypes.EventJobExecuted {
	t.Helper()
	require.Equal(t, h.App.LastBlockHeight()+1, height)
	var txs [][]byte
	if height >= 2 {
		lastSeq, err := h.App.SequencerKeeper.LastSeq(h.Ctx)
		require.NoError(t, err)
		lastHash, err := h.App.SequencerKeeper.LastSeqHash(h.Ctx)
		require.NoError(t, err)
		batch := seqtypes.MsgSequenceBatch{
			Height: uint64(height), FromSeq: lastSeq + 1, ToSeq: lastSeq,
			PrevSeqHash: lastHash[:], LastSeqHash: lastHash[:],
			TxRoot: seqtypes.EmptyTxRoot[:], KeyId: h.keyID[:],
		}
		digest, err := batch.SignBytes("pax-test")
		require.NoError(t, err)
		batch.Sig = ed25519.Sign(h.key, digest[:])
		encoded, err := app.EncodeSequenceBatch(batch)
		require.NoError(t, err)
		txs = [][]byte{encoded}
	}
	response, err := h.App.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{
		Header: &tmproto.Header{ChainID: "pax-test", Height: height,
			Time: time.Unix(1_800_000_000+height*2, 0).UTC()},
		Txs: txs,
	})
	require.NoError(t, err)
	require.Len(t, response.TxResults, len(txs))
	for _, result := range response.TxResults {
		require.Zero(t, result.Code, result.Log)
		require.Zero(t, result.GasUsed)
	}
	var executed []*agenttypes.EventJobExecuted
	for _, event := range response.Events {
		require.NotEqual(t, proto.MessageName(&agenttypes.EventJobFailed{}), event.Type, "%+v", event)
		if event.Type == proto.MessageName(&agenttypes.EventJobExecuted{}) {
			parsed, err := sdk.ParseTypedEvent(event)
			require.NoError(t, err)
			outcome, ok := parsed.(*agenttypes.EventJobExecuted)
			require.True(t, ok)
			require.Equal(t, h.creator.Hex(), outcome.Creator)
			require.Greater(t, outcome.GasUsed, uint64(21_000))
			require.Less(t, outcome.GasUsed, endBlockJobGas)
			executed = append(executed, outcome)
		}
	}
	_, err = h.App.Commit(context.Background())
	require.NoError(t, err)
	require.Equal(t, height, h.App.LastBlockHeight())
	h.Ctx = h.App.GetCheckCtx()
	return executed
}

func TestAppEndBlockRunsAgentJobs(t *testing.T) {
	h := newAgentEndBlockTest(t)
	first, second, rolled, future := h.schedule(t, 4), h.schedule(t, 4), h.schedule(t, 4), h.schedule(t, 6)
	require.Equal(t, []uint64{1, 2, 3, 4}, []uint64{first, second, rolled, future})
	for height := int64(1); height < 4; height++ {
		require.Empty(t, h.finalize(t, height))
		require.Equal(t, []uint64{first, second, rolled, future}, h.App.LayerXAgentKeeper.Pending(h.Ctx, h.creator))
		require.Equal(t, common.Hash{}, h.App.EvmKeeper.GetState(h.Ctx, h.target, common.Hash{}))
	}
	for _, block := range []struct {
		height  int64
		ran     []uint64
		pending []uint64
		counter int64
	}{
		{4, []uint64{first, second}, []uint64{rolled, future}, 2},
		{5, []uint64{rolled}, []uint64{future}, 3},
		{6, []uint64{future}, []uint64{}, 4},
		{7, []uint64{}, []uint64{}, 4},
	} {
		outcomes := h.finalize(t, block.height)
		ran := make([]uint64, 0, len(outcomes))
		var used uint64
		for _, outcome := range outcomes {
			ran = append(ran, outcome.JobId)
			used += outcome.GasUsed
			_, found := h.App.LayerXAgentKeeper.GetJob(h.Ctx, outcome.JobId)
			require.False(t, found)
		}
		require.Equal(t, block.ran, ran)
		params := h.App.LayerXAgentKeeper.GetParams(h.Ctx)
		require.LessOrEqual(t, uint64(len(ran))*endBlockJobGas, params.MaxBlockGas)
		require.LessOrEqual(t, used, params.MaxBlockGas)
		require.Equal(t, block.pending, h.App.LayerXAgentKeeper.Pending(h.Ctx, h.creator))
		for _, id := range block.pending {
			job, found := h.App.LayerXAgentKeeper.GetJob(h.Ctx, id)
			require.True(t, found)
			require.True(t, job.Active)
			if id == rolled {
				require.Equal(t, uint64(4), job.ExecuteAtBlock)
			}
		}
		require.Equal(t, common.BigToHash(big.NewInt(block.counter)), h.App.EvmKeeper.GetState(h.Ctx, h.target, common.Hash{}))
	}
}

func TestAppEndBlockCountsAgentGas(t *testing.T) {
	h := newAgentEndBlockTest(t)
	first, second := h.schedule(t, 3), h.schedule(t, 3)
	require.Empty(t, h.finalize(t, 1))
	require.Empty(t, h.finalize(t, 2))
	before := h.App.EvmKeeper.GetNextBaseFeePerGas(h.Ctx)
	params := h.App.EvmKeeper.GetParams(h.Ctx)
	outcomes := h.finalize(t, 3)
	require.Len(t, outcomes, 2)
	require.Equal(t, []uint64{first, second}, []uint64{outcomes[0].JobId, outcomes[1].JobId})
	gas := outcomes[0].GasUsed + outcomes[1].GasUsed
	require.Less(t, gas, params.TargetGasUsedPerBlock)
	require.Less(t, gas, 2*endBlockJobGas)
	target := sdk.NewDec(int64(params.TargetGasUsedPerBlock))
	emptyFraction := target.Sub(sdk.NewDec(int64(gas))).Quo(target)
	expected := before.Mul(sdk.OneDec().Sub(params.MaxDynamicBaseFeeDownwardAdjustment.Mul(emptyFraction)))
	emptyBlockFee := before.Mul(sdk.OneDec().Sub(params.MaxDynamicBaseFeeDownwardAdjustment))
	after := h.App.EvmKeeper.GetNextBaseFeePerGas(h.Ctx)
	require.True(t, expected.Equal(after), "base fee must include %d end-block gas: expected %s, got %s", gas, expected, after)
	require.True(t, after.GT(emptyBlockFee), "omitting agent gas would produce %s", emptyBlockFee)
	require.True(t, after.LT(before), "using reserved gas instead of actual gas would increase the fee")
	require.Empty(t, h.App.LayerXAgentKeeper.Pending(h.Ctx, h.creator))
	require.Equal(t, common.BigToHash(big.NewInt(2)), h.App.EvmKeeper.GetState(h.Ctx, h.target, common.Hash{}))
	require.Empty(t, h.finalize(t, 4))
	expectedEmpty := after.Mul(sdk.OneDec().Sub(params.MaxDynamicBaseFeeDownwardAdjustment))
	require.True(t, expectedEmpty.Equal(h.App.EvmKeeper.GetNextBaseFeePerGas(h.Ctx)), "job gas must only be counted at its execution height")
}
