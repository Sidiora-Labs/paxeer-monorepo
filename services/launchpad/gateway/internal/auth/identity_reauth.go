package auth

import (
	"context"
	"net"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cleanup"
)

// IssueStepUp requires an existing authoritative session, but permits its proof to be older than five minutes.
func (s *Identity) IssueStepUp(ctx context.Context, p Principal, ip string) (Challenge, error) {
	if net.ParseIP(ip) == nil {
		return Challenge{}, invalid()
	}
	if e := s.CheckSocket(ctx, p); e != nil {
		return Challenge{}, e
	}
	return s.issue(ctx, p.wallet, "step_up", p.accountID, p.sessionID, "", ip)
}

// VerifyStepUp returns a short-lived one-use action receipt, never a wallet capability.
func (s *Identity) VerifyStepUp(ctx context.Context, p Principal, id string, proof WalletProof) (string, error) {
	return s.verifyActionStepUp(ctx, p, id, proof, "step_up", walletUnlinkEvent)
}
func (s *Identity) IssueActionStepUp(ctx context.Context, p Principal, ip, action string) (Challenge, error) {
	if action != accountDeleteScope || net.ParseIP(ip) == nil {
		return Challenge{}, invalid()
	}
	if err := s.CheckSocket(ctx, p); err != nil {
		return Challenge{}, err
	}
	return s.issue(ctx, p.wallet, "step_up:"+action, p.accountID, p.sessionID, "", ip)
}
func (s *Identity) VerifyActionStepUp(ctx context.Context, p Principal, id string, proof WalletProof, action string) (string, error) {
	if action != accountDeleteScope {
		return "", invalid()
	}
	return s.verifyActionStepUp(ctx, p, id, proof, "step_up:"+action, action)
}
func (s *Identity) verifyActionStepUp(ctx context.Context, p Principal, id string, proof WalletProof, purpose, action string) (string, error) {
	wallet, e := normalizeWallet(proof.Wallet)
	if e != nil || wallet != p.wallet {
		return "", unauthorized()
	}
	if _, e := s.verifyChallenge(ctx, id, proof, purpose, p.accountID, p.sessionID, ""); e != nil {
		return "", e
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return "", unavailable()
	}
	defer cleanup.Rollback(ctx, tx)
	var epoch int64
	e = tx.QueryRow(ctx, `SELECT auth_epoch FROM identity_accounts WHERE account_id=$1 FOR UPDATE`, p.accountID).Scan(&epoch)
	if e != nil || epoch != p.epoch {
		return "", unauthorized()
	}
	var ok bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM identity_sessions WHERE session_id=$1 AND account_id=$2 AND revoked_at IS NULL AND access_expires_at>clock_timestamp())`, p.sessionID, p.accountID).Scan(&ok)
	if e != nil {
		return "", unavailable()
	}
	if !ok {
		return "", unauthorized()
	}
	if e = consume(ctx, tx, id, s.now()); e != nil {
		return "", e
	}
	secret, e := randomSecret()
	if e != nil {
		return "", e
	}
	now := s.now()
	_, e = tx.Exec(ctx, `UPDATE identity_sessions SET reauthenticated_at=$2 WHERE session_id=$1`, p.sessionID, now)
	if e != nil {
		return "", unavailable()
	}
	_, e = tx.Exec(ctx, `INSERT INTO identity_reauth(proof_hash,account_id,session_id,action,issued_at,expires_at) VALUES($1,$2,$3,$6,$4,$5)`, digest(secret), p.accountID, p.sessionID, now, now.Add(5*time.Minute), action)
	if e != nil {
		return "", unavailable()
	}
	if s.events != nil {
		if e = s.audit(ctx, tx, p, "step_up"); e != nil {
			return "", e
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return "", unavailable()
	}
	return secret, nil
}

// CollectExpired removes only expired operational records in bounded batches. Audit/outbox is retained.
func (s *Identity) CollectExpired(ctx context.Context, limit int) error {
	if limit < 1 || limit > 1000 {
		return invalid()
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return unavailable()
	}
	defer cleanup.Rollback(ctx, tx)
	for _, statement := range identityCollectionStatements {
		if _, e = tx.Exec(ctx, statement, limit); e != nil {
			return unavailable()
		}
	}
	if e = tx.Commit(ctx); e != nil {
		return unavailable()
	}
	return nil
}

var identityCollectionStatements = []string{
	`DELETE FROM identity_limits WHERE ctid IN(SELECT ctid FROM identity_limits WHERE window_at<clock_timestamp()-interval '2 minutes' LIMIT $1 FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM identity_challenges WHERE ctid IN(SELECT ctid FROM identity_challenges WHERE expires_at<clock_timestamp()-interval '1 day' LIMIT $1 FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM identity_tickets WHERE ctid IN(SELECT ctid FROM identity_tickets WHERE expires_at<clock_timestamp()-interval '1 day' LIMIT $1 FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM identity_reauth WHERE ctid IN(SELECT ctid FROM identity_reauth WHERE expires_at<clock_timestamp()-interval '1 day' LIMIT $1 FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM identity_challenge_requests WHERE ctid IN(SELECT ctid FROM identity_challenge_requests WHERE retain_until<=clock_timestamp() LIMIT $1 FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM identity_owner_requests WHERE ctid IN(SELECT ctid FROM identity_owner_requests WHERE retain_until<=clock_timestamp() LIMIT $1 FOR UPDATE SKIP LOCKED)`,
	`DELETE FROM identity_wallet_links WHERE link_id IN(SELECT w.link_id FROM identity_wallet_links w WHERE w.state='pending' AND NOT EXISTS(SELECT 1 FROM identity_challenges c WHERE c.link_id=w.link_id) LIMIT $1 FOR UPDATE OF w SKIP LOCKED)`,
}

// RunCollector owns no goroutine: the application starts it with its shutdown context.
func (s *Identity) RunCollector(ctx context.Context, interval, timeout time.Duration, limit int) error {
	if interval < time.Second || interval > time.Hour || timeout <= 0 || timeout > interval || limit < 1 || limit > 1000 {
		return invalid()
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		bounded, cancel := context.WithTimeout(ctx, timeout)
		err := s.CollectExpired(bounded, limit)
		cancel()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
