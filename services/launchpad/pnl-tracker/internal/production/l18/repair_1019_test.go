package l18

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nats-io/nats.go"
)

func TestMixedStrategyRewards(t *testing.T) {
	for _, strategy := range []uint8{1, 2, 3} {
		t.Run(fmt.Sprint(strategy), func(t *testing.T) {
			f := repair62Chain(t)
			f.advance(f.db.peers.Aged)
			f.db.mu.Lock()
			f.db.strategies = map[string]uint8{"3": strategy}
			f.db.mu.Unlock()
			page := repair62Rewards(t, f.handler(f.chain.Alice), "", 1)
			if len(page.Data.Items) != 1 || page.Data.Items[0].RewardId != "creator:3" || page.NextCursor == nil {
				t.Fatalf("mixed listing=%+v", page)
			}
			item := page.Data.Items[0]
			if item.ClaimableRaw != "0" || item.ClaimedRaw != "11000000" || item.EarnedRaw != "11000000" {
				t.Fatalf("nonclaim history=%+v", item)
			}
			next := repair62Rewards(t, f.handler(f.chain.Alice), *page.NextCursor, 1)
			if len(next.Data.Items) != 1 || next.Data.Items[0].RewardId != "creator:4" || next.Data.Items[0].ClaimableRaw != "10000000" || next.NextCursor != nil {
				t.Fatalf("eligible CLAIM continuation=%+v", next)
			}
			if _, err := f.creator.PrepareClaim(context.Background(), "3", f.chain.Alice, f.chain.Alice); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("nonclaim strategy prepared creator calldata: %v", err)
			}
		})
	}
	f := repair62Chain(t)
	f.db.strategies = map[string]uint8{"1": 255}
	if _, err := f.journey.Rewards(context.Background(), Actor{Wallet: f.chain.Alice}, "", "", 10); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unknown strategy must remain unavailable: %v", err)
	}
}

