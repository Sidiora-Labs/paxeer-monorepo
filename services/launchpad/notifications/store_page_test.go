package notifications

import (
	"context"
	"crypto/md5"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

func expectedV2ID(id string, createdAt time.Time) string {
	digest := md5.Sum([]byte(id))
	h := hex.EncodeToString(digest[:])
	uuid := fmt.Sprintf("%012x7%s%c%s", createdAt.UTC().UnixMilli(), h[13:16], "89ab"[(digest[8]>>4)&3], h[17:])
	return uuid[:8] + "-" + uuid[8:12] + "-" + uuid[12:16] + "-" + uuid[16:20] + "-" + uuid[20:]
}

func TestPostgresListPageUnreadOnlyIncludesItemMarkedUnread(t *testing.T) {
	s, account := notificationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	t.Cleanup(func() {
		if err := s.DeleteAccount(context.Background(), account); err != nil {
			t.Error("clean up account:", err)
		}
	})
	now := time.Now().UTC().Truncate(time.Microsecond)
	items := make([]InboxItem, 2)
	for i := range items {
		n := InboxItem{Identity: Identity{account, fmt.Sprintf("marked-unread-%d", i), "v2"}, Category: "chat", Summary: "Unread notification", CreatedAt: now.Add(-time.Duration(i) * time.Second)}
		item, created, err := s.CommitTrigger(ctx, n, nil)
		if err != nil || !created {
			t.Fatalf("insert %d: created=%v err=%v", i, created, err)
		}
		items[i] = item
	}
	if err := s.SetRead(ctx, account, items[0].ID, true); err != nil {
		t.Fatal("mark first item read:", err)
	}
	readPage, err := s.ListPage(ctx, account, false, "", 100)
	if err != nil || len(readPage.Items) != 2 {
		t.Fatalf("list after read: page=%+v err=%v", readPage, err)
	}
	if readPage.Items[0].ID != items[0].ID || !readPage.Items[0].Read || readPage.Items[0].ReadAt == nil {
		t.Fatalf("first item read state: %+v", readPage.Items[0])
	}
	firstReadAt := *readPage.Items[0].ReadAt
	if err := s.SetRead(ctx, account, items[0].ID, false); err != nil {
		t.Fatal("mark first item unread:", err)
	}
	var retainedReadAt time.Time
	if err := s.DB.QueryRowContext(ctx, `SELECT read_at FROM discovery_notifications.inbox WHERE account_id=$1 AND id=$2`, account, items[0].ID).Scan(&retainedReadAt); err != nil || !retainedReadAt.Equal(firstReadAt) {
		t.Fatalf("first-read timestamp: got=%v want=%v err=%v", retainedReadAt, firstReadAt, err)
	}
	unread, err := s.ListPage(ctx, account, true, "", 100)
	if err != nil || len(unread.Items) != 2 || unread.NextCursor != "" {
		t.Fatalf("unread list after marking unread: page=%+v err=%v", unread, err)
	}
	for i, item := range unread.Items {
		if item.InboxItem != items[i] || item.Read || item.ReadAt != nil {
			t.Fatalf("unread item %d: got=%+v want=%+v with no readAt", i, item, items[i])
		}
	}
	if err := s.SetRead(ctx, account, items[1].ID, true); err != nil {
		t.Fatal("leave second item read:", err)
	}
	unread, err = s.ListPage(ctx, account, true, "", 100)
	if err != nil || len(unread.Items) != 1 || unread.NextCursor != "" {
		t.Fatalf("unread list excluding read item: page=%+v err=%v", unread, err)
	}
	if item := unread.Items[0]; item.InboxItem != items[0] || item.Read || item.ReadAt != nil {
		t.Fatalf("item marked unread: got=%+v want=%+v with no readAt", item, items[0])
	}
	all, err := s.ListPage(ctx, account, false, "", 100)
	if err != nil || len(all.Items) != 2 || all.NextCursor != "" {
		t.Fatalf("all items after read changes: page=%+v err=%v", all, err)
	}
	if all.Items[0].ID != items[0].ID || all.Items[0].Read || all.Items[0].ReadAt != nil || all.Items[1].ID != items[1].ID || !all.Items[1].Read || all.Items[1].ReadAt == nil {
		t.Fatalf("list read states: %+v", all.Items)
	}
}

func TestPostgresListPageUnreadOnlyCursorAndLimit(t *testing.T) {
	s, account := notificationDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	other := account + "-other"
	t.Cleanup(func() {
		for _, actor := range []string{account, other} {
			if err := s.DeleteAccount(context.Background(), actor); err != nil {
				t.Error("clean up account:", err)
			}
		}
	})
	// Sub-microsecond precision exercises cursors built from the persisted timestamp.
	now := time.Now().UTC().Truncate(time.Second).Add(123456789 * time.Nanosecond)
	want := []InboxItem{}
	for i, offset := range []int{0, 2, 1, 2, 2} {
		n := InboxItem{Identity: Identity{account, fmt.Sprintf("page-%d", i), "v2"}, Category: "chat", Summary: "Page notification", CreatedAt: now.Add(time.Duration(offset) * time.Second)}
		item, created, err := s.CommitTrigger(ctx, n, nil)
		if err != nil || !created {
			t.Fatalf("insert %d: created=%v err=%v", i, created, err)
		}
		want = append(want, item)
	}
	if _, created, err := s.CommitTrigger(ctx, InboxItem{Identity: Identity{other, "other", "v2"}, Category: "chat", Summary: "Other account", CreatedAt: now.Add(time.Hour)}, nil); err != nil || !created {
		t.Fatalf("insert other account: created=%v err=%v", created, err)
	}
	sort.Slice(want, func(i, j int) bool {
		if want[i].CreatedAt.Equal(want[j].CreatedAt) {
			return want[i].ID < want[j].ID
		}
		return want[i].CreatedAt.After(want[j].CreatedAt)
	})
	cursor := ""
	var first InboxPage
	for pageIndex, size := range []int{2, 2, 1} {
		page, err := s.ListPage(ctx, account, false, cursor, 2)
		if err != nil || len(page.Items) != size {
			t.Fatalf("page %d: items=%v err=%v", pageIndex, page.Items, err)
		}
		if pageIndex == 0 {
			first = page
		}
		for i, item := range page.Items {
			expected := want[pageIndex*2+i]
			if item.InboxItem != expected {
				t.Fatalf("page %d item %d: got=%+v want=%+v", pageIndex, i, item, expected)
			}
			uuid := expectedV2ID(expected.ID, expected.CreatedAt)
			if item.V2ID != uuid {
				t.Fatalf("v2 UUID: got=%q want=%q", item.V2ID, uuid)
			}
		}
		if (page.NextCursor != "") != (pageIndex < 2) || len(page.NextCursor) > 256 {
			t.Fatalf("page %d: invalid next cursor %q", pageIndex, page.NextCursor)
		}
		cursor = page.NextCursor
	}
	for _, limit := range []int{-1, 0, 101} {
		if _, err := s.ListPage(ctx, account, false, "", limit); !errors.Is(err, ErrInvalid) {
			t.Fatalf("limit %d: %v", limit, err)
		}
	}
	for _, bad := range []string{"!", "a", strings.Repeat("x", 257), " ", base64.RawURLEncoding.EncodeToString([]byte("not-a-time|" + want[0].ID)), base64.RawURLEncoding.EncodeToString([]byte(now.Format(time.RFC3339Nano) + "|short")), base64.RawURLEncoding.EncodeToString([]byte(now.Format(time.RFC3339Nano) + "|" + strings.Repeat("z", 64)))} {
		if _, err := s.ListPage(ctx, account, false, bad, 2); !errors.Is(err, ErrInvalid) {
			t.Fatalf("cursor %q: %v", bad, err)
		}
	}
	if page, err := s.ListPage(ctx, account, false, "", 1); err != nil || len(page.Items) != 1 || page.NextCursor == "" {
		t.Fatalf("limit 1: page=%+v err=%v", page, err)
	}
	if page, err := s.ListPage(ctx, account, false, "", 100); err != nil || len(page.Items) != 5 || page.NextCursor != "" {
		t.Fatalf("limit 100: page=%+v err=%v", page, err)
	}
	readItem := first.Items[1]
	if err := s.SetReadV2(ctx, other, readItem.V2ID, true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("cross-account read accepted:", err)
	}
	if err := s.SetReadV2(ctx, account, "00000000-0000-0000-0000-000000000000", true); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("missing UUID read:", err)
	}
	for _, bad := range []string{"", readItem.ID, "zzzzzzzz-0000-0000-0000-000000000000"} {
		if err := s.SetReadV2(ctx, account, bad, true); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid UUID %q: %v", bad, err)
		}
	}
	for i := 0; i < 2; i++ {
		if err := s.SetReadV2(ctx, account, readItem.V2ID, true); err != nil {
			t.Fatal("read by UUID:", err)
		}
	}
	var marked bool
	if err := s.DB.QueryRowContext(ctx, `SELECT read_at IS NOT NULL AND (document->>'Read')::boolean FROM discovery_notifications.inbox WHERE id=$1`, readItem.ID).Scan(&marked); err != nil || !marked {
		t.Fatalf("read state not persisted: marked=%v err=%v", marked, err)
	}
	unread, err := s.ListPage(ctx, account, true, "", 100)
	if err != nil || len(unread.Items) != 4 || unread.NextCursor != "" {
		t.Fatalf("unread list: page=%+v err=%v", unread, err)
	}
	for _, item := range unread.Items {
		if item.Read || item.ID == readItem.ID || item.Identity.AccountID != account {
			t.Fatalf("unexpected unread item: %+v", item)
		}
	}
	// The cursor's anchor is now read; keyset pagination must still advance past it.
	afterRead, err := s.ListPage(ctx, account, true, first.NextCursor, 2)
	if err != nil || len(afterRead.Items) != 2 || afterRead.Items[0].ID != want[2].ID || afterRead.Items[1].ID != want[3].ID || afterRead.NextCursor == "" {
		t.Fatalf("unread cursor: page=%+v err=%v", afterRead, err)
	}
	last, err := s.ListPage(ctx, account, true, afterRead.NextCursor, 2)
	if err != nil || len(last.Items) != 1 || last.Items[0].ID != want[4].ID || last.NextCursor != "" {
		t.Fatalf("unread last page: page=%+v err=%v", last, err)
	}
	all, err := s.ListPage(ctx, account, false, "", 5)
	if err != nil || len(all.Items) != 5 || !all.Items[1].Read || all.NextCursor != "" {
		t.Fatalf("all items after read: page=%+v err=%v", all, err)
	}
	if page, err := s.ListPage(ctx, account+"-empty", true, "", 2); err != nil || len(page.Items) != 0 || page.NextCursor != "" {
		t.Fatalf("empty page: page=%+v err=%v", page, err)
	}
}
