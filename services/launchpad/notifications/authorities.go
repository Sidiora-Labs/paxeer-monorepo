package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
)

type CanonicalPools struct {
	DB       *sql.DB
	Accounts *AccountClient
}

var canonicalHash = regexp.MustCompile(`^0x[0-9a-f]{64}$`)

func (p CanonicalPools) CanonicalPool(ctx context.Context, pool string) (bool, error) {
	if p.DB == nil {
		return false, ErrUnavailable
	}
	if !notificationWalletPattern.MatchString(pool) || pool != strings.ToLower(pool) {
		return false, ErrInvalid
	}
	_, err := p.ResolvePublic(ctx, resourcePool, pool)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return err == nil, err
}

func (p CanonicalPools) VerifiedAccount(ctx context.Context, account string) (bool, error) {
	if p.Accounts == nil {
		return false, ErrUnavailable
	}
	return p.Accounts.VerifiedAccount(ctx, account)
}

func (p CanonicalPools) ResolvePublic(ctx context.Context, kind, id string) (discovery.PublicResource, error) {
	var out discovery.PublicResource
	if p.DB == nil {
		return out, ErrUnavailable
	}
	if kind != resourcePool {
		return out, ErrUnavailable
	}
	if !notificationWalletPattern.MatchString(id) || strings.ToLower(id) != id {
		return out, ErrInvalid
	}
	var pool, token, creator, blockHash, txHash string
	var height, timestamp int64
	var logIndex uint64
	err := p.DB.QueryRowContext(ctx, `SELECT pool,token,creator,height,block_hash,transaction_hash,log_index,block_timestamp FROM indexer.indexer_canonical_creations_v1 WHERE pool=$1`, id).Scan(&pool, &token, &creator, &height, &blockHash, &txHash, &logIndex, &timestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return out, sql.ErrNoRows
	}
	if err != nil {
		return out, ErrUnavailable
	}
	for _, address := range []string{pool, token, creator} {
		if !notificationWalletPattern.MatchString(address) || strings.ToLower(address) != address {
			return out, ErrInvalid
		}
	}
	if pool != id || height <= 0 || timestamp <= 0 || logIndex > uint64(^uint32(0)) || !canonicalHash.MatchString(blockHash) || !canonicalHash.MatchString(txHash) {
		return out, ErrInvalid
	}
	out.Ref = discovery.SourceRef{ID: pool, Kind: resourcePool, EventID: txHash + ":" + strconv.FormatUint(logIndex, 10), CreatorPublicID: creator, Pools: []string{pool}, SourceRevision: uint64(height), VisibilityRevision: 1, PublishedAt: time.Unix(timestamp, 0).UTC(), Public: true, Authorized: true}
	return out, out.Ref.Validate()
}

func (p CanonicalPools) PublicSources(ctx context.Context) ([]discovery.SourceRef, error) {
	if p.DB == nil {
		return nil, ErrUnavailable
	}
	rows, err := p.DB.QueryContext(ctx, `SELECT pool FROM indexer.indexer_canonical_creations_v1 ORDER BY block_timestamp DESC,transaction_hash,log_index LIMIT 2000`)
	if err != nil {
		return nil, ErrUnavailable
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, errors.Join(err, rows.Close())
		}
		ids = append(ids, id)
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return nil, err
	}
	out := make([]discovery.SourceRef, 0, len(ids))
	for _, id := range ids {
		resource, err := p.ResolvePublic(ctx, resourcePool, id)
		if err != nil {
			return nil, err
		}
		out = append(out, resource.Ref)
	}
	return out, nil
}

func NewRuntimeAuthorities(ctx context.Context, db *sql.DB, config RuntimeConfig, quote string) (discovery.SourceAuthority, discovery.IntervalAuthority, error) {
	if db == nil || !notificationWalletPattern.MatchString(quote) || quote != strings.ToLower(quote) {
		return nil, nil, ErrInvalid
	}
	// Read the exact owner interfaces up front, including permission and column checks.
	for _, query := range []string{
		`SELECT pool,token,creator,height,block_hash,transaction_hash,log_index,block_timestamp FROM indexer.indexer_canonical_creations_v1 LIMIT 0`,
		`SELECT snapshot_id,wallet,generation,start_at,end_at,input_digest,watermark FROM pnl.inventory_l16_wallet_ranges_v3 LIMIT 0`,
		`SELECT snapshot_id,interval,bucket,payload FROM pnl.inventory_l16_wallet_intervals_v3 LIMIT 0`,
	} {
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			return nil, nil, fmt.Errorf("discovery authority: %w", err)
		}
		if err = rows.Close(); err != nil {
			return nil, nil, err
		}
	}
	credentials, err := NewCredentials(ctx, config.Secrets, config.D11TokenRef)
	if err != nil {
		return nil, nil, err
	}
	accounts, err := NewAccountClient(config.D11Base, config.Client, credentials, config.D11TokenRef)
	if err != nil {
		return nil, nil, err
	}
	return CanonicalPools{DB: db, Accounts: accounts}, discovery.SQLIntervalAuthority{DB: db, Attribution: accounts, QuoteTokenAddress: quote}, nil
}
