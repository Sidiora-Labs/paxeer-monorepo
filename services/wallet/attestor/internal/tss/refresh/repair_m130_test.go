package refresh

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	cggmprefresh "github.com/getamis/alice/crypto/tss/ecdsa/cggmp/refresh"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealer"
)

func repairM130RequireWiped(t *testing.T, share *big.Int, words []big.Word) {
	t.Helper()
	if share.Sign() != 0 {
		t.Fatal("temporary share remains live")
	}
	for _, word := range words {
		if word != 0 {
			t.Fatal("temporary share backing storage was not overwritten")
		}
	}
}

func TestRepairM_130_VerificationCleanup(t *testing.T) {
	bundles, _, _ := splitSecp(t)
	for _, name := range []string{"proof_error", "share_mismatch", "cancellation"} {
		t.Run(name, func(t *testing.T) {
			input := bundles["p1"].Clone()
			defer dealer.Wipe(input.Share)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			var want error
			switch name {
			case "proof_error":
				curve, err := input.Curve.Elliptic()
				if err != nil {
					t.Fatal(err)
				}
				input.Share.Set(curve.Params().N)
			case "share_mismatch":
				input.Share.Add(input.Share, big.NewInt(1))
				want = ErrShareMismatch
			case "cancellation":
				cancel()
				want = context.Canceled
			}
			original := new(big.Int).Set(input.Share)
			defer dealer.Wipe(original)
			h := newHub(participantIDs...)
			defer h.close()
			net := h.node("p1", others(participantIDs, "p1"))
			ssid := sessionID(t)
			out, err := RefreshECDSA(ctx, net, input, ssid)
			if out != nil || err == nil || (want != nil && !errors.Is(err, want)) {
				t.Fatalf("public refresh: result=%v error=%v want=%v", out != nil, err, want)
			}
			if input.Share.Cmp(original) != 0 {
				t.Fatal("public refresh modified the caller's share")
			}
			owned := input.Clone()
			words := owned.Share.Bits()
			out, err = refreshECDSA(ctx, net, owned, ssid)
			if out != nil || err == nil || (want != nil && !errors.Is(err, want)) {
				t.Fatalf("owned refresh: result=%v error=%v want=%v", out != nil, err, want)
			}
			repairM130RequireWiped(t, owned.Share, words)
			if input.Share.Cmp(original) != 0 {
				t.Fatal("temporary cleanup modified the caller's share")
			}
		})
	}
}

