package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestAccountAndChatClientsReadVerifiedOwnerContracts(t *testing.T) {
	ctx := context.Background()
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	account, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	if err := account.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	preference, err := account.CurrentDeliveryPreference(ctx, peer.account.AccountID)
	if err != nil || preference.Revision != peer.preference.Revision {
		t.Fatalf("delivery preference: %+v %v", preference, err)
	}
	targets, err := account.Targets(ctx, peer.account.AccountID)
	if err != nil || len(targets) != 1 || targets[0] != peer.target {
		t.Fatalf("targets: %+v %v", targets, err)
	}
	target, err := account.CurrentTarget(ctx, peer.account.AccountID, peer.target.DeviceInstallationID)
	if err != nil || target != peer.target {
		t.Fatalf("current target: %+v %v", target, err)
	}
	endpoint, err := account.ResolveEndpoint(ctx, target)
	if err != nil || endpoint.Token == "" {
		t.Fatalf("endpoint: %+v %v", endpoint, err)
	}
	if err := account.DisableTarget(ctx, target, "invalid_endpoint"); err != nil {
		t.Fatal(err)
	}
	if err := account.DeleteTargets(ctx, peer.account.AccountID); err != nil {
		t.Fatal(err)
	}
	discoveryPreference, err := account.Preferences(ctx, peer.account.AccountID)
	if err != nil || discoveryPreference.Revision != 1 || !discoveryPreference.Linked {
		t.Fatalf("discovery preference: %+v %v", discoveryPreference, err)
	}
	watchlist, err := account.Watchlist(ctx, peer.account.AccountID)
	if err != nil || len(watchlist.Entries) != 1 || !watchlist.Entries[0].AlertEnabled {
		t.Fatalf("watchlist: %+v %v", watchlist, err)
	}
	verified, err := account.VerifiedAccount(ctx, peer.account.AccountID)
	if err != nil || !verified {
		t.Fatalf("verified account: %v %v", verified, err)
	}
	canonical := CanonicalPools{Accounts: account}
	if verified, err := canonical.VerifiedAccount(ctx, peer.account.AccountID); err != nil || !verified {
		t.Fatalf("canonical account verification: %v %v", verified, err)
	}
	if verified, err := (CanonicalPools{}).VerifiedAccount(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) || verified {
		t.Fatalf("missing account authority: %v %v", verified, err)
	}
	attribution, err := account.WalletAttribution(ctx, peer.account.Wallet)
	if err != nil || attribution.AccountID != peer.account.AccountID || !attribution.OptedIn {
		t.Fatalf("wallet attribution: %+v %v", attribution, err)
	}
	opted, profile, err := account.CurrentAttribution(ctx, peer.account.AccountID, peer.account.Wallet)
	if err != nil || !opted || profile != peer.account.Wallet {
		t.Fatalf("current attribution: %v %s %v", opted, profile, err)
	}
	chat, err := NewChatClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.Ready(ctx); err != nil {
		t.Fatal(err)
	}
	id, created, err := chat.EnqueueChat(ctx, "chat-event", peer.account.AccountID, "message", time.Now().UTC())
	if err != nil || id != "chat-1" || !created {
		t.Fatalf("chat response: %s %v %v", id, created, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := chat.Run(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("credential loop: %v", err)
	}
	if _, err := account.CurrentDeliveryPreference(ctx, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty preference account: %v", err)
	}
	if _, err := account.Targets(ctx, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty target account: %v", err)
	}
	if _, err := account.CurrentTarget(ctx, peer.account.AccountID, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty target identity: %v", err)
	}
	if err := account.DisableTarget(ctx, target, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty disable reason: %v", err)
	}
	if err := account.DeleteTargets(ctx, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty deletion account: %v", err)
	}
	if _, err := account.WalletAttribution(ctx, "invalid-wallet"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid wallet attribution: %v", err)
	}
	if _, _, err := account.CurrentAttribution(ctx, "", peer.account.Wallet); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty current attribution: %v", err)
	}
	originalPreference := peer.preference
	peer.preference.Timezone = "Unknown/Timezone"
	if _, err := account.CurrentDeliveryPreference(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown timezone admitted: %v", err)
	}
	peer.preference = originalPreference
	peer.target.Status = "unknown"
	if _, err := account.Targets(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown target status admitted: %v", err)
	}
	peer.target.Status = targetActive
	peer.preference.Revision = 0
	if _, err := account.CurrentDeliveryPreference(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unversioned preference admitted: %v", err)
	}
	peer.preference.Revision = originalPreference.Revision
	peer.preference.CategoryChannels = map[string]map[string]bool{"unknown": {channelPush: true}}
	if _, err := account.CurrentDeliveryPreference(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown preference category admitted: %v", err)
	}
	peer.preference.CategoryChannels = map[string]map[string]bool{categoryWatchlist: {"unknown": true}}
	if _, err := account.CurrentDeliveryPreference(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown preference channel admitted: %v", err)
	}
	peer.preference = originalPreference
	peer.target.Revision = 0
	if _, err := account.Targets(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unversioned target admitted: %v", err)
	}
	peer.target.Revision = target.Revision
	peer.target.Provider = "unknown"
	if _, err := account.Targets(ctx, peer.account.AccountID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown provider admitted: %v", err)
	}
	peer.target.Provider = target.Provider
	if _, err := account.Preferences(ctx, "foreign-account"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign preference read: %v", err)
	}
	if _, err := account.CurrentTarget(ctx, peer.account.AccountID, "foreign-device"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign target read: %v", err)
	}
	if _, err := NewChatClient("http://example.test", peer.client, credentials, "REPAIR958_D11"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unverified chat origin admitted: %v", err)
	}
	if _, err := NewAccountClient("http://example.test", peer.client, credentials, "REPAIR958_D11"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unverified account origin admitted: %v", err)
	}
	if _, err := NewAccountClient(peer.server.URL, nil, credentials, "REPAIR958_D11"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing transport admitted: %v", err)
	}
	if _, err := NewAccountClient(peer.server.URL, peer.client, credentials, "missing-token"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing authority token admitted: %v", err)
	}
	if err := (*AccountClient)(nil).Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil account client ready: %v", err)
	}
	if err := (*ChatClient)(nil).Ready(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil chat client ready: %v", err)
	}
	if err := (*ChatClient)(nil).Run(ctx); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil chat client running: %v", err)
	}
}

