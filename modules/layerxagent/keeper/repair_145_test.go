package keeper_test

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/scheduler"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

type repair145Disk struct {
	h         *harness
	dir       string
	committed storetypes.CommitMultiStore
}

func repair145Open(t *testing.T) *repair145Disk {
	t.Helper()
	d := &repair145Disk{h: newHarness(t), dir: t.TempDir()}
	d.open()
	t.Cleanup(func() { require.NoError(t, d.committed.Close()) })
	return d
}

func (d *repair145Disk) open() {
	t := d.h.t
	db, err := tmdb.NewGoLevelDB("repair145", d.dir)
	require.NoError(t, err)
	d.committed = store.NewCommitMultiStore(db)
	d.committed.MountStoreWithDB(d.h.key, sdk.StoreTypeIAVL, db)
	require.NoError(t, d.committed.LoadLatestVersion())
	d.h.ctx = withKVStore(d.h.ctx, d.h.key, d.committed.GetKVStore(d.h.key))
	d.h.keeper = keeper.NewKeeper(d.h.key, &testkeeper.EVMTestApp.EvmKeeper, precompiles.MergeUpgrade)
}

func (d *repair145Disk) restart(ctx sdk.Context) {
	d.h.ctx = ctx
	d.h.ctx.MultiStore().(storetypes.CacheMultiStore).Write()
	id := d.committed.Commit(true)
	require.NoError(d.h.t, d.committed.Close())
	d.h.keeper = keeper.Keeper{}
	d.open()
	require.Equal(d.h.t, id, d.committed.LastCommitID())
}

func repair145Precompile(t *testing.T, h *harness) *scheduler.SchedulerPrecompile {
	t.Helper()
	evm := &testkeeper.EVMTestApp.EvmKeeper
	previous := *evm
	t.Cleanup(func() { *evm = previous })
	p, err := scheduler.NewPrecompile(h.keeper, evm, evm.UpgradeKeeper(), precompiles.MergeUpgrade)
	require.NoError(t, err)
	evm.SetCustomPrecompiles(map[common.Address]utils.VersionedPrecompiles{
		p.Address(): {precompiles.MergeUpgrade: p},
	}, precompiles.MergeUpgrade)
	return p
}

func repair145Counter() []byte {
	return []byte{byte(vm.PUSH1), 0, byte(vm.SLOAD), byte(vm.PUSH1), 1,
		byte(vm.ADD), byte(vm.PUSH1), 0, byte(vm.SSTORE), byte(vm.STOP)}
}

func repair145Move(t *testing.T, h *harness, from, to sdk.AccAddress, amount *big.Int) {
	t.Helper()
	coins, wei := state.SplitUhpxWeiAmount(amount)
	require.NoError(t, testkeeper.EVMTestApp.BankKeeper.SendCoinsAndWei(h.ctx, from, to, coins, wei))
}

func TestRepair_145_CancelAdmitted(t *testing.T) {
	repair145Admitted(t, false)
}

func TestRepair_145_RescheduleAdmitted(t *testing.T) {
	repair145Admitted(t, true)
}

