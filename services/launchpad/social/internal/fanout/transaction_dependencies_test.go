package fanout_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func withoutChatRelation(t *testing.T, store *fanout.DurableStore, table string, run func()) {
	t.Helper()
	name := pgx.Identifier{table}.Sanitize()
	held := pgx.Identifier{table + "_unavailable"}.Sanitize()
	if _, err := store.Pool.Exec(context.Background(), "ALTER TABLE "+name+" RENAME TO "+held); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := store.Pool.Exec(context.Background(), "ALTER TABLE "+held+" RENAME TO "+name); err != nil {
			t.Fatal(err)
		}
	}()
	run()
}

func TestDurableChatRollsBackUnavailablePersistence(t *testing.T) {
	store := durableFixture(t)
	ctx := context.Background()
	if _, err := store.Pool.Exec(ctx, fanout.ConnectedSchemaSQL); err != nil {
		t.Fatal(err)
	}
	message := sendDM(t, store, "dependency-message")
	wire, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	serial := 0
	for _, tc := range []struct {
		name   string
		tables []string
		run    func(string) error
	}{
		{"send", []string{"chat_messages", "chat_conversations", "chat_outbox"}, func(key string) error {
			_, err := store.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: key, Content: "new message"})
			return err
		}},
		{"new conversation", []string{"chat_members"}, func(key string) error {
			_, err := store.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xcc), Kind: "dm", ClientID: key, Content: "new message"})
			return err
		}},
		{"retry", []string{"chat_members", "chat_outbox"}, func(string) error {
			_, err := store.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: "dependency-message", Content: message.Content})
			return err
		}},
		{"revise", []string{"chat_messages", "chat_conversations", "chat_outbox"}, func(string) error {
			_, err := store.Revise(ctx, addr(0xaa), message.ID, 1, "revised message", false)
			return err
		}},
		{"history", []string{"chat_conversations", "chat_messages"}, func(string) error {
			_, err := store.History(ctx, addr(0xaa), message.ConversationID, "", 10)
			return err
		}},
		{"authorize", []string{"chat_members"}, func(string) error { return store.Authorized(ctx, addr(0xaa), message.ConversationID) }},
		{"list", []string{"chat_conversations"}, func(string) error { _, err := store.List(ctx, addr(0xaa), 10); return err }},
		{"read", []string{"chat_members"}, func(string) error { return store.MarkRead(ctx, addr(0xbb), message.ConversationID, 1) }},
		{"connected read", []string{"chat_members", "chat_messages", "chat_control_outbox"}, func(string) error { return store.MarkReadConnected(ctx, addr(0xbb), message.ConversationID, 1) }},
		{"join", []string{"chat_conversations", "chat_members"}, func(string) error { return store.JoinPool(ctx, addr(0xaa), addr(1)) }},
		{"current event", []string{"chat_messages", "chat_conversations"}, func(string) error { _, err := store.CurrentEvent(ctx, wire); return err }},
		{"moderate", []string{"chat_messages", "chat_conversations", "chat_outbox"}, func(string) error {
			tx, err := store.Pool.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback(ctx) }()
			return store.ModerateTx(ctx, tx, message.ID)
		}},
	} {
		for _, table := range tc.tables {
			t.Run(tc.name+"/"+table, func(t *testing.T) {
				serial++
				withoutChatRelation(t, store, table, func() {
					err := tc.run(fmt.Sprintf("dependency-%d", serial))
					var postgres *pgconn.PgError
					if !errors.As(err, &postgres) || postgres.Code != "42P01" {
						t.Fatalf("chat dependency = %v", err)
					}
				})
				var content string
				var revision, read, sequence int64
				if err := store.Pool.QueryRow(ctx, `SELECT content,revision FROM chat_messages WHERE id=$1`, message.ID).Scan(&content, &revision); err != nil {
					t.Fatal(err)
				}
				if err := store.Pool.QueryRow(ctx, `SELECT read_sequence FROM chat_members WHERE conversation_id=$1 AND wallet=$2`, message.ConversationID, addr(0xbb)).Scan(&read); err != nil {
					t.Fatal(err)
				}
				if err := store.Pool.QueryRow(ctx, `SELECT next_sequence FROM chat_conversations WHERE id=$1`, message.ConversationID).Scan(&sequence); err != nil {
					t.Fatal(err)
				}
				if content != message.Content || revision != 1 || read != 0 || sequence != message.Sequence {
					t.Fatalf("partial chat state: content=%q revision=%d read=%d sequence=%d", content, revision, read, sequence)
				}
			})
		}
	}
	recovered, err := store.Revise(ctx, addr(0xaa), message.ID, 1, "recovered message", false)
	if err != nil || recovered.Revision != 2 || recovered.Content != "recovered message" {
		t.Fatalf("restored persistence = %+v: %v", recovered, err)
	}
}

func TestDurableChatRejectsConflictingCommittedSequence(t *testing.T) {
	store := durableFixture(t)
	ctx := context.Background()
	original := sendDM(t, store, "original-sequence")
	if _, err := store.Pool.Exec(ctx, `UPDATE chat_conversations SET next_sequence=0 WHERE id=$1`, original.ConversationID); err != nil {
		t.Fatal(err)
	}
	_, err := store.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: "conflicting-sequence", Content: "second message"})
	var postgres *pgconn.PgError
	if !errors.As(err, &postgres) || postgres.Code != "23505" {
		t.Fatalf("duplicate committed sequence = %v", err)
	}
	var count int
	if err := store.Pool.QueryRow(ctx, `SELECT count(*) FROM chat_messages WHERE conversation_id=$1`, original.ConversationID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("committed messages = %d", count)
	}
	if _, err := store.Pool.Exec(ctx, `UPDATE chat_conversations SET next_sequence=$2 WHERE id=$1`, original.ConversationID, original.Sequence); err != nil {
		t.Fatal(err)
	}
	next, err := store.Send(ctx, fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: "conflicting-sequence", Content: "second message"})
	if err != nil || next.Sequence != original.Sequence+1 {
		t.Fatalf("recovered sequence = %+v: %v", next, err)
	}
}
