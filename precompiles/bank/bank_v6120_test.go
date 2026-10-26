package bank_test

import (
	"testing"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
)

func TestV6120UpgradeMatchesNodePlan(t *testing.T) {
	if bank.V6120Upgrade != app.V6120Upgrade {
		t.Fatalf("bank upgrade plan %q does not match node plan %q", bank.V6120Upgrade, app.V6120Upgrade)
	}
}
