package discovery

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// coverageAuthority reads the same persisted resources and projections that the
// feed and interval services consume at their authority boundaries.
type coverageAuthority struct {
	db    *sql.DB
	store Store
}

func (a coverageAuthority) CanonicalPool(ctx context.Context, pool string) (bool, error) {
	var found bool
	err := a.db.QueryRowContext(ctx, `SELECT true FROM coverage_pools WHERE address=$1`, pool).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return found, err
}
func (a coverageAuthority) VerifiedAccount(ctx context.Context, account string) (bool, error) {
	var found bool
	err := a.db.QueryRowContext(ctx, `SELECT true FROM coverage_accounts WHERE id=$1`, account).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return found, err
}
func (a coverageAuthority) ResolvePublic(ctx context.Context, kind, id string) (PublicResource, error) {
	if kind == kindCallout {
		return (OwnedSources{Callouts: CalloutService{Store: a.store}}).ResolvePublic(ctx, kind, id)
	}
	var raw []byte
	if err := a.db.QueryRowContext(ctx, `SELECT document FROM coverage_resources WHERE kind=$1 AND id=$2`, kind, id).Scan(&raw); err != nil {
		return PublicResource{}, err
	}
	var result PublicResource
	err := json.Unmarshal(raw, &result)
	return result, err
}
func (a coverageAuthority) ReviewCallout(ctx context.Context, actor, pool, body string) (ModerationDecision, error) {
	var decision ModerationDecision
	err := a.db.QueryRowContext(ctx, `SELECT resource_id,revision,visible,observed_at FROM coverage_reviews WHERE actor=$1 AND pool=$2 AND body=$3`, actor, pool, body).Scan(&decision.ResourceID, &decision.Revision, &decision.Visible, &decision.ObservedAt)
	return decision, err
}
func (a coverageAuthority) Preferences(ctx context.Context, account string) (PreferenceSnapshot, error) {
	var raw []byte
	if err := a.db.QueryRowContext(ctx, `SELECT document FROM coverage_preferences WHERE account=$1`, account).Scan(&raw); err != nil {
		return PreferenceSnapshot{}, err
	}
	var value PreferenceSnapshot
	err := json.Unmarshal(raw, &value)
	return value, err
}
func (a coverageAuthority) Watchlist(ctx context.Context, account string) (WatchlistSnapshot, error) {
	var raw []byte
	if err := a.db.QueryRowContext(ctx, `SELECT document FROM coverage_watchlist WHERE account=$1`, account).Scan(&raw); err != nil {
		return WatchlistSnapshot{}, err
	}
	var value WatchlistSnapshot
	err := json.Unmarshal(raw, &value)
	return value, err
}
func (a coverageAuthority) Graph(ctx context.Context, account string) (GraphSnapshot, error) {
	var raw []byte
	if err := a.db.QueryRowContext(ctx, `SELECT document FROM coverage_graph WHERE account=$1`, account).Scan(&raw); err != nil {
		return GraphSnapshot{}, err
	}
	var value GraphSnapshot
	err := json.Unmarshal(raw, &value)
	return value, err
}
func (a coverageAuthority) ResolveVisibility(ctx context.Context, id, account string) (ModerationDecision, error) {
	var value ModerationDecision
	err := a.db.QueryRowContext(ctx, `SELECT resource_id,revision,visible,observed_at FROM coverage_visibility WHERE id=$1 AND account=$2`, id, account).Scan(&value.ResourceID, &value.Revision, &value.Visible, &value.ObservedAt)
	return value, err
}
func (a coverageAuthority) WalletAttribution(ctx context.Context, wallet string) (WalletAttribution, error) {
	var value WalletAttribution
	err := a.db.QueryRowContext(ctx, `SELECT account,profile,verified,opted_in FROM coverage_attribution WHERE wallet=$1`, wallet).Scan(&value.AccountID, &value.ProfileID, &value.Verified, &value.OptedIn)
	return value, err
}
func (a coverageAuthority) CurrentAttribution(ctx context.Context, account, wallet string) (bool, string, error) {
	var profile string
	err := a.db.QueryRowContext(ctx, `SELECT profile FROM coverage_attribution WHERE wallet=$1 AND account=$2 AND verified AND opted_in`, wallet, account).Scan(&profile)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", nil
	}
	return err == nil, profile, err
}

