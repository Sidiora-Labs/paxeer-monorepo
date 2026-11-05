package voice

import (
	"context"
	"errors"
	"fmt"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/jackc/pgx/v5"
	"sync"
	"time"
)

type SessionFinder interface {
	Find(context.Context, string) (string, bool, error)
}
type ReconcileResult struct{ Examined, Adopted, Terminated, Revoked, Orphans, Failed, Retried int }

type reconciliationRetry struct {
	attempts int
	next     time.Time
}
type reconciliationState struct {
	sync.Mutex
	roomCursor, recordingCursor string
	retries                     map[string]reconciliationRetry
}

func (c *Control) Reconcile(ctx context.Context) (ReconcileResult, error) {
	if c.DB == nil {
		return ReconcileResult{}, ErrUnavailable
	}
	return c.reconcileWithStore(ctx, postgresReconciliationStore{c})
}

func (c *Control) reconcileWithStore(ctx context.Context, store reconciliationStore) (ReconcileResult, error) {
	var result ReconcileResult
	if c.Provider == nil {
		return result, ErrUnavailable
	}
	finder, ok := c.Provider.(SessionFinder)
	if !ok {
		return result, ErrUnavailable
	}
	state := &c.reconciliation
	state.Lock()
	defer state.Unlock()
	if state.retries == nil {
		state.retries = make(map[string]reconciliationRetry)
	}
	var failures error
	attempt := func(key string, work func() error) {
		retry := state.retries[key]
		if c.now().Before(retry.next) {
			failures = errors.Join(failures, fmt.Errorf("%s retry deferred until %s: %w", key, retry.next.Format(time.RFC3339), ErrUnavailable))
			return
		}
		if retry.attempts > 0 {
			result.Retried++
		}
		if err := work(); err != nil {
			result.Failed++
			// Scheduling state never changes lifecycle timestamps or revisions.
			retry.attempts++
			retry.next = c.now().Add(time.Minute * time.Duration(1<<min(retry.attempts-1, 5)))
			state.retries[key] = retry
			failures = errors.Join(failures, fmt.Errorf("%s attempt %d: %w", key, retry.attempts, err))
		} else {
			delete(state.retries, key)
		}
	}
	ids, err := store.roomsAfter(ctx, state.roomCursor)
	if err == nil && len(ids) == 0 && state.roomCursor != "" {
		state.roomCursor = ""
		ids, err = store.roomsAfter(ctx, "")
	}
	if err != nil {
		failures = errors.Join(failures, err)
	} else {
		for _, id := range ids {
			if err := ctx.Err(); err != nil {
				return result, errors.Join(failures, err)
			}
			// Advance even on a locked room, a deferred retry, or a provider error.
			state.roomCursor = id
			attempt("room:"+id, func() error { return c.reconcileRoom(ctx, store, finder, id, &result) })
		}
	}
	if recorder, ok := c.Provider.(Recorder); ok {
		recordings, err := store.recordingsAfter(ctx, state.recordingCursor)
		if err == nil && len(recordings) == 0 && state.recordingCursor != "" {
			state.recordingCursor = ""
			recordings, err = store.recordingsAfter(ctx, "")
		}
		if err != nil {
			failures = errors.Join(failures, err)
		} else {
			for _, r := range recordings {
				if err := ctx.Err(); err != nil {
					return result, errors.Join(failures, err)
				}
				state.recordingCursor = r.id
				attempt("recording:"+r.id, func() error { return c.reconcileRecording(ctx, store, recorder, r) })
			}
		}
	}
	return result, failures
}

