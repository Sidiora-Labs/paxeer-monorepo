package feeds

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/records"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/testdb"
)

func TestPriceWindow(t *testing.T) {
	db := testdb.Open(t)
	now := time.Now().UTC().Truncate(time.Millisecond)
	testdb.SeedPrices(t, db, now)
	p, err := NewProviders(context.Background(), db, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, topic := range []string{TopicMarketsAll, "BTC@market", "SOL@market"} {
		p.Subscribed(topic)
		if err := p.Load(context.Background(), topic); err != nil {
			t.Fatal(err)
		}
		raw, ok := p.Held(topic)
		if !ok {
			t.Fatalf("%s has no snapshot", topic)
		}
		checkPriceWindowTopic(t, topic, raw)
	}
	frames := p.tick(context.Background(), now.Add(MarketInterval))
	seen := map[string]bool{}
	for _, frame := range frames {
		switch frame.Topic {
		case TopicMarketsAll, "BTC@market", "SOL@market":
			checkPriceWindowTopic(t, frame.Topic, frame.Data)
			seen[frame.Topic] = true
		}
	}
	if len(seen) != 3 {
		t.Fatalf("market frames: %v", seen)
	}
}

func checkPriceWindowTopic(t *testing.T, topic string, raw []byte) {
	t.Helper()
	var recs []records.MarketRecord
	if topic == TopicMarketsAll {
		if err := json.Unmarshal(raw, &recs); err != nil {
			t.Fatal(err)
		}
		if len(recs) != 18 {
			t.Fatalf("markets %d", len(recs))
		}
	} else {
		var rec records.MarketRecord
		if err := json.Unmarshal(raw, &rec); err != nil {
			t.Fatal(err)
		}
		recs = append(recs, rec)
	}
	for _, rec := range recs {
		switch rec.Symbol {
		case "BTC":
			if rec.Price24h == nil || *rec.Price24h != 83 || rec.Change24h != nil {
				t.Fatalf("%s: %+v", topic, rec)
			}
		case "ETH":
			if rec.Price24h == nil || *rec.Price24h != 85 {
				t.Fatalf("%s: %+v", topic, rec)
			}
		default:
			if rec.Price24h != nil {
				t.Fatalf("%s %s price_24h = %g", topic, rec.Symbol, *rec.Price24h)
			}
		}
	}
}