func coverageSetup(t *testing.T) (Store, coverageAuthority) {
	t.Helper()
	s, _, _ := proofDiscoveryStore(t)
	statements := []string{
		`CREATE TABLE coverage_pools(address text primary key)`,
		`CREATE TABLE coverage_accounts(id text primary key)`,
		`CREATE TABLE coverage_resources(kind text,id text,document jsonb,primary key(kind,id))`,
		`CREATE TABLE coverage_reviews(actor text,pool text,body text,resource_id text,revision bigint,visible boolean,observed_at timestamptz,primary key(actor,pool,body))`,
		`CREATE TABLE coverage_preferences(account text primary key,document jsonb)`,
		`CREATE TABLE coverage_watchlist(account text primary key,document jsonb)`,
		`CREATE TABLE coverage_graph(account text primary key,document jsonb)`,
		`CREATE TABLE coverage_visibility(id text,account text,resource_id text,revision bigint,visible boolean,observed_at timestamptz,primary key(id,account))`,
		`CREATE TABLE coverage_attribution(wallet text primary key,account text,profile text,verified boolean,opted_in boolean)`,
	}
	for _, statement := range statements {
		if _, err := s.DB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	return s, coverageAuthority{db: s.DB, store: s}
}

func coverageExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(query, args...); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoveryFeedUsesPersistedAuthorityAndCursor(t *testing.T) {
	s, a := coverageSetup(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	account, author := uuid.NewString(), uuid.NewString()
	coverageExec(t, s.DB, `INSERT INTO coverage_pools VALUES($1)`, pool)
	coverageExec(t, s.DB, `INSERT INTO coverage_accounts VALUES($1)`, author)
	for i := 0; i < 3; i++ {
		id := uuid.NewString()
		body := fmt.Sprintf("Analysis %d", i)
		coverageExec(t, s.DB, `INSERT INTO coverage_reviews VALUES($1,$2,$3,$4,1,true,$5)`, author, pool, body, id, now)
		callout := Callout{ID: id, AuthorAccountID: author, PoolAddress: pool, Body: body, Direction: "neutral", Disclosure: "organic", ModerationStatus: statusApproved, PositionDisclosure: "declared position", CreatedAt: now.Add(time.Duration(i) * time.Microsecond), Revision: 1}
		if err := (CalloutService{Store: s, Authority: a}).Publish(ctx, author, "", callout); err != nil {
			t.Fatal(err)
		}
		coverageExec(t, s.DB, `INSERT INTO coverage_visibility VALUES($1,$2,$1,1,true,$3)`, id, account, now)
	}
	pref := PreferenceSnapshot{AccountID: account, Revision: 1, Linked: true, ObservedAt: now, Languages: []string{"en"}}
	graph := GraphSnapshot{AccountID: account, Revision: 1, ObservedAt: now}
	coverageExec(t, s.DB, `INSERT INTO coverage_preferences VALUES($1,$2)`, account, encode(pref))
	coverageExec(t, s.DB, `INSERT INTO coverage_graph VALUES($1,$2)`, account, encode(graph))
	feed := Feed{Store: s, Sources: a, Preferences: a, Graph: a, Signer: CursorSigner{Key: []byte("01234567890123456789012345678901")}}
	first, err := feed.Page(ctx, account, "relevance", "", 2, now.Add(time.Second))
	if err != nil || !first.Personalized || len(first.Entries) != 2 || first.Cursor == "" || first.Entries[0].State != "available" || first.Entries[0].Body == "" || first.Entries[0].PositionDisclosureKind != "user_declared" {
		t.Fatalf("first page: %+v %v", first, err)
	}
	second, err := feed.Page(ctx, account, "relevance", first.Cursor, 2, now.Add(time.Second))
	if err != nil || len(second.Entries) != 1 || second.Cursor != "" || second.Snapshot != first.Snapshot {
		t.Fatalf("second page: %+v %v", second, err)
	}
	if second.Entries[0].Source.ID == first.Entries[0].Source.ID || second.Entries[0].Source.ID == first.Entries[1].Source.ID {
		t.Fatal("cursor repeated a resource")
	}
	if _, err := feed.Page(ctx, account, "chronological", first.Cursor, 2, now.Add(time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("algorithm change: %v", err)
	}
	if _, err := feed.Page(ctx, "another-account", "relevance", first.Cursor, 2, now.Add(time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("account change: %v", err)
	}
	card, err := feed.Detail(ctx, account, second.Entries[0].Source.ID, now.Add(time.Second))
	if err != nil || card.Body != second.Entries[0].Body {
		t.Fatalf("detail: %+v %v", card, err)
	}
	coverageExec(t, s.DB, `UPDATE coverage_visibility SET visible=false WHERE id=$1 AND account=$2`, second.Entries[0].Source.ID, account)
	if _, err := feed.Detail(ctx, account, second.Entries[0].Source.ID, now.Add(time.Second)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("hidden detail: %v", err)
	}
	visible, err := feed.Page(ctx, account, "relevance", "", 3, now.Add(time.Second))
	if err != nil || len(visible.Entries) != 2 {
		t.Fatalf("hidden resource omitted: %+v %v", visible, err)
	}
	coverageExec(t, s.DB, `UPDATE coverage_visibility SET visible=true WHERE id=$1 AND account=$2`, second.Entries[0].Source.ID, account)
	graph.BlockedActorIDs = []string{author}
	graph.Revision++
	coverageExec(t, s.DB, `UPDATE coverage_graph SET document=$2 WHERE account=$1`, account, encode(graph))
	blocked, err := feed.Page(ctx, account, "relevance", "", 2, now.Add(time.Second))
	if err != nil || len(blocked.Entries) != 0 {
		t.Fatalf("blocked feed: %+v %v", blocked, err)
	}
	if _, err := feed.Page(ctx, account, "relevance", first.Cursor, 2, now.Add(time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("graph revision change: %v", err)
	}
	graph.ObservedAt = now.Add(-time.Minute)
	coverageExec(t, s.DB, `UPDATE coverage_graph SET document=$2 WHERE account=$1`, account, encode(graph))
	fallback, err := feed.Page(ctx, account, "relevance", "", 2, now.Add(time.Second))
	if err != nil || fallback.Personalized || fallback.FallbackReason != "graph_unavailable" || len(fallback.Entries) != 2 {
		t.Fatalf("public fallback: %+v %v", fallback, err)
	}
	if err := s.ExpireSnapshots(ctx, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	var remaining int
	if err := s.DB.QueryRow(`SELECT count(*) FROM narrative_discovery.snapshots`).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("expired snapshots: %d %v", remaining, err)
	}
}

func TestDiscoveryRankIntervalsFromPersistedBuckets(t *testing.T) {
	s, a := coverageSetup(t)
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Hour).Add(-2 * time.Hour)
	end := start.Add(2 * time.Hour)
	coverageExec(t, s.DB, `CREATE SCHEMA pnl`)
	coverageExec(t, s.DB, `CREATE TABLE pnl.inventory_l16_wallet_ranges_v3(snapshot_id text,wallet text,input_digest text,watermark text,start_at timestamptz,end_at timestamptz,generation bigint)`)
	coverageExec(t, s.DB, `CREATE TABLE pnl.inventory_l16_wallet_intervals_v3(snapshot_id text,interval text,bucket timestamptz,payload jsonb)`)
	accounts := make([]string, 0, 2)
	snapshots := make([]string, 0, 2)
	for i, wallet := range []string{pool, "0x2222222222222222222222222222222222222222"} {
		account, profile, snapshot := uuid.NewString(), uuid.NewString(), uuid.NewString()
		accounts = append(accounts, account)
		snapshots = append(snapshots, snapshot)
		coverageExec(t, s.DB, `INSERT INTO coverage_attribution VALUES($1,$2,$3,true,true)`, wallet, account, profile)
		coverageExec(t, s.DB, `INSERT INTO pnl.inventory_l16_wallet_ranges_v3 VALUES($1,$2,'valuation-v1','watermark-v1',$3,$4,1)`, snapshot, wallet, start, end)
		net := "2500000"
		if i == 1 {
			net = "-1000000"
		}
		bucket := map[string]any{"Start": start, "End": start.Add(time.Hour), "grossProceedsUsdlRaw": "4000000", "knownAllocatedBasisUsdlRaw": "10000000", "realizedPnlUsdlRaw": net, "tradeCount": 2, "accountingComplete": true, "fees": []any{map[string]any{"asset": map[string]any{"kind": "erc20", "chainId": 125, "address": repair46USDL, "decimals": 6}, "amountRaw": "500000"}}}
		coverageExec(t, s.DB, `INSERT INTO pnl.inventory_l16_wallet_intervals_v3 VALUES($1,'1h',$2,$3)`, snapshot, start, encode(bucket))
	}
	intervals := SQLIntervalAuthority{DB: s.DB, Attribution: a, QuoteTokenAddress: repair46USDL}
	rows, err := intervals.Intervals(ctx, start, end)
	if err != nil || len(rows) != 2 {
		t.Fatalf("intervals: %+v %v", rows, err)
	}
	for _, row := range rows {
		if row.GrossProceeds != "4.000000" || row.CapitalBasis != "10.000000" || row.Fees != "0.500000" || row.TradeCount != 2 || !row.KnownBasis || !row.Reconciled {
			t.Fatalf("bucket: %+v", row)
		}
	}
	board, err := RankIntervals(ctx, intervals, start, end)
	if err != nil || len(board.Entries) != 2 || board.Entries[0].Wallet != pool || board.Entries[0].Rank != 1 || board.Entries[0].ReturnNumerator != "1" || board.Entries[0].ReturnDenominator != "4" || board.Entries[1].Rank != 2 {
		t.Fatalf("board: %+v %v", board, err)
	}
	aggregate, err := AggregateSquad(rows)
	if err != nil || aggregate.EligibleMembers != 2 || aggregate.NetPnL != "1.500000" {
		t.Fatalf("aggregate: %+v %v", aggregate, err)
	}
	q := Squad{ID: uuid.NewString(), OwnerAccountID: accounts[0], Name: "Research", Policy: "open", Visibility: "private", Revision: 1}
	if err := s.CreateSquad(ctx, accounts[0], q); err != nil {
		t.Fatal(err)
	}
	joined, err := s.RequestJoin(ctx, accounts[1], q.ID, 0)
	if err != nil || !joined.CanPublish() {
		t.Fatalf("join: %+v %v", joined, err)
	}
	performance, err := s.SquadPerformance(ctx, intervals, q.ID, start, end)
	if err != nil || performance.EligibleMembers != 2 || performance.NetPnL != "1.500000" {
		t.Fatalf("squad performance: %+v %v", performance, err)
	}
	if err := s.SetMemberStatus(ctx, accounts[0], q.ID, accounts[1], "removed", joined.Revision); err != nil {
		t.Fatal(err)
	}
	performance, err = s.SquadPerformance(ctx, intervals, q.ID, start, end)
	if err != nil || performance.EligibleMembers != 1 || performance.NetPnL != "2.500000" {
		t.Fatalf("active members only: %+v %v", performance, err)
	}
	if _, err := RankIntervals(ctx, intervals, end, start); !errors.Is(err, ErrInvalid) {
		t.Fatalf("reversed interval: %v", err)
	}
	if _, err := RankIntervals(ctx, nil, start, end); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing ranking authority: %v", err)
	}
	if _, err := RankIntervals(ctx, intervals, time.Time{}, end); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty ranking start: %v", err)
	}
	if _, err := intervals.Intervals(ctx, start.Add(time.Minute), end); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unaligned interval: %v", err)
	}
	if _, err := (SQLIntervalAuthority{DB: s.DB, Attribution: a, QuoteTokenAddress: "invalid"}).Intervals(ctx, start, end); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid quote asset: %v", err)
	}
	path := "/v2/discovery/leaderboards?start=" + url.QueryEscape(start.Format(time.RFC3339)) + "&end=" + url.QueryEscape(end.Format(time.RFC3339))
	response := httptest.NewRecorder()
	(HTTP{Intervals: intervals}).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
	var served LeaderboardV2
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &served) != nil || len(served.Entries) != 2 || served.Entries[0].Wallet != pool {
		t.Fatalf("leaderboard response: %d %s", response.Code, response.Body.String())
	}
	coverageExec(t, s.DB, `UPDATE pnl.inventory_l16_wallet_intervals_v3 SET payload=jsonb_set(payload,'{realizedPnlUsdlRaw}','null'::jsonb) WHERE snapshot_id=$1`, snapshots[1])
	unknown, err := intervals.Intervals(ctx, start, end)
	if err != nil || len(unknown) != 2 || unknown[1].KnownBasis || unknown[1].NetPnL != "" {
		t.Fatalf("unknown basis: %+v %v", unknown, err)
	}
	filtered, err := RankIntervals(ctx, intervals, start, end)
	if err != nil || len(filtered.Entries) != 1 || filtered.ExcludedUnknownBasis != 1 {
		t.Fatalf("ranked known basis: %+v %v", filtered, err)
	}
	coverageExec(t, s.DB, `UPDATE pnl.inventory_l16_wallet_intervals_v3 SET payload=jsonb_set(payload,'{realizedPnlUsdlRaw}','"invalid"'::jsonb) WHERE snapshot_id=$1`, snapshots[1])
	if _, err := intervals.Intervals(ctx, start, end); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid raw net: %v", err)
	}
	coverageExec(t, s.DB, `UPDATE pnl.inventory_l16_wallet_intervals_v3 SET payload=jsonb_set(payload,'{realizedPnlUsdlRaw}','"-1000000"'::jsonb) WHERE snapshot_id=$1`, snapshots[1])
	coverageExec(t, s.DB, `UPDATE pnl.inventory_l16_wallet_intervals_v3 SET payload=jsonb_set(payload,'{fees,0,asset,address}','"0x4444444444444444444444444444444444444444"'::jsonb) WHERE snapshot_id=$1`, snapshots[1])
	unvalued, err := intervals.Intervals(ctx, start, end)
	if err != nil || unvalued[1].Reconciled || unvalued[1].Fees != "" || len(unvalued[1].QualityFlags) != 1 {
		t.Fatalf("unvalued fee: %+v %v", unvalued, err)
	}
	filtered, err = RankIntervals(ctx, intervals, start, end)
	if err != nil || len(filtered.Entries) != 1 || filtered.ExcludedUnqualified != 1 {
		t.Fatalf("ranked qualified: %+v %v", filtered, err)
	}
	if _, _, err := (SQLIntervalAuthority{}).CurrentAttribution(ctx, accounts[0], pool); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing attribution authority: %v", err)
	}
	if _, err := s.SquadPerformance(ctx, nil, q.ID, start, end); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing interval authority: %v", err)
	}
}