func repair145Admitted(t *testing.T, reschedule bool) {
	d := repair145Open(t)
	h := d.h
	p := repair145Precompile(t, h)
	target := h.deploy(repair145Counter())
	collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	creatorBefore, collectorBefore := h.balance(h.ctx, h.creatorAcc), h.balance(h.ctx, collector)
	method, args := "cancel", []interface{}{big.NewInt(2)}
	if reschedule {
		method, args = "reschedule", append(args, later)
	}
	a := p.GetABI()
	input, err := a.Pack(method, args...)
	require.NoError(t, err)
	first, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, p.Address(), input, dueAt, gasLimit, deposit)
	require.NoError(t, err)
	require.Equal(t, uint64(1), first)
	second := h.schedule(target, dueAt)
	require.Equal(t, uint64(2), second)
	// A different creator's later deposit must never subsidize a stale job.
	_, other := testkeeper.MockAddressPair()
	third, err := h.keeper.Schedule(h.ctx, h.creatorAcc, other, target, nil, later+100, gasLimit, deposit)
	require.NoError(t, err)
	h.ctx = h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
	used, err := h.keeper.RunDue(h.ctx)
	require.NoError(t, err)
	require.Empty(t, failedEvents(t, h.ctx))
	events := executedEvents(t, h.ctx)
	require.Len(t, events, 1)
	require.Equal(t, first, events[0].JobId)
	require.Equal(t, used, events[0].GasUsed)
	require.Greater(t, used, intrinsic)
	require.Equal(t, common.Hash{}, testkeeper.EVMTestApp.EvmKeeper.GetState(h.ctx, target, common.Hash{}))
	remaining := int64(1)
	job, found := h.keeper.GetJob(h.ctx, second)
	if reschedule {
		remaining++
		require.True(t, found)
		require.Equal(t, later, job.ExecuteAtBlock)
		require.Equal(t, deposit, job.Deposit.BigInt())
	} else {
		require.False(t, found)
	}
	fee := new(big.Int).Mul(new(big.Int).SetUint64(used), big.NewInt(baseFee))
	escrow := new(big.Int).Mul(deposit, big.NewInt(remaining))
	require.Equal(t, escrow, h.balance(h.ctx, h.keeper.ModuleAddress()))
	require.Equal(t, new(big.Int).Add(collectorBefore, fee), h.balance(h.ctx, collector))
	require.Equal(t, new(big.Int).Sub(new(big.Int).Sub(creatorBefore, escrow), fee), h.balance(h.ctx, h.creatorAcc))
	require.Equal(t, []uint64{third}, h.keeper.Pending(h.ctx, other))
	require.Empty(t, proofAgentDueIDs(h, dueAt))
	require.Empty(t, h.keeper.ExportRecoveryGenesis(h.ctx).Settlements)
	before := h.keeper.ExportGenesis(h.ctx)
	d.restart(h.ctx)
	require.Equal(t, before, h.keeper.ExportGenesis(h.ctx))
	// Running beyond the moved height must consume its actual due index once.
	h.ctx = h.ctx.WithBlockHeight(int64(later + 1)).WithEventManager(sdk.NewEventManager())
	usedLater, err := h.keeper.RunDue(h.ctx)
	require.NoError(t, err)
	require.Empty(t, failedEvents(t, h.ctx))
	if reschedule {
		events = executedEvents(t, h.ctx)
		require.Len(t, events, 1)
		require.Equal(t, second, events[0].JobId)
		require.Equal(t, common.BigToHash(big.NewInt(1)), testkeeper.EVMTestApp.EvmKeeper.GetState(h.ctx, target, common.Hash{}))
	} else {
		require.Zero(t, usedLater)
		require.Empty(t, executedEvents(t, h.ctx))
	}
	fee.Add(fee, new(big.Int).Mul(new(big.Int).SetUint64(usedLater), big.NewInt(baseFee)))
	require.Equal(t, deposit, h.balance(h.ctx, h.keeper.ModuleAddress()))
	require.Equal(t, new(big.Int).Add(collectorBefore, fee), h.balance(h.ctx, collector))
	require.Equal(t, new(big.Int).Sub(new(big.Int).Sub(creatorBefore, deposit), fee), h.balance(h.ctx, h.creatorAcc))
	require.Empty(t, proofAgentDueIDs(h, later+2))
	d.restart(h.ctx)
	h.ctx = h.ctx.WithBlockHeight(int64(later + 2)).WithEventManager(sdk.NewEventManager())
	used, err = h.keeper.RunDue(h.ctx)
	require.NoError(t, err)
	require.Zero(t, used)
	require.Empty(t, executedEvents(t, h.ctx))
	require.Equal(t, deposit, h.balance(h.ctx, h.keeper.ModuleAddress()))
	require.Equal(t, []uint64{third}, h.keeper.Pending(h.ctx, other))
}

