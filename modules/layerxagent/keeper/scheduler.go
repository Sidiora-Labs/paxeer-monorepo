package keeper

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"unicode/utf8"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/prefix"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
)

// validatePendingGasLimits preserves FIFO admission across parameter reductions,
// including jobs that are not due yet. Rejection leaves their escrow untouched.
func (k Keeper) validatePendingGasLimits(ctx sdk.Context, params Params) error {
	iterator := prefix.NewStore(k.store(ctx), JobKeyPrefix).Iterator(nil, nil)
	defer func() { _ = iterator.Close() }()
	for ; iterator.Valid(); iterator.Next() {
		id := binary.BigEndian.Uint64(iterator.Key())
		job, found := k.GetJob(ctx, id)
		if !found {
			panic(fmt.Errorf("layerxagent missing job %d", id))
		}
		if job.Active && (job.GasLimit > params.MaxJobGasLimit || job.GasLimit > params.MaxBlockGas) {
			return fmt.Errorf("%w: job %d gas limit %d exceeds proposed max job gas limit %d or max block gas %d",
				agentcommon.ErrSchedulerInvalidGasLimit, id, job.GasLimit, params.MaxJobGasLimit, params.MaxBlockGas)
		}
	}
	return nil
}

// EndBlocker reads the current due index on every call, runs its admitted jobs
// and returns their outcome event encoding and settlement errors. Outcomes
// cross the module boundary as typed Cosmos events, decoded with sdk.ParseTypedEvent.
func (k Keeper) EndBlocker(ctx sdk.Context) error {
	_, err := k.RunDue(ctx)
	return err
}

// RunDue admits jobs due at the block height in (block, id) order within
// MaxJobsPerBlock and the sum of GasLimit bounded by MaxBlockGas; the rest
// stay due and roll to the next height. It returns the actual gas used.
// Each job runs as its creator in a cached context under its own gas
// limit; gasUsed x baseFee of its deposit goes to the fee collector whether
// it succeeds or fails and the rest returns to the creator. Nothing a job
// does halts the block; the returned error joins the encoding failures of
// outcome events and outstanding payments, reported after every admitted job.
func (k Keeper) RunDue(ctx sdk.Context) (gasUsed uint64, err error) {
	if ctx.BlockHeight() < 0 || k.Active(ctx) != nil {
		return 0, nil
	}
	params := k.GetParams(ctx)
	// Payments are independent of admission and remain payable while paused.
	var errs []error
	if err := k.RetrySettlements(ctx, params.MaxJobsPerBlock); err != nil {
		errs = append(errs, err)
	}
	if params.Paused {
		return 0, errors.Join(errs...)
	}
	var due []Job
	var reserved uint64
	k.IterateDue(ctx, uint64(ctx.BlockHeight()), func(job Job) bool {
		if uint64(len(due)) >= params.MaxJobsPerBlock || job.GasLimit > params.MaxBlockGas-reserved {
			return true
		}
		reserved += job.GasLimit
		due = append(due, job)
		return uint64(len(due)) >= params.MaxJobsPerBlock
	})
	baseFee := k.baseFee(ctx)
	for _, admitted := range due {
		job, found := k.GetJob(ctx, admitted.ID)
		if !found || !job.Active || job.ExecuteAtBlock != admitted.ExecuteAtBlock ||
			job.ExecuteAtBlock > uint64(ctx.BlockHeight()) {
			continue
		}
		used, err := k.runJob(ctx, job, baseFee, params.MaxReasonBytes)
		gasUsed += used
		if err != nil {
			errs = append(errs, fmt.Errorf("job %d: %w", job.ID, err))
		}
	}
	return gasUsed, errors.Join(errs...)
}

func (k Keeper) runJob(ctx sdk.Context, job Job, baseFee *big.Int, reasonCap uint64) (uint64, error) {
	// Commit target effects, execution outcome and the transition out of the
	// runnable indexes together. An executing job cannot cancel itself.
	execution, write := ctx.CacheContext()
	job.Active = false
	k.SetJob(execution, job)
	gasUsed, reasons := k.execute(execution, job, baseFee)

	fee := new(big.Int).Mul(new(big.Int).SetUint64(gasUsed), baseFee)
	if deposit := job.Deposit.BigInt(); fee.Cmp(deposit) > 0 {
		fee = deposit
	}
	feeAmount := sdk.NewIntFromBigInt(fee)
	refund := job.Deposit.Sub(feeAmount)
	obligation := Settlement{
		JobID: job.ID, Creator: job.Creator, GasUsed: gasUsed,
		Reason:          truncate(strings.Join(reasons, "; "), reasonCap),
		FeeRecipient:    authtypes.NewModuleAddress(authtypes.FeeCollectorName),
		RefundRecipient: k.evm.GetPaxAddressOrDefault(execution, job.Creator),
		Fee:             feeAmount, Refund: refund,
	}
	k.setSettlement(execution, obligation)
	k.DeleteJob(execution, job)
	write()
	ctx.EventManager().EmitEvents(execution.EventManager().Events())

	paymentErr := k.settleJob(ctx, obligation)
	if paymentErr != nil {
		reasons = append(reasons, paymentErr.Error())
	}

	// No transaction receipt exists at end block, so job outcomes are Cosmos
	// typed events rather than EVM logs.
	if len(reasons) == 0 {
		return gasUsed, ctx.EventManager().EmitTypedEvent(&types.EventJobExecuted{
			JobId:   job.ID,
			Creator: job.Creator.Hex(),
			GasUsed: gasUsed,
			Refund:  refund.String(),
		})
	}
	eventErr := ctx.EventManager().EmitTypedEvent(&types.EventJobFailed{
		JobId:   job.ID,
		Creator: job.Creator.Hex(),
		Reason:  truncate(strings.Join(reasons, "; "), reasonCap),
	})
	return gasUsed, errors.Join(paymentErr, eventErr)
}

