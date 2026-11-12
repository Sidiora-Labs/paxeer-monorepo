package fanout_test

import (
	"context"
	"testing"
)

func TestModerationTransactionQueuesDeletedMessageEvent(t *testing.T) {
	store := durableFixture(t)
	ctx := context.Background()
	message := sendDM(t, store, "moderated-direct-message")
	tx, err := store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := store.ModerateTx(ctx, tx, message.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var deleted bool
	var content string
	var revision int64
	if err := store.Pool.QueryRow(ctx, `SELECT deleted_at IS NOT NULL,content,revision FROM chat_messages WHERE id=$1`, message.ID).Scan(&deleted, &content, &revision); err != nil || !deleted || content != "" || revision != message.Revision+1 {
		t.Fatalf("moderated message = deleted %t, content %q, revision %d: %v", deleted, content, revision, err)
	}
	var pending int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chat_outbox WHERE message_id=$1 AND revision=$2`, message.ID, revision).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("moderation publication = %d: %v", pending, err)
	}
	page, err := store.History(ctx, addr(0xaa), message.ConversationID, "", 10)
	if err != nil || len(page.Messages) != 1 || !page.Messages[0].Deleted || page.Messages[0].Content != "" {
		t.Fatalf("moderated history = %+v: %v", page, err)
	}
	tx, err = store.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := store.ModerateTx(ctx, tx, message.ID); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chat_outbox WHERE message_id=$1 AND revision=$2`, message.ID, revision).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("repeated moderation publication = %d: %v", pending, err)
	}
}