func TestDiscoveryCalloutAndConsumerAuthority(t *testing.T) {
	s, a := coverageSetup(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	actor, id := uuid.NewString(), uuid.NewString()
	coverageExec(t, s.DB, `INSERT INTO coverage_accounts VALUES($1)`, actor)
	coverageExec(t, s.DB, `INSERT INTO coverage_pools VALUES($1)`, pool)
	for _, body := range []string{"First observation", "Updated observation"} {
		coverageExec(t, s.DB, `INSERT INTO coverage_reviews VALUES($1,$2,$3,$4,1,true,$5)`, actor, pool, body, id, now)
	}
	service := CalloutService{Store: s, Authority: a}
	callout := Callout{ID: id, AuthorAccountID: actor, PoolAddress: pool, Body: "First observation", Direction: "neutral", Disclosure: "organic", ModerationStatus: statusApproved, CreatedAt: now, Revision: 1}
	if err := (CalloutService{Store: s}).Publish(ctx, actor, "", callout); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing authority: %v", err)
	}
	if err := service.Publish(ctx, "unverified", "", callout); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unverified author: %v", err)
	}
	uncanonical := callout
	uncanonical.PoolAddress = "0x3333333333333333333333333333333333333333"
	if err := service.Publish(ctx, actor, "", uncanonical); !errors.Is(err, ErrInvalid) {
		t.Fatalf("uncanonical pool: %v", err)
	}
	wrongRevision := callout
	wrongRevision.Revision = 2
	if err := service.Publish(ctx, actor, "", wrongRevision); !errors.Is(err, ErrForbidden) {
		t.Fatalf("initial revision: %v", err)
	}
	coverageExec(t, s.DB, `INSERT INTO coverage_reviews VALUES($1,$2,'Hidden analysis',$3,1,false,$4)`, actor, pool, id, now)
	hidden := callout
	hidden.Body = "Hidden analysis"
	if err := service.Publish(ctx, actor, "", hidden); !errors.Is(err, ErrForbidden) {
		t.Fatalf("hidden publication: %v", err)
	}
	markup := callout
	markup.Body = "<script>"
	coverageExec(t, s.DB, `INSERT INTO coverage_reviews VALUES($1,$2,$3,$4,1,true,$5)`, actor, pool, markup.Body, id, now)
	if err := service.Publish(ctx, actor, "", markup); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid publication body: %v", err)
	}
	if err := service.Publish(ctx, actor, uuid.NewString(), callout); !errors.Is(err, ErrForbidden) {
		t.Fatalf("nonmember publication: %v", err)
	}
	if err := service.Publish(ctx, actor, "", callout); err != nil {
		t.Fatal(err)
	}
	read, err := service.Read(ctx, id)
	if err != nil || read != callout {
		t.Fatalf("published callout: %+v %v", read, err)
	}
	owned := OwnedSources{External: a, Callouts: service}
	public, err := owned.ResolvePublic(ctx, kindCallout, id)
	if err != nil || public.Body != callout.Body || public.Ref.SourceRevision != 1 {
		t.Fatalf("public callout: %+v %v", public, err)
	}
	if _, err := service.Edit(ctx, "another-account", id, "Updated observation", 1, false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign edit: %v", err)
	}
	changed, err := service.Edit(ctx, actor, id, "Updated observation", 1, false)
	if err != nil || changed.Revision != 2 || changed.Body != "Updated observation" {
		t.Fatalf("updated callout: %+v %v", changed, err)
	}
	public, err = owned.ResolvePublic(ctx, kindCallout, id)
	if err != nil || public.Body != changed.Body || public.Ref.SourceRevision != 2 || !public.Ref.Corrected {
		t.Fatalf("updated public callout: %+v %v", public, err)
	}
	if _, err := service.Edit(ctx, actor, id, "stale", 1, false); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale edit: %v", err)
	}
	removed, err := service.Edit(ctx, actor, id, "", 2, true)
	if err != nil || !removed.Deleted || removed.Body != "" || removed.Revision != 3 {
		t.Fatalf("removed callout: %+v %v", removed, err)
	}
	if _, err := owned.ResolvePublic(ctx, kindCallout, id); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("removed public callout: %v", err)
	}
	if _, err := service.Edit(ctx, actor, id, "Updated observation", 3, false); !errors.Is(err, ErrForbidden) {
		t.Fatalf("removed edit: %v", err)
	}

	resourceID := uuid.NewString()
	ref := SourceRef{ID: resourceID, EventID: uuid.NewString(), Kind: "pool", CreatorPublicID: actor, Pools: []string{pool}, SourceRevision: 1, VisibilityRevision: 1, PublishedAt: now, Public: true, Authorized: true}
	resource := PublicResource{Ref: ref, Body: "Public pool", Language: "en", Topics: []string{"research"}}
	coverageExec(t, s.DB, `INSERT INTO coverage_resources VALUES($1,$2,$3)`, ref.Kind, ref.ID, encode(resource))
	event := PublicEvent{Consumer: "pool-index", EventID: ref.EventID, Source: ref, ObservedAt: now}
	if err := s.Consume(ctx, a, event); err != nil {
		t.Fatal(err)
	}
	stored, err := s.Source(ctx, ref.ID)
	if err != nil || !sameSource(stored, ref) {
		t.Fatalf("consumed source: %+v %v", stored, err)
	}
	if err := s.Consume(ctx, a, event); err != nil {
		t.Fatalf("idempotent consumption: %v", err)
	}
	ref.SourceRevision++
	ref.EventID = uuid.NewString()
	if err := s.Consume(ctx, a, PublicEvent{Consumer: event.Consumer, EventID: ref.EventID, Source: ref, ObservedAt: now}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unverified source revision: %v", err)
	}
	resource.Ref = ref
	coverageExec(t, s.DB, `UPDATE coverage_resources SET document=$3 WHERE kind=$1 AND id=$2`, ref.Kind, ref.ID, encode(resource))
	if err := s.Consume(ctx, a, PublicEvent{Consumer: event.Consumer, EventID: ref.EventID, Source: ref, ObservedAt: now}); err != nil {
		t.Fatalf("updated source: %v", err)
	}
	stored, err = s.Source(ctx, ref.ID)
	if err != nil || stored.SourceRevision != 2 {
		t.Fatalf("updated stored source: %+v %v", stored, err)
	}
	ref.Public, ref.Authorized, ref.Tombstone = false, false, true
	ref.SourceRevision++
	ref.VisibilityRevision++
	ref.EventID = uuid.NewString()
	if err := s.Consume(ctx, a, PublicEvent{Consumer: event.Consumer, EventID: ref.EventID, Source: ref, ObservedAt: now}); err != nil {
		t.Fatalf("tombstone: %v", err)
	}
	stored, err = s.Source(ctx, ref.ID)
	if err != nil || !stored.Tombstone {
		t.Fatalf("stored tombstone: %+v %v", stored, err)
	}
	ok, err := owned.CanonicalPool(ctx, pool)
	if err != nil || !ok {
		t.Fatalf("canonical pool: %t %v", ok, err)
	}
	ok, err = owned.VerifiedAccount(ctx, actor)
	if err != nil || !ok {
		t.Fatalf("verified account: %t %v", ok, err)
	}
	decision, err := owned.ReviewCallout(ctx, actor, pool, "First observation")
	if err != nil || !decision.Visible || decision.Revision != 1 {
		t.Fatalf("review: %+v %v", decision, err)
	}
	if _, err := (OwnedSources{}).CanonicalPool(ctx, pool); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing pool authority: %v", err)
	}
	if _, err := (OwnedSources{}).VerifiedAccount(ctx, actor); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing account authority: %v", err)
	}
	if _, err := (OwnedSources{}).ResolvePublic(ctx, "pool", id); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing resource authority: %v", err)
	}
	if _, err := (OwnedSources{}).ReviewCallout(ctx, actor, pool, "First observation"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing review authority: %v", err)
	}
	if err := s.EraseAttribution(ctx, actor); err != nil {
		t.Fatal(err)
	}
	stored, err = s.Source(ctx, ref.ID)
	if err != nil || stored.CreatorPublicID != "" {
		t.Fatalf("erased attribution: %+v %v", stored, err)
	}
}