func TestRecipientConsumerPersistsChatThroughOwnerTargets(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	consumer := RecipientConsumer{Store: Store{DB: db}, Authority: authority}
	item, created, err := consumer.EnqueueChat(ctx, "message-event", peer.account.AccountID, "message-1", time.Now().UTC())
	if err != nil || !created || item.Category != "chat" {
		t.Fatalf("chat commit: %+v %v %v", item, created, err)
	}
	replayed, created, err := consumer.EnqueueChat(ctx, "message-event", peer.account.AccountID, "message-1", item.CreatedAt)
	if err != nil || created || replayed.ID != item.ID {
		t.Fatalf("chat replay: %+v %v %v", replayed, created, err)
	}
	claims, err := (Store{DB: db}).Claim(ctx, "chat-worker", time.Now(), 1)
	if err != nil || len(claims) != 1 || len(claims[0].Deliveries) != 1 || claims[0].Deliveries[0].TargetID != peer.target.DeviceInstallationID {
		t.Fatalf("chat delivery: %+v %v", claims, err)
	}
}

func TestSocialAuthorityReadsVerifiedPublicResourceAndPublishesCallout(t *testing.T) {
	ctx := context.Background()
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	account, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	social := socialAuthority{rpc: account.rpc}
	graph, err := social.Graph(ctx, peer.account.AccountID)
	if err != nil || graph.Revision != 1 || graph.AccountID != peer.account.AccountID {
		t.Fatalf("social graph: %+v %v", graph, err)
	}
	resource, err := social.ResolvePublic(ctx, "callout", "callout-1")
	if err != nil || resource.Ref.ID != "callout-1" || !resource.Ref.Public {
		t.Fatalf("public callout: %+v %v", resource, err)
	}
	decision, err := social.ReviewCallout(ctx, peer.account.AccountID, "0x3333333333333333333333333333333333333333", "New callout")
	if err != nil || !decision.Visible || decision.ResourceID != resource.Ref.ID {
		t.Fatalf("callout review: %+v %v", decision, err)
	}
	publisher := eventPublisher{social: social}
	if err := publisher.Publish(ctx, "callout-target-ref-v2", resource.Ref.ID, json.RawMessage(`{"id":"callout-1"}`)); err != nil {
		t.Fatal(err)
	}
	if err := publisher.Publish(ctx, "unknown", resource.Ref.ID, json.RawMessage(`{}`)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown event kind accepted: %v", err)
	}
	boundary := sourceAuthority{accounts: account, social: social}
	verified, err := boundary.VerifiedAccount(ctx, peer.account.AccountID)
	if err != nil || !verified {
		t.Fatalf("source account verification: %v %v", verified, err)
	}
	resource, err = boundary.ResolvePublic(ctx, "callout", "callout-1")
	if err != nil || resource.Ref.Kind != "callout" {
		t.Fatalf("source callout: %+v %v", resource, err)
	}
	decision, err = boundary.ReviewCallout(ctx, peer.account.AccountID, "0x3333333333333333333333333333333333333333", "New callout")
	if err != nil || decision.ResourceID != "callout-1" {
		t.Fatalf("source review: %+v %v", decision, err)
	}
	if _, err := boundary.CanonicalPool(ctx, "0x3333333333333333333333333333333333333333"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing canonical owner: %v", err)
	}
	if _, err := boundary.ResolvePublic(ctx, resourcePool, "0x3333333333333333333333333333333333333333"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing pool owner: %v", err)
	}
}
