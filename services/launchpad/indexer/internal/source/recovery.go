package source

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// RecoverEndpoints probes one transiently quarantined endpoint per invocation.
// The application schedules these calls; rotating candidates prevents a dead
// endpoint from starving the others. Rejoin retains all 128 identity checks.
func (s *CommittedSource) RecoverEndpoints(ctx context.Context) error {
	ctx, done, err := s.lifecycle.begin(ctx)
	if err != nil {
		return err
	}
	defer done()
	ctx, cancel := context.WithTimeout(ctx, s.opts.BulkDeadline)
	defer cancel()
	s.mu.Lock()
	var candidate *committedEndpoint
	for i := 0; i < len(s.endpoints); i++ {
		n := (s.recoveryNext + i) % len(s.endpoints)
		e := s.endpoints[n]
		if e.quarantined && !e.recovering && (e.health.QuarantineReason == string(InvalidError) || e.health.QuarantineReason == string(TimeoutError)) {
			candidate = e
			s.recoveryNext = (n + 1) % len(s.endpoints)
			break
		}
	}
	s.mu.Unlock()
	if candidate == nil {
		return nil
	}
	probe, cancelProbe := context.WithTimeout(ctx, s.opts.LiveDeadline)
	var chain string
	err = s.rpc(probe, candidate, false, "eth_chainId", []any{}, &chain)
	cancelProbe()
	if err != nil {
		return err
	}
	if chain != "0x7d" {
		return sourceError(IdentityError, candidate.id, nil)
	}
	return s.RejoinEndpoint(ctx, candidate.id)
}

// SignedSourceSelection carries two independent operator approvals. Keys are
// public trust anchors supplied by the application's bound lifecycle document.
type SignedSourceSelection struct {
	Authorization                       SourceSelectionAuthorization
	RequestSignature, ApprovalSignature []byte
}

type SelectionAuthority struct {
	scope string
	keys  map[string]ed25519.PublicKey
}

type selectionContextKey struct{}

func NewSelectionAuthority(scope string, keys map[string][]byte) (*SelectionAuthority, error) {
	if scope == "" || len(scope) > 128 || len(keys) < 2 || len(keys) > 64 {
		return nil, sourceError(AuthorizationError, "selection-config", nil)
	}
	authority := &SelectionAuthority{scope: scope, keys: make(map[string]ed25519.PublicKey, len(keys))}
	for id, key := range keys {
		if !validSourceID(id) || len(key) != ed25519.PublicKeySize {
			return nil, sourceError(AuthorizationError, "selection-config", nil)
		}
		for _, known := range authority.keys {
			if ed25519.PublicKey(key).Equal(known) {
				return nil, sourceError(AuthorizationError, "selection-config", nil)
			}
		}
		authority.keys[id] = append(ed25519.PublicKey(nil), key...)
	}
	return authority, nil
}

// SourceSelectionPayloadHash binds approval to this chain, stream and exact
// canonical identity; signatures additionally bind all approval metadata.
func SourceSelectionPayloadHash(scope, sourceID string, height uint64, hash common.Hash) string {
	payload, _ := json.Marshal(struct {
		ChainID         uint64
		Scope, SourceID string
		Height          uint64
		Hash            common.Hash
	}{125, scope, sourceID, height, hash})
	digest := sha256.Sum256(payload)
	return hex.EncodeToString(digest[:])
}

func (a *SelectionAuthority) AuthorizeSourceSelection(ctx context.Context, id, sourceID string, height uint64, hash common.Hash) (SourceSelectionAuthorization, error) {
	if err := ctx.Err(); err != nil {
		return SourceSelectionAuthorization{}, err
	}
	signed, ok := ctx.Value(selectionContextKey{}).(SignedSourceSelection)
	auth := signed.Authorization
	requestKey, requestOK := a.keys[auth.RequestedBy]
	approvalKey, approvalOK := a.keys[auth.ApprovedBy]
	payload, err := json.Marshal(auth)
	if !ok || err != nil || !requestOK || !approvalOK || auth.RequestedBy == auth.ApprovedBy || auth.ID != id || auth.SourceID != sourceID || auth.Height != height || auth.Hash != hash || auth.Scope != a.scope || auth.PayloadHash != SourceSelectionPayloadHash(a.scope, sourceID, height, hash) || auth.Kind != "incident_recovery" || auth.State != "approved" || auth.Reason == "" || auth.Revision == 0 || !auth.Expires.After(time.Now()) || auth.Expires.After(time.Now().Add(24*time.Hour)) || !ed25519.Verify(requestKey, payload, signed.RequestSignature) || !ed25519.Verify(approvalKey, payload, signed.ApprovalSignature) {
		return SourceSelectionAuthorization{}, sourceError(AuthorizationError, sourceID, nil)
	}
	return auth, nil
}

// Handler is the signed private incident operation, mounted by the app. Merely
// restoring a transport cannot clear an incident fence or select a source.
func (a *SelectionAuthority) Handler(s *CommittedSource) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.opts.BulkDeadline)
		defer cancel()
		var signed SignedSourceSelection
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&signed); err != nil {
			http.Error(w, "invalid selection", http.StatusBadRequest)
			return
		}
		if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
			http.Error(w, "invalid selection", http.StatusBadRequest)
			return
		}
		ctx = context.WithValue(ctx, selectionContextKey{}, signed)
		auth := signed.Authorization
		if _, err := a.AuthorizeSourceSelection(ctx, auth.ID, auth.SourceID, auth.Height, auth.Hash); err != nil {
			http.Error(w, "selection denied", http.StatusForbidden)
			return
		}
		// The provider must use this exact verifier when it applies selection.
		if s.opts.SelectionAuthority != a {
			http.Error(w, "selection unavailable", http.StatusServiceUnavailable)
			return
		}
		for _, health := range s.EndpointHealth() {
			if health.SourceID == auth.SourceID && health.QuarantineReason != "" {
				if err := s.RejoinEndpoint(ctx, auth.SourceID); err != nil {
					http.Error(w, "identity verification failed", http.StatusConflict)
					return
				}
				break
			}
		}
		if err := s.ApplySourceSelection(ctx, auth.ID, auth.SourceID, auth.Height, auth.Hash); err != nil {
			http.Error(w, "selection refused", http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
