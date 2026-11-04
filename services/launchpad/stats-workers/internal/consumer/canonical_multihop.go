package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

const canonicalMultihopTopic = "0xef2dce1faa347130cf25c6d059af1f6f181983e69a2fadd79ed8751a532d3d96"
const canonicalMultihopSignature = "MultihopSwap(address,address,address,uint256,uint256,uint256)"

var canonicalRouteAddress = regexp.MustCompile(`^0x[0-9a-f]{40}$`)

type canonicalRoute struct {
	position      accounting.SourcePosition
	emitter       string
	topics        []string
	data, payload []byte
	previous      int64
}

func (r canonicalRoute) event() (MultihopEvent, error) {
	var ev MultihopEvent
	if accounting.ValidateSourcePosition(r.position) != nil || !canonicalRouteAddress.MatchString(r.emitter) || r.emitter == "0x"+strings.Repeat("0", 40) || len(r.topics) != 4 || r.topics[0] != canonicalMultihopTopic || len(r.data) != 96 {
		return ev, ErrLegacyBridgeIntegrity
	}
	addresses := make([]string, 3)
	for i, topic := range r.topics[1:] {
		if len(topic) != 66 || !strings.HasPrefix(topic, "0x000000000000000000000000") {
			return ev, ErrLegacyBridgeIntegrity
		}
		addresses[i] = "0x" + topic[26:]
		if !canonicalRouteAddress.MatchString(addresses[i]) || addresses[i] == "0x"+strings.Repeat("0", 40) {
			return ev, ErrLegacyBridgeIntegrity
		}
	}
	if addresses[1] == addresses[2] {
		return ev, ErrLegacyBridgeIntegrity
	}
	ev.Sender, ev.TokenIn, ev.TokenOut = addresses[0], addresses[1], addresses[2]
	ev.AmountIn = new(big.Int).SetBytes(r.data[:32]).String()
	ev.IntermediateUsdl = new(big.Int).SetBytes(r.data[32:64]).String()
	ev.AmountOut = new(big.Int).SetBytes(r.data[64:]).String()
	if ev.AmountIn == "0" || ev.IntermediateUsdl == "0" || ev.AmountOut == "0" {
		return ev, ErrLegacyBridgeIntegrity
	}
	var issued struct {
		EventName, Sig, TxHash string
		BlockNumber            uint64
		LogIndex               uint64
		Args                   map[string]json.RawMessage
	}
	if json.Unmarshal(r.payload, &issued) != nil || issued.EventName != "MultihopSwap" || issued.Sig != canonicalMultihopSignature || issued.TxHash != r.position.TransactionHash || strconv.FormatUint(issued.BlockNumber, 10) != r.position.Height || strconv.FormatUint(issued.LogIndex, 10) != r.position.LogIndex {
		return ev, ErrLegacyBridgeIntegrity
	}
	for key, want := range map[string]string{"sender": ev.Sender, "tokenIn": ev.TokenIn, "tokenOut": ev.TokenOut, "amountIn": ev.AmountIn, "intermediateUsdl": ev.IntermediateUsdl, "amountOut": ev.AmountOut} {
		var got string
		if json.Unmarshal(issued.Args[key], &got) != nil || got != want {
			return ev, ErrLegacyBridgeIntegrity
		}
	}
	index, err := strconv.ParseUint(r.position.LogIndex, 10, 32)
	if err != nil {
		return ev, ErrLegacyBridgeIntegrity
	}
	ev.TxHash, ev.LogIndex = r.position.TransactionHash, int(index)
	return ev, nil
}

