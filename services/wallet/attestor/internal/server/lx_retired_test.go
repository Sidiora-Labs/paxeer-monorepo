package server

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/lxwire"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/lx"
)

// KindLXActivity is retained only as a rejected wire value in regression tests.
const KindLXActivity = "lx_activity"

func assertNoLXSigningSessions(t *testing.T, registries map[string]*prometheus.Registry) {
	t.Helper()
	for id, registry := range registries {
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		for _, family := range families {
			if family.GetName() == "attestor_sessions_started_total" {
				for _, metric := range family.Metric {
					if metric.GetCounter().GetValue() != 0 {
						t.Fatalf("%s opened a protocol session", id)
					}
				}
			}
		}
	}
}

func TestSignRefusesRetiredLXActivityKind(t *testing.T) {
	registries := map[string]*prometheus.Registry{}
	c := newTestClusterWith(t, 5, true, testPolicy(), metricOptions(t, registries))
	const keyID = "retired-lx"
	pub, _ := importNativeSendOwner(t, c, keyID, "retired activity owner", "0x4444444444444444444444444444444444444444")
	now := uint64(time.Now().Unix())
	pending := newPendingSend(t, pub, nil, 1, 1_000, now-60, now+600)
	unsigned := pending.envelope(t)
	disclosure, approval := approvedSend(t, pending, keyID, "retired-sign")
	preimage, err := lxwire.SignaturePreimage(pending.activity)
	if err != nil {
		t.Fatal(err)
	}
	approval.ActivityDigest = lx.ID(preimage)
	signers := []string{"node-1", "node-3", "node-5"}
	for _, kind := range SignKinds() {
		if kind == KindLXActivity {
			t.Fatal("retired activity kind is still advertised")
		}
	}
	for _, result := range c.callAll(t, c.byID(signers...), PathSign, func(*testNode) any {
		return SignRequest{SessionID: "retired-sign", KeyID: keyID, Kind: KindLXActivity, Signers: signers,
			Activity: hex.EncodeToString(unsigned), Disclosure: disclosure, Approval: approval}
	}, c.idp.mint(t, c.idp.key, testOwner)) {
		if result.status != http.StatusBadRequest {
			t.Fatalf("retired kind status %d: %s", result.status, result.body)
		}
		expectError(t, "retired activity signing", result, CodeSessionKind)
	}
	assertNoLXSigningSessions(t, registries)
	for _, node := range c.nodes {
		spent, requests := nodeLedger(t, node, "0x4444444444444444444444444444444444444444")
		if spent.Sign() != 0 || requests != 0 {
			t.Fatalf("%s: retired kind recorded %s spend and %d requests", node.id, spent, requests)
		}
	}
}

func TestLXReviewDefaultsToSendAuthorization(t *testing.T) {
	registries := map[string]*prometheus.Registry{}
	c := newTestClusterWith(t, 5, true, testPolicy(), metricOptions(t, registries))
	const keyID = "review-send"
	const account = "0x5555555555555555555555555555555555555555"
	pub, _ := importNativeSendOwner(t, c, keyID, "review send owner", account)
	node := c.nodes[0]
	for _, profile := range []string{"", lx.NativeClockProfileV1} {
		t.Run("clock="+profile, func(t *testing.T) {
			now := uint64(time.Now().Unix())
			window := uint64(600)
			if profile != "" {
				now = uint64(time.Now().UnixMilli())
				window *= 1000
			}
			pending := newPendingSend(t, pub, nil, 7, 5_000_000, now-window, now+window)
			if profile != "" {
				pending.activity.ProtocolVersion = lxwire.StateCommitmentProtocolVersion
				pending.send.ProtocolVersion = lxwire.StateCommitmentProtocolVersion
			}
			unsigned := pending.envelope(t)
			activityPreimage, err := lxwire.SignaturePreimage(pending.activity)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(activityPreimage[:], pending.digest(t)) {
				t.Fatal("fixture must distinguish activity and send authorization digests")
			}
			wantDisclosure, _ := approvedSend(t, pending, keyID, "review-send")
			for i, kind := range []string{"", KindLXSendAuth} {
				request := LXReviewRequest{SessionID: fmt.Sprintf("review-send-%d-%s", i, profile), KeyID: keyID,
					Kind: kind, ClockProfile: profile, Activity: hex.EncodeToString(unsigned)}
				seq, _ := node.audit.Head()
				response := decodeOK[LXReviewResponse](t, "send authorization review", []apiResult{
					c.call(t, node, PathLXReview, request, c.idp.mint(t, c.idp.key, testOwner)),
				})[0]
				if response.Kind != KindLXSendAuth || response.NodeID != node.id || response.KeyID != keyID ||
					response.PublicKey != hex.EncodeToString(pub) || response.Activity != request.Activity ||
					response.SigningPreimage != hex.EncodeToString(pending.digest(t)) || response.NetworkID != testChainID ||
					response.ProtocolVersion != pending.activity.ProtocolVersion || response.NotAfter != strconv.FormatUint(pending.activity.NotAfter, 10) ||
					response.Epoch != 0 || response.AuditSequence != seq+1 || !reflect.DeepEqual(response.Disclosure, *wantDisclosure) {
					t.Fatalf("send authorization review returned %+v", response)
				}
				if after, _ := node.audit.Head(); after != seq+1 {
					t.Fatalf("review wrote %d audit records, want one", after-seq)
				}
			}
			for _, kind := range []string{KindLXActivity, KindLXBind, KindEVMTransaction, "unknown"} {
				request := LXReviewRequest{SessionID: "review-refused-kind", KeyID: keyID, Kind: kind,
					ClockProfile: profile, Activity: hex.EncodeToString(unsigned)}
				result := c.call(t, node, PathLXReview, request, c.idp.mint(t, c.idp.key, testOwner))
				if result.status != http.StatusBadRequest {
					t.Fatalf("review kind %q status %d: %s", kind, result.status, result.body)
				}
				expectError(t, "unsupported review kind "+kind, result, CodeSessionKind)
			}
			pending.send.Signature[0] = 1
			result := c.call(t, node, PathLXReview, LXReviewRequest{SessionID: "review-signed-send", KeyID: keyID,
				ClockProfile: profile, Activity: hex.EncodeToString(pending.envelope(t))}, c.idp.mint(t, c.idp.key, testOwner))
			if e := expectError(t, "send already carrying a signature", result, CodePolicyDenied); e.PolicyCode != policy.CodeDecodeError {
				t.Fatalf("signed send review policy code %s, want %s", e.PolicyCode, policy.CodeDecodeError)
			}
		})
	}
	assertNoLXSigningSessions(t, registries)
	for _, node := range c.nodes {
		spent, requests := nodeLedger(t, node, account)
		if spent.Sign() != 0 || requests != 0 {
			t.Fatalf("%s: review recorded %s spend and %d requests", node.id, spent, requests)
		}
	}
	if err := node.audit.Verify(); err != nil {
		t.Fatal(err)
	}
}
