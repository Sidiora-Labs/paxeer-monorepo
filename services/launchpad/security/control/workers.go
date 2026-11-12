package control

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

type SettlementWorker struct {
	store    *Store
	provider *PayoutProvider
}
type payoutAttempt struct {
	Claim string
	Until time.Time
	// Submitted records durable dispatch intent, including attempts written before
	// acceptance was tracked separately. It never proves the provider accepted it.
	Submitted        bool
	Accepted         bool
	State, Reference string
}

func NewSettlementWorker(store *Store, provider *PayoutProvider) (*SettlementWorker, error) {
	if store == nil || (store.pool == nil && store.local == nil) || store.identity == nil || store.verifier == nil || provider == nil || provider.endpoint == nil || provider.client == nil || !text(provider.Controller, 256) || !destination(provider.AuthorityReference) {
		return nil, Unavailable
	}
	return &SettlementWorker{store, provider}, nil
}
func (w *SettlementWorker) Process(ctx context.Context, credential, scope, k, reason string) (Obligation, error) {
	var obligation Obligation
	claim := id()
	var attempt payoutAttempt
	err := w.store.run(ctx, credential, controlSettlement, scope, reason, func(t *transaction) error {
		if err := t.get(controlSettlement, k, &obligation); err != nil {
			return err
		}
		if obligation.Scope != scope {
			return Denied
		}
		if obligation.IdempotencyKey != k {
			return Conflict
		}
		if obligation.State == controlSettled {
			return nil
		}
		if obligation.AuthorityReference != w.provider.AuthorityReference {
			return Denied
		}
		if err := t.get("payout_attempt", k, &attempt); err != nil && err != Missing {
			return err
		}
		if attempt.Claim != "" && attempt.Until.After(t.now) {
			return Unavailable
		}
		attempt.Claim = claim
		attempt.Until = t.now.Add(15 * time.Second)
		return t.put("payout_attempt", k, attempt)
	})
	if err != nil || obligation.State == controlSettled {
		return obligation, err
	}
	result, providerErr := w.provider.Lookup(ctx, k)
	if providerErr == nil && result.State == controlAbsent {
		err = w.store.run(ctx, credential, controlSettlement, scope, reason, func(t *transaction) error {
			var current payoutAttempt
			if err := t.get("payout_attempt", k, &current); err != nil {
				return err
			}
			if current.Claim != claim || !current.Until.After(t.now) {
				return Conflict
			}
			current.Submitted = true
			current.State = controlUnknown
			return t.put("payout_attempt", k, current)
		})
		if err != nil {
			return obligation, err
		}
		result, providerErr = w.provider.Submit(ctx, obligation)
	}
	if providerErr != nil || result.State == controlAbsent {
		result = PayoutResult{State: controlUnknown}
	}
	err = w.store.run(ctx, credential, controlSettlement, scope, reason, func(t *transaction) error {
		var current payoutAttempt
		if err := t.get("payout_attempt", k, &current); err != nil {
			return err
		}
		if current.Claim != claim {
			return Conflict
		}
		current.State = result.State
		current.Reference = result.Reference
		if providerErr == nil && (result.State == controlPending || result.State == controlConfirmed) {
			current.Accepted = true
		}
		current.Claim = ""
		current.Until = time.Time{}
		var o Obligation
		if err := t.get(controlSettlement, k, &o); err != nil {
			return err
		}
		if o.Scope != scope {
			return Denied
		}
		if o.State != controlSettled {
			if o.Revision == ^uint64(0) {
				return Conflict
			}
			o.State = controlPending
			if result.State == controlUnknown {
				o.State = controlUnknown
			}
			o.Revision++
			if err := t.put(controlSettlement, k, o); err != nil {
				return err
			}
		}
		return t.put("payout_attempt", k, current)
	})
	if err != nil {
		return obligation, err
	}
	if result.Reference != "" {
		if err = w.store.ObserveSettlement(ctx, credential, scope, k, reason, result.Reference); err != nil {
			return obligation, err
		}
	}
	err = w.store.Read(ctx, credential, scope, reason, controlSettlement, k, &obligation)
	if err == nil && (providerErr != nil || result.State == controlUnknown) {
		err = Unavailable
	}
	return obligation, err
}

