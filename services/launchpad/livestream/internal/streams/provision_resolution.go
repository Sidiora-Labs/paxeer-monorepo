package streams

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/livepeer"
	"github.com/jackc/pgx/v5"
)

type ProvisionResolutionAction string

const (
	ProvisionResume  ProvisionResolutionAction = "resume"
	ProvisionRelease ProvisionResolutionAction = "release"
	ProvisionAdopt   ProvisionResolutionAction = "adopt"
)

// ProvisionResolutionDecision comes from a trusted operator authorization path,
// never request JSON. NonallocationConfirmed attests that the provider settled
// this exact attempt without allocating, including any request still in flight.
// An empty discovery result alone cannot establish that fact.
type ProvisionResolutionDecision struct {
	OperationID                              string
	Revision, Attempt                        int64
	Actor, ConfirmationReference, ProviderID string
	Action                                   ProvisionResolutionAction
	Allowed, NonallocationConfirmed          bool
	ConfirmedAt, ExpiresAt                   time.Time
}

type ProvisionRecoveryState struct {
	Operation               Operation
	Attempt                 int64
	CreateSent, LeaseActive bool
	LastReason              string
	Resolution              *ProvisionResolutionReceipt
}

func (s ProvisionRecoveryState) Actions() []ProvisionResolutionAction {
	if !s.CreateSent || s.LeaseActive || s.Operation.ProviderID != "" {
		return nil
	}
	switch s.Operation.State {
	case "PROVISIONING":
		return []ProvisionResolutionAction{ProvisionResume, ProvisionRelease, ProvisionAdopt}
	case "ENDING", "SUSPENDED":
		return []ProvisionResolutionAction{ProvisionRelease, ProvisionAdopt}
	default:
		return nil
	}
}

type ProvisionResolutionReceipt struct {
	Action                ProvisionResolutionAction `json:"action"`
	Attempt               int64                     `json:"attempt"`
	ConfirmationReference string                    `json:"confirmationReference"`
	ConfirmedAt           time.Time                 `json:"confirmedAt"`
	ProviderID            string                    `json:"providerId,omitempty"`
}

type provisionRecoveryRepository interface {
	ProvisionRecovery(context.Context, string) (ProvisionRecoveryState, error)
	applyProvisionResolution(context.Context, ProvisionResolutionDecision, ProvisionResolutionReceipt) (Operation, error)
}

// ProvisionAmbiguityResolver never issues create. Adoption schedules the normal
// worker's discovery, policy and encrypted-credential binding path. Only an
// authorized provider-confirmed nonallocation can re-arm one create attempt.
type ProvisionAmbiguityResolver struct {
	Store    provisionRecoveryRepository
	Provider *livepeer.Client
}

func validateProvisionResolution(s ProvisionRecoveryState, d ProvisionResolutionDecision, now time.Time) error {
	if !d.Allowed || !identityPattern.MatchString(d.Actor) || !identityPattern.MatchString(d.ConfirmationReference) || !d.ExpiresAt.After(now) || d.ConfirmedAt.IsZero() || d.ConfirmedAt.After(now) || d.ConfirmedAt.Before(now.Add(-5*time.Minute)) {
		return ErrDenied
	}
	if d.OperationID != s.Operation.ID || d.Revision != s.Operation.Revision || d.Attempt != s.Attempt || d.Attempt < 1 || s.Operation.ProviderID != "" || !s.CreateSent || s.LeaseActive {
		return ErrConflict
	}
	admitted := false
	for _, action := range s.Actions() {
		if action == d.Action {
			admitted = true
		}
	}
	if !admitted {
		return ErrConflict
	}
	if d.Action == ProvisionAdopt {
		if !identityPattern.MatchString(d.ProviderID) || d.NonallocationConfirmed {
			return ErrInvalid
		}
	} else if !d.NonallocationConfirmed || d.ProviderID != "" {
		return ErrDenied
	}
	return nil
}

func resolvedProvisionState(state ProvisionRecoveryState, d ProvisionResolutionDecision, now time.Time) (ProvisionRecoveryState, error) {
	if err := validateProvisionResolution(state, d, now); err != nil {
		return ProvisionRecoveryState{}, err
	}
	next, reason, sent := state.Operation.State, "provider_adoption_verified", true
	switch d.Action {
	case ProvisionResume:
		reason, sent = "provider_nonallocation_resume", false
	case ProvisionRelease:
		reason, next = "provider_nonallocation_released", "FAILED"
		if state.Operation.State != "PROVISIONING" {
			next = "ENDED"
		}
	}
	state.Operation.State = next
	state.Operation.Revision++
	state.Operation.RetryAt = nil
	state.LastReason, state.CreateSent, state.LeaseActive = reason, sent, false
	state.Resolution = &ProvisionResolutionReceipt{Action: d.Action, Attempt: d.Attempt, ConfirmationReference: d.ConfirmationReference, ConfirmedAt: d.ConfirmedAt.UTC(), ProviderID: d.ProviderID}
	return state, nil
}