func TestRepair_145_SettlementRecovery(t *testing.T) {
	for _, tc := range []struct {
		name                string
		escrow              int64
		jobDeposit          int64
		feePaid, refundPaid bool
	}{
		{"both refused", 0, 2_000_000, false, false},
		{"refund refused", 1_000_000, 2_000_000, true, false},
		{"fee refused", 300_000, 1_000_000, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := repair145Open(t)
			h := d.h
			params := h.keeper.GetParams(h.ctx)
			params.MinDepositFactor = 1
			require.NoError(t, h.keeper.SetParams(h.ctx, params))
			// The successful target changes real EVM state. Its cost is above
			// 700k wei, making the 300k balance unable to pay its fee.
			target := h.deploy(burner(30_000, repair145Counter()))
			id, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, target, nil, dueAt, gasLimit, big.NewInt(tc.jobDeposit))
			require.NoError(t, err)
			collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
			collectorBefore := h.balance(h.ctx, collector)
			// Drain through the real bank to provoke its insufficient-funds
			// refusal; neither the bank nor scheduler is replaced.
			repair145Move(t, h, h.keeper.ModuleAddress(), h.creatorAcc, big.NewInt(tc.jobDeposit-tc.escrow))
			creatorBefore := h.balance(h.ctx, h.creatorAcc)
			h.ctx = h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
			used, err := h.keeper.RunDue(h.ctx)
			require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
			require.Greater(t, used, uint64(70_000))
			require.Less(t, used, uint64(80_000))
			require.Len(t, failedEvents(t, h.ctx), 1)
			require.Empty(t, executedEvents(t, h.ctx))
			require.Equal(t, common.BigToHash(big.NewInt(1)), testkeeper.EVMTestApp.EvmKeeper.GetState(h.ctx, target, common.Hash{}))
			_, found := h.keeper.GetJob(h.ctx, id)
			require.False(t, found)
			require.Empty(t, h.keeper.Pending(h.ctx, h.creator))
			require.Empty(t, proofAgentDueIDs(h, later))
			obligation, found := h.keeper.GetSettlement(h.ctx, id)
			require.True(t, found)
			require.Equal(t, used, obligation.GasUsed)
			require.Empty(t, obligation.Reason)
			require.Equal(t, tc.feePaid, obligation.FeePaid)
			require.Equal(t, tc.refundPaid, obligation.RefundPaid)
			fee := new(big.Int).Mul(new(big.Int).SetUint64(used), big.NewInt(baseFee))
			refund := new(big.Int).Sub(big.NewInt(tc.jobDeposit), fee)
			require.Equal(t, fee, obligation.Fee.BigInt())
			require.Equal(t, refund, obligation.Refund.BigInt())
			paidFee, paidRefund := new(big.Int), new(big.Int)
			if tc.feePaid {
				paidFee.Set(fee)
			}
			if tc.refundPaid {
				paidRefund.Set(refund)
			}
			require.Equal(t, new(big.Int).Add(collectorBefore, paidFee), h.balance(h.ctx, collector))
			require.Equal(t, new(big.Int).Add(creatorBefore, paidRefund), h.balance(h.ctx, h.creatorAcc))
			require.PanicsWithValue(t, "layerxagent genesis: outstanding settlements require ExportRecoveryGenesis", func() { h.keeper.ExportGenesis(h.ctx) })
			before := h.keeper.ExportRecoveryGenesis(h.ctx)
			d.restart(h.ctx)
			require.Equal(t, before, h.keeper.ExportRecoveryGenesis(h.ctx))
			restored, found := h.keeper.GetSettlement(h.ctx, id)
			require.True(t, found)
			require.Equal(t, obligation, restored)
			// An unfunded retry after reopen does not execute the target or
			// pay an already completed leg a second time.
			h.ctx = h.ctx.WithBlockHeight(int64(dueAt + 1)).WithEventManager(sdk.NewEventManager())
			used, err = h.keeper.RunDue(h.ctx)
			require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
			require.Zero(t, used)
			require.Empty(t, executedEvents(t, h.ctx))
			require.Empty(t, failedEvents(t, h.ctx))
			require.Equal(t, before, h.keeper.ExportRecoveryGenesis(h.ctx))
			require.Equal(t, common.BigToHash(big.NewInt(1)), testkeeper.EVMTestApp.EvmKeeper.GetState(h.ctx, target, common.Hash{}))
			wire, err := json.Marshal(h.keeper.ExportRecoveryGenesis(h.ctx))
			require.NoError(t, err)
			var decoded keeper.RecoveryGenesis
			require.NoError(t, json.Unmarshal(wire, &decoded))
			fresh := newHarness(t)
			empty := fresh.keeper.ExportRecoveryGenesis(fresh.ctx)
			require.Error(t, fresh.keeper.InitRecoveryGenesis(fresh.ctx, decoded))
			require.Equal(t, empty, fresh.keeper.ExportRecoveryGenesis(fresh.ctx))
			outstanding := new(big.Int).Sub(new(big.Int).Sub(big.NewInt(tc.jobDeposit), paidFee), paidRefund)
			fundAgentGenesis(t, fresh, outstanding)
			require.NoError(t, fresh.keeper.InitRecoveryGenesis(fresh.ctx, decoded))
			require.Equal(t, before, fresh.keeper.ExportRecoveryGenesis(fresh.ctx))
			fresh.ctx = fresh.ctx.WithBlockHeight(int64(later)).WithEventManager(sdk.NewEventManager())
			freshCollector, freshCreator := fresh.balance(fresh.ctx, collector), fresh.balance(fresh.ctx, h.creatorAcc)
			used, err = fresh.keeper.RunDue(fresh.ctx)
			require.NoError(t, err)
			require.Zero(t, used)
			require.Equal(t, new(big.Int).Add(freshCollector, new(big.Int).Sub(fee, paidFee)), fresh.balance(fresh.ctx, collector))
			require.Equal(t, new(big.Int).Add(freshCreator, new(big.Int).Sub(refund, paidRefund)), fresh.balance(fresh.ctx, h.creatorAcc))
			require.Empty(t, fresh.keeper.ExportRecoveryGenesis(fresh.ctx).Settlements)
			// Repair the bank shortfall and pay the original durable record.
			replenish := new(big.Int).Sub(outstanding, h.balance(h.ctx, h.keeper.ModuleAddress()))
			repair145Move(t, h, h.creatorAcc, h.keeper.ModuleAddress(), replenish)
			creatorFunded := h.balance(h.ctx, h.creatorAcc)
			params.Paused = true
			require.NoError(t, h.keeper.SetParams(h.ctx, params))
			d.restart(h.ctx)
			h.ctx = h.ctx.WithBlockHeight(int64(later)).WithEventManager(sdk.NewEventManager())
			used, err = h.keeper.RunDue(h.ctx)
			require.NoError(t, err)
			require.Zero(t, used)
			require.Equal(t, new(big.Int).Add(collectorBefore, fee), h.balance(h.ctx, collector))
			require.Equal(t, new(big.Int).Add(creatorFunded, new(big.Int).Sub(refund, paidRefund)), h.balance(h.ctx, h.creatorAcc))
			require.Zero(t, h.balance(h.ctx, h.keeper.ModuleAddress()).Sign())
			require.Equal(t, common.BigToHash(big.NewInt(1)), testkeeper.EVMTestApp.EvmKeeper.GetState(h.ctx, target, common.Hash{}))
			require.Empty(t, h.keeper.ExportRecoveryGenesis(h.ctx).Settlements)
			require.Empty(t, executedEvents(t, h.ctx))
			require.Empty(t, failedEvents(t, h.ctx))
			paidCreator := h.balance(h.ctx, h.creatorAcc)
			d.restart(h.ctx)
			h.ctx = h.ctx.WithBlockHeight(int64(later + 1)).WithEventManager(sdk.NewEventManager())
			used, err = h.keeper.RunDue(h.ctx)
			require.NoError(t, err)
			require.Zero(t, used)
			require.Equal(t, paidCreator, h.balance(h.ctx, h.creatorAcc))
			require.Equal(t, new(big.Int).Add(collectorBefore, fee), h.balance(h.ctx, collector))
			require.Equal(t, common.BigToHash(big.NewInt(1)), testkeeper.EVMTestApp.EvmKeeper.GetState(h.ctx, target, common.Hash{}))
			require.Equal(t, uint64(2), h.keeper.NextJobID(h.ctx))
		})
	}
}

