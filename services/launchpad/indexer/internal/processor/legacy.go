package processor

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/checked"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
)

// projectQualifiedLegacy writes the typed read tables from a qualified
// canonical batch through the transaction-bound store. It applies exactly the
// projection of the legacy log path (EventEmitter logs decoded by the protocol
// registry, in chain order) to records that were already qualified, so the
// tables only ever hold source-qualified facts. Block timestamps and senders
// come from the qualified receipts; NFT ids come from the qualified factory
// MarketCreated facts of the same batch, so no chain read happens inside the
// canonical transaction.
func (p *Processor) projectQualifiedLegacy(ctx context.Context, st *store.Store, b store.CanonicalBatch, receipts []*receipt.QualifiedReceipt, reserves []reserveSnapshot) error {
	if st == nil {
		return errors.New("legacy projection dependencies required")
	}
	timestamps := make(map[string]int64, len(receipts))
	senders := make(map[string]string, len(receipts))
	envelopes := make([]*decode.DecodedEnvelope, 0)
	for _, q := range receipts {
		txHash, sender, timestamp, err := q.SourceContext(ctx)
		if err != nil {
			return err
		}
		if timestamp <= 0 {
			return store.ErrIntegrity
		}
		qualified, err := q.QualifiedEvents(ctx)
		if err != nil {
			return err
		}
		envelopes = append(envelopes, qualified...)
		timestamps[txHash] = timestamp
		senders[txHash] = sender
	}
	facts, err := qualifiedCreationFacts(ctx, receipts)
	if err != nil {
		return err
	}
	q := *p
	q.deps.Store = st
	q.nftCache = make(map[string]*int64, len(facts))
	for _, f := range facts {
		id, err := strconv.ParseInt(f.NFTID, 10, 64)
		if err != nil {
			return fmt.Errorf("creation nft id: %w", err)
		}
		q.nftCache[strings.ToLower(f.Pool)] = &id
	}
	return q.projectLegacyEnvelopes(ctx, b, envelopes, timestamps, senders, reserves)
}

// projectLegacyEnvelopes consumes registry envelopes already authenticated by
// the receipt authority. Raw records supply canonical order, never ABI or role
// authority. Historical mirrors and native family events share immutable keys.
func (p *Processor) projectLegacyEnvelopes(ctx context.Context, b store.CanonicalBatch, envelopes []*decode.DecodedEnvelope, timestamps map[string]int64, senders map[string]string, reserves []reserveSnapshot) error {
	type key struct {
		tx    string
		index uint
	}
	events := make(map[key]*decode.Event)
	for _, envelope := range envelopes {
		if envelope == nil || envelope.Status != "decoded" || envelope.Event == nil || envelope.Family == nil {
			continue
		}
		if envelope.Family.Authority != "primary" && envelope.Family.Authority != "mirror" {
			continue
		}
		if envelope.EmitterRole != "EventEmitter" && !nativeLegacyRole(envelope) {
			continue
		}
		k := key{envelope.Raw.Identity.TransactionHash, envelope.Raw.Identity.LogIndex}
		if _, exists := events[k]; exists {
			return store.ErrIntegrity
		}
		events[k] = envelope.Event
	}
	for _, r := range b.Records {
		rLogIndexUint, conversionErr := checked.Convert[uint](r.LogIndex)
		if conversionErr != nil {
			return conversionErr
		}
		ev := events[key{r.TransactionHash, rLogIndexUint}]
		if ev == nil {
			continue
		}
		timestamp, ok := timestamps[r.TransactionHash]
		rHeightUint64, conversionErr := checked.Convert[uint64](r.Height)
		if conversionErr != nil {
			return conversionErr
		}
		if !ok || timestamp <= 0 || ev.TxHash != r.TransactionHash || ev.LogIndex != rLogIndexUint || ev.BlockNumber != rHeightUint64 {
			return store.ErrIntegrity
		}
		if _, err := p.processEvent(ctx, ev, timestamp, senders); err != nil {
			return fmt.Errorf("legacy projection %s @ %d: %w", ev.EventName, ev.BlockNumber, err)
		}
	}
	for _, r := range reserves {
		timestamp, ok := timestamps[r.TransactionHash]
		if !ok {
			return store.ErrIntegrity
		}
		rLogIndexInt32, conversionErr := checked.Convert[int32](r.LogIndex)
		if conversionErr != nil {
			return conversionErr
		}
		if err := p.deps.Store.InsertPoolStateSnapshot(ctx, sqlcdb.InsertPoolStateSnapshotParams{
			ID:             fmt.Sprintf("%s-%d-%d", r.PoolID, r.Height, r.LogIndex),
			PoolID:         r.PoolID,
			VirtualReserve: r.Reserves.VirtualUSDL.String(),
			RealReserve:    r.Reserves.RealUSDL.String(),
			TokenReserve:   r.Reserves.Token.String(),
			Price:          r.Reserves.Price.String(),
			BlockNumber:    r.Height,
			BlockTimestamp: timestamp,
			TxHash:         r.TransactionHash,
			LogIndex:       rLogIndexInt32,
		}); err != nil {
			return fmt.Errorf("legacy projection reserves: %w", err)
		}
	}
	return nil
}