// execute applies the job as a message from its creator to its target with
// the job's gas limit, priced at baseFee for GASPRICE. State changes are written only on success. It returns
// the gas used and, on failure, the deterministic reason.
func (k Keeper) execute(ctx sdk.Context, job Job, baseFee *big.Int) (gasUsed uint64, reasons []string) {
	cached, write := ctx.CacheContext()
	cached = cached.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(cached))
	defer func() {
		if recovered := recover(); recovered != nil {
			gasUsed, reasons = job.GasLimit, []string{fmt.Sprintf("execution panicked: %v", recovered)}
		}
	}()
	stateDB := state.NewDBImpl(cached, k.evm, false)
	gasPool := k.evm.GetGasPool()
	blockCtx, err := k.evm.GetVMBlockContext(cached, gasPool)
	if err != nil {
		return 0, []string{err.Error()}
	}
	sstore := k.evm.GetSstoreSetGasEIP2200(cached)
	cfg := evmtypes.DefaultChainConfig().EthereumConfigWithSstore(k.evm.ChainID(cached), &sstore)
	target := job.Target
	msg := &core.Message{
		From:      job.Creator,
		To:        &target,
		Value:     new(big.Int),
		GasLimit:  job.GasLimit,
		GasPrice:  new(big.Int),
		GasFeeCap: new(big.Int),
		GasTipCap: new(big.Int),
		Data:      job.CallData,
	}
	txCtx := core.NewEVMTxContext(msg)
	txCtx.GasPrice = new(big.Int).Set(baseFee)
	evm := vm.NewEVM(*blockCtx, stateDB, cfg, vm.Config{}, k.evm.CustomPrecompiles(cached))
	evm.SetTxContext(txCtx)
	result, err := core.NewStateTransition(evm, msg, &gasPool, true, false).Execute()
	if err != nil {
		return 0, []string{err.Error()}
	}
	if result.Err != nil {
		return result.UsedGas, []string{revertReason(result)}
	}
	if _, err := stateDB.Finalize(); err != nil {
		return result.UsedGas, []string{err.Error()}
	}
	if err := k.sweepCoinbase(cached); err != nil {
		return result.UsedGas, []string{err.Error()}
	}
	write()
	ctx.EventManager().EmitEvents(cached.EventManager().Events())
	return result.UsedGas, nil
}

// sweepCoinbase moves what the job sent to block.coinbase, held by the
// temporary coinbase account of ctx's transaction index, to the fee
// collector, as the EVM end block does for transactions.
func (k Keeper) sweepCoinbase(ctx sdk.Context) error {
	bank := k.evm.BankKeeper()
	coinbase := state.GetCoinbaseAddress(ctx.TxIndex())
	denom := k.evm.GetBaseDenom(ctx)
	uhpx := bank.GetBalance(ctx, coinbase, denom).Amount.Sub(bank.LockedCoins(ctx, coinbase).AmountOf(denom))
	wei := bank.GetWeiBalance(ctx, coinbase)
	if uhpx.IsZero() && wei.IsZero() {
		return nil
	}
	return bank.SendCoinsAndWei(ctx, coinbase, authtypes.NewModuleAddress(authtypes.FeeCollectorName), uhpx, wei)
}

// revertReason is the Error(string) message of a reverted job, else the
// EVM error.
func revertReason(result *core.ExecutionResult) string {
	if data := result.Revert(); data != nil {
		if reason, err := abi.UnpackRevert(data); err == nil {
			return reason
		}
	}
	return result.Err.Error()
}

// truncate cuts reason to at most limit bytes without splitting a rune.
func truncate(reason string, limit uint64) string {
	if uint64(len(reason)) <= limit {
		return reason
	}
	cut := int(limit)
	for cut > 0 && !utf8.RuneStart(reason[cut]) {
		cut--
	}
	return reason[:cut]
}