func TestDiscoveryConnectedTransferAndHTTPCallout(t *testing.T) {
	s, a := coverageSetup(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, recipient := uuid.NewString(), uuid.NewString()
	q := Squad{ID: uuid.NewString(), OwnerAccountID: owner, Name: "Research", Policy: "invite", Visibility: "private", Revision: 1}
	if err := s.CreateSquad(ctx, owner, q); err != nil {
		t.Fatal(err)
	}
	token := "01234567890123456789012345678901"
	if err := s.Invite(ctx, owner, q.ID, recipient, token, 0, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RedeemInvitation(ctx, recipient, q.ID, token, 1, now); err != nil {
		t.Fatal(err)
	}
	h := HTTP{Store: s, Callouts: CalloutService{Store: s, Authority: a}, Authenticate: func(r *http.Request) (string, error) { return r.Header.Get("X-Account"), nil }}
	handler := h.Handler()
	request := func(actor, method, path string, body any, want int) []byte {
		t.Helper()
		var raw []byte
		if body != nil {
			raw = encode(body)
		}
		req := httptest.NewRequest(method, path, bytes.NewReader(raw))
		req.Header.Set("X-Account", actor)
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, res.Code, want, res.Body.String())
		}
		return res.Body.Bytes()
	}
	request(owner, "POST", "/v2/discovery/squads/"+q.ID+"/transfer", map[string]any{"Recipient": recipient, "Expected": 1}, http.StatusOK)
	request(owner, "POST", "/v2/discovery/squads/"+q.ID+"/transfer", map[string]any{"Confirm": true}, http.StatusForbidden)
	request(recipient, "POST", "/v2/discovery/squads/"+q.ID+"/transfer", map[string]any{"Confirm": true}, http.StatusOK)
	read, err := s.Squad(ctx, q.ID)
	if err != nil || read.OwnerAccountID != recipient || read.Revision != 2 {
		t.Fatalf("new owner: %+v %v", read, err)
	}
	coverageExec(t, s.DB, `INSERT INTO coverage_accounts VALUES($1)`, recipient)
	coverageExec(t, s.DB, `INSERT INTO coverage_pools VALUES($1)`, pool)
	id := uuid.NewString()
	coverageExec(t, s.DB, `INSERT INTO coverage_reviews VALUES($1,$2,'Published analysis',$3,1,true,$4)`, recipient, pool, id, now)
	coverageExec(t, s.DB, `INSERT INTO coverage_reviews VALUES($1,$2,'Revised analysis',$3,1,true,$4)`, recipient, pool, id, now)
	created := request(recipient, "POST", "/v2/discovery/callouts", map[string]any{"ID": id, "PoolAddress": pool, "Body": "Published analysis", "Direction": "neutral", "Disclosure": "organic"}, http.StatusOK)
	var callout Callout
	if err := json.Unmarshal(created, &callout); err != nil || callout.ID != id || callout.Revision != 1 {
		t.Fatalf("created callout: %s %v", created, err)
	}
	changed := request(recipient, "PATCH", "/v2/discovery/callouts/"+id, map[string]any{"Body": "Revised analysis", "Expected": 1}, http.StatusOK)
	if err := json.Unmarshal(changed, &callout); err != nil || callout.Body != "Revised analysis" || callout.Revision != 2 {
		t.Fatalf("revised callout: %s %v", changed, err)
	}
	request(owner, "PATCH", "/v2/discovery/callouts/"+id, map[string]any{"Delete": true, "Expected": 2}, http.StatusForbidden)
	request(recipient, "PATCH", "/v2/discovery/callouts/"+id, map[string]any{"Delete": true, "Expected": 2}, http.StatusOK)
	stored, err := (CalloutService{Store: s}).Read(ctx, id)
	if err != nil || !stored.Deleted || stored.Body != "" {
		t.Fatalf("deleted callout: %+v %v", stored, err)
	}
}