func nativeLegacyRole(envelope *decode.DecodedEnvelope) bool {
	if envelope.Raw.Emitter != strings.ToLower(decode.LaunchpadPrecompileAddress.Hex()) {
		return false
	}
	switch envelope.EmitterRole {
	case decode.LaunchpadPrecompileRole, "SidioraPool", "SidioraFactory", "SidioraNFT", "PoolRegistry", "Router", "ProtocolConfig", "FeeAccumulator", "FeesRouter":
		return true
	}
	return false
}

// reserveSnapshot is a pool's pinned post-block reserve state, attributed to
// the block's last qualified swap on that pool.
type reserveSnapshot struct {
	PoolID, TransactionHash string
	Height                  int64
	LogIndex                uint32
	Reserves                receipt.PoolReserves
}

// readSwapReserves reads, outside the canonical transaction, the reserves of
// every pool with a qualified swap, pinned to the swap's block hash (EIP-1898).
// SidioraPool emits PoolStateUpdated only from syncReserves(), so a swap does
// not announce reserves. One read per (pool, block): the state after that
// block, attributed to its last swap on the pool.
func (p *Processor) readSwapReserves(ctx context.Context, fills []accounting.FillV1) ([]reserveSnapshot, error) {
	if len(fills) == 0 {
		return nil, nil
	}
	if p.deps.Reserves == nil {
		return nil, errors.New("legacy projection reserve reader required")
	}
	type key struct{ pool, block string }
	last := map[key]accounting.FillV1{}
	order := []key{}
	for _, f := range fills {
		k := key{f.PoolAddress, f.Position.BlockHash}
		prior, seen := last[k]
		if !seen {
			order = append(order, k)
			last[k] = f
			continue
		}
		cmp, err := accounting.CompareSourcePosition(prior.Position, f.Position)
		if err != nil {
			return nil, err
		}
		if cmp < 0 {
			last[k] = f
		}
	}
	out := make([]reserveSnapshot, 0, len(order))
	for _, k := range order {
		f := last[k]
		if !common.IsHexAddress(k.pool) || len(k.block) != 66 {
			return nil, store.ErrIntegrity
		}
		pool := common.HexToAddress(k.pool)
		reserves, err := p.deps.Reserves.PoolReserves(ctx, pool, common.HexToHash(k.block))
		if err != nil {
			return nil, fmt.Errorf("pinned pool reserves %s: %w", k.pool, err)
		}
		height, err := strconv.ParseInt(f.Position.Height, 10, 64)
		if err != nil {
			return nil, err
		}
		logIndex, err := strconv.ParseUint(f.Position.LogIndex, 10, 32)
		if err != nil {
			return nil, err
		}
		logIndexUint32, conversionErr := checked.Convert[uint32](logIndex)
		if conversionErr != nil {
			return nil, conversionErr
		}
		out = append(out, reserveSnapshot{PoolID: poolIDFor(pool), TransactionHash: f.Position.TransactionHash, Height: height, LogIndex: logIndexUint32, Reserves: reserves})
	}
	return out, nil
}

// poolIDFor is the bytes32 pool id the protocol events carry for a pool: the
// address left-padded to 32 bytes, lowercase hex.
func poolIDFor(pool common.Address) string {
	return "0x" + hex.EncodeToString(common.LeftPadBytes(pool.Bytes(), 32))
}