// multihops reads source-issued Router route intents, never inferring a route
// from neighbouring pool fills. The raw marker and each qualified leg remain
// bound to the same complete canonical receipt and source position.
func (b *CanonicalLegacyBridge) multihops(ctx context.Context, tx pgx.Tx, typed int64) (int, error) {
	rows, err := tx.Query(ctx, `SELECT r.transaction_hash,r.log_index,r.height,r.transaction_index,r.block_hash,r.emitter,r.topics,r.data,o.payload,
 coalesce((SELECT max(p.log_index) FROM `+b.source("indexer_ingest_outbox")+` p WHERE p.chain_id=o.chain_id AND p.transaction_hash=o.transaction_hash AND p.log_index<o.log_index AND p.event_kind IN ('Buy','Sell','MultihopSwap')),-1)
 FROM `+b.source("indexer_ingest_outbox")+` o JOIN `+b.source("indexer_raw_logs_v1")+` r USING(chain_id,transaction_hash,log_index)
 WHERE o.chain_id=125 AND o.event_kind='MultihopSwap' AND r.height<=$1
 AND NOT EXISTS(SELECT 1 FROM stats.cross_token_swaps s WHERE s.id=r.transaction_hash||'-'||r.log_index::text)
 ORDER BY r.height,r.transaction_index,r.log_index LIMIT $2`, typed, b.Batch)
	if err != nil {
		return 0, err
	}
	routes := []canonicalRoute{}
	for rows.Next() {
		var r canonicalRoute
		var topics []byte
		var index, height, transaction int64
		if err = rows.Scan(&r.position.TransactionHash, &index, &height, &transaction, &r.position.BlockHash, &r.emitter, &topics, &r.data, &r.payload, &r.previous); err != nil {
			rows.Close()
			return 0, err
		}
		r.position.ChainID, r.position.Height, r.position.TransactionIndex, r.position.LogIndex = "125", strconv.FormatInt(height, 10), strconv.FormatInt(transaction, 10), strconv.FormatInt(index, 10)
		if json.Unmarshal(topics, &r.topics) != nil {
			rows.Close()
			return 0, ErrLegacyBridgeIntegrity
		}
		routes = append(routes, r)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	applied := 0
	for _, r := range routes {
		inserted, ready, err := b.projectMultihop(ctx, r)
		if err != nil {
			return applied, fmt.Errorf("%w: multihop %s-%s", err, r.position.TransactionHash, r.position.LogIndex)
		}
		// Market partitions can arrive in either order; keep the earliest route
		// pending until both original source facts have committed locally.
		if !ready {
			break
		}
		if inserted {
			applied++
		}
	}
	return applied, nil
}

type canonicalRouteLeg struct {
	fill      accounting.FillV1
	swap      SwapEvent
	committed bool
}

func (b *CanonicalLegacyBridge) routeLegs(ctx context.Context, tx pgx.Tx, r canonicalRoute) ([]canonicalRouteLeg, error) {
	commits := pgx.Identifier{b.ProjectionSchema, "accounting_market_commits"}.Sanitize()
	routeIndex, _ := strconv.ParseInt(r.position.LogIndex, 10, 64)
	rows, err := tx.Query(ctx, `SELECT s.source_fill_id,s.source_fact_digest,s.pool_address,s.payload,l.emitter,l.topics,l.data,l.height,l.transaction_index,l.log_index,l.block_hash,
 c.fill,c.source_fact_digest
 FROM `+b.source("indexer_accounting_fills_v1")+` s JOIN `+b.source("indexer_raw_logs_v1")+` l USING(chain_id,transaction_hash,log_index)
 LEFT JOIN `+commits+` c ON c.source_fill_id=s.source_fill_id AND c.projection_version=$4
 WHERE s.chain_id=125 AND s.transaction_hash=$1 AND s.log_index>$2 AND s.log_index<$3
 ORDER BY s.log_index LIMIT 3`, r.position.TransactionHash, r.previous, routeIndex, b.ProjectionVersion)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	legs := []canonicalRouteLeg{}
	for rows.Next() {
		var id, digest, pool, emitter, block string
		var source, topics, data, committed []byte
		var commitDigest *string
		var height, transaction, index int64
		if err = rows.Scan(&id, &digest, &pool, &source, &emitter, &topics, &data, &height, &transaction, &index, &block, &committed, &commitDigest); err != nil {
			return nil, err
		}
		fill, err := accounting.DecodeFillV1(source)
		if err != nil {
			return nil, ErrLegacyBridgeIntegrity
		}
		actual, err := accounting.FillDigest(fill)
		if err != nil || string(actual) != digest || fill.SourceFillID != id || fill.PoolAddress != pool || fill.Position.LogIndex != strconv.FormatInt(index, 10) || fill.Position.TransactionHash != r.position.TransactionHash || fill.Position.Height != r.position.Height || fill.Position.Height != strconv.FormatInt(height, 10) || fill.Position.TransactionIndex != r.position.TransactionIndex || fill.Position.TransactionIndex != strconv.FormatInt(transaction, 10) || fill.Position.BlockHash != r.position.BlockHash || block != r.position.BlockHash {
			return nil, ErrLegacyBridgeIntegrity
		}
		var decoded []string
		if json.Unmarshal(topics, &decoded) != nil {
			return nil, ErrLegacyBridgeIntegrity
		}
		swap, err := LegacySwapFromCanonical(fill, emitter, decoded, data)
		if err != nil {
			return nil, err
		}
		leg := canonicalRouteLeg{fill: fill, swap: swap}
		if committed != nil {
			local, decodeErr := accounting.DecodeFillV1(committed)
			localDigest, digestErr := accounting.FillDigest(local)
			if decodeErr != nil || digestErr != nil || localDigest != actual || commitDigest == nil || *commitDigest != digest {
				return nil, ErrLegacyBridgeIntegrity
			}
			leg.committed = true
		}
		legs = append(legs, leg)
	}
	return legs, rows.Err()
}

func (b *CanonicalLegacyBridge) projectMultihop(ctx context.Context, r canonicalRoute) (bool, bool, error) {
	ev, err := r.event()
	if err != nil {
		return false, false, err
	}
	tx, err := b.Pool.Begin(ctx)
	if err != nil {
		return false, false, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var complete bool
	height, _ := strconv.ParseInt(r.position.Height, 10, 64)
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+b.source("indexer_block_completeness_v1")+` WHERE chain_id=125 AND stream_id=$1 AND height=$2 AND block_hash=$3 AND fetch_complete AND required_decode_complete)`, b.SourceStream, height, r.position.BlockHash).Scan(&complete)
	if err != nil {
		return false, false, err
	}
	if !complete {
		return false, false, ErrAccountingSourceUnavailable
	}
	legs, err := b.routeLegs(ctx, tx, r)
	if err != nil {
		return false, false, err
	}
	if len(legs) != 2 {
		return false, false, ErrLegacyBridgeIntegrity
	}
	in, out := legs[0], legs[1]
	if in.fill.Side != "sell" || out.fill.Side != "buy" || in.fill.Token.Address != ev.TokenIn || out.fill.Token.Address != ev.TokenOut || in.fill.PoolAddress == out.fill.PoolAddress || in.fill.TokenAmountRaw != ev.AmountIn || in.fill.USDLAmountRaw != ev.IntermediateUsdl || out.fill.USDLAmountRaw != ev.IntermediateUsdl || out.fill.TokenAmountRaw != ev.AmountOut || in.fill.Settlement != out.fill.Settlement || in.fill.Provenance.DeploymentID != out.fill.Provenance.DeploymentID || in.fill.Provenance.ReceiptSHA256 != out.fill.Provenance.ReceiptSHA256 || in.fill.Provenance.HeaderSHA256 != out.fill.Provenance.HeaderSHA256 || in.swap.Sender != ev.Sender || out.swap.Sender != ev.Sender || in.swap.BlockTimestamp != out.swap.BlockTimestamp {
		return false, false, ErrLegacyBridgeIntegrity
	}
	// Resolve associations from canonical creations, not current mutable metadata.
	for _, leg := range legs {
		var token string
		err = tx.QueryRow(ctx, `SELECT token FROM `+b.source("indexer_canonical_creations_v1")+` WHERE pool=$1 AND height<=$2`, leg.fill.PoolAddress, height).Scan(&token)
		if errors.Is(err, pgx.ErrNoRows) {
			return false, false, ErrAccountingSourceUnavailable
		}
		if err != nil {
			return false, false, err
		}
		if token != leg.fill.Token.Address {
			return false, false, ErrLegacyBridgeIntegrity
		}
	}
	if !in.committed || !out.committed {
		return false, false, nil
	}
	ev.PoolIn, ev.PoolOut, ev.FeeIn, ev.FeeOut, ev.BlockTimestamp = in.fill.PoolAddress, out.fill.PoolAddress, in.fill.FeeRaw, out.fill.FeeRaw, in.swap.BlockTimestamp
	c := NewMultihopConsumer(b.Swap.store, b.Swap.logger)
	inserted, err := c.ProjectCanonicalTx(ctx, tx, ev)
	if err != nil {
		return false, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, false, err
	}
	return inserted, true, nil
}
