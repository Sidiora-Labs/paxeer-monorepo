package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/store"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func snapshotSDKRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate SDK source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
}

func snapshotSDKSDK(t *testing.T, endpoint, script string, expected any) {
	t.Helper()
	raw, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	sdk := &url.URL{Scheme: "file", Path: filepath.Join(snapshotSDKRoot(t), "sdk/launchpad/packages/ranking-algo-sdk/src/index.ts")}
	cmd := exec.CommandContext(ctx, "node", "--input-type=module", "-e", `
import assert from 'node:assert/strict';
const { RankingHttpClient, RankingHttpError, rankingCategories } = await import(process.argv[1]);
const client = new RankingHttpClient();
client.configure({ baseUrl: process.argv[2], headers: { 'X-Snapshot-SDK': 'snapshot-sdk-response' } });
const expected = JSON.parse(process.argv[3]);
`+script, sdk.String(), endpoint, string(raw))
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("actual TypeScript SDK / production HTTP handler: %v\n%s", err, output)
	}
}

func snapshotSDKServer(t *testing.T, repository snapshotReader, visibility VisibilityFence) (*httptest.Server, *Snapshots) {
	t.Helper()
	codec, err := ranker.NewCursorCodec([]byte(strings.Repeat("snapshotSDK-cursor-key", 2)))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newSnapshots(repository, nil, codec, visibility, 1)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	RegisterSnapshots(router, handler)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return server, handler
}

// Only persistence and visibility are doubled. The HTTP handler, wire encoder,
// cursor codec and SDK client execute their production implementations.
type snapshotSDKStore struct {
	headers map[string]ranker.Header
	items   []ranker.RankedItem
}

func (s *snapshotSDKStore) Current(_ context.Context, category string) (ranker.Header, error) {
	header, ok := s.headers[category]
	if !ok {
		return ranker.Header{}, pgx.ErrNoRows
	}
	return header, header.Validate()
}
func (s *snapshotSDKStore) Header(ctx context.Context, runID, category string) (ranker.Header, error) {
	header, err := s.Current(ctx, category)
	if err == nil && header.RunID != runID {
		err = pgx.ErrNoRows
	}
	return header, err
}
func (s *snapshotSDKStore) Page(_ context.Context, header ranker.Header, offset int64, limit int) (ranker.StoredPage, error) {
	page := ranker.StoredPage{Header: header, Offset: offset, Items: []ranker.RankedItem{}}
	if header.Validate() != nil || offset < 0 || offset > header.Total || limit < 1 || limit > 100 || header.Total > int64(len(s.items)) {
		return page, ranker.ErrUnavailable
	}
	end := min(offset+int64(limit), header.Total)
	page.Items = append(page.Items, s.items[offset:end]...)
	return page, page.Validate()
}
func (s *snapshotSDKStore) ValidateBoundary(_ context.Context, header ranker.Header, ref ranker.PageReference) error {
	if header.Validate() != nil || ref.RunID != header.RunID || ref.Category != header.Category || ref.InputDigest != header.InputDigest || ref.OutputDigest != header.OutputDigest || ref.Expires != header.CutoffTime+900 || ref.Offset < 1 || ref.Offset >= header.Total || ref.Offset > int64(len(s.items)) {
		return ranker.ErrCursorInvalid
	}
	last := s.items[ref.Offset-1]
	if last.Address != ref.LastAddress || last.Score != ref.LastScore {
		return ranker.ErrCursorInvalid
	}
	return nil
}

type snapshotSDKVisibility struct {
	observed time.Time
}

func (v snapshotSDKVisibility) CurrentRankingRevision(context.Context) (string, time.Time, error) {
	return "moderation-33", v.observed, nil
}

func snapshotSDKMemoryStore(t *testing.T) (*snapshotSDKStore, VisibilityFence) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	repository := &snapshotSDKStore{headers: map[string]ranker.Header{}}
	for i, score := range []string{"0.123456789012345678901234567890", "0.012345678901234567890", "0"} {
		repository.items = append(repository.items, ranker.RankedItem{Address: fmt.Sprintf("0x%040x", i+1), Score: score, Rank: int64(i + 1)})
	}
	for _, category := range ranker.Categories {
		header := ranker.Header{
			RunID: "be0ebdd7-e6e2-4f9a-b856-68e7fb31bf22", Category: category,
			InputDigest: strings.Repeat("a", 64), OutputDigest: strings.Repeat("b", 64),
			AlgorithmVersion: ranker.AlgorithmVersion, CutoffTime: now.Unix(), CompletedAt: now,
			Revision: 33, Total: int64(len(repository.items)),
			Provenance: ranker.Provenance{
				Watermark:      v1.Watermark{ChainId: "125", Height: "125", BlockHash: "0x" + strings.Repeat("c", 64), ProjectionRevision: "33", ObservedAt: now.Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"},
				UniverseDigest: strings.Repeat("d", 64), StatsRevision: "stats-33", ModerationRevision: "moderation-33", ActivityRevision: "activity-33",
				WindowStart: now.Unix() - 3600, WindowEnd: now.Unix(),
			},
		}
		if err := header.Validate(); err != nil {
			t.Fatal(err)
		}
		if err := header.Provenance.Validate(ranker.InputSnapshot{CutoffHeight: 125, CutoffTimeUTC: now.Unix()}); err != nil {
			t.Fatal(err)
		}
		repository.headers[category] = header
	}
	return repository, snapshotSDKVisibility{observed: now}
}

