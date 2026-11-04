package simulation

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	simtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/simulation"
)

const AverageBlockTime = 6 * time.Second

// SimulationConfig configures a fresh, deterministic application simulation.
// BeginBlock is the application's begin-block handler; the BaseApp's registered
// EndBlocker and InitChainer are used directly. Mount stores before Simulate,
// but leave the BaseApp unsealed: Simulate installs its FinalizeBlock handler.
type SimulationConfig struct {
	Seed         int64
	NumBlocks    int
	BlockSize    int
	Params       Params
	Genesis      abci.RequestInitChain
	Operations   WeightedOperations
	Accounts     []simtypes.Account
	BeginBlock   func(sdk.Context, abci.RequestBeginBlock) error
	LogDirectory string
}

type SimulationResult struct {
	Blocks     int
	Operations int
	LogPath    string
	Events     EventStats
}

// Simulate is the fork's supported in-process operation simulation entry point.
// It executes queued and selected operations inside FinalizeBlock, applies
// validator updates, and commits the real application store after each block.
func Simulate(tb testing.TB, app *baseapp.BaseApp, config SimulationConfig) (result SimulationResult, err error) {
	selectOp, err := config.Operations.getSelectOpFn()
	if err != nil {
		return result, err
	}
	if app == nil || config.BeginBlock == nil {
		return result, fmt.Errorf("simulation requires a BaseApp and application BeginBlock handler")
	}
	if config.NumBlocks <= 0 || config.BlockSize < 0 {
		return result, fmt.Errorf("simulation requires positive NumBlocks and nonnegative BlockSize")
	}
	p := config.Params
	if p.LivenessTransitionMatrix() == nil || math.IsNaN(p.EvidenceFraction()) || p.EvidenceFraction() < 0 || p.EvidenceFraction() >= 1 || math.IsNaN(p.PastEvidenceFraction()) || p.PastEvidenceFraction() < 0 || p.PastEvidenceFraction() > 1 {
		return result, fmt.Errorf("invalid simulation liveness matrix or evidence fractions")
	}
	total := 0
	for _, weight := range p.InitialLivenessWeightings() {
		if weight < 0 || weight > int(^uint(0)>>1)-total {
			return result, fmt.Errorf("invalid initial liveness weights")
		}
		total += weight
	}
	if total == 0 {
		return result, fmt.Errorf("initial liveness weights must have positive total")
	}
	start := config.Genesis.InitialHeight
	if start == 0 {
		start = 1
	}
	if start < 1 || int64(config.NumBlocks) > math.MaxInt64-start {
		return result, fmt.Errorf("invalid simulation height range")
	}
	r := rand.New(rand.NewSource(config.Seed))
	result.Events = NewEventStats()
	writer := NewLogWriter(true).(*StandardLogWriter)
	writer.Directory = config.LogDirectory
	defer func() {
		var logErr error
		result.LogPath, logErr = writer.PrintLogs()
		err = errors.Join(err, logErr)
	}()
	queue := NewOperationQueue()
	var timed []simtypes.FutureOperation
	var validators mockValidators
	history := VoteHistory{StartHeight: start}
	runOperation := func(ctx sdk.Context, op simtypes.Operation, queued bool, order int64) error {
		if op == nil {
			return fmt.Errorf("nil operation at height %d", ctx.BlockHeight())
		}
		msg, future, opErr := op(r, app, ctx, config.Accounts, ctx.ChainID())
		if opErr != nil {
			return fmt.Errorf("operation at height %d: %w", ctx.BlockHeight(), opErr)
		}
		for i, next := range future {
			if next.Op == nil {
				return fmt.Errorf("future operation %d at height %d is nil", i, ctx.BlockHeight())
			}
			if next.BlockHeight != 0 && int64(next.BlockHeight) <= ctx.BlockHeight() {
				return fmt.Errorf("future operation %d: height %d must follow current height %d", i, next.BlockHeight, ctx.BlockHeight())
			}
		}
		timed = queueOperations(queue, timed, future)
		msg.LogEvent(result.Events.Tally)
		if queued {
			writer.AddEntry(QueuedMsgEntry(ctx.BlockHeight(), msg))
		} else {
			writer.AddEntry(MsgEntry(ctx.BlockHeight(), order, msg))
		}
		result.Operations++
		return nil
	}
	app.SetFinalizeBlocker(func(ctx sdk.Context, req *abci.RequestFinalizeBlock) (*abci.ResponseFinalizeBlock, error) {
		begin, beginErr := RandomRequestBeginBlock(r, p, validators, history, result.Events.Tally, *req.Header)
		if beginErr != nil {
			return nil, beginErr
		}
		ctx = ctx.WithVoteInfos(begin.LastCommitInfo.Votes)
		*app.DeliverContext() = ctx
		writer.AddEntry(BeginBlockEntry(ctx.BlockHeight()))
		if beginErr = config.BeginBlock(ctx, begin); beginErr != nil {
			return nil, beginErr
		}
		history.Times = append(history.Times, req.Header.Time)
		history.Votes = append(history.Votes, begin.LastCommitInfo.Votes)
		// Snapshot due work so an operation's newly scheduled work cannot recursively
		// execute in the same block. Time insertion is stable for equal deadlines.
		heightOps := queue[int(ctx.BlockHeight())]
		delete(queue, int(ctx.BlockHeight()))
		due := 0
		for due < len(timed) && !timed[due].BlockTime.After(ctx.BlockTime()) {
			due++
		}
		timeOps := append([]simtypes.FutureOperation(nil), timed[:due]...)
		timed = timed[due:]
		for _, op := range heightOps {
			if opErr := runOperation(ctx, op, true, -1); opErr != nil {
				return nil, opErr
			}
		}
		for _, op := range timeOps {
			if opErr := runOperation(ctx, op.Op, true, -1); opErr != nil {
				return nil, opErr
			}
		}
		for i := 0; i < config.BlockSize; i++ {
			if opErr := runOperation(ctx, selectOp(r), false, int64(i)); opErr != nil {
				return nil, opErr
			}
		}
		app.MidBlock(ctx, ctx.BlockHeight())
		end := app.EndBlock(ctx, abci.RequestEndBlock{Height: ctx.BlockHeight()})
		validators = updateValidators(tb, r, p, validators, end.ValidatorUpdates, result.Events.Tally)
		writer.AddEntry(EndBlockEntry(ctx.BlockHeight()))
		return &abci.ResponseFinalizeBlock{Events: end.Events, ValidatorUpdates: end.ValidatorUpdates}, nil
	})
	if err = app.LoadLatestVersion(); err != nil {
		return result, err
	}
	if app.LastBlockHeight() != 0 {
		return result, fmt.Errorf("simulation requires a fresh application store")
	}
	genesis, err := app.InitChain(context.Background(), &config.Genesis)
	if err != nil {
		return result, err
	}
	if genesis == nil {
		return result, fmt.Errorf("simulation requires an application InitChainer response")
	}
	validators = newMockValidators(r, genesis.Validators, p)
	for i := 0; i < config.NumBlocks; i++ {
		header := tmproto.Header{ChainID: config.Genesis.ChainId, Height: start + int64(i), Time: config.Genesis.Time.Add(time.Duration(i) * AverageBlockTime), ProposerAddress: validators.randomProposer(r)}
		if _, err = app.FinalizeBlock(context.Background(), &abci.RequestFinalizeBlock{Header: &header}); err != nil {
			return result, err
		}
		app.SetDeliverStateToCommit()
		if _, err = app.Commit(context.Background()); err != nil {
			return result, err
		}
		result.Blocks++
	}
	return result, nil
}
