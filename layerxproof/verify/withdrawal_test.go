package verify_test

import (
	"bytes"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
)

func TestWithdrawalOutcomeMatchesRustVerifier(t *testing.T) {
	for _, v := range load(t, "withdrawal") {
		_, effect, err := verify.WithdrawalReceipt(field(t, v, "receipt"), array(t, v, "public_key"))
		if (err == nil) != v.Valid {
			t.Fatalf("%s: valid=%v, got %v", v.Name, v.Valid, err)
		}
		if !v.Valid {
			continue
		}
		amount := effect.Amount.Bytes()
		recipient := field(t, v, "recipient")
		if uint64(effect.NetworkID) != number(t, v, "network_id") ||
			effect.WithdrawalID != array(t, v, "withdrawal_id") ||
			effect.Account != array(t, v, "account") ||
			effect.Asset != array(t, v, "asset") ||
			effect.Amount.Hi != 0 || effect.Amount.Lo != number(t, v, "amount") || len(amount) != 16 ||
			!bytes.Equal(effect.Recipient[:], recipient) ||
			effect.Anchor != array(t, v, "anchor") ||
			effect.Nullifier != array(t, v, "nullifier") ||
			effect.FeeLimit != number(t, v, "fee_limit") {
			t.Fatalf("%s: decoded withdrawal differs from the Rust facts: %+v", v.Name, effect)
		}
	}
}
