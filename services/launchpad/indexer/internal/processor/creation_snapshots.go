package processor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/checked"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
)

const creationSnapshotPoolLimit = 100
const creationSnapshotPassTimeout = 30 * time.Second
const creationSnapshotReadTimeout = 5 * time.Second

func creationStateUnavailable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if receipt.IsPruned(err) || errors.Is(err, ethereum.NotFound) {
		return true
	}
	message := strings.ToLower(err.Error())
	for _, reason := range []string{"missing trie node", "historical state unavailable", "state unavailable", "state is not available", "state not available", "header not found", "block not found"} {
		if strings.Contains(message, reason) {
			return true
		}
	}
	return false
}

func (p *Processor) backfillCreationSnapshots(ctx context.Context) error {
	if p.deps.Reserves == nil {
		return errors.New("creation snapshot reserve reader required")
	}
	pass, cancel := context.WithTimeout(ctx, creationSnapshotPassTimeout)
	defer cancel()
	candidates, err := p.deps.Store.CreationSnapshotCandidates(pass, p.deps.ChainID, creationSnapshotPoolLimit)
	if err != nil {
		return fmt.Errorf("creation snapshot candidates: %w", err)
	}
	for _, c := range candidates {
		if pass.Err() != nil {
			break
		}
		if time.Since(p.canonicalRenewed) >= canonicalLeaseRenewInterval {
			if err = p.deps.Store.RenewLease(pass, p.deps.ChainID, p.deps.CanonicalStreamID, p.deps.CanonicalOwner, p.canonicalFence, canonicalLeaseTTL); err != nil {
				return err
			}
			p.canonicalRenewed = time.Now()
		}
		var snapshot *sqlcdb.InsertPoolStateSnapshotParams
		var block store.BlockCompleteness
		fallback := false
		index, readErr := checked.Convert[int32](c.LogIndex)
		if readErr == nil && c.BlockHash != "" && c.TransactionIndex >= 0 && c.LogIndex >= 0 && c.Height >= 0 && c.Timestamp > 0 && common.IsHexAddress(c.PoolAddress) {
			read, stop := context.WithTimeout(pass, creationSnapshotReadTimeout)
			reserves, e := p.deps.Reserves.PoolReserves(read, common.HexToAddress(c.PoolAddress), common.HexToHash(c.BlockHash))
			stop()
			readErr = e
			if creationStateUnavailable(e) && !c.LaterLogs {
				block, readErr = p.deps.Store.LastCompleteSnapshotBlock(pass, p.deps.ChainID, p.deps.CanonicalStreamID)
				if readErr == nil && block.Height >= c.Height {
					read, stop = context.WithTimeout(pass, creationSnapshotReadTimeout)
					reserves, readErr = p.deps.Reserves.PoolReserves(read, common.HexToAddress(c.PoolAddress), common.HexToHash(block.Hash))
					stop()
					fallback = true
				} else if readErr == nil {
					readErr = store.ErrIntegrity
				}
			}
			if readErr == nil {
				snapshot = &sqlcdb.InsertPoolStateSnapshotParams{ID: fmt.Sprintf("%s-%d-%d", c.PoolID, c.Height, c.LogIndex), PoolID: c.PoolID, VirtualReserve: reserves.VirtualUSDL.String(), RealReserve: reserves.RealUSDL.String(), TokenReserve: reserves.Token.String(), Price: reserves.Price.String(), BlockNumber: c.Height, BlockTimestamp: c.Timestamp, TxHash: c.TransactionHash, LogIndex: index}
			}
		} else if readErr == nil {
			readErr = store.ErrIntegrity
		}
		if snapshot == nil {
			p.deps.Logger.Warn("pool creation snapshot unavailable", slog.String("pool", c.PoolAddress), slog.Any("err", readErr))
		}
		if pass.Err() != nil {
			break
		}
		if err = p.deps.Store.CommitCreationSnapshot(pass, p.deps.ChainID, p.deps.CanonicalStreamID, p.deps.CanonicalOwner, p.canonicalFence, c, block, fallback, snapshot); err != nil {
			return fmt.Errorf("creation snapshot %s: %w", c.PoolAddress, err)
		}
	}
	return ctx.Err()
}
