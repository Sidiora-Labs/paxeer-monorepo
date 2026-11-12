package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

type MembershipMutation struct {
	Squad      Squad
	Membership Membership
}

// ChangeMembership returns the two independent revisions under the same lock
// as the mutation; a subsequent HTTP read cannot race the response identity.
func (s Store) ChangeMembership(ctx context.Context, actor, id, account, action string, expected uint64) (MembershipMutation, error) {
	var result MembershipMutation
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := lock(ctx, tx, "squad:"+id); e != nil {
			return e
		}
		q, e := squad(ctx, tx, id)
		if e != nil {
			return e
		}
		var m Membership
		switch action {
		case "invite":
			if account == "" || account == actor {
				return ErrInvalid
			}
			e = inviteTx(ctx, tx, actor, id, account, uuid.NewString()+uuid.NewString(), expected, time.Now().UTC().Add(24*time.Hour))
		case actionRequest:
			if actor != account {
				return ErrForbidden
			}
			m, e = requestJoinTx(ctx, tx, actor, id, expected)
		case "approve":
			m, e = approveRequestTx(ctx, tx, actor, id, account, expected, time.Now().UTC())
		case "remove", "ban":
			status := stateRemoved
			if action == "ban" {
				status = statusBanned
			}
			e = setMemberStatusTx(ctx, tx, actor, id, account, status, expected)
		case "promote", "demote":
			role := roleMember
			if action == "promote" {
				role = roleModerator
			}
			e = setMemberRoleTx(ctx, tx, actor, id, account, role, expected)
		default:
			return ErrInvalid
		}
		if e != nil {
			return e
		}
		m, e = member(ctx, tx, id, account)
		if e != nil {
			return e
		}
		if m.Role == "" && m.JoinedAt.IsZero() {
			m.Role = roleMember
		}
		result = MembershipMutation{Squad: q, Membership: m}
		return nil
	})
	if err != nil {
		return MembershipMutation{}, err
	}
	return result, nil
}

type InvitationDelivery struct {
	SquadID   string
	AccountID string
	Token     string
	ExpiresAt time.Time
	Revision  uint64
}

// InvitationDelivery is an authenticated private inbox, never a public squad
// field or a general social outbox event. Its token survives process restarts.
func (s Store) InvitationDelivery(ctx context.Context, actor, id string, now time.Time) (InvitationDelivery, error) {
	var d InvitationDelivery
	if actor == "" {
		return d, ErrForbidden
	}
	if s.DB == nil {
		return d, ErrUnavailable
	}
	var raw []byte
	e := s.DB.QueryRowContext(ctx, `SELECT d.document FROM narrative_discovery.invitation_deliveries d JOIN narrative_discovery.invitations i USING(squad_id,account_id) JOIN narrative_discovery.members m USING(squad_id,account_id) WHERE d.squad_id=$1 AND d.account_id=$2 AND i.document->>'Consumed'='false' AND m.document->>'Status'='invited'`, id, actor).Scan(&raw)
	if e != nil {
		return d, e
	}
	if e = json.Unmarshal(raw, &d); e != nil {
		return InvitationDelivery{}, e
	}
	if d.SquadID != id || d.AccountID != actor || !now.Before(d.ExpiresAt) {
		return InvitationDelivery{}, sql.ErrNoRows
	}
	return d, nil
}

// RedeemInvitation deliberately refuses replay. The older Accept transition
// remains idempotent for its existing internal callers.
func (s Store) RedeemInvitation(ctx context.Context, actor, id, token string, expected uint64, now time.Time) (MembershipMutation, error) {
	var result MembershipMutation
	e := s.transaction(ctx, func(tx *sql.Tx) error {
		if e := lock(ctx, tx, "squad:"+id); e != nil {
			return e
		}
		q, e := squad(ctx, tx, id)
		if e != nil {
			return e
		}
		var raw []byte
		var inv Invitation
		if e = tx.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.invitations WHERE squad_id=$1 AND account_id=$2 FOR UPDATE`, id, actor).Scan(&raw); e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &inv); e != nil {
			return e
		}
		if inv.Consumed {
			return ErrForbidden
		}
		m, e := member(ctx, tx, id, actor)
		if e != nil {
			return e
		}
		if m.Status != statusInvited {
			return ErrForbidden
		}
		inv, m, e = Accept(inv, m, token, expected, now)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE narrative_discovery.invitations SET document=$1 WHERE squad_id=$2 AND account_id=$3`, encode(inv), id, actor); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `UPDATE narrative_discovery.members SET document=$1 WHERE squad_id=$2 AND account_id=$3`, encode(m), id, actor); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `DELETE FROM narrative_discovery.invitation_deliveries WHERE squad_id=$1 AND account_id=$2`, id, actor); e != nil {
			return e
		}
		result = MembershipMutation{Squad: q, Membership: m}
		return nil
	})
	if e != nil {
		return MembershipMutation{}, e
	}
	return result, nil
}

type InvitationPage struct {
	Items      []InvitationDelivery
	NextCursor string
}

func (s Store) InvitationInbox(ctx context.Context, actor, after string, now time.Time) (result InvitationPage, err error) {
	page := InvitationPage{Items: []InvitationDelivery{}}
	if actor == "" {
		return page, ErrForbidden
	}
	if s.DB == nil {
		return page, ErrUnavailable
	}
	rows, e := s.DB.QueryContext(ctx, `SELECT d.document FROM narrative_discovery.invitation_deliveries d JOIN narrative_discovery.invitations i USING(squad_id,account_id) JOIN narrative_discovery.members m USING(squad_id,account_id) WHERE d.account_id=$1 AND d.squad_id>$2 AND i.document->>'Consumed'='false' AND m.document->>'Status'='invited' AND (d.document->>'ExpiresAt')::timestamptz>$3 ORDER BY d.squad_id LIMIT 101`, actor, after, now)
	if e != nil {
		return page, e
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var raw []byte
		var d InvitationDelivery
		if e = rows.Scan(&raw); e != nil {
			return InvitationPage{}, e
		}
		if e = json.Unmarshal(raw, &d); e != nil {
			return InvitationPage{}, e
		}
		if d.AccountID != actor || d.SquadID == "" || !now.Before(d.ExpiresAt) {
			return InvitationPage{}, ErrInvalid
		}
		if len(page.Items) == 100 {
			page.NextCursor = page.Items[99].SquadID
			break
		}
		page.Items = append(page.Items, d)
	}
	return page, rows.Err()
}
