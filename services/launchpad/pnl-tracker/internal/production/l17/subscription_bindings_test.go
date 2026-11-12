package l17

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestPortfolioSubscriptionBindsWalletAndOrigin(t *testing.T) {
	p := auth.IdentityPrincipal{Wallet: wallet, AccountID: uuid.NewString(), SessionID: uuid.NewString(), AuthEpoch: 1, ProofAt: time.Now().UTC(), AccessExpiresAt: time.Now().Add(time.Hour)}
	if err := AuthorizePortfolioSubscription(p, wallet, "https://sidiora.fun"); err != nil {
		t.Fatal(err)
	}
	if err := AuthorizePortfolioSubscription(p, token, "https://sidiora.fun"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if err := AuthorizePortfolioSubscription(p, wallet, "http://sidiora.fun"); !errors.Is(err, auth.ErrScopeDenied) {
		t.Fatal(err)
	}
	tracker := RevisionTracker{}
	notice := Notice{Wallet: wallet, SnapshotID: uuid.NewString(), Revision: "1", Algorithm: Algorithm}
	for _, mutate := range []func(*auth.IdentityPrincipal, *Notice){func(p *auth.IdentityPrincipal, n *Notice) { p.AccountID = "" }, func(p *auth.IdentityPrincipal, n *Notice) { n.Wallet = token }, func(p *auth.IdentityPrincipal, n *Notice) { n.Revision = "0" }, func(p *auth.IdentityPrincipal, n *Notice) { n.SnapshotID = "invalid" }} {
		principal, candidate := p, notice
		mutate(&principal, &candidate)
		if accepted, err := tracker.Accept(principal, candidate); err == nil || accepted {
			t.Fatal(accepted, err)
		}
	}
	if accepted, err := tracker.Accept(p, notice); err != nil || !accepted {
		t.Fatal(accepted, err)
	}
	conflict := notice
	conflict.SnapshotID = uuid.NewString()
	if accepted, err := tracker.Accept(p, conflict); !errors.Is(err, ErrConflict) || accepted {
		t.Fatal(accepted, err)
	}
}

func TestDisplayConversionPreservesStaleProvenanceAndBounds(t *testing.T) {
	quote := DisplayQuote{Numerator: "1", Denominator: "1", Provider: "oracle", ObservedAt: time.Now().UTC(), Stale: true}
	display, err := ConvertDisplay("100", &quote)
	if err != nil || display.UsdMicro != nil || display.Quote != &quote || display.UsdlRaw != "100" {
		t.Fatal(display, err)
	}
	quote.Stale = false
	for _, tc := range []struct{ raw, numerator string }{{"invalid", "1"}, {"1", "invalid"}, {strings.Repeat("9", 156), "10"}} {
		candidate := quote
		candidate.Numerator = tc.numerator
		if _, err := ConvertDisplay(tc.raw, &candidate); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
}
