package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/pkg/types"
	iface "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCoinTextCanonicalCreation(t *testing.T) {
	ctx := context.Background()
	pool := edgePool(t, "")
	chain := newCoinTextChain(t)
	cacheSchema := edgeSchema(t, pool, "coin_cache")
	cache, err := NewProductionStore(pool, cacheSchema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "CREATE TABLE "+cache.table("development_canonical_tokens")+"(token text PRIMARY KEY,pool text NOT NULL,creator text NOT NULL,tx_hash text NOT NULL,token_json jsonb NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		label, name, symbol, wantName, wantSymbol string
	}{
		{"symbol17", "Coin", strings.Repeat("界", 17), "Coin", strings.Repeat("界", 16)},
		{"name101", strings.Repeat("🪙", 101), "COIN", strings.Repeat("🪙", 100), "COIN"},
		{"empty", "", "", "", ""},
		{"invalidUTF8Name", string([]byte{0xff, 0xfe}), "UTF", "", "UTF"},
		{"invalidUTF8Symbol", "Coin", string([]byte{0xff}), "Coin", ""},
		{"zeroName", "Coin\x00text", "ZERO", "", "ZERO"},
		{"zeroSymbol", "Coin", "C\x00N", "Coin", ""},
		{"whitespace", " \t\n", " ", " \t\n", " "},
		{"displayCharacters", "<Coin>\u202e", "<C>", "<Coin>\u202e", "<C>"},
	} {
		t.Run(tc.label, func(t *testing.T) {
			creation := chain.creation(t, tc.name, tc.symbol)
			row := developmentPoolRow{Token: creation.Token, Pool: creation.Pool, Creator: creation.Creator, TxHash: creation.TxHash, CreatedBlock: int64(creation.Height)}
			for i := 0; i < 2; i++ {
				reads := NewDevelopmentV2Reads(pool, chain.backend.Client(), time.Now, cache)
				token, err := reads.chainToken(ctx, row)
				if err != nil || token.Name != tc.wantName || token.Symbol != tc.wantSymbol {
					t.Fatalf("development token = %+v, %v", token, err)
				}
			}
			schema := edgeSchema(t, pool, "coin_text")
			const stream = "coin-text-canonical"
			seedCanonical(t, pool, schema, stream, creation)
			source := PostgresCanonicalSource{Schema: schema, CanonicalStreamID: stream}
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			identity, err := source.ResolveTokenInTx(ctx, tx, creation.Token)
			if err != nil {
				t.Fatal(err)
			}
			if identity.Token.Name != tc.wantName || identity.Token.Symbol != tc.wantSymbol {
				t.Fatalf("carried name/symbol = %q/%q, want %q/%q", identity.Token.Name, identity.Token.Symbol, tc.wantName, tc.wantSymbol)
			}
			raw, err := json.Marshal(identity.Token)
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := iface.DecodeToken(raw)
			if err != nil || decoded.Name != tc.wantName || decoded.Symbol != tc.wantSymbol || decoded.SourcePosition == nil || decoded.SourcePosition.TransactionHash != creation.TxHash {
				t.Fatalf("shared token = %+v, %v", decoded, err)
			}
			data := V2Data{PoolAddress: creation.Pool, TokenAddress: creation.Token, Revision: "0", Display: V2Display{Tags: []string{}, Socials: []V2Social{}}}
			mapped, err := V2ContractData(data, identity)
			if err != nil {
				t.Fatal(err)
			}
			recorder := httptest.NewRecorder()
			V2ContractHandlers{Clock: time.Now}.respond(recorder, mapped)
			if err := iface.ValidateJSON("MetadataGetResponse", recorder.Body.Bytes()); err != nil {
				t.Fatalf("metadata response: %v: %s", err, recorder.Body.String())
			}
			if _, err := MetadataResponse("11111111-1111-4111-8111-111111111111", creation.Token, decoded.Name, decoded.Symbol, "Coin", "0", nil, nil, nil, nil, time.Now()); err != nil {
				t.Fatalf("bounded metadata response: %v", err)
			}
			// Equality uses the complete indexed text before the display prefix.
			if _, err := tx.Exec(ctx, "UPDATE "+pgx.Identifier{schema, "indexer_canonical_creations"}.Sanitize()+" SET payload=jsonb_set(payload,'{metadata,name}',to_jsonb($1::text))", creation.Name+"different"); err != nil {
				t.Fatal(err)
			}
			if _, err := source.ResolveTokenInTx(ctx, tx, creation.Token); !errors.Is(err, ErrUnrepresentable) {
				t.Fatalf("different indexed text = %v", err)
			}
		})
	}
}

