package l18

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/google/uuid"
)

func recordedAwardAuthority(t *testing.T) (*Ledger, *CanonicalAuthority, []accounting.FillV1) {
	t.Helper()
	ctx := context.Background()
	l := storage(t)
	path := filepath.Join("..", "l16", "testdata")
	raw, err := os.ReadFile(filepath.Join(path, "indexer_real_market", "traded.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tables map[string]json.RawMessage
	if err = json.Unmarshal(raw, &tables); err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Payload json.RawMessage `json:"payload"`
	}
	if err = json.Unmarshal(tables["indexer.indexer_accounting_fills"], &rows); err != nil {
		t.Fatal(err)
	}
	if _, err = l.pool.Exec(ctx, `CREATE SCHEMA indexer; CREATE TABLE indexer.indexer_accounting_fills_v1(payload jsonb NOT NULL,source_fact_digest text NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	fills := []accounting.FillV1{}
	for _, row := range rows {
		fill, err := accounting.DecodeFillV1(bytes.ReplaceAll(row.Payload, []byte(`"feeBps":`), []byte(`"feeBPS":`)))
		if err != nil {
			t.Fatal(err)
		}
		digest, err := accounting.FillDigest(fill)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = l.pool.Exec(ctx, `INSERT INTO indexer.indexer_accounting_fills_v1 VALUES($1,$2)`, raw, string(digest)); err != nil {
			t.Fatal(err)
		}
		fills = append(fills, fill)
	}
	headers, err := os.ReadFile(filepath.Join(path, "repair_63_headers.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	byHeight := map[string]json.RawMessage{}
	transcript, err := os.ReadFile(filepath.Join(fixtureRoot(), "services/launchpad/indexer/internal/internaltest/chainfixture/testdata/paxeer125-market.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	compressed, err := gzip.NewReader(bytes.NewReader(transcript))
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	scanner := bufio.NewScanner(io.MultiReader(compressed, bytes.NewReader(headers)))
	scanner.Buffer(make([]byte, 4096), 2<<20)
	for scanner.Scan() {
		var row struct {
			Key    string          `json:"k"`
			Result json.RawMessage `json:"result"`
		}
		if err = json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(row.Key, "eth_getBlockByNumber ") && !strings.HasPrefix(row.Key, "eth_getBlockByHash ") {
			continue
		}
		if len(row.Result) == 0 || string(row.Result) == "null" {
			continue
		}
		var header struct {
			Number string `json:"number"`
		}
		if err = json.Unmarshal(row.Result, &header); err != nil {
			t.Fatal(err)
		}
		byHeight[header.Number] = row.Result
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid RPC request", 400)
			return
		}
		var result json.RawMessage
		switch request.Method {
		case "eth_chainId":
			result = json.RawMessage(`"0x7d"`)
		case "eth_getBlockByNumber":
			var height string
			if len(request.Params) > 0 && json.Unmarshal(request.Params[0], &height) == nil {
				result = byHeight[height]
			}
		}
		if result == nil {
			http.Error(w, "block outside recorded market", 400)
			return
		}
		if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
			t.Error(err)
		}
	}))
	t.Cleanup(server.Close)
	rpc, err := newFixtureRPC(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	latest := fills[len(fills)-1]
	height, err := strconv.ParseUint(latest.Position.Height, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	creator := NewCreator(rpc, time.Now)
	creator.committed = func(context.Context) (Checkpoint, error) { return Checkpoint{height, latest.Position.BlockHash}, nil }
	p := Program{ID: "campaign", Version: "v1", DeploymentID: latest.Provenance.DeploymentID, Asset: USDL, ReferralRaw: "1000", CashbackBPS: 100, CashbackStartHeight: "1", CashbackEndHeight: latest.Position.Height, AwardCapRaw: "1000000", DailyCapRaw: "10000000", BeneficiaryCapRaw: "1000000"}
	authority, err := NewCanonicalAuthority(l.pool, l.schema, creator, []Program{p})
	if err != nil {
		t.Fatal(err)
	}
	return l, authority, fills
}

func TestCanonicalAwardsUseRecordedFillAndConsent(t *testing.T) {
	l, a, fills := recordedAwardAuthority(t)
	ctx := context.Background()
	sell := fills[1]
	provenance := "125:" + sell.Position.TransactionHash + ":" + sell.Position.LogIndex
	read, err := a.Fill(ctx, provenance)
	if err != nil || read.SourceFillID != sell.SourceFillID || read.USDLAmountRaw != sell.USDLAmountRaw {
		t.Fatal(read, err)
	}
	p, err := a.Program("campaign", "v1")
	if err != nil {
		t.Fatal(err)
	}
	seconds, err := strconv.ParseInt(sell.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	amount, err := Cashback(sell.USDLAmountRaw, p.CashbackBPS, p.AwardCapRaw)
	if err != nil {
		t.Fatal(err)
	}
	id, err := CashbackAwardIdentity(p.ID, p.Version, sell.SourceFillID)
	if err != nil {
		t.Fatal(err)
	}
	award := Award{Identity: id, Program: p.ID, Version: p.Version, Day: time.Unix(seconds, 0).UTC().Format("2006-01-02"), Asset: USDL, Beneficiary: *sell.Attribution.Trader, Kind: "cashback", EligibleRaw: amount, Provenance: provenance}
	if err = a.VerifyAward(ctx, award); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*Award)
		want   error
	}{
		{"asset", func(v *Award) { v.Asset = actorA }, ErrInvalid},
		{"day", func(v *Award) { v.Day = "2000-01-01" }, ErrInvalid},
		{"owner", func(v *Award) { v.Beneficiary = actorA }, ErrUnauthorized},
		{"identity", func(v *Award) { v.Identity = "different" }, ErrInvalid},
		{"amount", func(v *Award) { v.EligibleRaw = "1" }, ErrConflict},
		{"kind", func(v *Award) { v.Kind = "points" }, ErrInvalid},
		{"program", func(v *Award) { v.Program = "unknown" }, ErrUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := award
			tc.change(&bad)
			if err := a.VerifyAward(ctx, bad); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(*Program)
		want   error
	}{
		{"deployment", func(p *Program) { p.DeploymentID = "different" }, ErrUnavailable},
		{"activation", func(p *Program) { p.CashbackStartHeight = "" }, ErrUnavailable},
		{"before activation", func(p *Program) { p.CashbackStartHeight = "99999999" }, ErrUnauthorized},
		{"after expiration", func(p *Program) { p.CashbackEndHeight = "1" }, ErrUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := p
			tc.change(&bad)
			a.programs[fingerprint([]string{p.ID, p.Version})] = bad
			defer func() { a.programs[fingerprint([]string{p.ID, p.Version})] = p }()
			if err := a.VerifyAward(ctx, award); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	buy := award
	buy.Provenance = "125:" + fills[0].Position.TransactionHash + ":" + fills[0].Position.LogIndex
	buy.Identity, err = CashbackAwardIdentity(p.ID, p.Version, fills[0].SourceFillID)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.VerifyAward(ctx, buy); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	attribution, err := NewAttribution(l, []byte(strings.Repeat("k", 32)), func() time.Time { return time.Unix(seconds-60, 0) }, p.ID, p.Version)
	if err != nil {
		t.Fatal(err)
	}
	token, err := attribution.IssueClick(actorA, "referral-click")
	if err != nil {
		t.Fatal(err)
	}
	binding, err := attribution.Bind(ctx, uuid.NewString(), award.Beneficiary, true, []string{token})
	if err != nil || binding.Referrer != actorA {
		t.Fatal(binding, err)
	}
	if _, err = attribution.SetConsent(ctx, award.Beneficiary, uuid.NewString(), true); err != nil {
		t.Fatal(err)
	}
	referral := award
	referral.Kind = sourceReferral
	referral.Referred = award.Beneficiary
	referral.Beneficiary = actorA
	referral.EligibleRaw = p.ReferralRaw
	referral.Identity, err = ReferralAwardIdentity(p.ID, p.Version, referral.Referred)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.VerifyAward(ctx, referral); err != nil {
		t.Fatal(err)
	}
	if _, err = attribution.SetConsent(ctx, referral.Referred, uuid.NewString(), false); err != nil {
		t.Fatal(err)
	}
	if err = a.VerifyAward(ctx, referral); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	if _, err = a.Fill(ctx, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = a.Fill(ctx, "125:0x"+strings.Repeat("e", 64)+":0"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err = l.pool.Exec(ctx, `UPDATE indexer.indexer_accounting_fills_v1 SET source_fact_digest=$1`, strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = a.Fill(ctx, provenance); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestRewardProgramConfigurationBounds(t *testing.T) {
	l, a, _ := recordedAwardAuthority(t)
	p, _ := a.Program("campaign", "v1")
	for _, change := range []func(*Program){
		func(p *Program) { p.ID = "invalid id" }, func(p *Program) { p.Asset = actorA }, func(p *Program) { p.CashbackBPS = 10001 }, func(p *Program) { p.ReferralRaw = "-1" }, func(p *Program) { p.CashbackStartHeight = "0" }, func(p *Program) { p.CashbackEndHeight = "0" }, func(p *Program) { p.CashbackStartHeight = "" },
	} {
		bad := p
		change(&bad)
		if _, err := NewCanonicalAuthority(l.pool, l.schema, a.creator, []Program{bad}); !errors.Is(err, ErrInvalid) {
			t.Fatal(bad, err)
		}
	}
	if _, err := NewCanonicalAuthority(l.pool, l.schema, a.creator, []Program{p, p}); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if _, err := NewCanonicalAuthority(nil, l.schema, a.creator, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := (*CanonicalAuthority)(nil).Program("campaign", "v1"); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := CashbackAwardIdentity("invalid id", "v1", "fill"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		schema string
		epoch  int64
		region string
	}{{"bad schema", 1, "region"}, {"recovery", 0, "region"}, {"recovery", 1, "bad region"}} {
		if _, err := PostgresEpochGuard(tc.schema, tc.epoch, tc.region); !errors.Is(err, ErrInvalid) {
			t.Fatal(err)
		}
	}
	guard, err := PostgresEpochGuard("recovery", 1, "region")
	if err != nil {
		t.Fatal(err)
	}
	if err = guard(context.Background(), nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := NewIndexedCreator(context.Background(), a.creator.rpc, nil, "stream", time.Now, ManifestDigest, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}
