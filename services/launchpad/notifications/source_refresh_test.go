package notifications

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
)

type sourceRefreshAuthority struct {
	discovery.SourceAuthority
	resolved      map[string]int
	canonical     map[string]int
	beforeResolve func(string)
	afterResolve  func(string)
}

func (a *sourceRefreshAuthority) ResolvePublic(ctx context.Context, kind, id string) (discovery.PublicResource, error) {
	a.resolved[id]++
	if a.beforeResolve != nil {
		a.beforeResolve(id)
	}
	resource, err := a.SourceAuthority.ResolvePublic(ctx, kind, id)
	if a.afterResolve != nil {
		a.afterResolve(id)
	}
	return resource, err
}

func (a *sourceRefreshAuthority) CanonicalPool(ctx context.Context, pool string) (bool, error) {
	a.canonical[pool]++
	return a.SourceAuthority.CanonicalPool(ctx, pool)
}

func sourceRefreshFixture(t *testing.T) (*HTTPRuntime, *notificationDeliveryPeers, *sourceRefreshAuthority) {
	t.Helper()
	runtime, peer := runtimeRetriesFixture(t)
	authority := &sourceRefreshAuthority{SourceAuthority: runtime.Discovery.Feed.Sources, resolved: map[string]int{}, canonical: map[string]int{}}
	runtime.Discovery.Feed.Sources = authority
	return runtime, peer, authority
}

const sourceRefreshPool = "0x3333333333333333333333333333333333333333"

