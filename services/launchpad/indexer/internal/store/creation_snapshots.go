package store

import (
	"context"
	"errors"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5"
)

type CreationSnapshotCandidate struct {
	PoolAddress, PoolID, TransactionHash, BlockHash string
	Height, TransactionIndex, LogIndex, Timestamp   int64
	LaterLogs                                       bool
}

const creationSnapshotCandidatesSQL = `SELECT lower(p.pool_address),p.pool_id,p.tx_hash,p.created_block,p.created_at,
 COALESCE(c.block_hash,r.block_hash,''),COALESCE(c.transaction_index,r.transaction_index,-1),COALESCE(c.log_index,r.log_index,-1),
 EXISTS(SELECT 1 FROM indexer.indexer_raw_logs l WHERE l.chain_id=$1
 AND (l.height,l.transaction_index)>(p.created_block,COALESCE(c.transaction_index,r.transaction_index,-1))
 AND (lower(l.emitter)=lower(p.pool_address) OR l.topics @> jsonb_build_array(p.pool_id)))
 FROM indexer.pools p
 LEFT JOIN indexer.indexer_canonical_creations c ON c.pool=lower(p.pool_address) AND c.transaction_hash=p.tx_hash AND c.height=p.created_block
 LEFT JOIN LATERAL (SELECT block_hash,transaction_index,log_index FROM indexer.indexer_raw_logs
 WHERE chain_id=$1 AND transaction_hash=p.tx_hash AND height=p.created_block
 AND topics->>0=ANY($3::text[]) AND (topics->>1=p.pool_id OR topics->>2=p.pool_id)
 ORDER BY log_index LIMIT 1) r ON c.pool IS NULL
 LEFT JOIN indexer.backfill_jobs j ON j.id='pool-snapshot:'||lower(p.pool_address) AND j.chain_id=$1
 WHERE NOT EXISTS(SELECT 1 FROM indexer.pool_state_snapshots s WHERE s.pool_id=p.pool_id)
 ORDER BY COALESCE(j.updated_at,'-infinity'::timestamp),p.created_block,lower(p.pool_address) LIMIT $2`

func (s *Store) CreationSnapshotCandidates(ctx context.Context, chain int32, limit int) ([]CreationSnapshotCandidate, error) {
	if chain != 125 || limit <= 0 || limit > 100 {
		return nil, ErrIntegrity
	}
	rows, err := s.db.Query(ctx, creationSnapshotCandidatesSQL, chain, limit, creationSnapshotTopics)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CreationSnapshotCandidate
	for rows.Next() {
		var c CreationSnapshotCandidate
		if err = rows.Scan(&c.PoolAddress, &c.PoolID, &c.TransactionHash, &c.Height, &c.Timestamp, &c.BlockHash, &c.TransactionIndex, &c.LogIndex, &c.LaterLogs); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const lastCompleteSnapshotBlockSQL = `SELECT b.height,b.block_hash FROM indexer.indexer_ingest_watermarks w
 JOIN indexer.indexer_block_completeness b ON b.chain_id=w.chain_id AND b.stream_id=w.stream_id
 AND b.height=LEAST(w.raw_complete_height,w.typed_complete_height)
 WHERE w.chain_id=$1 AND w.stream_id=$2 AND b.fetch_complete AND b.required_decode_complete`

func (s *Store) LastCompleteSnapshotBlock(ctx context.Context, chain int32, stream string) (BlockCompleteness, error) {
	var b BlockCompleteness
	err := s.db.QueryRow(ctx, lastCompleteSnapshotBlockSQL, chain, stream).Scan(&b.Height, &b.Hash)
	return b, err
}

const creationSnapshotLeaseSQL = `SELECT fence_token FROM indexer.indexer_ingest_leases WHERE chain_id=$1 AND stream_id=$2 AND owner_instance=$3 AND expires_at>clock_timestamp() FOR UPDATE`
const creationSnapshotAttemptSQL = `INSERT INTO indexer.backfill_jobs(id,chain_id,from_block,to_block,last_processed_block,total_blocks,status)
 VALUES($1,$2,$3::bigint,$3::bigint,CASE WHEN $4 THEN $3::bigint ELSE $3::bigint-1 END,1,CASE WHEN $4 THEN 'completed' ELSE 'pending' END) ON CONFLICT(id) DO UPDATE SET last_processed_block=EXCLUDED.last_processed_block,status=EXCLUDED.status,updated_at=clock_timestamp()`
const creationSnapshotAbsentSQL = `SELECT NOT EXISTS(SELECT 1 FROM indexer.pool_state_snapshots WHERE pool_id=$1)`

func (s *Store) CommitCreationSnapshot(ctx context.Context, chain int32, stream, owner string, fence int64, c CreationSnapshotCandidate, block BlockCompleteness, fallback bool, snapshot *sqlcdb.InsertPoolStateSnapshotParams) error {
	db, ok := s.db.(beginner)
	if !ok || !s.CanonicalEpochGuarded() || chain != 125 || !addressPattern.MatchString(c.PoolAddress) {
		return ErrIntegrity
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	var current int64
	err = tx.QueryRow(ctx, creationSnapshotLeaseSQL, chain, stream, owner).Scan(&current)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && current != fence {
		return ErrFenced
	}
	if err != nil {
		return err
	}
	if snapshot != nil {
		if snapshot.PoolID != c.PoolID || snapshot.BlockNumber != c.Height || int64(snapshot.LogIndex) != c.LogIndex || snapshot.TxHash != c.TransactionHash || snapshot.BlockTimestamp != c.Timestamp {
			return ErrIntegrity
		}
		var absent bool
		if err = tx.QueryRow(ctx, creationSnapshotAbsentSQL, c.PoolID).Scan(&absent); err != nil {
			return err
		}
		if absent {
			if fallback {
				bound := NewFromDBTX(tx)
				latest, e := bound.LastCompleteSnapshotBlock(ctx, chain, stream)
				if e != nil {
					return e
				}
				if latest.Height != block.Height || latest.Hash != block.Hash || c.LaterLogs {
					return ErrIntegrity
				}
			}
			if err = NewFromDBTX(tx).InsertPoolStateSnapshot(ctx, *snapshot); err != nil {
				return err
			}
		}
	}
	if _, err = tx.Exec(ctx, creationSnapshotAttemptSQL, "pool-snapshot:"+c.PoolAddress, chain, c.Height, snapshot != nil); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var creationSnapshotTopics = []string{
	crypto.Keccak256Hash([]byte("MarketCreated(address,address,address,uint256,address)")).Hex(),
	crypto.Keccak256Hash([]byte("MarketCreated(bytes32,address,address,address,address,uint256,uint256)")).Hex(),
}
