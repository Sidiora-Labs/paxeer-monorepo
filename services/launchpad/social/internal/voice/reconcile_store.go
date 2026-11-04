package voice

import (
	"context"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/jackc/pgx/v5"
)

// reconciliationStore separates durable lifecycle state from scan scheduling.
// Each room transaction retains the row lock across provider acknowledgements.
type reconciliationStore interface {
	roomsAfter(context.Context, string) ([]string, error)
	beginRoom(context.Context, string) (reconciliationRoomTx, error)
	recordingsAfter(context.Context, string) ([]reconciliationRecording, error)
	transitionRecording(context.Context, reconciliationRecording, string, string, safety.Principal) error
}

type reconciliationRoomTx interface {
	room(context.Context) (Room, time.Time, error)
	adopt(context.Context, string, time.Time) error
	end(context.Context, time.Time) error
	pendingGrants(context.Context) ([]reconciliationGrant, error)
	revokeGrant(context.Context, reconciliationGrant) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type reconciliationGrant struct {
	wallet   string
	revision int64
}
type reconciliationRecording struct {
	id, room, provider, session, state string
	revision                           int64
	roomState                          string
}

type postgresReconciliationStore struct{ control *Control }

func (s postgresReconciliationStore) roomsAfter(ctx context.Context, after string) ([]string, error) {
	rows, err := s.control.DB.Query(ctx, `SELECT id FROM voice_rooms WHERE state IN ('draft','live','ending') AND ($1='' OR id>NULLIF($1,'')::uuid) ORDER BY id LIMIT 100`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
func (s postgresReconciliationStore) beginRoom(ctx context.Context, id string) (reconciliationRoomTx, error) {
	tx, err := s.control.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return postgresReconciliationRoom{Tx: tx, id: id}, nil
}
func (s postgresReconciliationStore) recordingsAfter(ctx context.Context, after string) ([]reconciliationRecording, error) {
	rows, err := s.control.DB.Query(ctx, `SELECT rec.id,rec.room_id,rec.provider_id,r.provider_session,rec.state,rec.revision,r.state FROM voice_recordings rec JOIN voice_rooms r ON r.id=rec.room_id WHERE (rec.state IN ('starting','stopping') OR (rec.state='recording' AND r.state IN ('ending','ended'))) AND ($1='' OR rec.id>NULLIF($1,'')::uuid) ORDER BY rec.id LIMIT 100`, after)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var recordings []reconciliationRecording
	for rows.Next() {
		var r reconciliationRecording
		if err := rows.Scan(&r.id, &r.room, &r.provider, &r.session, &r.state, &r.revision, &r.roomState); err != nil {
			return nil, err
		}
		recordings = append(recordings, r)
	}
	return recordings, rows.Err()
}
func (s postgresReconciliationStore) transitionRecording(ctx context.Context, r reconciliationRecording, to, provider string, actor safety.Principal) error {
	return s.control.recordingTransition(ctx, r.id, r.room, r.revision, r.state, to, provider, actor)
}

type postgresReconciliationRoom struct {
	pgx.Tx
	id string
}

func (t postgresReconciliationRoom) room(ctx context.Context) (Room, time.Time, error) {
	var r Room
	var updated time.Time
	err := t.QueryRow(ctx, `SELECT id,start_key,state,provider_session,updated_at,host_wallet,pool_id FROM voice_rooms WHERE id=$1 FOR UPDATE SKIP LOCKED`, t.id).Scan(&r.ID, &r.StartKey, &r.State, &r.ProviderSession, &updated, &r.HostWallet, &r.PoolID)
	return r, updated, err
}
func (t postgresReconciliationRoom) adopt(ctx context.Context, session string, at time.Time) error {
	_, err := t.Exec(ctx, `UPDATE voice_rooms SET state='live',provider_session=$2,revision=revision+1,updated_at=$3 WHERE id=$1`, t.id, session, at)
	return err
}
func (t postgresReconciliationRoom) end(ctx context.Context, at time.Time) error {
	if _, err := t.Exec(ctx, `UPDATE voice_participants SET revoked=true,muted=true,grant_state='revoked',revision=revision+1 WHERE room_id=$1 AND (NOT revoked OR grant_state<>'revoked')`, t.id); err != nil {
		return err
	}
	_, err := t.Exec(ctx, `UPDATE voice_rooms SET state='ended',revision=revision+1,updated_at=$2 WHERE id=$1`, t.id, at)
	return err
}
func (t postgresReconciliationRoom) pendingGrants(ctx context.Context) ([]reconciliationGrant, error) {
	rows, err := t.Query(ctx, `SELECT wallet,revision FROM voice_participants WHERE room_id=$1 AND grant_state IN ('granting','revoking') ORDER BY wallet LIMIT 100 FOR UPDATE`, t.id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var grants []reconciliationGrant
	for rows.Next() {
		var g reconciliationGrant
		if err := rows.Scan(&g.wallet, &g.revision); err != nil {
			return nil, err
		}
		grants = append(grants, g)
	}
	return grants, rows.Err()
}
func (t postgresReconciliationRoom) revokeGrant(ctx context.Context, g reconciliationGrant) error {
	_, err := t.Exec(ctx, `UPDATE voice_participants SET muted=true,grant_state='revoked' WHERE room_id=$1 AND wallet=$2 AND revision=$3 AND grant_state IN ('granting','revoking')`, t.id, g.wallet, g.revision)
	return err
}
