package server

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

const signIntentRecordKind store.RecordKind = "sign-intent-v1"

var errSignIntentConflict = errors.New("signing session already binds a different intent or key epoch")

type signIntentBinding struct {
	KeyID     string `json:"key_id"`
	SessionID string `json:"session_id"`
	Epoch     uint64 `json:"epoch"`
	Kind      string `json:"kind"`
	Digest    string `json:"digest"`
}

type signIntentState struct {
	Binding  signIntentBinding `json:"binding"`
	Response *SignResponse     `json:"response,omitempty"`
}

func signingIntent(req SignRequest, epoch uint64, signed []byte) signIntentBinding {
	// The prepared bytes are the canonical cryptographic intent, independent of
	// credentials, JSON encoding and the selected threshold signer set.
	digest := sha256.Sum256(signed)
	return signIntentBinding{KeyID: req.KeyID, SessionID: req.SessionID, Epoch: epoch, Kind: req.Kind, Digest: hex.EncodeToString(digest[:])}
}

func (s *Server) loadSignIntent(keyID, sessionID string) (signIntentState, error) {
	var state signIntentState
	err := s.opts.Store.WithRecord(signIntentRecordKind, requestID(keyID, sessionID), func(raw []byte) error {
		return json.Unmarshal(raw, &state)
	})
	return state, err
}

func (s *Server) bindSignIntent(binding signIntentBinding) (signIntentState, *Error) {
	var state signIntentState
	err := s.opts.Store.UpdateRecord(signIntentRecordKind, requestID(binding.KeyID, binding.SessionID), func(prev []byte) ([]byte, error) {
		if prev != nil {
			if err := json.Unmarshal(prev, &state); err != nil {
				return nil, err
			}
			if state.Binding != binding {
				return nil, errSignIntentConflict
			}
			return prev, nil
		}
		state.Binding = binding
		return json.Marshal(state)
	})
	if errors.Is(err, errSignIntentConflict) {
		return state, newError(CodeSessionBadRequest, "%v", err)
	}
	if err != nil {
		return state, newError(CodeStoreFailed, "cannot persist signing intent")
	}
	return state, nil
}

func (s *Server) validateSignResult(rec store.ShareRecord, binding signIntentBinding, signed []byte, resp SignResponse) *Error {
	if resp.NodeID != s.opts.NodeID || resp.KeyID != binding.KeyID || resp.Kind != binding.Kind || resp.SignedBytes != hex.EncodeToString(signed) || resp.AuditSequence == 0 {
		return newError(CodeStoreFailed, "invalid durable signing result identity")
	}
	sig, err := hex.DecodeString(resp.Signature)
	valid := false
	switch rec.Curve {
	case store.CurveEd25519:
		valid = len(rec.PublicKey) == ed25519.PublicKeySize && ed25519.Verify(rec.PublicKey, signed, sig)
	case store.CurveSecp256k1:
		if pub, recoverErr := gethcrypto.SigToPub(signed, sig); recoverErr == nil {
			valid = bytes.Equal(gethcrypto.FromECDSAPub(pub), rec.PublicKey) && resp.RecoveryID != nil && *resp.RecoveryID == sig[64]
		}
	}
	if err != nil || !valid {
		return newError(CodeStoreFailed, "invalid durable signing result signature")
	}
	return s.requireAuditSequence(resp.AuditSequence)
}

func (s *Server) saveSignResult(binding signIntentBinding, resp SignResponse) *Error {
	err := s.opts.Store.UpdateRecord(signIntentRecordKind, requestID(binding.KeyID, binding.SessionID), func(prev []byte) ([]byte, error) {
		var state signIntentState
		if err := json.Unmarshal(prev, &state); err != nil {
			return nil, err
		}
		if state.Binding != binding || state.Response != nil {
			return nil, errSignIntentConflict
		}
		state.Response = &resp
		return json.Marshal(state)
	})
	if err != nil {
		return newError(CodeStoreFailed, "cannot persist completed signing intent")
	}
	return nil
}