func TestRepairM_130_PeerRefusalCleanup(t *testing.T) {
	bundles, _, _ := splitSecp(t)
	owned := make(map[string]dealer.ShareBundle, len(bundles))
	words := make(map[string][]big.Word, len(bundles))
	for id, b := range bundles {
		copy := b.Clone()
		if id == "p3" {
			copy.Share.Add(copy.Share, big.NewInt(1))
		}
		owned[id] = copy
		words[id] = copy.Share.Bits()
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	h := newHub(participantIDs...)
	defer h.close()
	ssid := sessionID(t)
	results, errs := runAll(participantIDs, func(id string) (*ECDSAShare, error) {
		return refreshECDSA(ctx, h.node(id, others(participantIDs, id)), owned[id], ssid)
	})
	for _, id := range participantIDs {
		want := ErrPeerShareRefused
		if id == "p3" {
			want = ErrShareMismatch
		}
		if results[id] != nil || !errors.Is(errs[id], want) {
			t.Fatalf("participant %s: result=%v error=%v want=%v", id, results[id] != nil, errs[id], want)
		}
		repairM130RequireWiped(t, owned[id].Share, words[id])
		if err := bundles[id].Validate(); err != nil {
			t.Fatalf("participant %s caller share: %v", id, err)
		}
	}
}

type repairM130CancelNetwork struct {
	*memNet
	cancel context.CancelFunc
}

func (n *repairM130CancelNetwork) MustSend(id string, msg interface{}) {
	n.memNet.MustSend(id, msg)
	if _, ok := msg.(*cggmprefresh.Message); ok {
		n.cancel()
	}
}

func TestRepairM_130_ProtocolCancellationCleanup(t *testing.T) {
	bundles, _, _ := splitSecp(t)
	owned := bundles["p1"].Clone()
	words := owned.Share.Bits()
	ctx, cancel := context.WithTimeout(context.Background(), ecdsaDeadline)
	defer cancel()
	h := newHub(participantIDs...)
	defer h.close()
	ssid := sessionID(t)
	_, errs := runAll(participantIDs, func(id string) (struct{}, error) {
		net := h.node(id, others(participantIDs, id))
		if id == "p1" {
			out, err := refreshECDSA(ctx, &repairM130CancelNetwork{memNet: net, cancel: cancel}, owned, ssid)
			if out != nil {
				return struct{}{}, errors.New("cancelled ceremony returned a share")
			}
			return struct{}{}, err
		}
		d := newDispatcher(net.NumPeers())
		net.Bind(d)
		return struct{}{}, verifyShares(ctx, net, d.proofs, bundles[id], ssid)
	})
	if !errors.Is(errs["p1"], context.Canceled) {
		t.Fatalf("protocol cancellation: %v", errs["p1"])
	}
	for _, id := range participantIDs[1:] {
		if errs[id] != nil {
			t.Fatalf("proof producer %s: %v", id, errs[id])
		}
	}
	repairM130RequireWiped(t, owned.Share, words)
	if err := bundles["p1"].Validate(); err != nil {
		t.Fatalf("caller share after cancellation: %v", err)
	}
}

func TestRepairM_130_SuccessAndResultValidationCleanup(t *testing.T) {
	bundles, pub, address := splitSecp(t)
	owned := make(map[string]dealer.ShareBundle, len(bundles))
	words := make(map[string][]big.Word, len(bundles))
	for id, b := range bundles {
		if id == "p1" {
			continue
		}
		owned[id] = b.Clone()
		words[id] = owned[id].Share.Bits()
	}
	ctx, cancel := context.WithTimeout(context.Background(), ecdsaDeadline)
	defer cancel()
	h := newHub(participantIDs...)
	defer h.close()
	ssid := sessionID(t)
	results, errs := runAll(participantIDs, func(id string) (*ECDSAShare, error) {
		if id == "p1" {
			return RefreshECDSA(ctx, h.node(id, others(participantIDs, id)), bundles[id], ssid)
		}
		return refreshECDSA(ctx, h.node(id, others(participantIDs, id)), owned[id], ssid)
	})
	requireNoErrors(t, errs)
	for _, id := range participantIDs {
		if id != "p1" {
			repairM130RequireWiped(t, owned[id].Share, words[id])
		}
		if err := bundles[id].Validate(); err != nil {
			t.Fatalf("participant %s caller share: %v", id, err)
		}
		box := h.boxes[id]
		box.mu.Lock()
		d := box.recv.(*dispatcher)
		box.mu.Unlock()
		d.mu.Lock()
		r := d.main.(*cggmprefresh.Refresh)
		d.mu.Unlock()
		res, err := r.GetResult()
		if err != nil {
			t.Fatal(err)
		}
		if res.Share == results[id].Share {
			t.Fatal("returned share aliases the temporary protocol result")
		}
		repairM130RequireWiped(t, res.Share, res.Share.Bits())
		repairM130RequireWiped(t, res.YSecret, res.YSecret.Bits())

		// Corrupt the real producer's result and exercise the validation error exit.
		res.Share.Set(results[id].Share)
		res.Share.Add(res.Share, big.NewInt(1))
		resultWords := res.Share.Bits()
		out, err := ecdsaRefreshResult(bundles[id], res)
		if out != nil || !errors.Is(err, ErrProtocolFailed) {
			t.Fatalf("invalid result: result=%v error=%v", out != nil, err)
		}
		repairM130RequireWiped(t, res.Share, resultWords)
	}
	after := ecdsaBundles(results)
	defer func() {
		for _, b := range after {
			dealer.Wipe(b.Share)
		}
		for _, s := range results {
			dealer.Wipe(s.Share)
		}
	}()
	assertRefreshed(t, bundles, after, pub)
	signECDSA(t, results, []string{"p1", "p3", "p5"}, crypto.Keccak256([]byte("repair 22.33.130 cleanup ownership")), address)
}