func TestRepair_145_RecoveryGenesisValidation(t *testing.T) {
	h := newHarness(t)
	target := h.deploy(repair145Counter())
	id := h.schedule(target, dueAt)
	repair145Move(t, h, h.keeper.ModuleAddress(), h.creatorAcc, deposit)
	h.ctx = h.ctx.WithBlockHeight(int64(dueAt))
	_, err := h.keeper.RunDue(h.ctx)
	require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
	produced := h.keeper.ExportRecoveryGenesis(h.ctx)
	wire, err := json.Marshal(produced)
	require.NoError(t, err)
	for _, corrupt := range []func(*keeper.RecoveryGenesis){
		func(gs *keeper.RecoveryGenesis) { gs.Version++ },
		func(gs *keeper.RecoveryGenesis) { gs.Settlements[0].JobID = gs.Scheduler.NextJobId },
		func(gs *keeper.RecoveryGenesis) { gs.Settlements = append(gs.Settlements, gs.Settlements[0]) },
		func(gs *keeper.RecoveryGenesis) { gs.Settlements[0].Fee = sdk.NewInt(-1) },
		func(gs *keeper.RecoveryGenesis) { gs.Settlements[0].Refund = sdk.Int{} },
		func(gs *keeper.RecoveryGenesis) { gs.Settlements[0].RefundRecipient = nil },
		func(gs *keeper.RecoveryGenesis) { gs.Settlements[0].FeeRecipient = h.creatorAcc },
		func(gs *keeper.RecoveryGenesis) { gs.Settlements[0].FeePaid, gs.Settlements[0].RefundPaid = true, true },
		func(gs *keeper.RecoveryGenesis) {
			gs.Scheduler.Jobs = []types.Job{{Id: id, Creator: h.creator.Hex(), Target: target.Hex(), Deposit: deposit.String(), Active: true}}
		},
	} {
		var malformed keeper.RecoveryGenesis
		require.NoError(t, json.Unmarshal(wire, &malformed))
		corrupt(&malformed)
		fresh := newHarness(t)
		fundAgentGenesis(t, fresh, deposit)
		before := fresh.keeper.ExportRecoveryGenesis(fresh.ctx)
		require.Error(t, fresh.keeper.InitRecoveryGenesis(fresh.ctx, malformed))
		require.Equal(t, before, fresh.keeper.ExportRecoveryGenesis(fresh.ctx))
	}
}

