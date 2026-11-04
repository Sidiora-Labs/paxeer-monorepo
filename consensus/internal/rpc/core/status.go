package core

import (
	"bytes"
	"context"
	"fmt"
	"time"

	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	tmbytes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/bytes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

// Status returns Tendermint status including node info, pubkey, latest block
// hash, app hash, block height, current max peer block height, and time.
// More: https://docs.tendermint.com/master/rpc/#/Info/status
//
// TODO(autobahn): several SyncInfo fields remain unpopulated under Autobahn
// because the CometBFT BlockStore / ConsensusReactor are not fed. Not
// blocking integration-test CI today (no reader), but external tooling will
// want them:
//
//   - MaxPeerBlockHeight / TotalSyncedTime / RemainingTime: require an
//     Autobahn equivalent of BlockSyncReactor.
//
// Autobahn height validation uses the app's head and GigaRouter's retained
// lower bound; classic height validation uses the BlockStore's range.
func (env *Environment) Status(ctx context.Context) (*coretypes.ResultStatus, error) {
	var (
		earliestBlockHeight   int64
		earliestBlockHash     tmbytes.HexBytes
		earliestAppHash       tmbytes.HexBytes
		earliestBlockTimeNano int64
	)

	var (
		latestBlockHash     tmbytes.HexBytes
		latestAppHash       tmbytes.HexBytes
		latestBlockTimeNano int64

		latestHeight = env.BlockStore.Height()
	)

	if latestHeight != 0 {
		if latestBlockMeta := env.BlockStore.LoadBlockMeta(latestHeight); latestBlockMeta != nil {
			latestBlockHash = latestBlockMeta.BlockID.Hash
			latestAppHash = latestBlockMeta.Header.AppHash
			latestBlockTimeNano = latestBlockMeta.Header.Time.UnixNano()
		}
	}

	// Under Autobahn the CometBFT block store isn't populated, so the height
	// above is stuck at 0. Pull the live height and app hash from the app —
	// ABCIInfo reports both the last height the app committed in FinalizeBlock
	// and the matching app hash.
	//
	// LastCommittedBlockHeight reports the last block consensus has finalized.
	// Under CometBFT commit == app-apply in one step, so latestHeight IS the
	// committed height. Under Autobahn they can briefly differ; pull the
	// consensus-committed number from the GigaRouter's cached CommitQC watch
	// (lock-free per-call atomic load).
	lastCommittedBlockHeight := latestHeight
	catchingUp := true
	var applicationInfo coretypes.ApplicationInfo
	gigaRouter := env.gigaRouter()
	if giga, ok := gigaRouter.Get(); ok {
		abciInfo, err := env.ABCIInfo(ctx)
		if err != nil {
			return nil, fmt.Errorf("load Autobahn application info: %w", err)
		}
		latestHeight = abciInfo.Response.LastBlockHeight
		if latestHeight < 0 {
			return nil, fmt.Errorf("invalid Autobahn executed height %d", latestHeight)
		}
		latestAppHash = abciInfo.Response.LastBlockAppHash
		applicationInfo.Version = fmt.Sprint(abciInfo.Response.AppVersion)
		// Height zero is the startup-only empty executed state. It has no
		// block to look up, even if consensus has already finalized blocks.
		latestBlockHash = nil
		latestBlockTimeNano = 0
		if latestHeight > 0 {
			block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(latestHeight))
			if err != nil {
				return nil, fmt.Errorf("load latest Autobahn block at height %d: %w", latestHeight, err)
			}
			latestBlockHash = block.BlockID.Hash
			latestBlockTimeNano = block.Block.Header.Time.UnixNano()
		}
		lastCommittedBlockHeight = giga.LastCommittedBlockNumber()
		// The CommitQC head is Autobahn's known synchronization target. Keep
		// startup and recovery pending until the app has executed that head.
		catchingUp = latestHeight == 0 || latestHeight != lastCommittedBlockHeight
		earliestBlockHeight = giga.EarliestBlockNumber()
		if earliestBlockHeight <= lastCommittedBlockHeight {
			block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(earliestBlockHeight))
			if err != nil {
				return nil, fmt.Errorf("load earliest Autobahn block at height %d: %w", earliestBlockHeight, err)
			}
			earliestBlockHash = block.BlockID.Hash
			earliestAppHash = block.Block.Header.AppHash
			earliestBlockTimeNano = block.Block.Header.Time.UnixNano()
		}
		// Invariant: under Autobahn, consensus finalizes before the app
		// executes, so the committed height leads (or equals) the executed
		// height. Committed=0 is a legitimate startup window before the first
		// CommitQC is loaded; skip the log there.
		if lastCommittedBlockHeight > 0 && lastCommittedBlockHeight < latestHeight {
			logger.Error("autobahn /status invariant violated: LastCommittedBlockHeight < LatestBlockHeight",
				"committed", lastCommittedBlockHeight,
				"executed", latestHeight)
		}
	} else if earliestBlockMeta := env.BlockStore.LoadBaseMeta(); earliestBlockMeta != nil {
		earliestBlockHeight = earliestBlockMeta.Header.Height
		earliestAppHash = earliestBlockMeta.Header.AppHash
		earliestBlockHash = earliestBlockMeta.BlockID.Hash
		earliestBlockTimeNano = earliestBlockMeta.Header.Time.UnixNano()
	}

	// Return the very last voting power, not the voting power of this validator
	// during the last block.
	validatorInfo := coretypes.ValidatorInfo{PubKey: env.PubKey}
	if val, ok := env.validatorAtHeight(env.latestUncommittedHeight()).Get(); ok {
		validatorInfo.VotingPower = val.VotingPower
	}
	if !gigaRouter.IsPresent() {
		if abciInfo, err := env.ABCIInfo(ctx); err == nil {
			applicationInfo.Version = fmt.Sprint(abciInfo.Response.AppVersion)
		}
	}

	result := &coretypes.ResultStatus{
		NodeInfo:        env.NodeInfo,
		ApplicationInfo: applicationInfo,
		SyncInfo: coretypes.SyncInfo{
			LatestBlockHash:          latestBlockHash,
			LatestAppHash:            latestAppHash,
			LatestBlockHeight:        latestHeight,
			LatestBlockTime:          time.Unix(0, latestBlockTimeNano),
			LastCommittedBlockHeight: lastCommittedBlockHeight,
			EarliestBlockHash:        earliestBlockHash,
			EarliestAppHash:          earliestAppHash,
			EarliestBlockHeight:      earliestBlockHeight,
			EarliestBlockTime:        time.Unix(0, earliestBlockTimeNano),
			// this should start as true, if consensus
			// hasn't started yet, and then flip to false
			// (or true,) depending on what's actually
			// happening.
			CatchingUp: catchingUp,
		},
		ValidatorInfo: validatorInfo,
	}

	if reactor, ok := env.ConsensusReactor.Get(); ok && !gigaRouter.IsPresent() {
		result.SyncInfo.CatchingUp = reactor.WaitSync()
	}

	if reactor, ok := env.BlockSyncReactor.Get(); ok {
		result.SyncInfo.MaxPeerBlockHeight = reactor.GetMaxPeerBlockHeight()
		result.SyncInfo.TotalSyncedTime = reactor.GetTotalSyncedTime()
		result.SyncInfo.RemainingTime = reactor.GetRemainingSyncTime()
	}

	if reactor, ok := env.StateSyncReactor.Get(); ok {
		result.SyncInfo.TotalSnapshots = reactor.TotalSnapshots()
		result.SyncInfo.ChunkProcessAvgTime = reactor.ChunkProcessAvgTime()
		result.SyncInfo.SnapshotHeight = reactor.SnapshotHeight()
		result.SyncInfo.SnapshotChunksCount = reactor.SnapshotChunksCount()
		result.SyncInfo.SnapshotChunksTotal = reactor.SnapshotChunksTotal()
		result.SyncInfo.BackFilledBlocks = reactor.BackFilledBlocks()
		result.SyncInfo.BackFillBlocksTotal = reactor.BackFillBlocksTotal()
	}

	return result, nil
}

func (env *Environment) validatorAtHeight(h int64) utils.Option[*types.Validator] {
	none := utils.None[*types.Validator]()
	k, ok := env.PubKey.Get()
	if !ok {
		return none
	}
	valsWithH, err := env.StateStore.LoadValidators(h)
	if err != nil {
		return none
	}
	privValAddress := k.Address()

	// Skip the in-memory consensus-state lookup under Autobahn: the CometBFT
	// consensus State is never advanced, so GetValidators would nil-deref
	// on an unpopulated validator set. The state-store lookup below is kept
	// in sync under both engines.
	if consensusState, ok := env.ConsensusState.Get(); ok && !env.gigaRouter().IsPresent() {
		lastBlockHeight, vals := consensusState.GetValidators()
		if lastBlockHeight == h {
			for _, val := range vals {
				if bytes.Equal(val.Address, privValAddress) {
					return utils.Some(val)
				}
			}
		}
	}

	_, val, ok := valsWithH.GetByAddress(privValAddress)
	if ok {
		return utils.Some(val)
	}
	return none
}