func repair1019Reopen(t *testing.T, f *repair62Fixture, approved []string) *CommunityWatcher {
	t.Helper()
	// Tear down every client and reopen against the peer's committed records.
	// No CommunityWatcher or policy cache survives this restart.
	f.ledger.pool.Close()
	f.rpc.Close()
	server := httptest.NewServer(http.HandlerFunc(f.db.rpcResponse))
	t.Cleanup(server.Close)
	client, err := rpc.DialHTTP(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	latest := NewCreator(client, time.Now)
	hashes := map[string]string{}
	for _, target := range []string{NFT, Accumulator, FeesRouter, USDL} {
		hashes[target] = strings.ToLower(crypto.Keccak256Hash([]byte{0x60, 0, 0x60, 0}).Hex())
	}
	creator, err := NewCommittedCreator(context.Background(), client, time.Now, latest.checkpoint, ManifestDigest, hashes)
	if err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "fixture-loopback")
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := NewRuntimeLedger(f.db.pool(t), "pnl", "repair-1019", nil, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	journey, err := NewJourneyService(ledger, creator, []byte(strings.Repeat("r", 32)), time.Now, strings.Repeat("a", 64), strings.Repeat("b", 64))
	if err != nil {
		t.Fatal(err)
	}
	f.ledger, f.creator, f.journey, f.rpc = ledger, creator, journey, client
	watcher, err := NewCommunityWatcher(journey, approved)
	if err != nil {
		t.Fatal(err)
	}
	return watcher
}

func TestCommunityPolicyRestart(t *testing.T) {
	f := repair62Chain(t)
	ctx := context.Background()
	var policies []string
	for i, approved := range [][]string{{f.chain.Alice}, {}, {f.chain.Alice}} {
		watcher := repair1019Reopen(t, f, approved)
		policies = append(policies, watcher.policyDigest)
		if err := watcher.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		f.db.mu.Lock()
		history := append([][]string(nil), f.db.communityHistory["1"]...)
		outbox := len(f.db.outbox)
		f.db.mu.Unlock()
		if len(history) != i+1 || outbox != i+1 {
			t.Fatalf("restart %d history=%v outbox=%d", i, history, outbox)
		}
		want := []string{"active", "revoked", "active"}[i]
		last := history[len(history)-1]
		if last[2] != want || last[3] != f.chain.Alice || last[4] != policies[i] {
			t.Fatalf("recovered authority=%v want=%s", last, want)
		}
		var authority CommunityAuthority
		if err := json.Unmarshal([]byte(last[8]), &authority); err != nil || authority.State != want || authority.Owner != f.chain.Alice {
			t.Fatalf("durable authority=%+v: %v", authority, err)
		}
		if err := watcher.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if len(f.db.communityHistory["1"]) != i+1 || len(f.db.outbox) != i+1 {
			t.Fatal("unchanged owner/policy duplicated observation")
		}
	}
	if policies[0] == policies[1] || policies[0] != policies[2] {
		t.Fatal("policy digest does not bind the approved wallet set")
	}
}

func TestOutboxRuntimeRights(t *testing.T) {
	db := repair62Bootstrap(t)
	sql, err := OutboxRuntimeGrantSQL("pnl", "pnl_runtime")
	if err != nil {
		t.Fatal(err)
	}
	if err = db.applyGrant(sql); err != nil {
		t.Fatal(err)
	}
	if err = db.permit("pnl.l18_outbox", "UPDATE", "published_at"); err != nil {
		t.Fatalf("handoff marker denied: %v", err)
	}
	for _, column := range []string{"identity", "topic", "payload", "sequence"} {
		var denied *pgconn.PgError
		err = db.permit("pnl.l18_outbox", "UPDATE", column)
		if !errors.As(err, &denied) || denied.Code != "42501" {
			t.Fatalf("runtime mutated %s: %v", column, err)
		}
	}
	if _, err = OutboxRuntimeGrantSQL("pnl; DROP SCHEMA pnl", "pnl_runtime"); !errors.Is(err, ErrInvalid) {
		t.Fatal("untrusted schema entered a runtime grant", err)
	}
}

func TestOutboxCrashRecovery(t *testing.T) {
	f := repair62Chain(t)
	l := f.ledger
	f.db.transport = repair1019NewStore(f.db)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	policy := RewardDeliveryPolicy(delivery.Policy{})
	store, err := delivery.NewStoreWithEpochGuard(l.pool, l.schema, policy, l.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = NewOutboxBridge(ctx, l, store, 32); !errors.Is(err, delivery.ErrAuthorization) {
		t.Fatalf("missing runtime update right admitted: %v", err)
	}
	grant, err := OutboxRuntimeGrantSQL("pnl", "pnl_runtime")
	if err != nil {
		t.Fatal(err)
	}
	if err = f.db.applyGrant(grant); err != nil {
		t.Fatal(err)
	}
	bridge, err := NewOutboxBridge(ctx, l, store, 32)
	if err != nil {
		t.Fatal(err)
	}
	payload := ClaimEvidence{Identity: "125:" + repair62Hash(99) + ":1", Pool: actorA, Beneficiary: actorB, Raw: "123456789012345678901234567890", Asset: USDL, Block: Checkpoint{10, repair62Hash(10)}, TxHash: repair62Hash(99), LogIndex: 1, Version: RuleVersion}
	// The production producer supplies the JSON persisted in l18_outbox.
	tx, err := l.begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = emit(ctx, tx, "claim:"+payload.Identity, "fee.claimed", payload); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	q := pgx.Identifier{l.schema}.Sanitize()
	f.db.transport.failHandoff = true
	if err = bridge.RunOnce(ctx); err == nil {
		t.Fatal("handoff interruption was suppressed")
	}
	if query(t, l, "SELECT count(*)::text FROM delivery_outbox") != "0" || query(t, l, "SELECT count(*)::text FROM l18_outbox WHERE published_at IS NULL") != "1" {
		t.Fatal("handoff committed only one side")
	}
	f.db.transport.failHandoff = false
	if err = bridge.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	peer, nc := repair1019Broker(t)
	pub, err := natsdriver.NewPublisher(nc, "L18")
	if err != nil {
		t.Fatal(err)
	}
	sub, err := nc.SubscribeSync("fee.claimed")
	if err != nil {
		t.Fatal(err)
	}
	if err = nc.Flush(); err != nil {
		t.Fatal(err)
	}
	leases, err := store.Claim(ctx, "before-crash", 1, time.Second)
	if err != nil || len(leases) != 1 {
		t.Fatalf("committed handoff=%v: %v", leases, err)
	}
	first := leases[0]
	peer.dropPublishAck.Store(true)
	publication, stopPublication := context.WithTimeout(ctx, 100*time.Millisecond)
	_, err = pub.Publish(publication, first.Subject, first.EventID, first.Wire)
	stopPublication()
	if err == nil {
		t.Fatal("lost broker acknowledgement reported success")
	}
	message, err := sub.NextMsg(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if message.Header.Get("Nats-Msg-Id") != first.EventID || string(message.Data) != string(first.Wire) {
		t.Fatal("transport changed producer identity or payload")
	}
	effect := func(ctx context.Context, tx pgx.Tx, envelope delivery.Envelope) error {
		_, err := tx.Exec(ctx, `INSERT INTO `+q+`.repair1019_effects(event_id,payload) VALUES($1,$2)`, envelope.EventID, envelope.Payload)
		return err
	}
	peer.dropConsumerAck.Store(true)
	consumption, stopConsumption := context.WithTimeout(ctx, 100*time.Millisecond)
	err = store.Consume(consumption, "reward-projection", "generation-1", message.Subject, message.Data, repair1019Acknowledgement{message, nc}, effect)
	stopConsumption()
	if err == nil || query(t, l, "SELECT count(*)::text FROM repair1019_effects") != "1" {
		t.Fatalf("consumer commit before ack=%v", err)
	}
	// Database clients are closed and reopened against only committed peer state.
	l.pool.Close()
	l, err = NewRuntimeLedger(f.db.pool(t), "pnl", "repair-1019", nil, f.ledger.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	store, err = delivery.NewStoreWithEpochGuard(l.pool, l.schema, policy, l.epochGuard)
	if err != nil {
		t.Fatal(err)
	}
	bridge, err = NewOutboxBridge(ctx, l, store, 32)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = l.pool.Exec(ctx, `UPDATE `+q+`.delivery_outbox SET lease_until=clock_timestamp()-interval '1 second' WHERE event_id=$1`, first.EventID); err != nil {
		t.Fatal(err)
	}
	relay := delivery.Relay{Store: store, Publisher: pub, Owner: "after-crash", Batch: 1, LeaseDuration: 2 * time.Second, PublishTimeout: time.Second, RetryDelay: time.Second, MaxAttempts: 8}
	if err = relay.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	redelivery, err := sub.NextMsg(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if string(redelivery.Data) != string(message.Data) || redelivery.Header.Get("Nats-Msg-Id") != first.EventID {
		t.Fatal("redelivery changed immutable event")
	}
	if err = store.Consume(ctx, "reward-projection", "generation-1", redelivery.Subject, redelivery.Data, repair1019Acknowledgement{redelivery, nc}, effect); err != nil {
		t.Fatal(err)
	}
	if err = bridge.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if query(t, l, "SELECT count(*)::text FROM repair1019_effects") != "1" || query(t, l, "SELECT count(*)::text FROM delivery_inbox") != "1" || query(t, l, "SELECT count(*)::text FROM delivery_outbox WHERE state='published' AND attempts=2") != "1" || query(t, l, "SELECT count(*)::text FROM l18_outbox WHERE published_at IS NOT NULL") != "1" {
		t.Fatal("restart duplicated effects or lost acknowledgement")
	}
	var recovered ClaimEvidence
	if err = json.Unmarshal([]byte(query(t, l, "SELECT payload::text FROM repair1019_effects")), &recovered); err != nil || recovered != payload {
		t.Fatalf("consumer monetary payload=%+v: %v", recovered, err)
	}
}

type repair1019Acknowledgement struct {
	message *nats.Msg
	conn    *nats.Conn
}

func (a repair1019Acknowledgement) Ack(ctx context.Context) error {
	_, err := a.conn.RequestWithContext(ctx, a.message.Reply, []byte("+ACK"))
	return err
}
