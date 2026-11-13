package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestIdentityDatabaseOperationsRespectCanceledRequest(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, principal := ownerCredentials(t, f)
	canceled, stop := context.WithCancel(f.ctx)
	stop()
	for _, tc := range []struct {
		name string
		run  func(context.Context) error
	}{
		{"login challenge", func(ctx context.Context) error {
			_, err := f.s.IssueLogin(ctx, ChallengeRequest{Wallet: principal.Wallet(), Origin: "https://kindle.test"}, "127.0.0.1")
			return err
		}},
		{"account authentication", func(ctx context.Context) error {
			_, err := f.s.Authenticate(ctx, credentials.AccessToken)
			return err
		}},
		{"wallet link challenge", func(ctx context.Context) error {
			_, _, err := f.s.IssueLink(ctx, principal, WalletLinkChallengeRequest{Wallet: principal.Wallet(), ChainID: "125", Visibility: privateVisibility}, "127.0.0.1")
			return err
		}},
		{"wallet link verification", func(ctx context.Context) error {
			_, err := f.s.VerifyLink(ctx, principal, WalletLinkVerifyRequest{LinkID: uuid.NewString(), ChallengeID: uuid.NewString(), Proof: WalletProof{Wallet: principal.Wallet()}})
			return err
		}},
		{"wallet unlink", func(ctx context.Context) error {
			return f.s.Unlink(ctx, principal, WalletLinkRemoveRequest{LinkID: uuid.NewString(), ReauthProof: "proof"})
		}},
		{"session logout", func(ctx context.Context) error { return f.s.Logout(ctx, principal, false) }},
		{"refresh", func(ctx context.Context) error {
			_, err := f.s.Refresh(ctx, credentials.RefreshSecret)
			return err
		}},
		{"stream ticket", func(ctx context.Context) error {
			_, err := f.s.IssueTicket(ctx, principal, "https://kindle.test", "/v2/stream")
			return err
		}},
		{"ticket consumption", func(ctx context.Context) error {
			_, err := f.s.ConsumeTicket(ctx, "1111111111111111111111111111111111111111111111111111111111111111", "https://kindle.test", "/v2/stream")
			return err
		}},
		{"socket authority", func(ctx context.Context) error { return f.s.CheckSocket(ctx, principal) }},
		{"wallet link list", func(ctx context.Context) error {
			_, err := f.s.WalletLinks(ctx, principal)
			return err
		}},
		{"session revocation", func(ctx context.Context) error { return f.s.LogoutSession(ctx, principal, principal.SessionID()) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(canceled); err == nil {
				t.Fatal("canceled identity operation succeeded")
			}
		})
	}
}