func TestCoinTextMissingMetadataBatch(t *testing.T) {
	ctx := context.Background()
	pool := edgePool(t, "")
	chain := newCoinTextChain(t)
	missing := chain.creation(t, "Unavailable", "NONE")
	ordinary := chain.creation(t, "Ordinary coin", "COIN")
	const stream = "coin-text-canonical"
	schema := edgeSchema(t, pool, "coin_missing")
	other := edgeSchema(t, pool, "coin_ordinary")
	seedCanonical(t, pool, schema, stream, missing)
	seedCanonical(t, pool, other, stream, ordinary)
	for _, table := range []string{"indexer_raw_logs", "indexer_block_completeness", "indexer_canonical_creations"} {
		if _, err := pool.Exec(ctx, "INSERT INTO "+pgx.Identifier{schema, table}.Sanitize()+" SELECT * FROM "+pgx.Identifier{other, table}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, "UPDATE "+pgx.Identifier{schema, "indexer_canonical_creations"}.Sanitize()+" SET payload=payload-'metadata' WHERE token=$1", missing.Token); err != nil {
		t.Fatal(err)
	}
	source := PostgresCanonicalSource{Schema: schema, CanonicalStreamID: stream}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err := source.ResolveTokenInTx(ctx, tx, missing.Token); !errors.Is(err, ErrUnrepresentable) || !errors.Is(err, errCoinMetadataUnavailable) {
		t.Fatalf("creation without metadata = %v", err)
	}
	storeSchema := edgeSchema(t, pool, "coin_documents")
	store, err := NewProductionStore(pool, storeSchema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()); err != nil {
		t.Fatal(err)
	}
	media := ConnectedMedia{Metadata: V2ContractHandlers{Store: store, Canonical: source}}
	for i := 0; i < 2; i++ {
		reads := NewProductionLegacyReads(media, "https://media.example.com")
		if _, err := reads.buildRecords(ctx, []string{missing.Token}, false); !errors.Is(err, ErrUnrepresentable) {
			t.Fatalf("single unavailable metadata = %v", err)
		}
		result, err := reads.build(ctx, []string{missing.Token, ordinary.Token})
		if err != nil {
			t.Fatal(err)
		}
		if result[missing.Token].Name != nil || result[ordinary.Token].Name == nil || *result[ordinary.Token].Name != ordinary.Name || result[ordinary.Token].Symbol == nil || *result[ordinary.Token].Symbol != ordinary.Symbol {
			t.Fatalf("batch metadata = %+v", result)
		}
	}
}

func TestCoinTextCarriedBoundaries(t *testing.T) {
	for _, tc := range []struct {
		value, want string
		limit       int
	}{
		{strings.Repeat("a", 64<<10), strings.Repeat("a", 100), 100},
		{strings.Repeat("a", (64<<10)+1), "", 100},
		{strings.Repeat("界", (64<<10)/3) + "a", strings.Repeat("界", 100), 100},
		{strings.Repeat("界", (64<<10)/3) + "ab", "", 100},
		{"a\u0301b", "a\u0301", 2},
		{"", "", 16},
	} {
		if got := carriedCoinText(tc.value, tc.limit); got != tc.want || !utf8.ValidString(got) {
			t.Fatalf("carried text of %d bytes = %q, want %q", len(tc.value), got, tc.want)
		}
	}
}

func TestCoinTextLegacyStorageAndView(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	queries := sqlcdb.New(pool)
	const token = "0x1111111111111111111111111111111111111111"
	const poolAddress = "0x2222222222222222222222222222222222222222"
	const creator = "0x3333333333333333333333333333333333333333"
	internaltest.SeedPool(t, pool, poolAddress, token, creator)
	handlers := &Handlers{q: queries}
	for i, tc := range []struct{ name, symbol, wantName, wantSymbol string }{
		{strings.Repeat("🪙", 101), strings.Repeat("界", 17), strings.Repeat("🪙", 100), strings.Repeat("界", 16)},
		{"", "", "", ""},
	} {
		metadata := &types.UploadMetadata{Name: tc.name, Symbol: tc.symbol}
		if err := handlers.recordMetadata(ctx, token, poolAddress, creator, metadata, int64(i+1)); err != nil {
			t.Fatal(err)
		}
		stored, err := queries.GetTokenMetadata(ctx, token)
		if err != nil || !stored.Name.Valid || !stored.Symbol.Valid || stored.Name.String != tc.wantName || stored.Symbol.String != tc.wantSymbol {
			t.Fatalf("stored metadata = %+v, %v", stored, err)
		}
		var name, symbol string
		if err := pool.QueryRow(ctx, "SELECT name,symbol FROM metadata.token_search_v1 WHERE token_address=$1", token).Scan(&name, &symbol); err != nil || name != tc.wantName || symbol != tc.wantSymbol {
			t.Fatalf("search text = %q/%q, %v", name, symbol, err)
		}
		result, err := handlers.buildOne(ctx, token, "https://media.example.com")
		if err != nil || result.Name == nil || result.Symbol == nil || *result.Name != tc.wantName || *result.Symbol != tc.wantSymbol {
			t.Fatalf("legacy metadata = %+v, %v", result, err)
		}
	}
	if err := queries.UpsertTokenMetadata(ctx, sqlcdb.UpsertTokenMetadataParams{TokenAddress: token, PoolAddress: poolAddress, Name: pgtype.Text{String: strings.Repeat("界", 101), Valid: true}, Symbol: pgtype.Text{String: strings.Repeat("🪙", 17), Valid: true}, CreatedBy: creator, CreatedAt: 1, UpdatedAt: 3}); err != nil {
		t.Fatal(err)
	}
	stored, err := queries.GetTokenMetadata(ctx, token)
	if err != nil || stored.Name.String != strings.Repeat("界", 100) || stored.Symbol.String != strings.Repeat("🪙", 16) {
		t.Fatalf("query writer text = %+v, %v", stored, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE metadata.token_metadata SET name=$2,symbol=$3 WHERE token_address=$1", token, strings.Repeat("界", 101), strings.Repeat("🪙", 17)); err != nil {
		t.Fatal(err)
	}
	var name, symbol string
	if err := pool.QueryRow(ctx, "SELECT name,symbol FROM metadata.token_search_v1 WHERE token_address=$1", token).Scan(&name, &symbol); err != nil || name != strings.Repeat("界", 100) || symbol != strings.Repeat("🪙", 16) {
		t.Fatalf("existing search text = %q/%q, %v", name, symbol, err)
	}
	result := handlers.assemble(token, nil, map[string]sqlcdb.MetadataTokenMetadatum{token: {Name: pgtype.Text{String: name + "extra", Valid: true}, Symbol: pgtype.Text{String: symbol + "extra", Valid: true}}}, nil, "https://media.example.com")
	if result.Name == nil || *result.Name != name || result.Symbol == nil || *result.Symbol != symbol {
		t.Fatalf("legacy batch text = %+v", result)
	}
}

func TestCoinTextPublicationProjection(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	const schema = "coin_projection"
	if err := EnsureDevelopmentConnectedSchema(ctx, pool, schema); err != nil {
		t.Fatal(err)
	}
	store, err := NewProductionStore(pool, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	media := ConnectedMedia{Metadata: V2ContractHandlers{Store: store}}
	legacy := internaltest.NewStore(t, "coin-text-projection")
	queries := sqlcdb.New(pool)
	const token = "0x4444444444444444444444444444444444444444"
	const poolAddress = "0x5555555555555555555555555555555555555555"
	const creator = "0x6666666666666666666666666666666666666666"
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,token,creator) VALUES($1,$2,$3)", poolAddress, token, creator); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "INSERT INTO "+store.table("v2_documents")+"(pool) VALUES($1)", poolAddress); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, symbol, wantName, wantSymbol string }{
		{strings.Repeat("界", 101), strings.Repeat("🪙", 17), strings.Repeat("界", 100), strings.Repeat("🪙", 16)},
		{"", "", "", ""},
	} {
		document := mustJSON(t, map[string]any{"name": tc.name, "symbol": tc.symbol, "tags": []string{}, "socials": []V2Social{}})
		if _, err := pool.Exec(ctx, "UPDATE "+store.table("v2_documents")+" SET revision=revision+1,document=$2::jsonb WHERE pool=$1", poolAddress, document); err != nil {
			t.Fatal(err)
		}
		for i := 0; i < 2; i++ {
			if err := media.ProjectDevelopmentLegacy(ctx, queries, legacy, time.Now); err != nil {
				t.Fatal(err)
			}
			stored, err := queries.GetTokenMetadata(ctx, token)
			if err != nil || !stored.Name.Valid || !stored.Symbol.Valid || stored.Name.String != tc.wantName || stored.Symbol.String != tc.wantSymbol {
				t.Fatalf("published text = %+v, %v", stored, err)
			}
		}
	}
}
