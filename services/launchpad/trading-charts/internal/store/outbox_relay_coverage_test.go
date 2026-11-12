package store

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
)

func TestCommittedCandleOutboxPublishesOnce(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	const payload = `{"poolAddress":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","interval":"1m","start":60,"revision":"1"}`
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_outbox(chain_id,pool,interval,start,revision,payload) VALUES('125',$1,'1m',60,1,$2)`, address, payload); err != nil {
		t.Fatal(err)
	}
	nc, stream := internaltest.NewBroker(t, "CHART_OUTBOX", []string{"candle.updated.v2"})
	publisher, err := natsdriver.NewPublisher(nc, "CHART_OUTBOX")
	if err != nil {
		t.Fatal(err)
	}
	if published, err := exact.PublishNextCommitted(ctx, publisher, "candle.updated.v2"); err != nil || !published {
		t.Fatalf("committed candle publication: %t, %v", published, err)
	}
	if published, err := exact.PublishNextCommitted(ctx, publisher, "candle.updated.v2"); err != nil || published {
		t.Fatalf("empty outbox after publication: %t, %v", published, err)
	}
	message, err := stream.GetMsg(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	identity := "candle_" + accounting.HashTuple("chart-outbox-v2", "125", address, "1m", "60", "1")
	var actual, expected map[string]any
	if err = json.Unmarshal(message.Data, &actual); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal([]byte(payload), &expected); err != nil {
		t.Fatal(err)
	}
	if message.Header.Get("Nats-Msg-Id") != identity || !reflect.DeepEqual(actual, expected) {
		t.Fatalf("broker candle identity and payload: %+v", message)
	}
	var pending int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 0 {
		t.Fatalf("published outbox row: %d, %v", pending, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_outbox(chain_id,pool,interval,start,revision,payload) VALUES('125',$1,'1m',60,2,$2)`, address, payload); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE FUNCTION candles.defer_outbox_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$;
CREATE TRIGGER defer_outbox_update BEFORE UPDATE ON candles.chart_outbox FOR EACH ROW EXECUTE FUNCTION candles.defer_outbox_update()`); err != nil {
		t.Fatal(err)
	}
	if published, err := exact.PublishNextCommitted(ctx, publisher, "candle.updated.v2"); published || !errors.Is(err, ErrIntegrity) {
		t.Fatalf("unacknowledged durable outbox update: %t, %v", published, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("outbox row after refused update: %d, %v", pending, err)
	}
	if _, err = pool.Exec(ctx, `DROP TRIGGER defer_outbox_update ON candles.chart_outbox`); err != nil {
		t.Fatal(err)
	}
	if published, err := exact.PublishNextCommitted(ctx, publisher, "candle.updated.v2"); err != nil || !published {
		t.Fatalf("retained outbox row publication: %t, %v", published, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_outbox(chain_id,pool,interval,start,revision,payload) VALUES('125',$1,'1m',60,3,$2)`, address, payload); err != nil {
		t.Fatal(err)
	}
	nc.Close()
	if published, err := exact.PublishNextCommitted(ctx, publisher, "candle.updated.v2"); published || err == nil {
		t.Fatalf("outbox publish without broker: %t, %v", published, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_outbox WHERE published_at IS NULL`).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("outbox row after broker disconnect: %d, %v", pending, err)
	}
}