type ReconciliationWorker struct {
	Store      *Store
	Credential string
	Assets     []Asset
	Interval   time.Duration
	Observe    func(string, error)
}

func (w ReconciliationWorker) Run(ctx context.Context) error {
	if w.Store == nil || w.Credential == "" || len(w.Assets) == 0 || len(w.Assets) > 2 || w.Interval <= 0 || w.Interval > 60*time.Second || w.Observe == nil {
		return Invalid
	}
	for _, a := range w.Assets {
		if !a.Valid() {
			return Invalid
		}
	}
	ticker := time.NewTicker(w.Interval)
	defer ticker.Stop()
	for {
		for _, asset := range w.Assets {
			err := w.Store.Reconcile(ctx, w.Credential, controlPlatform, "scheduled confirmed inventory reconciliation", asset)
			w.Observe(asset.Key(), err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

type IncidentWorker struct {
	Store      *Store
	Identity   *PostgresIdentity
	Revocation *RevocationProvider
	Credential string
	Observe    func(string, error)
}

func (w IncidentWorker) Drain(ctx context.Context) error {
	if w.Store == nil || w.Credential == "" || w.Observe == nil {
		return Invalid
	}
	items, err := w.Store.claimWork(ctx, w.Credential, controlIncident, controlIncident)
	if err != nil {
		return err
	}
	var pending []Incident
	for _, item := range items {
		var i Incident
		if err = json.Unmarshal(item.Body, &i); err != nil {
			return err
		}
		pending = append(pending, i)
	}
	for _, incident := range pending {
		err = w.Store.run(ctx, w.Credential, controlIncident, incident.Scope, "revoke compromised identity credentials", func(t *transaction) error {
			var outbox map[string]string
			if err := t.get("incident_outbox", incident.ID, &outbox); err != nil {
				return err
			}
			if outbox[controlState] == "credentials_revoked" || outbox[controlState] == controlCompleted {
				return nil
			}
			if outbox["identity"] != incident.ServiceIdentity {
				return Conflict
			}
			if err := t.revokeIdentity(incident.ServiceIdentity); err != nil {
				return err
			}
			outbox[controlState] = "credentials_revoked"
			outbox["rotation"] = "credential-revocation:" + incident.ID
			return t.put("incident_outbox", incident.ID, outbox)
		})
		w.Observe(incident.ID, err)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		if w.Revocation == nil {
			w.Observe(incident.ID, Unavailable)
			continue
		}
		ack, revocationErr := w.Revocation.Revoke(ctx, incident)
		if revocationErr != nil {
			w.Observe(incident.ID, revocationErr)
			continue
		}
		err = w.Store.run(ctx, w.Credential, controlIncident, incident.Scope, "record verified secret revocation and cache invalidation", func(t *transaction) error {
			var outbox map[string]string
			if err := t.get("incident_outbox", incident.ID, &outbox); err != nil {
				return err
			}
			if outbox["identity"] != ack.Identity {
				return Conflict
			}
			outbox[controlState] = controlCompleted
			outbox["rotation"] = ack.Rotation
			outbox["integrity"] = ack.Integrity
			return t.put("incident_outbox", incident.ID, outbox)
		})
		w.Observe(incident.ID, err)
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return nil
}
func (w IncidentWorker) Run(ctx context.Context) error {
	timer := time.NewTicker(30 * time.Second)
	defer timer.Stop()
	for {
		if err := w.Drain(ctx); err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (w *SettlementWorker) Drain(ctx context.Context, credential string, observe func(string, error)) error {
	if observe == nil || credential == "" {
		return Invalid
	}
	items, err := w.store.claimWork(ctx, credential, controlSettlement, controlSettlement)
	if err != nil {
		return err
	}
	for _, item := range items {
		_, err = w.Process(ctx, credential, item.Scope, item.Key, "reconcile durable settlement obligation")
		observe(item.Key, err)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}

	return nil
}
func (w *SettlementWorker) Run(ctx context.Context, credential string, observe func(string, error)) error {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		if err := w.Drain(ctx, credential, observe); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
