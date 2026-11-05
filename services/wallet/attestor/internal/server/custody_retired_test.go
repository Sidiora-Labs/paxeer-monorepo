package server

import (
	"encoding/hex"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestSignRefusesRetiredCustodyKind(t *testing.T) {
	wantKinds := []string{"evm_tx", "eip712", "personal_message", "eth_sign_digest", "lx_bind", "lx_grant", "lx_send_authorization"}
	if got := SignKinds(); !reflect.DeepEqual(got, wantKinds) {
		t.Fatalf("SignKinds() = %v, want %v", got, wantKinds)
	}
	c := newTestCluster(t, 2, false)
	result := c.call(t, c.nodes[0], PathSign, SignRequest{
		SessionID: "retired-custody", KeyID: "absent", Kind: "custody",
		Signers: []string{"node-1", "node-2", "node-3"},
		Message: hex.EncodeToString([]byte("LX:CUSTODY:v2")),
	}, "")
	if result.status != http.StatusBadRequest {
		t.Fatalf("retired custody kind status %d: %s", result.status, result.body)
	}
	expectError(t, "retired custody kind", result, CodeSessionKind)
}

func TestSignRefusesCustodyProofField(t *testing.T) {
	c := newTestCluster(t, 2, false)
	request := map[string]any{
		"session_id": "custody-proof", "key_id": "absent", "kind": KindEVMTransaction,
		"signers":     []string{"node-1", "node-2", "node-3"},
		"transaction": "ec098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a764000080018080",
	}
	control := c.call(t, c.nodes[0], PathSign, request, "")
	expectError(t, "EVM transaction without custody passes request decoding", control, CodeKeyNotFound)
	for _, tc := range []struct {
		name  string
		proof any
	}{
		{"proof", map[string]any{"bytes": hex.EncodeToString([]byte("LX:CUSTODY:v2")), "signature": strings.Repeat("11", 65)}},
		{"empty", map[string]any{}},
		{"null", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request["custody"] = tc.proof
			result := c.call(t, c.nodes[0], PathSign, request, "")
			if result.status != http.StatusBadRequest {
				t.Fatalf("custody proof status %d: %s", result.status, result.body)
			}
			err := expectError(t, "unknown custody field", result, CodeSessionBadRequest)
			if !strings.Contains(err.Message, `unknown field "custody"`) {
				t.Fatalf("custody field was not refused as unknown: %s", err.Message)
			}
		})
	}
}