func (c *Control) reconcileRecording(ctx context.Context, store reconciliationStore, recorder Recorder, r reconciliationRecording) error {
	call, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	service := safety.Principal{Wallet: "service:voice-reconciler", SessionID: "runtime-worker"}
	if r.state == "starting" && r.roomState == "live" && c.recordingReady() && c.Gates.Ready() {
		id, err := recorder.Record(call, r.session, r.id)
		if err != nil {
			return err
		}
		return store.transitionRecording(ctx, r, "recording", id, service)
	}
	if r.provider == "" {
		finder, ok := c.Provider.(interface {
			FindRecording(context.Context, string, string) (string, bool, error)
		})
		if !ok {
			return ErrUnavailable
		}
		id, exists, err := finder.FindRecording(call, r.session, r.id)
		if err != nil {
			return err
		}
		if !exists {
			return ErrUnavailable
		}
		r.provider = id
	}
	if err := recorder.StopRecording(call, r.session, r.provider); err != nil {
		return err
	}
	return store.transitionRecording(ctx, r, "stopped", r.provider, service)
}
func (c *Control) RunReconciler(ctx context.Context, observe func(ReconcileResult, error)) error {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		result, e := c.Reconcile(ctx)
		if c.RecordingPolicy.Enabled {
			_, cleanupErr := c.DeleteExpiredRecordings(ctx)
			e = errors.Join(e, cleanupErr)
		}
		if observe != nil {
			observe(result, e)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (c *Control) RequestSpeaker(ctx context.Context, room string, expected int64) error {
	p, e := safety.Actor(ctx, c.now())
	if e != nil {
		return e
	}
	r, e := c.Read(ctx, room)
	if e != nil {
		return e
	}
	if r.State != "live" || !c.ready() {
		return ErrUnavailable
	}
	if e = c.permission(ctx, p.Wallet, room, r.PoolID); e != nil {
		return e
	}
	tag, e := c.DB.Exec(ctx, `UPDATE voice_participants SET role='requested_speaker',revision=revision+1 WHERE room_id=$1 AND wallet=$2 AND revision=$3 AND role='listener' AND NOT revoked AND grant_state='active'`, room, p.Wallet, expected)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return safety.ErrConflict
	}
	return nil
}
func (c *Control) Leave(ctx context.Context, room string) error {
	p, e := safety.Actor(ctx, c.now())
	if e != nil {
		return e
	}
	r, e := c.Read(ctx, room)
	if e != nil {
		return e
	}
	if c.Provider == nil {
		return ErrUnavailable
	}
	_, e = c.DB.Exec(ctx, `UPDATE voice_participants SET revoked=true,muted=true,grant_state='revoking',revision=revision+1 WHERE room_id=$1 AND wallet=$2 AND NOT revoked`, room, p.Wallet)
	if e != nil {
		return e
	}
	call, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	if e = c.Provider.Revoke(call, r.ProviderSession, p.Wallet); e != nil {
		return ErrUnavailable
	}
	_, e = c.DB.Exec(ctx, `UPDATE voice_participants SET grant_state='revoked' WHERE room_id=$1 AND wallet=$2 AND revoked AND grant_state='revoking'`, room, p.Wallet)
	return e
}
func (c *Control) DeleteAccount(ctx context.Context, wallet string) error {
	if c.DB == nil || c.Provider == nil {
		return ErrUnavailable
	}
	rows, e := c.DB.Query(ctx, `SELECT DISTINCT r.id,r.provider_session,r.host_wallet FROM voice_rooms r LEFT JOIN voice_participants p ON p.room_id=r.id WHERE r.state IN ('live','ending') AND (r.host_wallet=$1 OR p.wallet=$1) ORDER BY r.id LIMIT 100`, wallet)
	if e != nil {
		return e
	}
	type item struct{ id, session, host string }
	rooms := []item{}
	for rows.Next() {
		var r item
		if e = rows.Scan(&r.id, &r.session, &r.host); e != nil {
			rows.Close()
			return e
		}
		rooms = append(rooms, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, r := range rooms {
		if r.host == wallet {
			if _, e = c.DB.Exec(ctx, `UPDATE voice_rooms SET state='ending',revision=revision+1 WHERE id=$1 AND state='live'`, r.id); e != nil {
				return e
			}
			call, cancel := context.WithTimeout(ctx, 4*time.Second)
			e = c.Provider.End(call, r.session)
			cancel()
			if e != nil {
				return e
			}
			if _, e = c.DB.Exec(ctx, `UPDATE voice_rooms SET state='ended',revision=revision+1 WHERE id=$1 AND state='ending'`, r.id); e != nil {
				return e
			}
		} else {
			if _, e = c.DB.Exec(ctx, `UPDATE voice_participants SET revoked=true,muted=true,grant_state='revoking',revision=revision+1 WHERE room_id=$1 AND wallet=$2`, r.id, wallet); e != nil {
				return e
			}
			call, cancel := context.WithTimeout(ctx, 4*time.Second)
			e = c.Provider.Revoke(call, r.session, wallet)
			cancel()
			if e != nil {
				return e
			}
			if _, e = c.DB.Exec(ctx, `UPDATE voice_participants SET grant_state='revoked' WHERE room_id=$1 AND wallet=$2`, r.id, wallet); e != nil {
				return e
			}
		}
	}
	if len(rooms) == 100 {
		return errors.New("account voice cleanup needs another bounded batch")
	}
	return nil
}

func (c *Control) ApplyPolicyUpdate(ctx context.Context, banID string, expected int64) error {
	if c.DB == nil || c.Provider == nil {
		return ErrUnavailable
	}
	var wallet, scope, resource string
	var revision int64
	var active bool
	e := c.DB.QueryRow(ctx, `SELECT wallet,scope,resource,revision,revoked_at IS NULL AND (expires_at IS NULL OR expires_at>$2) FROM safety_bans WHERE id=$1`, banID, c.now()).Scan(&wallet, &scope, &resource, &revision, &active)
	if e != nil {
		return e
	}
	if expected < 1 || revision < expected {
		return safety.ErrConflict
	}
	if revision > expected || !active {
		return nil
	}
	rows, e := c.DB.Query(ctx, `WITH batch AS (SELECT p.room_id FROM voice_participants p JOIN voice_rooms r ON p.room_id=r.id WHERE r.state='live' AND p.wallet=$1 AND NOT p.revoked AND (p.grant_state<>'revoked' OR NOT p.muted) AND ($2='global' OR ($2='pool' AND r.pool_id=$3) OR ($2='voice' AND r.id::text=$3)) ORDER BY p.room_id LIMIT 100) UPDATE voice_participants p SET muted=true,grant_state='revoking',revision=p.revision+1 FROM voice_rooms r,batch WHERE p.room_id=r.id AND p.room_id=batch.room_id AND p.wallet=$1 RETURNING p.room_id::text,r.provider_session,p.revision`, wallet, scope, resource)
	if e != nil {
		return e
	}
	type item struct {
		room, session string
		revision      int64
	}
	items := []item{}
	for rows.Next() {
		var r item
		if e = rows.Scan(&r.room, &r.session, &r.revision); e != nil {
			rows.Close()
			return e
		}
		items = append(items, r)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	bounded, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	sem := make(chan struct{}, 16)
	var wg sync.WaitGroup
	errs := make(chan error, len(items))
	for _, r := range items {
		wg.Add(1)
		go func(r item) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-bounded.Done():
				errs <- bounded.Err()
				return
			}
			defer func() { <-sem }()
			if e := c.Provider.Revoke(bounded, r.session, wallet); e != nil {
				errs <- e
				return
			}
			_, e := c.DB.Exec(bounded, `UPDATE voice_participants SET grant_state='revoked' WHERE room_id=$1 AND wallet=$2 AND revision=$3 AND grant_state='revoking'`, r.room, wallet, r.revision)
			if e != nil {
				errs <- e
			}
		}(r)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			return e
		}
	}
	if len(items) == 100 {
		return errors.New("policy invalidation needs another bounded batch")
	}
	return nil
}

func (c *Control) reconcileRoom(ctx context.Context, store reconciliationStore, finder SessionFinder, id string, result *ReconcileResult) error {
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, e := store.beginRoom(call, id)
	if e != nil {
		return e
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	r, updated, e := tx.room(call)
	if e == pgx.ErrNoRows {
		return nil
	}
	if e != nil {
		return e
	}
	if r.State == "ended" {
		return nil
	}
	result.Examined++
	if r.State == "draft" {
		session, exists, err := finder.Find(call, r.StartKey)
		if err != nil {
			if c.now().Sub(updated) > 5*time.Minute {
				result.Orphans++
			}
			return err
		}
		if exists {
			if c.now().Sub(updated) > 5*time.Minute {
				result.Orphans++
			}
			if !safety.Bounded(session, 256) {
				return ErrUnavailable
			}
			if c.ready() && c.permission(call, r.HostWallet, r.ID, r.PoolID) == nil {
				if e = tx.adopt(call, session, c.now()); e != nil {
					return e
				}
				r.State = "live"
				r.ProviderSession = session
				if e = tx.Commit(call); e != nil {
					return e
				}
				result.Adopted++
				return nil
			}
			if e = c.Provider.End(call, session); e != nil {
				return e
			}
			if e = tx.Commit(call); e != nil {
				return e
			}
			result.Terminated++
			return nil
		}
		return tx.Commit(call)
	}
	if r.State == "ending" {
		if c.now().Sub(updated) > 5*time.Minute {
			result.Orphans++
		}
		if e = c.Provider.End(call, r.ProviderSession); e != nil {
			return e
		}
		if e = tx.end(call, c.now()); e != nil {
			return e
		}
		if e = tx.Commit(call); e != nil {
			return e
		}
		result.Terminated++
		return nil
	}
	session, exists, e := finder.Find(call, r.StartKey)
	if e != nil || !exists || session != r.ProviderSession {
		return ErrUnavailable
	}
	grants, e := tx.pendingGrants(call)
	if e != nil {
		return e
	}
	for _, g := range grants {
		if e = c.Provider.Revoke(call, r.ProviderSession, g.wallet); e != nil {
			return e
		}
		if e = tx.revokeGrant(call, g); e != nil {
			return e
		}
	}
	if e = tx.Commit(call); e != nil {
		return e
	}
	result.Revoked += len(grants)
	return nil
}
