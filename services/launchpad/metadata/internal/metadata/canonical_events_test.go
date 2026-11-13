package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
)

func TestCanonicalSourceChecksStoredEventBytes(t *testing.T) {
	ctx := context.Background()
	pool := edgePool(t, "")
	schema := edgeSchema(t, pool, "canonical_events")
	creation := loadCreationFixture(t)
	const stream = "paxeer125-canonical"
	seedCanonical(t, pool, schema, stream, creation)
	source := PostgresCanonicalSource{Schema: schema, CanonicalStreamID: stream}
	position := accounting.SourcePosition{
		ChainID: "125", Height: strconv.FormatUint(creation.Height, 10), BlockHash: creation.BlockHash,
		TransactionHash: creation.TxHash, TransactionIndex: strconv.FormatUint(creation.TxIndex, 10),
		LogIndex: strconv.FormatInt(mustHex(creation.Market.LogIndex), 10),
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	values, err := source.rawEvent(ctx, tx, position, creation.Factory, "MarketCreated")
	if err != nil || !canonicalAddress(values[metadataTokenTarget], creation.Token) {
		t.Fatalf("canonical creation event = %+v, %v", values, err)
	}
	if _, err := source.rawEvent(ctx, tx, position, "invalid", "MarketCreated"); !errors.Is(err, ErrUnrepresentable) {
		t.Fatalf("invalid event emitter = %v", err)
	}
	if _, err := source.rawEvent(ctx, tx, position, creation.Factory, "UnknownEvent"); !errors.Is(err, ErrUnrepresentable) {
		t.Fatalf("unknown event ABI = %v", err)
	}
	missing := position
	missing.LogIndex = "1000"
	if _, err := source.rawEvent(ctx, tx, missing, creation.Factory, "MarketCreated"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing raw event = %v", err)
	}
	originalTopics, err := json.Marshal(creation.Market.Topics)
	if err != nil {
		t.Fatal(err)
	}
	wrongSignature := append([]string(nil), creation.Market.Topics...)
	wrongSignature[0] = "0x" + strings.Repeat("0", 64)
	badAddress := append([]string(nil), creation.Market.Topics...)
	badAddress[1] = "0x1234"
	for _, tc := range []struct {
		name, column      string
		changed, original any
	}{
		{"height", "height", int64(creation.Height + 1), int64(creation.Height)},
		{"emitter", "emitter", "0x" + strings.Repeat("f", 40), creation.Factory},
		{"topic count", "topics", `[]`, string(originalTopics)},
		{"event signature", "topics", mustJSON(t, wrongSignature), string(originalTopics)},
		{"indexed address", "topics", mustJSON(t, badAddress), string(originalTopics)},
		{"event data", "data", []byte{0}, common.FromHex(creation.Market.Data)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			update := "UPDATE " + pgx.Identifier{schema, "indexer_raw_logs"}.Sanitize() + " SET " + tc.column + "=$1 WHERE transaction_hash=$2 AND log_index=$3"
			if _, err := pool.Exec(ctx, update, tc.changed, creation.TxHash, mustHex(creation.Market.LogIndex)); err != nil {
				t.Fatal(err)
			}
			if _, err := source.rawEvent(ctx, tx, position, creation.Factory, "MarketCreated"); !errors.Is(err, ErrUnrepresentable) {
				t.Fatalf("changed %s = %v", tc.name, err)
			}
			if _, err := pool.Exec(ctx, update, tc.original, creation.TxHash, mustHex(creation.Market.LogIndex)); err != nil {
				t.Fatal(err)
			}
		})
	}
	identity, err := source.ResolveTokenInTx(ctx, tx, creation.Token)
	if err != nil || identity.PoolAddress != creation.Pool || identity.Creator != creation.Creator {
		t.Fatalf("canonical identity = %+v, %v", identity, err)
	}
}

func TestCanonicalSourceRejectsDivergentProjection(t *testing.T) {
	ctx := context.Background()
	pool := edgePool(t, "")
	schema := edgeSchema(t, pool, "canonical_projection")
	creation := loadCreationFixture(t)
	const stream = "paxeer125-canonical"
	seedCanonical(t, pool, schema, stream, creation)
	source := PostgresCanonicalSource{Schema: schema, CanonicalStreamID: stream}
	creationTable := pgx.Identifier{schema, "indexer_canonical_creations"}.Sanitize()
	logsTable := pgx.Identifier{schema, "indexer_raw_logs"}.Sanitize()
	completenessTable := pgx.Identifier{schema, "indexer_block_completeness"}.Sanitize()
	wrongTopics := append([]string(nil), creation.Market.Topics...)
	wrongTopics[1] = common.BytesToHash(common.HexToAddress("0x" + strings.Repeat("f", 40)).Bytes()).Hex()
	for _, tc := range []struct {
		name, statement string
		args            []any
	}{
		{"invalid creation body", "UPDATE " + creationTable + ` SET payload='"invalid"'::jsonb`, nil},
		{"zero pool address", "UPDATE " + creationTable + ` SET pool=$1,payload=jsonb_set(payload,'{pool}',to_jsonb($1::text))`, []any{"0x" + strings.Repeat("0", 40)}},
		{"invalid optical address", "UPDATE " + creationTable + ` SET optical=$1,payload=jsonb_set(payload,'{optical}',to_jsonb($1::text))`, []any{"invalid"}},
		{"incomplete canonical block", "UPDATE " + completenessTable + ` SET required_decode_complete=false`, nil},
		{"different creation event", "UPDATE " + logsTable + ` SET topics=$1::jsonb WHERE log_index=$2`, []any{mustJSON(t, wrongTopics), mustHex(creation.Market.LogIndex)}},
		{"missing metadata mirror", "UPDATE " + creationTable + ` SET payload=payload-'metadata'`, nil},
		{"different metadata mirror", "UPDATE " + creationTable + ` SET payload=jsonb_set(payload,'{metadata,name}',to_jsonb('changed'::text))`, nil},
		{"invalid deployment event", "UPDATE " + logsTable + ` SET data=$1 WHERE log_index=$2`, []any{[]byte{0}, mustHex(creation.Deployed.LogIndex)}},
		{"extra creation event bytes", "UPDATE " + logsTable + ` SET data=data||decode(repeat('00',32),'hex') WHERE log_index=$1`, []any{mustHex(creation.Market.LogIndex)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err := tx.Exec(ctx, tc.statement, tc.args...); err != nil {
				t.Fatal(err)
			}
			if _, err := source.ResolveTokenInTx(ctx, tx, creation.Token); !errors.Is(err, ErrUnrepresentable) {
				t.Fatalf("canonical projection = %v", err)
			}
		})
	}
}

func mustJSON(t *testing.T, value any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
