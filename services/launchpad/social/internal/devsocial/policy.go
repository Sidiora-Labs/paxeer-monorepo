package devsocial

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

// usdl is the chain-125 settlement asset (lowercase).
var usdl = strings.ToLower(deployment.SettlementAddress)

// errChatDenied is the send/join refusal the hub reports to the client.
var errChatDenied = errors.New("chat policy denied")

// Policy is the write authority of this surface. Its inputs are chat.chat_bans
// (legacy bans), chat.safety_bans (moderation bans issued through the safety
// store: global, or pool-scoped), chat.interaction_erased_wallets, and the
// indexer's pools (Pools) for pool existence, token and creator.
type Policy struct {
	DB    *pgxpool.Pool
	Clock func() time.Time
	Pools PoolSource
}

// PoolSource is the indexer relation pool facts are read from.
type PoolSource int

const (
	// LegacyPools reads the indexer's typed indexer.pools table (the
	// development database's indexer projection).
	LegacyPools PoolSource = iota
	// CanonicalPools reads indexer.indexer_canonical_creations_v1, the
	// production indexer's versioned canonical-creations interface: the
	// indexer relation the migration coordinator grants chat_runtime
	// (column-scoped: pool, token, creator).
	CanonicalPools
)

// LookupPool returns the lowercase token and creator of an indexed pool from
// source; found is false when the indexer has not seen it.
func LookupPool(ctx context.Context, q pgxQuerier, source PoolSource, pool string) (token, creator string, found bool, err error) {
	query := `SELECT lower(token_address),lower(creator) FROM indexer.pools WHERE lower(pool_address)=lower($1) LIMIT 1`
	if source == CanonicalPools {
		query = `SELECT lower(token),lower(creator) FROM indexer.indexer_canonical_creations_v1 WHERE pool=lower($1)`
	}
	err = q.QueryRow(ctx, query, pool).Scan(&token, &creator)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	return token, creator, err == nil, err
}

func (p *Policy) now() time.Time {
	if p.Clock != nil {
		return p.Clock()
	}
	return time.Now()
}

// denied reports whether wallet may not write: an active global ban, an
// active ban on pool (when non-empty), or an erased account.
func (p *Policy) denied(ctx context.Context, q pgxQuerier, wallet, pool string) (bool, error) {
	var denied bool
	err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat.chat_bans WHERE wallet=$1 AND (expires_at IS NULL OR expires_at>$3) AND (pool_address IS NULL OR pool_address=NULLIF($2,'')))
 OR EXISTS(SELECT 1 FROM chat.safety_bans WHERE wallet=$1 AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at>to_timestamp($3)) AND (scope='global' OR (scope='pool' AND resource=NULLIF($2,''))))
 OR EXISTS(SELECT 1 FROM chat.interaction_erased_wallets WHERE wallet=$1)`, wallet, pool, p.now().Unix()).Scan(&denied)
	return denied, err
}

// pool returns the canonical (lowercase) token and creator of an indexed pool;
// errNotFound when the indexer has not seen it.
func (p *Policy) pool(ctx context.Context, q pgxQuerier, pool string) (token, creator string, err error) {
	token, creator, found, err := LookupPool(ctx, q, p.Pools, pool)
	if err == nil && !found {
		return "", "", errNotFound
	}
	return token, creator, err
}

// Check implements interactions.Authority: bans and erasure for every write,
// and pool existence when a comment is created.
func (p *Policy) Check(ctx context.Context, tx pgx.Tx, r interactions.Request) error {
	pool := r.Pool
	if pool == "" && r.ID != "" && (r.Action == "comment" || r.Action == "reaction") {
		if err := tx.QueryRow(ctx, `SELECT pool FROM chat.interaction_comments WHERE id=$1`, r.ID).Scan(&pool); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}
	denied, err := p.denied(ctx, tx, r.Actor, pool)
	if err != nil {
		return interactions.ErrDependency
	}
	if denied {
		return interactions.ErrDenied
	}
	if r.Action == "comment" && !r.Remove && r.Pool != "" {
		if _, _, err = p.pool(ctx, tx, r.Pool); err != nil {
			if errors.Is(err, errNotFound) {
				return pgx.ErrNoRows
			}
			return interactions.ErrDependency
		}
	}
	return nil
}

// CheckCreator implements interactions.CreatorAuthority: only the pool's
// creator may hide comments under it.
func (p *Policy) CheckCreator(ctx context.Context, tx pgx.Tx, r interactions.Request) error {
	_, creator, err := p.pool(ctx, tx, r.Pool)
	if errors.Is(err, errNotFound) {
		return interactions.ErrDenied
	}
	if err != nil {
		return interactions.ErrDependency
	}
	if creator != r.Actor {
		return interactions.ErrDenied
	}
	return nil
}

// SendChat is the fanout.SendPolicy for the development chat store: the
// sender must not be banned (globally or from the pool) or erased, a pool
// conversation needs an indexed pool, and a DM peer must not be erased. The
// platform profile also applies the authoritative identity and DM privacy checks.
func (p *Policy) SendChat(ctx context.Context, tx pgx.Tx, in fanout.SendInput) error {
	if p.Pools == CanonicalPools {
		principal, err := interactions.Identity(ctx)
		if err != nil {
			return err
		}
		if principal.Wallet != in.Actor {
			return interactions.ErrDenied
		}
	}
	pool := ""
	if in.Kind == "pool" {
		pool = in.Target
	}
	denied, err := p.denied(ctx, tx, in.Actor, pool)
	if err != nil {
		return err
	}
	if denied {
		return errChatDenied
	}
	switch in.Kind {
	case "pool":
		if _, _, err = p.pool(ctx, tx, in.Target); errors.Is(err, errNotFound) {
			return fanout.ErrNotFound
		}
		return err
	case "dm":
		if p.Pools == CanonicalPools {
			return p.chatAuthority().SendPolicy(ctx, tx, in)
		}
		var erased bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat.interaction_erased_wallets WHERE wallet=$1)`, in.Target).Scan(&erased); err != nil {
			return err
		}
		if erased {
			return errChatDenied
		}
		return nil
	default:
		return fanout.ErrInvalid
	}
}

// chatAuthority uses the same identity guards as the platform social graph.
func (p *Policy) chatAuthority() *interactions.PGPolicy {
	return &interactions.PGPolicy{
		DB:               p.DB,
		Accounts:         pgx.Identifier{"users", "account_state_v2"},
		Sessions:         pgx.Identifier{"users", "identity_sessions_v2"},
		IdentityAccounts: pgx.Identifier{"users", "identity_accounts_v2"},
		WalletLinks:      pgx.Identifier{"users", "identity_wallet_links_v2"},
		SessionGuard:     pgx.Identifier{"chat", "interaction_session_valid"},
		AccountGuard:     pgx.Identifier{"chat", "interaction_account_active"},
	}
}

func (p *Policy) ReadChat(ctx context.Context, tx pgx.Tx, actor, conversation string) error {
	principal, err := interactions.Identity(ctx)
	if err != nil {
		return err
	}
	if principal.Wallet != actor || tx == nil {
		return interactions.ErrDenied
	}
	var valid bool
	if err = tx.QueryRow(ctx, `SELECT chat.interaction_session_valid($1::uuid,$2::text)`, principal.SessionID, actor).Scan(&valid); err != nil {
		return interactions.ErrDependency
	}
	if !valid {
		return interactions.ErrDenied
	}
	return p.chatAuthority().ReadChatPolicy(ctx, tx, actor, conversation)
}