func TestDiscoveryHTTPRejectsInvalidRequests(t *testing.T) {
	h := HTTP{Authenticate: func(r *http.Request) (string, error) { return r.Header.Get("X-Account"), nil }, Wallet: func(*http.Request) (string, error) { return pool, nil }}
	handler := h.Handler()
	id := uuid.NewString()
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/v2/discovery/feed?limit=invalid", "", http.StatusBadRequest},
		{http.MethodGet, "/v2/discovery/feed?order=unknown", "", http.StatusBadRequest},
		{http.MethodGet, "/v2/discovery/leaderboards?start=invalid&end=invalid", "", http.StatusBadRequest},
		{http.MethodGet, "/v2/discovery/leaderboards?start=2026-01-01T00:00:00Z&end=invalid", "", http.StatusBadRequest},
		{http.MethodPost, "/v2/discovery/callouts", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPatch, "/v2/discovery/callouts/" + id, `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/me/alerts", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodDelete, "/v2/me/alerts/invalid", "", http.StatusBadRequest},
		{http.MethodPut, "/v2/discovery/alerts/" + id, `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/squads", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPut, "/v2/squads/" + id + "/members/" + id, `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/me/squad-invitations/" + id + "/accept", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/discovery/squads", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPatch, "/v2/discovery/squads/" + id, `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPatch, "/v2/discovery/squads/" + id + "/members/" + id, `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/discovery/squads/" + id + "/invitations", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/discovery/squads/" + id + "/accept", `{"unknown":true}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/discovery/squads/" + id + "/transfer", `{"unknown":true}`, http.StatusBadRequest},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("X-Account", id)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != tc.status {
				t.Fatalf("status=%d want=%d body=%s", res.Code, tc.status, res.Body.String())
			}
		})
	}
	for _, path := range []string{"/v2/discovery/feed", "/v2/discovery/feed/" + id} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer invalid")
		res := httptest.NewRecorder()
		handler.ServeHTTP(res, req)
		if res.Code != http.StatusUnauthorized {
			t.Fatalf("unauthorized feed: %s %d", path, res.Code)
		}
	}
}