func TestSnapshotSDKSuccessAndContinuation(t *testing.T) {
	repository, visibility := snapshotSDKMemoryStore(t)
	server, _ := snapshotSDKServer(t, repository, visibility)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	type expectedPage struct {
		Snapshot v1.RankingSnapshot `json:"snapshot"`
		Revision string             `json:"revision"`
	}
	expected := map[string]expectedPage{}
	for _, category := range ranker.Categories {
		header, err := repository.Current(ctx, category)
		if err != nil {
			t.Fatalf("in-process %s snapshot: %v", category, err)
		}
		page, err := repository.Page(ctx, header, 0, 1)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := store.WireSnapshot(header, page.Items)
		if err != nil {
			t.Fatal(err)
		}
		expected[category] = expectedPage{snapshot, strconv.FormatInt(header.Revision, 10)}
	}
	header, err := repository.Current(ctx, "trending")
	if err != nil || header.Total < 2 || time.Now().Unix() >= header.CutoffTime+900 {
		t.Fatal("current trending snapshot with at least two entries required", err)
	}
	page, err := repository.Page(ctx, header, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.WireSnapshot(header, page.Items)
	if err != nil {
		t.Fatal(err)
	}
	expected["second"] = expectedPage{second, strconv.FormatInt(header.Revision, 10)}
	snapshotSDKSDK(t, server.URL, `
assert.deepEqual(rankingCategories, ['trending','breakout','volume','gainers','losers','movers','unusual','new']);
let first;
for (const category of rankingCategories) {
  const response = await client.getRankings(category, { limit: 1 });
  assert.deepEqual(response.data.snapshot, expected[category].snapshot);
  assert.equal(response.revision, expected[category].revision);
  assert.deepEqual(response.watermark, response.data.snapshot.watermark);
  assert.match(response.requestId, /^[0-9a-f-]{36}$/);
  assert.ok(Number.isFinite(Date.parse(response.generatedAt)));
  assert.equal(response.quality, 'complete');
  assert.equal(response.data.snapshot.items[0].score, '0.123456789012345678901234567890');
  assert.equal(response.freshness.cutoffHeight, response.watermark.height);
  assert.equal(response.freshness.stale, response.quality === 'stale');
  for (const item of response.data.snapshot.items) assert.equal(typeof item.score, 'string');
  if (category === 'trending') first = response;
}
assert.equal(typeof first.nextCursor, 'string');
const second = await client.getRankings('trending', { limit: 1, cursor: first.nextCursor });
assert.deepEqual(second.data.snapshot, expected.second.snapshot);
assert.equal(second.data.snapshot.snapshotId, first.data.snapshot.snapshotId);
assert.equal(second.revision, first.revision);
assert.equal(second.data.snapshot.items[0].rank, 2);
assert.notEqual(second.data.snapshot.items[0].poolAddress, first.data.snapshot.items[0].poolAddress);
let page = second;
while (page.nextCursor !== null) {
  page = await client.getRankings('trending', { limit: 100, cursor: page.nextCursor });
  assert.equal(page.data.snapshot.snapshotId, first.data.snapshot.snapshotId);
  assert.equal(page.revision, first.revision);
}
assert.equal(page.nextCursor, null);
assert.equal(page.data.snapshot.items[0].rank, 3);
assert.equal(page.data.snapshot.items[0].score, '0');
`, expected)
}

func TestSnapshotSDKExpiry(t *testing.T) {
	repository, visibility := snapshotSDKMemoryStore(t)
	server, handler := snapshotSDKServer(t, repository, visibility)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	header, err := repository.Current(ctx, "trending")
	if err != nil {
		t.Fatal(err)
	}
	header.RunID = "ce0ebdd7-e6e2-4f9a-b856-68e7fb31bf22"
	header.CutoffTime = time.Now().Unix() - 901
	if header.Total < 2 || time.Now().Unix() < header.CutoffTime+900 {
		t.Fatal("expired snapshot with a continuation boundary required")
	}
	page, err := repository.Page(ctx, header, 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	cursor, err := handler.cursor.EncodePage(header, page.Items[0])
	if err != nil {
		t.Fatal(err)
	}
	snapshotSDKSDK(t, server.URL, `
await assert.rejects(client.getRankings('trending', { limit: 1, cursor: expected.cursor }), error => {
  assert.ok(error instanceof RankingHttpError);
  assert.equal(error.status, 410);
  assert.equal(error.error.code, 'SNAPSHOT_EXPIRED');
  assert.equal(error.error.restart, true);
  assert.equal(error.error.retryable, false);
  assert.equal(error.error.retryAfterMs, null);
  assert.equal(error.retryAfter, null);
  assert.deepEqual(error.error.fieldErrors, []);
  assert.ok(error.error.requestId);
  assert.deepEqual(JSON.parse(error.body), error.error);
  return true;
});
const restarted = await client.getRankings('trending', { limit: 1 });
assert.notEqual(restarted.data.snapshot.snapshotId, expected.runId);
assert.equal(restarted.data.snapshot.items[0].rank, 1);
`, map[string]string{"cursor": cursor, "runId": header.RunID})
}

func TestSnapshotSDKUnavailableAndInvalidRequests(t *testing.T) {
	repository := &snapshotSDKStore{headers: map[string]ranker.Header{}}
	server, handler := snapshotSDKServer(t, repository, snapshotSDKVisibility{})
	snapshotSDKSDK(t, server.URL, `
await assert.rejects(client.getRankings('trending'), error => {
  assert.ok(error instanceof RankingHttpError);
  assert.equal(error.status, 503);
  assert.equal(error.error.code, 'RANKING_UNAVAILABLE');
  assert.equal(error.error.retainLastResult, true);
  assert.equal(error.error.retryable, true);
  assert.equal(error.error.retryAfterMs, 10000);
  assert.equal(error.retryAfter, '10');
  assert.deepEqual(JSON.parse(error.body), error.error);
  return true;
});
for (const limit of [0,-1,101,1.5,NaN,Infinity]) await assert.rejects(client.getRankings('trending',{limit}), RangeError);
await assert.rejects(client.getRankings('trending',{offset:0}), /cursor pagination/);
await assert.rejects(client.getRankings('top_volume'), TypeError);
await assert.rejects(client.getRankings('bogus'), TypeError);
await assert.rejects(client.getRankings('trending',{cursor:42}), TypeError);
await assert.rejects(client.getRankings('trending',{cursor:'a'.repeat(2049)}), TypeError);
await assert.rejects(client.getRankings('trending',{cursor:'invalid'}), error => {
  assert.equal(error.status,400); assert.equal(error.error.code,'INVALID_CURSOR'); return true;
});
const invalid = await fetch(process.argv[2] + '/rankings/unsupported');
const invalidBody = await invalid.text();
const error = new RankingHttpError(invalid.status, invalidBody);
assert.equal(error.error.code, 'INVALID_CATEGORY');
assert.deepEqual(error.error.supported, rankingCategories);
const offset = await fetch(process.argv[2] + '/rankings/trending?offset=0');
assert.equal(offset.status,400);
assert.equal((await offset.json()).code,'INVALID_PAGINATION');
const controller = new AbortController(); controller.abort();
await assert.rejects(client.getRankings('trending',{signal:controller.signal}), {name:'AbortError'});
`, nil)
	handler.admission <- struct{}{}
	defer func() { <-handler.admission }()
	snapshotSDKSDK(t, server.URL, `
await assert.rejects(client.getRankings('trending'), error => {
  assert.ok(error instanceof RankingHttpError);
  assert.equal(error.status,503);
  assert.equal(error.error.code,'RANKING_UNAVAILABLE');
  assert.equal(error.error.retainLastResult,true);
  assert.equal(error.error.retryAfterMs,1000);
  assert.equal(error.retryAfter,'1');
  return true;
});
`, nil)
}

func TestSnapshotSDKEmptyAndStale(t *testing.T) {
	repository, visibility := snapshotSDKMemoryStore(t)
	empty := repository.headers["new"]
	empty.Total = 0
	repository.headers["new"] = empty
	stale := repository.headers["trending"]
	stale.CutoffTime -= 120
	stale.CompletedAt = time.Unix(stale.CutoffTime, 0).UTC()
	stale.Provenance.Watermark.ObservedAt = stale.CompletedAt.Format("2006-01-02T15:04:05.000Z")
	stale.Provenance.WindowStart -= 120
	stale.Provenance.WindowEnd -= 120
	repository.headers["trending"] = stale
	server, _ := snapshotSDKServer(t, repository, visibility)
	snapshotSDKSDK(t, server.URL, `
const empty = await client.getRankings('new');
assert.deepEqual(empty.data.snapshot.items, []);
assert.equal(empty.data.snapshot.totalEligible, '0');
assert.equal(empty.data.snapshot.replace, true);
assert.equal(empty.data.snapshot.truncated, false);
assert.equal(empty.nextCursor, null);
assert.equal(empty.quality, 'complete');
const stale = await client.getRankings('trending', {limit: 1});
assert.equal(stale.quality, 'stale');
assert.equal(stale.freshness.stale, true);
assert.equal(stale.freshness.staleReason, 'refresh-delayed');
assert.ok(stale.freshness.ageSeconds >= 120);
assert.equal(stale.freshness.retryAfterSeconds, 10);
assert.equal(stale.data.snapshot.items[0].score, '0.123456789012345678901234567890');
assert.equal(typeof stale.nextCursor, 'string');
`, nil)
}
