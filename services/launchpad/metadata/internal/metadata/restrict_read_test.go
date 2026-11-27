package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/pkg/types"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newRestrictReadStore(t *testing.T) (*ProductionStore, creationFixture) {
	t.Helper()
	ctx := t.Context()
	config, err := pgxpool.ParseConfig(edgeDatabaseURL())
	if err != nil {
		t.Fatal(err)
	}
	config.MaxConns = 1
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	schema := edgeSchema(t, db, "restrict_read")
	store, err := NewProductionStore(db, schema, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()); err != nil {
		t.Fatal(err)
	}
	creation := loadCreationFixture(t)
	if _, err := db.Exec(ctx, "INSERT INTO "+store.table("pools")+"(pool,creator,token) VALUES($1,$2,$3)", creation.Pool, creation.Creator, creation.Token); err != nil {
		t.Fatal(err)
	}
	return store, creation
}

func seedRestrictReadState(t *testing.T, store *ProductionStore, pool string, blocked bool) {
	t.Helper()
	if _, err := store.Pool.Exec(t.Context(), "INSERT INTO "+store.table("documents")+"(pool,moderation_revision,blocked) VALUES($1,3,$2)", pool, blocked); err != nil {
		t.Fatal(err)
	}
}

func assertRestrictReadState(t *testing.T, store *ProductionStore, pool, revision string, blocked bool) {
	t.Helper()
	var storedRevision string
	var storedBlocked bool
	if err := store.Pool.QueryRow(t.Context(), "SELECT moderation_revision::text,blocked FROM "+store.table("documents")+" WHERE pool=$1", pool).Scan(&storedRevision, &storedBlocked); err != nil {
		t.Fatal(err)
	}
	if storedRevision != revision || storedBlocked != blocked {
		t.Fatalf("stored moderation = (%s, %v), want (%s, %v)", storedRevision, storedBlocked, revision, blocked)
	}
}

func restrictReadUnderLock(t *testing.T, store *ProductionStore, pool string, read func(context.Context) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var readerPID uint32
	if err := store.Pool.QueryRow(ctx, "SELECT pg_backend_pid()").Scan(&readerPID); err != nil {
		t.Fatal(err)
	}
	locker, err := pgx.ConnectConfig(ctx, store.Pool.Config().ConnConfig.Copy())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := locker.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	tx, err := locker.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(context.Background()); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			t.Error(err)
		}
	})
	var lockedPool string
	if err := tx.QueryRow(ctx, "SELECT pool FROM "+store.table("pools")+" WHERE pool=$1 FOR UPDATE", pool).Scan(&lockedPool); err != nil {
		t.Fatal(err)
	}
	if lockedPool != pool {
		t.Fatalf("locked pool = %s, want %s", lockedPool, pool)
	}
	if err := tx.QueryRow(ctx, "SELECT pool FROM "+store.table("documents")+" WHERE pool=$1 FOR UPDATE", pool).Scan(&lockedPool); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() { result <- read(ctx) }()
	for {
		select {
		case err := <-result:
			if err != nil {
				t.Fatal(err)
			}
			return
		default:
		}
		var waiting bool
		if err := tx.QueryRow(ctx, "SELECT $1::integer=ANY(pg_blocking_pids($2::integer))", locker.PgConn().PID(), readerPID).Scan(&waiting); err != nil {
			cancel()
			<-result
			t.Fatal(err)
		}
		if waiting {
			cancel()
			<-result
			t.Fatal("read waits for the pool or moderation row lock")
		}
	}
}

func TestRestrictWithoutWriteUnderALock(t *testing.T) {
	store, creation := newRestrictReadStore(t)
	seedRestrictReadState(t, store, creation.Pool, false)
	restrictReadUnderLock(t, store, creation.Pool, func(ctx context.Context) error {
		for _, revision := range []string{"3", "2"} {
			if err := store.Restrict(ctx, creation.Pool, revision, true); err != nil {
				return fmt.Errorf("restrict at revision %s: %w", revision, err)
			}
		}
		return nil
	})
}

func TestRestrictWithoutWriteKeepsTheStoredState(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprintf("blocked=%v", blocked), func(t *testing.T) {
			store, creation := newRestrictReadStore(t)
			seedRestrictReadState(t, store, creation.Pool, blocked)
			var before, after string
			query := "SELECT xmin::text FROM " + store.table("documents") + " WHERE pool=$1"
			if err := store.Pool.QueryRow(t.Context(), query, creation.Pool).Scan(&before); err != nil {
				t.Fatal(err)
			}
			for _, revision := range []string{"3", "2"} {
				if err := store.Restrict(t.Context(), creation.Pool, revision, !blocked); err != nil {
					t.Fatal(err)
				}
			}
			assertRestrictReadState(t, store, creation.Pool, "3", blocked)
			if err := store.Pool.QueryRow(t.Context(), query, creation.Pool).Scan(&after); err != nil {
				t.Fatal(err)
			}
			if before != after {
				t.Fatal("equal or older revision rewrote the stored row")
			}
		})
	}
}