func TestDiscoveryModelRejectsInvalidProjections(t *testing.T) {
	now := time.Now().UTC()
	ref := SourceRef{ID: "resource", EventID: "event", Kind: "pool", Pools: []string{pool}, SourceRevision: 1, VisibilityRevision: 1, PublishedAt: now, Public: true, Authorized: true}
	for _, change := range []func(*SourceRef){
		func(s *SourceRef) { s.ID = "" },
		func(s *SourceRef) { s.Kind = "unknown" },
		func(s *SourceRef) { s.Pools = []string{"invalid"} },
		func(s *SourceRef) { s.VisibilityRevision = 0 },
	} {
		bad := ref
		change(&bad)
		if !errors.Is(bad.Validate(), ErrInvalid) {
			t.Fatalf("accepted source: %+v", bad)
		}
		if _, err := Candidates([]SourceRef{bad}, now, now, 1); !errors.Is(err, ErrInvalid) {
			t.Fatalf("candidate validation: %v", err)
		}
	}
	if _, err := Candidates([]SourceRef{ref}, now.Add(time.Second), now, 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("future visibility: %v", err)
	}
	badRef := ref
	badRef.ID = "other"
	if _, err := ApplySource(ref, badRef); !errors.Is(err, ErrConflict) {
		t.Fatalf("foreign source: %v", err)
	}

	rule := AlertRule{ID: "rule", OwnerAccountID: "account", PoolAddress: pool, Metric: metricPrice, Threshold: "10", Hysteresis: "1", Direction: "crosses_below", Cooldown: time.Minute, Channels: []string{"inapp"}, Revision: 1}
	for _, change := range []func(*AlertRule){
		func(r *AlertRule) { r.ID = "" },
		func(r *AlertRule) { r.Direction = "unknown" },
		func(r *AlertRule) { r.Threshold = "invalid" },
		func(r *AlertRule) { r.Hysteresis = "invalid" },
		func(r *AlertRule) { r.Channels = nil },
		func(r *AlertRule) { r.Channels = []string{"unknown"} },
	} {
		bad := rule
		change(&bad)
		if !errors.Is(bad.Validate(), ErrInvalid) {
			t.Fatalf("accepted rule: %+v", bad)
		}
	}
	metric := Metric{Value: "11", Revision: 1, ObservedAt: now, Quality: qualityComplete}
	state, cross, reason, err := Evaluate(rule, AlertCheckpoint{}, metric, now)
	if err != nil || cross || reason != "baseline" || !state.Armed {
		t.Fatalf("below-direction baseline: %+v %t %s %v", state, cross, reason, err)
	}
	metric.Value = "9"
	metric.Revision = 2
	state, cross, reason, err = Evaluate(rule, state, metric, now)
	if err != nil || !cross || reason != "crossed" || state.Armed {
		t.Fatalf("below-direction crossing: %+v %t %s %v", state, cross, reason, err)
	}
	metric.Revision = 3
	metric.Value = "invalid"
	if _, _, _, err = Evaluate(rule, state, metric, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid metric value: %v", err)
	}
	metric.Value = "9"
	metric.Revision = 0
	if _, _, _, err = Evaluate(rule, state, metric, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero metric revision: %v", err)
	}
	metric.Revision = 3
	rule.Disabled = true
	if _, cross, reason, err = Evaluate(rule, state, metric, now); err != nil || cross || reason != "disabled" {
		t.Fatalf("disabled rule: %t %s %v", cross, reason, err)
	}

	callout := Callout{ID: "callout", AuthorAccountID: "account", PoolAddress: pool, Body: "analysis", Direction: "neutral", Disclosure: "organic", ModerationStatus: statusApproved, CreatedAt: now, Revision: 1}
	if err := callout.ValidatePublication("account", pool); err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Callout){
		func(c *Callout) { c.ID = "" },
		func(c *Callout) { c.Direction = "unknown" },
		func(c *Callout) { c.Disclosure = "unknown" },
		func(c *Callout) { c.Body = "<b>analysis</b>" },
		func(c *Callout) { c.Deleted = true },
	} {
		bad := callout
		change(&bad)
		if !errors.Is(bad.ValidatePublication("account", pool), ErrInvalid) {
			t.Fatalf("accepted callout: %+v", bad)
		}
	}
	if !errors.Is(callout.ValidatePublication("other", pool), ErrForbidden) {
		t.Fatal("foreign publication")
	}
}