func TestRepair_145_FailedOutcomeRecovery(t *testing.T) {
	d := repair145Open(t)
	h := d.h
	target := h.deploy(reverter(t, 0, "boom"))
	id := h.schedule(target, dueAt)
	repair145Move(t, h, h.keeper.ModuleAddress(), h.creatorAcc, deposit)
	h.ctx = h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
	used, err := h.keeper.RunDue(h.ctx)
	require.ErrorIs(t, err, sdkerrors.ErrInsufficientFunds)
	events := failedEvents(t, h.ctx)
	require.Len(t, events, 1)
	require.Contains(t, events[0].Reason, "boom")
	obligation, found := h.keeper.GetSettlement(h.ctx, id)
	require.True(t, found)
	require.Equal(t, "boom", obligation.Reason)
	require.Equal(t, used, obligation.GasUsed)
	d.restart(h.ctx)
	recovered, found := h.keeper.GetSettlement(h.ctx, id)
	require.True(t, found)
	require.Equal(t, obligation, recovered)
	repair145Move(t, h, h.creatorAcc, h.keeper.ModuleAddress(), deposit)
	h.ctx = h.ctx.WithBlockHeight(int64(later)).WithEventManager(sdk.NewEventManager())
	collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	collectorBefore, creatorBefore := h.balance(h.ctx, collector), h.balance(h.ctx, h.creatorAcc)
	used, err = h.keeper.RunDue(h.ctx)
	require.NoError(t, err)
	require.Zero(t, used)
	require.Equal(t, new(big.Int).Add(collectorBefore, obligation.Fee.BigInt()), h.balance(h.ctx, collector))
	require.Equal(t, new(big.Int).Add(creatorBefore, obligation.Refund.BigInt()), h.balance(h.ctx, h.creatorAcc))
	require.Empty(t, h.keeper.ExportRecoveryGenesis(h.ctx).Settlements)
	require.Empty(t, failedEvents(t, h.ctx))
	require.Empty(t, executedEvents(t, h.ctx))
}

func TestRepair_145_SelfCancellation(t *testing.T) {
	h := newHarness(t)
	p := repair145Precompile(t, h)
	a := p.GetABI()
	input, err := a.Pack("cancel", big.NewInt(1))
	require.NoError(t, err)
	id, err := h.keeper.Schedule(h.ctx, h.creatorAcc, h.creator, p.Address(), input, dueAt, gasLimit, deposit)
	require.NoError(t, err)
	h.ctx = h.ctx.WithBlockHeight(int64(dueAt)).WithEventManager(sdk.NewEventManager())
	creatorBefore := h.balance(h.ctx, h.creatorAcc)
	collector := authtypes.NewModuleAddress(authtypes.FeeCollectorName)
	collectorBefore := h.balance(h.ctx, collector)
	used, err := h.keeper.RunDue(h.ctx)
	require.NoError(t, err)
	events := failedEvents(t, h.ctx)
	require.Len(t, events, 1)
	require.Contains(t, events[0].Reason, "LXP_AGENT_SCHEDULER_JOB_INACTIVE")
	fee := new(big.Int).Mul(new(big.Int).SetUint64(used), big.NewInt(baseFee))
	require.Equal(t, new(big.Int).Add(collectorBefore, fee), h.balance(h.ctx, collector))
	require.Equal(t, new(big.Int).Add(creatorBefore, new(big.Int).Sub(deposit, fee)), h.balance(h.ctx, h.creatorAcc))
	require.Zero(t, h.balance(h.ctx, h.keeper.ModuleAddress()).Sign())
	_, found := h.keeper.GetJob(h.ctx, id)
	require.False(t, found)
	require.Empty(t, h.keeper.ExportRecoveryGenesis(h.ctx).Settlements)
	require.Empty(t, h.keeper.Pending(h.ctx, h.creator))
}