func TestRestrictWithoutWriteStillWritesAHigherRevision(t *testing.T) {
	store, creation := newRestrictReadStore(t)
	seedRestrictReadState(t, store, creation.Pool, false)
	if err := store.Restrict(t.Context(), creation.Pool, "4", true); err != nil {
		t.Fatal(err)
	}
	assertRestrictReadState(t, store, creation.Pool, "4", true)
	if err := store.Restrict(t.Context(), creation.Pool, "5", false); err != nil {
		t.Fatal(err)
	}
	assertRestrictReadState(t, store, creation.Pool, "5", false)
}

func TestRestrictWithoutWriteCreatesTheFirstRow(t *testing.T) {
	for _, revision := range []string{"0", "3"} {
		t.Run(revision, func(t *testing.T) {
			store, creation := newRestrictReadStore(t)
			var exists bool
			if err := store.Pool.QueryRow(t.Context(), "SELECT EXISTS(SELECT 1 FROM "+store.table("documents")+" WHERE pool=$1)", creation.Pool).Scan(&exists); err != nil || exists {
				t.Fatalf("initial moderation row exists = %v: %v", exists, err)
			}
			if err := store.Restrict(t.Context(), creation.Pool, revision, true); err != nil {
				t.Fatal(err)
			}
			assertRestrictReadState(t, store, creation.Pool, revision, true)
		})
	}
}

func TestRestrictWithoutWriteReadOfACoin(t *testing.T) {
	store, creation := newRestrictReadStore(t)
	ctx := t.Context()
	seedRestrictReadState(t, store, creation.Pool, false)
	canonicalSchema := edgeSchema(t, store.Pool, "restrict_canonical")
	const stream = "paxeer125-canonical"
	seedCanonical(t, store.Pool, canonicalSchema, stream, creation)
	if _, err := store.Pool.Exec(ctx, "INSERT INTO "+store.table("v2_documents")+"(pool,revision,document) VALUES($1,1,'{\"description\":\"Coin description\",\"tags\":[],\"socials\":[]}')", creation.Pool); err != nil {
		t.Fatal(err)
	}
	visibilitySchema := edgeSchema(t, store.Pool, "restrict_visibility")
	visibilityTable := pgx.Identifier{visibilitySchema, "safety_visibility"}.Sanitize()
	visibilityView := pgx.Identifier{visibilitySchema, "safety_visibility_v2"}.Sanitize()
	if _, err := store.Pool.Exec(ctx, "CREATE TABLE "+visibilityTable+"(target_type text NOT NULL,target_id text NOT NULL,status text NOT NULL,revision bigint NOT NULL,PRIMARY KEY(target_type,target_id)); CREATE VIEW "+visibilityView+" AS SELECT target_type,target_id,status,revision FROM "+visibilityTable); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Pool.Exec(ctx, "INSERT INTO "+visibilityTable+" VALUES('token',$1,'restored',3)", creation.Pool); err != nil {
		t.Fatal(err)
	}
	media := ConnectedMedia{
		Metadata:   V2ContractHandlers{Store: store, Canonical: PostgresCanonicalSource{Schema: canonicalSchema, CanonicalStreamID: stream}},
		Moderation: PostgresVisibilityAuthority{Pool: store.Pool, Schema: visibilitySchema},
	}
	router := chi.NewRouter()
	NewProductionLegacyReads(media, "https://metadata.example").RegisterRoutes(router)
	var response *httptest.ResponseRecorder
	restrictReadUnderLock(t, store, creation.Pool, func(ctx context.Context) error {
		response = httptest.NewRecorder()
		request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/metadata/"+creation.Token, nil)
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			return fmt.Errorf("coin read = %d: %s", response.Code, response.Body.String())
		}
		return nil
	})
	var record types.PublicMetadata
	if err := json.Unmarshal(response.Body.Bytes(), &record); err != nil {
		t.Fatal(err)
	}
	if record.TokenAddress != creation.Token || record.PoolAddress == nil || *record.PoolAddress != creation.Pool || record.Creator == nil || *record.Creator != creation.Creator || record.Name == nil || *record.Name != creation.Name || record.Symbol == nil || *record.Symbol != creation.Symbol || record.Description == nil || *record.Description != "Coin description" {
		t.Fatalf("coin record = %+v", record)
	}
	assertRestrictReadState(t, store, creation.Pool, "3", false)
}

func TestRestrictWithoutWriteNumericRevision(t *testing.T) {
	store, creation := newRestrictReadStore(t)
	const revision = "1000000000000000000000000000000"
	if err := store.Restrict(t.Context(), creation.Pool, revision, true); err != nil {
		t.Fatal(err)
	}
	restrictReadUnderLock(t, store, creation.Pool, func(ctx context.Context) error {
		return store.Restrict(ctx, creation.Pool, "999999999999999999999999999999", false)
	})
	assertRestrictReadState(t, store, creation.Pool, revision, true)
}