func TestDiscoverySquadInvitationAndRevisionBoundaries(t *testing.T) {
	s, _ := coverageSetup(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	owner, recipient, outsider := uuid.NewString(), uuid.NewString(), uuid.NewString()
	q := Squad{ID: uuid.NewString(), OwnerAccountID: owner, Name: "Research", Policy: "invite", Visibility: "private", Revision: 1}
	if err := s.CreateSquad(ctx, outsider, q); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign creation: %v", err)
	}
	invalid := q
	invalid.Name = ""
	if err := s.CreateSquad(ctx, owner, invalid); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid squad: %v", err)
	}
	if err := s.CreateSquad(ctx, owner, q); err != nil {
		t.Fatal(err)
	}
	if err := s.CreateSquad(ctx, owner, q); err == nil {
		t.Fatal("duplicate squad created")
	}
	token := "01234567890123456789012345678901"
	if err := s.Invite(ctx, owner, q.ID, recipient, "short", 0, now.Add(time.Hour)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short token: %v", err)
	}
	if err := s.Invite(ctx, owner, q.ID, recipient, token, 0, now.Add(-time.Second)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("expired invitation: %v", err)
	}
	if err := s.Invite(ctx, owner, q.ID, owner, token, 0, now.Add(time.Hour)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("self invitation: %v", err)
	}
	if err := s.Invite(ctx, outsider, q.ID, recipient, token, 0, now.Add(time.Hour)); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign invitation: %v", err)
	}
	if err := s.Invite(ctx, owner, q.ID, recipient, token, 0, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := s.Invite(ctx, owner, q.ID, recipient, token, 0, now.Add(time.Hour)); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale invitation: %v", err)
	}
	if _, err := s.InvitationInbox(ctx, "", "", now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("anonymous inbox: %v", err)
	}
	inbox, err := s.InvitationInbox(ctx, recipient, "", now)
	if err != nil || len(inbox.Items) != 1 || inbox.Items[0].Token != token {
		t.Fatalf("private inbox: %+v %v", inbox, err)
	}
	delivery, err := s.InvitationDelivery(ctx, recipient, q.ID, now)
	if err != nil || delivery.Token != token || delivery.Revision != 1 {
		t.Fatalf("private delivery: %+v %v", delivery, err)
	}
	if _, err := s.InvitationDelivery(ctx, outsider, q.ID, now); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign delivery: %v", err)
	}
	if _, err := s.InvitationDelivery(ctx, recipient, q.ID, now.Add(2*time.Hour)); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expired delivery: %v", err)
	}
	if _, err := s.RedeemInvitation(ctx, recipient, q.ID, "wrong-token-with-at-least-thirty-two-characters", 1, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("wrong token: %v", err)
	}
	if _, err := s.RedeemInvitation(ctx, recipient, q.ID, token, 0, now); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale redemption: %v", err)
	}
	redeemed, err := s.RedeemInvitation(ctx, recipient, q.ID, token, 1, now)
	if err != nil || !redeemed.Membership.CanPublish() || redeemed.Membership.Revision != 2 {
		t.Fatalf("redeemed: %+v %v", redeemed, err)
	}
	if _, err := s.RedeemInvitation(ctx, recipient, q.ID, token, 2, now); !errors.Is(err, ErrForbidden) {
		t.Fatalf("redeemed twice: %v", err)
	}
	inbox, err = s.InvitationInbox(ctx, recipient, "", now)
	if err != nil || len(inbox.Items) != 0 {
		t.Fatalf("consumed inbox: %+v %v", inbox, err)
	}
	if err := s.SetMemberRole(ctx, owner, q.ID, recipient, "owner", 2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unsupported role: %v", err)
	}
	if err := s.SetMemberRole(ctx, outsider, q.ID, recipient, "moderator", 2); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign promotion: %v", err)
	}
	if err := s.SetMemberRole(ctx, owner, q.ID, recipient, "moderator", 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetMemberRole(ctx, owner, q.ID, recipient, "member", 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale role: %v", err)
	}
	if err := s.SetMemberStatus(ctx, owner, q.ID, recipient, "unknown", 3); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unsupported status: %v", err)
	}
	if err := s.SetMemberStatus(ctx, outsider, q.ID, recipient, "removed", 3); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign removal: %v", err)
	}
	if err := s.SetMemberStatus(ctx, owner, q.ID, owner, "removed", 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("owner removal: %v", err)
	}
	if err := s.SetMemberStatus(ctx, owner, q.ID, recipient, "removed", 2); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale removal: %v", err)
	}
	if err := s.SetMemberStatus(ctx, owner, q.ID, recipient, "removed", 3); err != nil {
		t.Fatal(err)
	}
	if err := s.ProposeTransfer(ctx, owner, q.ID, recipient, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("removed recipient transfer: %v", err)
	}
	if _, err := s.RequestJoin(ctx, recipient, q.ID, 4); !errors.Is(err, ErrForbidden) {
		t.Fatalf("join under invitation policy: %v", err)
	}
	if _, err := s.ChangeMembership(ctx, owner, q.ID, recipient, "unknown", 4); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown action: %v", err)
	}
	next := q
	next.Name = "Updated research"
	next.Revision = 2
	if err := s.UpdateSquad(ctx, outsider, next, 1); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign squad edit: %v", err)
	}
	stale := next
	stale.Revision = 3
	if err := s.UpdateSquad(ctx, owner, stale, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("nonsequential edit: %v", err)
	}
	if err := s.UpdateSquad(ctx, owner, next, 1); err != nil {
		t.Fatal(err)
	}
	read, err := s.Squad(ctx, q.ID)
	if err != nil || read.Name != next.Name || read.Revision != 2 {
		t.Fatalf("updated squad: %+v %v", read, err)
	}
}