func sourceRefreshPublishPool(t *testing.T, runtime *HTTPRuntime) discovery.SourceRef {
	t.Helper()
	if _, err := runtime.DB.Exec(`INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$1,$1,10,$2,$3,0,$4)`, sourceRefreshPool, "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64), time.Now().Add(-time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	resource, err := runtime.Config.Pools.ResolvePublic(context.Background(), resourcePool, sourceRefreshPool)
	if err != nil {
		t.Fatal(err)
	}
	return resource.Ref
}

func sourceRefreshPublishCallout(t *testing.T, runtime *HTTPRuntime, peer *notificationDeliveryPeers, id string, published time.Time) {
	t.Helper()
	callout := discovery.Callout{ID: id, AuthorAccountID: peer.account.AccountID, PoolAddress: sourceRefreshPool, Body: "A public pool observation", Direction: "neutral", Disclosure: "organic", ModerationStatus: "approved", CreatedAt: published, Revision: 1}
	if err := runtime.Discovery.Callouts.Publish(context.Background(), peer.account.AccountID, "", callout); err != nil {
		t.Fatal(err)
	}
}

func sourceRefreshAge(t *testing.T, db *sql.DB) time.Time {
	t.Helper()
	old := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	if _, err := db.Exec(`UPDATE narrative_discovery.sources SET observed_at=$1`, old); err != nil {
		t.Fatal(err)
	}
	return old
}

func sourceRefreshObserved(t *testing.T, db *sql.DB, id string) time.Time {
	t.Helper()
	var observed time.Time
	if err := db.QueryRow(`SELECT observed_at FROM narrative_discovery.sources WHERE id=$1`, id).Scan(&observed); err != nil {
		t.Fatal(err)
	}
	return observed
}

func sourceRefreshMarks(t *testing.T, db *sql.DB) int {
	t.Helper()
	var marks int
	if err := db.QueryRow(`SELECT count(*) FROM narrative_discovery.consumer_events`).Scan(&marks); err != nil {
		t.Fatal(err)
	}
	return marks
}

func TestSourceRefreshUnchangedPoolRenewsWithoutMarks(t *testing.T) {
	runtime, _, _ := sourceRefreshFixture(t)
	ref := sourceRefreshPublishPool(t, runtime)
	if err := runtime.Discovery.Store.Consume(context.Background(), runtime.Discovery.Feed.Sources, discovery.PublicEvent{Consumer: "pool-publication", EventID: ref.EventID, Source: ref, ObservedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	marks := sourceRefreshMarks(t, runtime.DB)
	previous := sourceRefreshAge(t, runtime.DB)
	for pass := 0; pass < 2; pass++ {
		if err := runtime.refreshSources(context.Background()); err != nil {
			t.Fatal(err)
		}
		observed := sourceRefreshObserved(t, runtime.DB, ref.ID)
		if !observed.After(previous) || sourceRefreshMarks(t, runtime.DB) != marks {
			t.Fatalf("pass %d: observed=%v previous=%v marks=%d want=%d", pass, observed, previous, sourceRefreshMarks(t, runtime.DB), marks)
		}
		previous = observed
	}
}

func TestSourceRefreshContinuesAfterAuthorityFailure(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	sourceRefreshPublishPool(t, runtime)
	for index := 0; index < 3; index++ {
		sourceRefreshPublishCallout(t, runtime, peer, fmt.Sprintf("refresh-callout-%d", index), time.Now().UTC().Add(-time.Duration(index+1)*time.Minute).Truncate(time.Microsecond))
	}
	if _, err := runtime.DB.Exec(`DELETE FROM narrative_discovery.callouts WHERE id=$1`, "refresh-callout-1"); err != nil {
		t.Fatal(err)
	}
	old := sourceRefreshAge(t, runtime.DB)
	err := runtime.refreshSources(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 failed source") || !strings.Contains(err.Error(), "callout") || !strings.Contains(err.Error(), "refresh-callout-1") || !strings.Contains(err.Error(), sql.ErrNoRows.Error()) {
		t.Fatalf("source failure summary: %v", err)
	}
	for _, id := range []string{"refresh-callout-0", "refresh-callout-2"} {
		if !sourceRefreshObserved(t, runtime.DB, id).After(old) || authority.resolved[id] != 1 {
			t.Fatalf("source %s not renewed once: %+v", id, authority.resolved)
		}
	}
	if !sourceRefreshObserved(t, runtime.DB, "refresh-callout-1").Equal(old) || authority.resolved["refresh-callout-1"] != 1 {
		t.Fatal("failed source renewed or resolved more than once")
	}
	if sourceRefreshMarks(t, runtime.DB) != 1 {
		t.Fatal("unchanged callout wrote a mark")
	}
	if errors.Is(err, sql.ErrNoRows) {
		t.Fatal("source summary exposes the cause through errors.Is")
	}
}

func TestSourceRefreshNewPoolMarksOnlyItsFirstPass(t *testing.T) {
	runtime, _, authority := sourceRefreshFixture(t)
	ref := sourceRefreshPublishPool(t, runtime)
	for pass := 0; pass < 2; pass++ {
		if err := runtime.refreshSources(context.Background()); err != nil {
			t.Fatal(err)
		}
		stored, err := runtime.Discovery.Store.Source(context.Background(), ref.ID)
		if err != nil || !stored.Equal(ref) || sourceRefreshMarks(t, runtime.DB) != 1 {
			t.Fatalf("pass %d: source=%+v marks=%d error=%v", pass, stored, sourceRefreshMarks(t, runtime.DB), err)
		}
	}
	if len(authority.resolved) != 0 || len(authority.canonical) != 0 {
		t.Fatalf("listed pool read again: resolved=%v canonical=%v", authority.resolved, authority.canonical)
	}
}

func TestSourceRefreshInvalidSourceRemainsRetryable(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	sourceRefreshPublishPool(t, runtime)
	sourceRefreshPublishCallout(t, runtime, peer, "refresh-orphaned-callout", time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond))
	if _, err := runtime.DB.Exec(`DELETE FROM indexer.indexer_canonical_creations_v1`); err != nil {
		t.Fatal(err)
	}
	old := sourceRefreshAge(t, runtime.DB)
	for pass := 0; pass < 2; pass++ {
		err := runtime.refreshSources(context.Background())
		if err == nil || !strings.Contains(err.Error(), discovery.ErrInvalid.Error()) || !strings.Contains(err.Error(), "refresh-orphaned-callout") || errors.Is(err, ErrInvalid) || errors.Is(err, discovery.ErrInvalid) {
			t.Fatalf("invalid source stopped retry: %v", err)
		}
		if !sourceRefreshObserved(t, runtime.DB, "refresh-orphaned-callout").Equal(old) {
			t.Fatal("noncanonical source renewed")
		}
	}
	if authority.resolved["refresh-orphaned-callout"] != 2 || authority.canonical[sourceRefreshPool] != 2 || sourceRefreshMarks(t, runtime.DB) != 0 {
		t.Fatalf("invalid source reads: resolved=%v canonical=%v", authority.resolved, authority.canonical)
	}
}

func TestSourceRefreshResolvesEachSourceOnceAndReusesCanonicalList(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	ref := sourceRefreshPublishPool(t, runtime)
	if err := runtime.refreshSources(context.Background()); err != nil {
		t.Fatal(err)
	}
	sourceRefreshPublishCallout(t, runtime, peer, "refresh-counted-callout", time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond))
	old := sourceRefreshAge(t, runtime.DB)
	if err := runtime.refreshSources(context.Background()); err != nil {
		t.Fatal(err)
	}
	if authority.resolved["refresh-counted-callout"] != 1 || authority.resolved[ref.ID] != 0 || len(authority.canonical) != 0 {
		t.Fatalf("source reads: resolved=%v canonical=%v", authority.resolved, authority.canonical)
	}
	for _, id := range []string{ref.ID, "refresh-counted-callout"} {
		if !sourceRefreshObserved(t, runtime.DB, id).After(old) {
			t.Fatalf("source %s not renewed", id)
		}
	}
	if sourceRefreshMarks(t, runtime.DB) != 1 {
		t.Fatal("unchanged sources wrote marks")
	}
}

func TestSourceRefreshConcurrentDocumentChangeKeepsObservation(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	sourceRefreshPublishPool(t, runtime)
	sourceRefreshPublishCallout(t, runtime, peer, "refresh-concurrent-callout", time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond))
	old := sourceRefreshAge(t, runtime.DB)
	authority.afterResolve = func(id string) {
		if id == "refresh-concurrent-callout" {
			if err := runtime.Discovery.Store.EraseAttribution(context.Background(), peer.account.AccountID); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := runtime.refreshSources(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, err := runtime.Discovery.Store.Source(context.Background(), "refresh-concurrent-callout")
	if err != nil || stored.CreatorPublicID != "" || !sourceRefreshObserved(t, runtime.DB, stored.ID).Equal(old) {
		t.Fatalf("concurrent document observed: %+v %v", stored, err)
	}
	if !sourceRefreshObserved(t, runtime.DB, sourceRefreshPool).After(old) || sourceRefreshMarks(t, runtime.DB) != 1 {
		t.Fatal("pool renewal or mark count changed")
	}
}

func TestSourceRefreshPoolsRenewBeforeAllRemainingSourcesFail(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	sourceRefreshPublishPool(t, runtime)
	if err := runtime.refreshSources(context.Background()); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		sourceRefreshPublishCallout(t, runtime, peer, fmt.Sprintf("refresh-unavailable-%d", index), time.Now().UTC().Add(-time.Duration(index+1)*time.Minute).Truncate(time.Microsecond))
	}
	if _, err := runtime.DB.Exec(`DELETE FROM narrative_discovery.callouts`); err != nil {
		t.Fatal(err)
	}
	old := sourceRefreshAge(t, runtime.DB)
	authority.beforeResolve = func(id string) {
		if !sourceRefreshObserved(t, runtime.DB, sourceRefreshPool).After(old) {
			t.Fatalf("pool not renewed before resolving %s", id)
		}
	}
	err := runtime.refreshSources(context.Background())
	if err == nil || !strings.Contains(err.Error(), "2 failed sources") || !strings.Contains(err.Error(), "refresh-unavailable-0") {
		t.Fatalf("source failures: %v", err)
	}
	for index := 0; index < 2; index++ {
		id := fmt.Sprintf("refresh-unavailable-%d", index)
		if authority.resolved[id] != 1 || !sourceRefreshObserved(t, runtime.DB, id).Equal(old) {
			t.Fatalf("failed source %s: resolved=%v", id, authority.resolved)
		}
	}
	if sourceRefreshMarks(t, runtime.DB) != 1 {
		t.Fatal("renewal wrote a mark")
	}
}

func TestSourceRefreshCancellationKeepsCompletedPoolRenewal(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	sourceRefreshPublishPool(t, runtime)
	if err := runtime.refreshSources(context.Background()); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 2; index++ {
		sourceRefreshPublishCallout(t, runtime, peer, fmt.Sprintf("refresh-canceled-%d", index), time.Now().UTC().Add(-time.Duration(index+1)*time.Minute).Truncate(time.Microsecond))
	}
	old := sourceRefreshAge(t, runtime.DB)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	authority.beforeResolve = func(string) { cancel() }
	if err := runtime.refreshSources(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled refresh: %v", err)
	}
	if !sourceRefreshObserved(t, runtime.DB, sourceRefreshPool).After(old) || len(authority.resolved) != 1 || !sourceRefreshObserved(t, runtime.DB, "refresh-canceled-1").Equal(old) {
		t.Fatalf("canceled refresh continued: resolved=%v", authority.resolved)
	}
}

func TestSourceRefreshChangedCalloutConsumesOnce(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	sourceRefreshPublishPool(t, runtime)
	sourceRefreshPublishCallout(t, runtime, peer, "refresh-changed-callout", time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond))
	authority.beforeResolve = func(id string) {
		if id == "refresh-changed-callout" {
			if _, err := runtime.Discovery.Callouts.Edit(context.Background(), peer.account.AccountID, id, "Updated public observation", 1, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := runtime.refreshSources(context.Background()); err != nil {
		t.Fatal(err)
	}
	stored, err := runtime.Discovery.Store.Source(context.Background(), "refresh-changed-callout")
	if err != nil || stored.SourceRevision != 2 || authority.resolved[stored.ID] != 1 || sourceRefreshMarks(t, runtime.DB) != 2 {
		t.Fatalf("changed source: %+v %v reads=%v marks=%d", stored, err, authority.resolved, sourceRefreshMarks(t, runtime.DB))
	}
}

func TestSourceRefreshContinuesAfterConsumeConflict(t *testing.T) {
	runtime, peer, authority := sourceRefreshFixture(t)
	sourceRefreshPublishPool(t, runtime)
	for index := 0; index < 2; index++ {
		sourceRefreshPublishCallout(t, runtime, peer, fmt.Sprintf("refresh-conflict-%d", index), time.Now().UTC().Add(-time.Duration(index+1)*time.Minute).Truncate(time.Microsecond))
	}
	old := sourceRefreshAge(t, runtime.DB)
	authority.beforeResolve = func(id string) {
		if id == "refresh-conflict-0" {
			if _, err := runtime.Discovery.Callouts.Edit(context.Background(), peer.account.AccountID, id, "First update", 1, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	authority.afterResolve = func(id string) {
		if id == "refresh-conflict-0" {
			if _, err := runtime.Discovery.Callouts.Edit(context.Background(), peer.account.AccountID, id, "Second update", 2, false); err != nil {
				t.Fatal(err)
			}
		}
	}
	err := runtime.refreshSources(context.Background())
	if err == nil || !strings.Contains(err.Error(), "1 failed source") || !strings.Contains(err.Error(), "refresh-conflict-0") || !strings.Contains(err.Error(), discovery.ErrConflict.Error()) {
		t.Fatalf("source conflict: %v", err)
	}
	if !sourceRefreshObserved(t, runtime.DB, "refresh-conflict-1").After(old) || authority.resolved["refresh-conflict-1"] != 1 || sourceRefreshMarks(t, runtime.DB) != 1 {
		t.Fatalf("source behind conflict not renewed: resolved=%v marks=%d", authority.resolved, sourceRefreshMarks(t, runtime.DB))
	}
}
