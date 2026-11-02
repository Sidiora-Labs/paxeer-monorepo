package server

import (
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

type EVMReviewRequest struct {
	SessionID   string `json:"session_id"`
	KeyID       string `json:"key_id"`
	Transaction string `json:"transaction"`
}

type EVMReviewResponse struct {
	NodeID        string `json:"node_id"`
	KeyID         string `json:"key_id"`
	To            string `json:"to"`
	Value         string `json:"value"`
	Data          string `json:"data"`
	Method        string `json:"method"`
	Destination   string `json:"destination"`
	SigningHash   string `json:"signing_hash"`
	ChainID       uint64 `json:"chain_id"`
	Decision      string `json:"decision"`
	AuditSequence uint64 `json:"audit_sequence"`
}

func (s *Server) HandleEVMReview(w http.ResponseWriter, r *http.Request) {
	body, e := readBody(r)
	var response EVMReviewResponse
	if e == nil {
		response, e = s.doEVMReview(r, body)
	}
	if e != nil {
		s.writeError(w, e)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) doEVMReview(r *http.Request, body []byte) (EVMReviewResponse, *Error) {
	var req EVMReviewRequest
	if e := decodeRequest(body, &req); e != nil {
		return EVMReviewResponse{}, e
	}
	if e := requireIDs(req.SessionID, req.KeyID); e != nil {
		return EVMReviewResponse{}, e
	}
	if r.Header.Get("Authorization") == "" || r.Header.Get(HeaderAgentKey) != "" {
		return EVMReviewResponse{}, newError(CodeTokenMissing, "original owner bearer authorization is required for review")
	}
	unlock := s.lockKey(req.KeyID)
	defer unlock()
	rec, err := s.opts.Store.Get(req.KeyID)
	if errors.Is(err, store.ErrNotFound) {
		return EVMReviewResponse{}, newError(CodeKeyNotFound, "key %q is not held by this node", req.KeyID)
	}
	if err != nil {
		return EVMReviewResponse{}, newError(CodeStoreFailed, "%v", err)
	}
	owner, account, e := s.publicIdentityOwner(req.KeyID)
	if e != nil {
		return EVMReviewResponse{}, e
	}
	if strings.HasPrefix(owner, "agent:") {
		return EVMReviewResponse{}, newError(CodeTokenNotOwner, "review requires the original standard wallet identity")
	}
	subject, e := s.authenticateAt(r, PathEVMReview, req.KeyID, body, owner)
	if e != nil {
		return EVMReviewResponse{}, e
	}
	if subject != owner {
		return EVMReviewResponse{}, newError(CodeTokenNotOwner, "review requires the original key owner")
	}
	if rec.Curve != store.CurveSecp256k1 {
		return EVMReviewResponse{}, newError(CodeKeyCurve, "EVM review requires a secp256k1 key")
	}
	if err := s.opts.Inventory.Check(req.KeyID, "sign", owner, rec.Curve, &rec); err != nil {
		return EVMReviewResponse{}, newError(CodeKeyInvalidShare, "owner-approved wallet inventory differs from held share")
	}
	raw, e := decodeHex("transaction", req.Transaction)
	if e != nil {
		return EVMReviewResponse{}, e
	}
	tx, err := evm.DecodeTransaction(raw, new(big.Int).SetUint64(s.opts.ChainID))
	if err != nil {
		code := policy.CodeDecodeError
		if errors.Is(err, evm.ErrChainID) {
			code = policy.CodeChainMismatch
		}
		return EVMReviewResponse{}, policyError(code, err.Error())
	}
	if tx.To == nil {
		return EVMReviewResponse{}, policyError(policy.CodeContractCreation, "contract creation is not signed by the attestors")
	}
	call, err := evm.DecodeCalldata(*tx.To, tx.Data)
	if err != nil {
		return EVMReviewResponse{}, policyError(policy.CodeDecodeError, err.Error())
	}
	unlockAccount := s.lockKey("ledger\x00" + policy.AccountKey(account))
	decision := s.opts.Policy.Evaluate(account, policy.Request{Kind: policy.KindEVMTransaction, View: tx}, policy.ReadOnlyLedger(s.spends))
	unlockAccount()
	if !decision.Allowed {
		return EVMReviewResponse{}, policyError(decision.Code, decision.Reason)
	}
	seq, e := s.audit("evm_review", req.KeyID, subject, decision.Code, "decoded review is not signing approval", req.SessionID)
	if e != nil {
		return EVMReviewResponse{}, e
	}
	return EVMReviewResponse{
		NodeID: s.opts.NodeID, KeyID: req.KeyID, To: tx.To.Hex(), Value: tx.Value.String(),
		Data: "0x" + hex.EncodeToString(tx.Data), Method: call.Method, Destination: call.Destination,
		SigningHash: tx.SigningDigest.Hex(), ChainID: tx.ChainID.Uint64(), Decision: decision.Code, AuditSequence: seq,
	}, nil
}