func TestDiscoveryAlertRulePersistenceBoundaries(t *testing.T) {
	s, _ := coverageSetup(t)
	ctx := context.Background()
	now := time.Now().UTC()
	actor := uuid.NewString()
	r := AlertRule{ID: uuid.NewString(), OwnerAccountID: actor, PoolAddress: pool, Metric: metricPrice, Threshold: "10", Direction: "crosses_above", Channels: []string{"inapp"}, Revision: 1}
	if err := s.SaveRule(ctx, "outsider", r, 0); !errors.Is(err, ErrForbidden) {
		t.Fatalf("foreign rule: %v", err)
	}
	bad := r
	bad.Metric = ""
	if err := s.SaveRule(ctx, actor, bad, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing metric: %v", err)
	}
	if err := s.SaveRule(ctx, actor, r, 1); !errors.Is(err, ErrConflict) {
		t.Fatalf("new rule revision: %v", err)
	}
	if err := s.SaveRule(ctx, actor, r, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SaveRule(ctx, actor, r, 0); err == nil {
		t.Fatal("duplicate rule created")
	}
	update := r
	update.Revision = 2
	update.Threshold = "12"
	if err := s.SaveRule(ctx, actor, update, 0); !errors.Is(err, ErrConflict) {
		t.Fatalf("update expected revision: %v", err)
	}
	if err := s.SaveRule(ctx, actor, update, 1); err != nil {
		t.Fatal(err)
	}
	if err := s.DisableRule(ctx, "outsider", r.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign disable: %v", err)
	}
	if err := s.DisableRule(ctx, actor, r.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DisableRule(ctx, actor, r.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("already disabled: %v", err)
	}
	update.Revision = 4
	update.Disabled = false
	if err := s.SaveRule(ctx, actor, update, 3); err != nil {
		t.Fatalf("re-enabled rule: %v", err)
	}
	cross, reason, err := s.EvaluateRule(ctx, r.ID, Metric{Value: "11", Revision: 1, ObservedAt: now, Quality: qualityComplete}, now)
	if err != nil || cross || reason != "baseline" {
		t.Fatalf("persisted rule baseline: %t %s %v", cross, reason, err)
	}
}

func TestDiscoveryHTTPReportsUnavailableStorage(t *testing.T) {
	id := uuid.NewString()
	h := HTTP{Authenticate: func(r *http.Request) (string, error) { return r.Header.Get("X-Account"), nil }}
	handler := h.Handler()
	rule := AlertRule{ID: id, OwnerAccountID: id, PoolAddress: pool, Metric: metricPrice, Threshold: "10", Direction: "crosses_above", Channels: []string{"inapp"}, Revision: 1}
	for _, tc := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, "/v2/discovery/callouts", map[string]any{"ID": id, "PoolAddress": pool, "Body": "analysis", "Direction": "neutral", "Disclosure": "organic"}},
		{http.MethodPatch, "/v2/discovery/callouts/" + id, map[string]any{"Body": "edit", "Expected": 1}},
		{http.MethodPut, "/v2/discovery/alerts/" + id, map[string]any{"Rule": rule, "Expected": 0}},
		{http.MethodGet, "/v2/squads/" + id, nil},
		{http.MethodPost, "/v2/squads", v2.SquadCreateBody{Name: "Research", Description: "Analysis", Visibility: "private", JoinPolicy: "invite"}},
		{http.MethodPut, "/v2/squads/" + id + "/members/" + id, v2.SquadMembershipBody{Action: "invite", ExpectedRevision: "0"}},
		{http.MethodGet, "/v2/me/squad-invitations", nil},
		{http.MethodGet, "/v2/me/squad-invitations/" + id, nil},
		{http.MethodPatch, "/v2/discovery/squads/" + id + "/members/" + id, map[string]any{"Status": "removed", "Expected": 1}},
		{http.MethodPost, "/v2/discovery/squads/" + id + "/invitations", map[string]any{"Account": uuid.NewString(), "Token": "01234567890123456789012345678901", "Expected": 0, "Expires": time.Now().UTC().Add(time.Hour)}},
		{http.MethodPost, "/v2/discovery/squads/" + id + "/accept", map[string]any{"Token": "01234567890123456789012345678901", "Expected": 1}},
		{http.MethodPost, "/v2/discovery/squads/" + id + "/transfer", map[string]any{"Recipient": uuid.NewString(), "Expected": 1}},
		{http.MethodGet, "/v2/discovery/leaderboards?start=2026-01-01T00:00:00Z&end=2026-01-02T00:00:00Z", nil},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			var raw []byte
			if tc.body != nil {
				raw = encode(tc.body)
			}
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(raw))
			req.Header.Set("X-Account", id)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			if res.Code != http.StatusServiceUnavailable {
				t.Fatalf("status=%d body=%s", res.Code, res.Body.String())
			}
		})
	}
}

func TestDiscoveryCursorAndPoolBoundaries(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	signer := CursorSigner{Key: []byte("01234567890123456789012345678901")}
	valid := Cursor{Snapshot: "snapshot", AccountID: "account", Algorithm: "chronological-v1", ExpiresAt: now.Add(time.Minute)}
	if _, err := (CursorSigner{}).Sign(valid); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short signing key: %v", err)
	}
	if _, err := signer.Sign(Cursor{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty cursor: %v", err)
	}
	for _, token := range []string{"no-dot", "a.b.c", "!.e30", "e30.!", strings.Repeat("a", 4097)} {
		if _, err := signer.Verify(token, "account", 0, 0, now); !errors.Is(err, ErrInvalid) {
			t.Fatalf("malformed cursor %q: %v", token, err)
		}
	}
	if _, err := (CursorSigner{}).Verify("no-dot", "account", 0, 0, now); !errors.Is(err, ErrInvalid) {
		t.Fatalf("short verification key: %v", err)
	}
	if _, err := NewStore(nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing pool: %v", err)
	}
	_, dsn, _ := proofDiscoveryStore(t)
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store, err := NewStore(pool)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := store.DB.Close(); err != nil {
			t.Error(err)
		}
	}()
	if err := store.DB.PingContext(ctx); err != nil {
		t.Fatalf("pooled store: %v", err)
	}
}
