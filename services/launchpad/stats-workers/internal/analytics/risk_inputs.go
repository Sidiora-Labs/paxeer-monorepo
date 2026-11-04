package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

// CanonicalRiskInputs produces the input row in the projection transaction.
// Coverage is checked at the requested block, rather than borrowing the live
// holder balances (which may already include later blocks).
type CanonicalRiskInputs struct {
	Authority                                                   ReactionAuthority
	HolderSchema, SourceSchema, SourceStream, ProjectionVersion string
}

func (p CanonicalRiskInputs) Validate() error {
	if _, err := p.Authority.table("analytics_risk_inputs"); err != nil {
		return err
	}
	for _, schema := range []string{p.HolderSchema, p.SourceSchema} {
		if !schemaPattern.MatchString(schema) {
			return errors.New("canonical risk schema required")
		}
	}
	if p.SourceStream == "" || len(p.SourceStream) > 128 || !noncePattern.MatchString(p.ProjectionVersion) {
		return errors.New("canonical risk source binding required")
	}
	return nil
}
func (p CanonicalRiskInputs) source(name string) string {
	return pgx.Identifier{p.SourceSchema, name}.Sanitize()
}
func (p CanonicalRiskInputs) holder(name string) string {
	return pgx.Identifier{p.HolderSchema, name}.Sanitize()
}
func (p CanonicalRiskInputs) LoadRiskInputs(ctx context.Context, tx pgx.Tx, pool string, wm Watermark) (RiskInputs, error) {
	if tx == nil {
		return RiskInputs{}, ErrDependencyUnavailable
	}
	var now time.Time
	if err := tx.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&now); err != nil {
		return RiskInputs{}, err
	}
	return p.LoadRiskInputsAt(ctx, tx, pool, wm, now.Unix())
}
func (p CanonicalRiskInputs) LoadRiskInputsAt(ctx context.Context, tx pgx.Tx, pool string, wm Watermark, now int64) (RiskInputs, error) {
	inputs := RiskInputs{UniqueTraders24h: -1, HolderCount: -1, ProjectionRevision: wm.ProjectionRevision}
	if err := p.Validate(); err != nil {
		return inputs, err
	}
	if tx == nil || !address.MatchString(pool) || wm.Validate() != nil || wm.Integrity != "healthy" || now <= 0 {
		return inputs, ErrInvalidInput
	}
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", p.Authority.Schema+":analytics:"+pool); err != nil {
		return inputs, err
	}
	var healthy bool
	err := tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s b JOIN %s w ON w.chain_id=b.chain_id AND w.stream_id=b.stream_id WHERE b.chain_id=125 AND b.stream_id=$1 AND b.height=$2::bigint AND b.block_hash=$3 AND b.fetch_complete AND b.required_decode_complete AND w.raw_complete_height=w.typed_complete_height AND w.typed_complete_height>=b.height) AND EXISTS(SELECT 1 FROM %s c WHERE c.chain_id=125 AND c.stream_id=$1 AND c.previous_height<$2::bigint AND c.last_complete_height>=$2::bigint)`, p.source("indexer_block_completeness_v1"), p.source("indexer_ingest_watermarks_v1"), p.source("indexer_ingest_commits_v1")), p.SourceStream, wm.Height, wm.BlockHash).Scan(&healthy)
	if err != nil {
		return inputs, err
	}
	if !healthy {
		return inputs, ErrDependencyUnavailable
	}
	var token, creator, creationHeight, creationTransaction string
	var metadata []byte
	err = tx.QueryRow(ctx, "SELECT token,lower(creator),block_timestamp,height::text,transaction_hash,payload FROM "+p.source("indexer_canonical_creations_v1")+" WHERE pool=$1 AND height<=$2::bigint", pool, wm.Height).Scan(&token, &creator, &inputs.CreatedAt, &creationHeight, &creationTransaction, &metadata)
	if err != nil {
		return inputs, err
	}
	if !address.MatchString(token) || !address.MatchString(creator) || token == holdings.Zero || inputs.CreatedAt <= 0 || inputs.CreatedAt > now {
		return inputs, ErrConflict
	}
	var creation struct {
		Metadata *struct {
			TotalSupply *string `json:"totalSupply"`
		} `json:"metadata"`
	}
	if err = json.Unmarshal(metadata, &creation); err != nil {
		return inputs, ErrConflict
	}
	var initialSupply string
	if creation.Metadata != nil && creation.Metadata.TotalSupply != nil {
		initialSupply = *creation.Metadata.TotalSupply
		if _, err = holdings.UInt(initialSupply); err != nil {
			return inputs, ErrConflict
		}
	}
	// Both directions are checked: a missing Transfer is lag, while an extra or
	// mutated holdings event must never be accepted as canonical ownership.
	transfers := p.source("indexer_token_transfers_v1")
	ledger := p.holder("transfers")
	match := `h.identity='125:'||s.token||':'||s.transaction_hash||':'||s.log_index::text AND h.event->>'chainId'='125' AND h.event->>'token'=s.token AND h.event->>'transactionHash'=s.transaction_hash AND h.event->>'blockHash'=s.block_hash AND h.event->>'height'=s.height::text AND h.event->>'transactionIndex'=s.transaction_index::text AND h.event->>'logIndex'=s.log_index::text AND h.event->>'from'=s.from_address AND h.event->>'to'=s.to_address AND h.event->>'amount'=s.amount_raw::text`
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s t WHERE t.token=$1 AND t.coverage='complete' AND t.expected_initial_supply=$4::numeric AND t.creation_transaction_hash=$5) AND (SELECT count(*) FROM %s b WHERE b.chain_id=125 AND b.stream_id=$3 AND b.height BETWEEN $6::bigint AND $2::bigint AND b.fetch_complete AND b.required_decode_complete)=($2::bigint-$6::bigint+1) AND NOT EXISTS(SELECT 1 FROM %s s WHERE s.chain_id=125 AND s.token=$1 AND s.height<=$2::bigint AND NOT EXISTS(SELECT 1 FROM %s h WHERE h.token=s.token AND %s)) AND NOT EXISTS(SELECT 1 FROM %s h WHERE h.token=$1 AND (h.event->>'height')::numeric<=$2::numeric AND NOT EXISTS(SELECT 1 FROM %s s WHERE s.chain_id=125 AND s.token=h.token AND %s))`, p.holder("tokens"), p.source("indexer_block_completeness_v1"), transfers, ledger, match, ledger, transfers, match), token, wm.Height, p.SourceStream, nullableSupply(initialSupply), creationTransaction, creationHeight).Scan(&healthy)
	if err != nil {
		return inputs, err
	}
	if healthy {
		rows, e := tx.Query(ctx, fmt.Sprintf(`WITH deltas AS (
 SELECT h.event->>'to' AS wallet,(h.event->>'amount')::numeric AS delta FROM %s h WHERE h.token=$1 AND (h.event->>'height')::numeric<=$2::numeric
 UNION ALL SELECT h.event->>'from',-(h.event->>'amount')::numeric FROM %s h WHERE h.token=$1 AND (h.event->>'height')::numeric<=$2::numeric
 ), balances AS (SELECT wallet,sum(delta) AS raw FROM deltas WHERE wallet<>$3 GROUP BY wallet HAVING sum(delta)<>0)
 SELECT wallet,raw::text,(sum(raw) OVER())::text FROM balances ORDER BY wallet`, ledger, ledger), token, wm.Height, holdings.Zero)
		if e != nil {
			return inputs, e
		}
		var accumulator *holdings.Accumulator
		inputs.SupplyRaw = "0"
		inputs.HolderCount = 0
		for rows.Next() {
			var wallet, raw, supply string
			if e = rows.Scan(&wallet, &raw, &supply); e != nil {
				rows.Close()
				return inputs, e
			}
			if accumulator == nil {
				accumulator, e = holdings.NewAccumulator(supply)
				if e != nil {
					rows.Close()
					return inputs, ErrConflict
				}
				inputs.SupplyRaw = supply
			}
			if supply != inputs.SupplyRaw {
				rows.Close()
				return inputs, ErrConflict
			}
			if e = accumulator.Add(holdings.Balance{Address: wallet, Raw: raw, Category: "unknown"}, wallet == creator); e != nil {
				rows.Close()
				return inputs, ErrConflict
			}
		}
		rows.Close()
		if e = rows.Err(); e != nil {
			return inputs, e
		}
		if accumulator != nil {
			distribution, e := accumulator.Finish()
			if e != nil {
				return inputs, ErrConflict
			}
			if distribution.AllPositiveCount > uint64(^uint(0)>>1) {
				return inputs, ErrConflict
			}
			inputs.HolderCount = int(distribution.AllPositiveCount)
			if distribution.Top10.BasisPoints != nil {
				inputs.Top10Concentration = *distribution.Top10.BasisPoints
			}
			if distribution.Creator.BasisPoints != nil {
				inputs.CreatorHoldingsPct = *distribution.Creator.BasisPoints
			}
		}
	}
	commits, _ := p.Authority.table("accounting_market_commits")
	rows, err := tx.Query(ctx, fmt.Sprintf(`SELECT f.payload,f.source_fact_digest,COALESCE(c.fill,'null'::jsonb),COALESCE(c.source_fact_digest,'') FROM %s f LEFT JOIN %s c ON c.source_fill_id=f.source_fill_id AND c.projection_version=$3 WHERE f.chain_id=125 AND f.pool_address=$1 AND (f.payload->'position'->>'height')::numeric<=$2::numeric ORDER BY (f.payload->'position'->>'height')::numeric,(f.payload->'position'->>'transactionIndex')::numeric,(f.payload->'position'->>'logIndex')::numeric`, p.source("indexer_accounting_fills_v1"), commits), pool, wm.Height, p.ProjectionVersion)
	if err != nil {
		return inputs, err
	}
	traders := map[string]bool{}
	complete := true
	for rows.Next() {
		var raw, committed []byte
		var digest, committedDigest string
		if err = rows.Scan(&raw, &digest, &committed, &committedDigest); err != nil {
			rows.Close()
			return inputs, err
		}
		fill, e := accounting.DecodeFillV1(raw)
		if e != nil {
			rows.Close()
			return inputs, ErrConflict
		}
		actual, e := accounting.FillDigest(fill)
		if e != nil || string(actual) != digest || fill.PoolAddress != pool || fill.Position.Height == wm.Height && fill.Position.BlockHash != wm.BlockHash {
			rows.Close()
			return inputs, ErrConflict
		}
		if committedDigest == "" {
			complete = false
			continue
		}
		c, e := accounting.DecodeFillV1(committed)
		if e != nil {
			rows.Close()
			return inputs, ErrConflict
		}
		cd, e := accounting.FillDigest(c)
		if e != nil || cd != actual || committedDigest != digest {
			rows.Close()
			return inputs, ErrConflict
		}
		timestamp, e := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
		if e != nil {
			rows.Close()
			return inputs, ErrConflict
		}
		if timestamp > now-86400 && timestamp <= now {
			traders[fill.Attribution.TransactionSender] = true
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return inputs, err
	}
	err = tx.QueryRow(ctx, fmt.Sprintf(`SELECT NOT EXISTS(SELECT 1 FROM %s c WHERE c.pool=$1 AND c.projection_version=$3 AND (c.fill->'position'->>'height')::numeric<=$2::numeric AND NOT EXISTS(SELECT 1 FROM %s f WHERE f.chain_id=125 AND f.source_fill_id=c.source_fill_id AND f.source_fact_digest=c.source_fact_digest AND f.payload=c.fill))`, commits, p.source("indexer_accounting_fills_v1")), pool, wm.Height, p.ProjectionVersion).Scan(&healthy)
	if err != nil {
		return inputs, err
	}
	if !healthy {
		return inputs, ErrConflict
	}
	if complete {
		inputs.UniqueTraders24h = len(traders)
	}
	if err = p.Authority.PersistRiskInputsTx(ctx, tx, pool, wm, inputs); err != nil {
		return inputs, err
	}
	return inputs, nil
}
func nullableSupply(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func validInputRevision(revision string) (*big.Int, error) {
	n, err := quantity(revision)
	if err != nil || n.Sign() == 0 {
		return nil, ErrConflict
	}
	return n, nil
}

// RefreshTokenInputs is called after durable holder catchup. It retries safely
// after restart because eligibility comes from durable analytics progress.
func (p *Projector) RefreshTokenInputs(ctx context.Context, token string) error {
	if p == nil {
		return ErrDependencyUnavailable
	}
	source, ok := p.Inputs.(CanonicalRiskInputs)
	if !ok || !address.MatchString(token) || p.Authority.Pool == nil {
		return ErrInvalidInput
	}
	progress, _ := p.Authority.table("analytics_progress")
	rows, err := p.Authority.Pool.Query(ctx, "SELECT p.pool FROM "+progress+" p JOIN "+source.source("indexer_canonical_creations_v1")+" k ON k.pool=p.pool WHERE k.token=$1 ORDER BY p.pool", token)
	if err != nil {
		return err
	}
	var pools []string
	for rows.Next() {
		var pool string
		if err = rows.Scan(&pool); err != nil {
			rows.Close()
			return err
		}
		pools = append(pools, pool)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	for _, pool := range pools {
		tx, e := p.Authority.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if e != nil {
			return e
		}
		e = p.RecomputeExpiryTx(ctx, tx, pool)
		if e == nil {
			e = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(context.WithoutCancel(ctx))
		}
		if e != nil {
			return e
		}
	}
	return nil
}
