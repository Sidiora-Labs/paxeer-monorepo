package interactions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
)

func TestCommittedInboxPrivacyControlsDirectDelivery(t *testing.T) {
	f := connected(t)
	sender, recipient := f.wallets[0], f.wallets[1]
	chat := fanout.DurableStore{Pool: f.db, Policy: f.policy.SendPolicy, ReadPolicy: f.policy.ReadChatPolicy, CursorKey: []byte(strings.Repeat("k", 32))}
	initial, err := f.policy.Privacy(f.ctx[recipient])
	if err != nil || initial.Inbox != "everyone" || initial.Revision < 1 {
		t.Fatalf("initial inbox = %+v: %v", initial, err)
	}
	if err := f.policy.SetPrivacy(f.ctx[recipient], "followed_only", initial.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := chat.Send(f.ctx[sender], fanout.SendInput{Actor: sender, Kind: "dm", Target: recipient, ClientID: "inbox-before-follow", Content: "first message"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("unfollowed direct send = %v", err)
	}
	if err := f.store.Follow(f.ctx[recipient], Request{Actor: recipient, ID: sender, Key: "recipient-follow"}); err != nil {
		t.Fatal(err)
	}
	message, err := chat.Send(f.ctx[sender], fanout.SendInput{Actor: sender, Kind: "dm", Target: recipient, ClientID: "inbox-after-follow", Content: "visible message"})
	if err != nil || message.Content != "visible message" || message.ConversationID == "" {
		t.Fatalf("followed direct send = %+v: %v", message, err)
	}
	privacy, err := f.policy.Privacy(f.ctx[recipient])
	if err != nil || privacy.Inbox != "followed_only" || privacy.Revision != initial.Revision+1 {
		t.Fatalf("followed inbox = %+v: %v", privacy, err)
	}
	if err := f.policy.SetPrivacy(f.ctx[recipient], "nobody", privacy.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := chat.Send(f.ctx[sender], fanout.SendInput{Actor: sender, Kind: "dm", Target: recipient, ClientID: "inbox-closed", Content: "blocked message"}); !errors.Is(err, ErrDenied) {
		t.Fatalf("closed inbox direct send = %v", err)
	}
	if err := f.policy.SetPrivacy(f.ctx[recipient], "everyone", privacy.Revision); !errors.Is(err, ErrRevision) {
		t.Fatalf("stale inbox revision = %v", err)
	}
	if err := f.policy.SetPrivacy(f.ctx[recipient], "invalid", privacy.Revision+1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid inbox mode = %v", err)
	}
	if err := f.policy.SetBlock(f.ctx[recipient], sender, false); err != nil {
		t.Fatal(err)
	}
	var edges int
	if err := f.db.QueryRow(f.ctx[recipient], `SELECT count(*) FROM interaction_follows WHERE follower=$1 AND followee=$2`, recipient, sender).Scan(&edges); err != nil || edges != 0 {
		t.Fatalf("blocked follow edge = %d: %v", edges, err)
	}
	if err := f.policy.SetBlock(f.ctx[recipient], sender, true); err != nil {
		t.Fatal(err)
	}
	var blocks int
	if err := f.db.QueryRow(f.ctx[recipient], `SELECT count(*) FROM interaction_blocks WHERE owner=$1 AND target=$2`, recipient, sender).Scan(&blocks); err != nil || blocks != 0 {
		t.Fatalf("unblocked pair = %d: %v", blocks, err)
	}
	tx, err := f.db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(tx)
	if err := f.policy.ReadChatPolicy(f.ctx[sender], tx, sender, message.ConversationID); err != nil {
		t.Fatalf("member read policy = %v", err)
	}
	if err := f.policy.ReadChatPolicy(f.ctx[f.wallets[2]], tx, f.wallets[2], message.ConversationID); !errors.Is(err, ErrDenied) {
		t.Fatalf("nonmember read policy = %v", err)
	}
	if err := f.policy.ReadChatPolicy(f.ctx[sender], tx, recipient, message.ConversationID); !errors.Is(err, ErrDenied) {
		t.Fatalf("different actor read policy = %v", err)
	}
	if count, err := f.store.PruneReplay(f.ctx[sender], 10); err != nil || count != 0 {
		t.Fatalf("expired replay count = %d: %v", count, err)
	}
	verifier, ok := f.http.Verifier.(CommittedReceiptVerifier)
	if !ok {
		t.Fatalf("receipt verifier type = %T", f.http.Verifier)
	}
	if err := verifier.Verify(context.Background(), "invalid-hash", 125); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid receipt hash = %v", err)
	}
	if err := verifier.Verify(context.Background(), "0x"+strings.Repeat("0", 64), 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong receipt chain = %v", err)
	}
	missingDatabase := verifier
	missingDatabase.DB = nil
	if err := missingDatabase.Verify(context.Background(), "0x"+strings.Repeat("0", 64), 125); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound receipt authority = %v", err)
	}
	if err := verifier.Verify(context.Background(), "0x"+strings.Repeat("0", 64), 125); !errors.Is(err, ErrDenied) {
		t.Fatalf("uncommitted receipt = %v", err)
	}
	var committedHash string
	if err := f.db.QueryRow(context.Background(), `SELECT transaction_hash FROM indexer.indexer_raw_logs_v1 ORDER BY height,transaction_index,log_index LIMIT 1`).Scan(&committedHash); err != nil {
		t.Fatal(err)
	}
	if err := verifier.Verify(context.Background(), committedHash, 125); err != nil {
		t.Fatalf("committed receipt = %v", err)
	}
}