func (r *ProvisionAmbiguityResolver) Resolve(ctx context.Context, d ProvisionResolutionDecision) (Operation, error) {
	if r == nil || r.Store == nil || r.Provider == nil {
		return Operation{}, ErrUnavailable
	}
	if !frozenUUID.MatchString(d.OperationID) {
		return Operation{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	state, err := r.Store.ProvisionRecovery(ctx, d.OperationID)
	if err != nil {
		return Operation{}, err
	}
	if err = validateProvisionResolution(state, d, time.Now()); err != nil {
		return Operation{}, err
	}
	remote, err := r.Provider.FindCorrelated(ctx, d.OperationID)
	if d.Action == ProvisionAdopt {
		if err != nil {
			return Operation{}, ErrUnavailable
		}
		if remote.ID != d.ProviderID {
			return Operation{}, ErrConflict
		}
	} else {
		if err == nil {
			return Operation{}, ErrConflict
		}
		if !errors.Is(err, livepeer.ErrCorrelationMissing) {
			return Operation{}, ErrUnavailable
		}
	}
	receipt := ProvisionResolutionReceipt{Action: d.Action, Attempt: d.Attempt, ConfirmationReference: d.ConfirmationReference, ConfirmedAt: d.ConfirmedAt.UTC(), ProviderID: d.ProviderID}
	return r.Store.applyProvisionResolution(ctx, d, receipt)
}

func (s *LifecycleStore) ProvisionRecovery(ctx context.Context, id string) (ProvisionRecoveryState, error) {
	if !frozenUUID.MatchString(id) {
		return ProvisionRecoveryState{}, ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ProvisionRecoveryState{}, err
	}
	defer rollbackBounded(tx)
	state, err := s.provisionRecoveryTx(ctx, tx, id)
	if err != nil {
		return ProvisionRecoveryState{}, err
	}
	return state, tx.Commit(ctx)
}

func (s *LifecycleStore) provisionRecoveryTx(ctx context.Context, tx pgx.Tx, id string) (ProvisionRecoveryState, error) {
	var state ProvisionRecoveryState
	var err error
	state.Operation, err = readOperation(tx.QueryRow(ctx, fmt.Sprintf("SELECT "+opColumns+" FROM %s.stream_operations WHERE id=$1 FOR UPDATE", s.schema), id))
	if err != nil {
		return state, err
	}
	// PostgreSQL evaluates lease validity, including its infinite timestamps.
	err = tx.QueryRow(ctx, fmt.Sprintf("SELECT attempt,create_sent,lease_until>clock_timestamp(),last_reason FROM %s.stream_worker_attempts WHERE operation_id=$1 FOR UPDATE", s.schema), id).Scan(&state.Attempt, &state.CreateSent, &state.LeaseActive, &state.LastReason)
	if err != nil {
		return state, err
	}
	state.Resolution, err = s.latestProvisionResolutionQuery(ctx, tx, id)
	return state, err
}

func (s *LifecycleStore) latestProvisionResolution(ctx context.Context, id string) (*ProvisionResolutionReceipt, error) {
	return s.latestProvisionResolutionQuery(ctx, s.pool, id)
}

func (s *LifecycleStore) latestProvisionResolutionQuery(ctx context.Context, query interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, id string) (*ProvisionResolutionReceipt, error) {
	var raw string
	err := query.QueryRow(ctx, fmt.Sprintf("SELECT reason FROM %s.stream_audit WHERE operation_id=$1 AND reason LIKE 'provider_ambiguity_resolution:%%' ORDER BY revision DESC,id DESC LIMIT 1", s.schema), id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	const prefix = "provider_ambiguity_resolution:"
	var receipt ProvisionResolutionReceipt
	if len(raw) <= len(prefix) || json.Unmarshal([]byte(raw[len(prefix):]), &receipt) != nil {
		return nil, ErrUnavailable
	}
	if receipt.Attempt < 1 || !identityPattern.MatchString(receipt.ConfirmationReference) || receipt.ConfirmedAt.IsZero() {
		return nil, ErrUnavailable
	}
	switch receipt.Action {
	case ProvisionAdopt:
		if !identityPattern.MatchString(receipt.ProviderID) {
			return nil, ErrUnavailable
		}
	case ProvisionResume, ProvisionRelease:
		if receipt.ProviderID != "" {
			return nil, ErrUnavailable
		}
	default:
		return nil, ErrUnavailable
	}
	return &receipt, nil
}

func (s *LifecycleStore) applyProvisionResolution(ctx context.Context, d ProvisionResolutionDecision, receipt ProvisionResolutionReceipt) (Operation, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Operation{}, err
	}
	defer rollbackBounded(tx)
	state, err := s.provisionRecoveryTx(ctx, tx, d.OperationID)
	if err != nil {
		return Operation{}, err
	}
	resolved, err := resolvedProvisionState(state, d, time.Now())
	if err != nil {
		return Operation{}, err
	}
	next, reason, sent := resolved.Operation.State, resolved.LastReason, resolved.CreateSent
	o, err := readOperation(tx.QueryRow(ctx, fmt.Sprintf("UPDATE %s.stream_operations SET state=$2,last_reason=$3,revision=revision+1,retry_at=NULL,terminal_at=CASE WHEN $2 IN('FAILED','ENDED') THEN clock_timestamp() ELSE terminal_at END WHERE id=$1 RETURNING "+opColumns, s.schema), d.OperationID, next, reason))
	if err != nil {
		return Operation{}, err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf("UPDATE %s.stream_worker_attempts SET create_sent=$2,owner='',lease_until='-infinity',retry_at='-infinity',last_reason=$3 WHERE operation_id=$1", s.schema), o.ID, sent, reason)
	if err != nil {
		return Operation{}, err
	}
	raw, err := json.Marshal(receipt)
	if err != nil {
		return Operation{}, err
	}
	_, err = tx.Exec(ctx, fmt.Sprintf("INSERT INTO %s.stream_audit(operation_id,actor,reason,revision)VALUES($1,$2,$3,$4)", s.schema), o.ID, d.Actor, "provider_ambiguity_resolution:"+string(raw), o.Revision)
	if err != nil {
		return Operation{}, err
	}
	if next == "PROVISIONING" {
		if err = s.enqueue(ctx, tx, o, "provision"); err != nil {
			return Operation{}, err
		}
	}
	return o, tx.Commit(ctx)
}
