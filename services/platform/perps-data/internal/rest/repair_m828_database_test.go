package rest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/services/platform/perps-data/internal/store"
	"github.com/lib/pq"
)

// Each scenario owns a fresh database, so the production schema and migration
// paths execute unchanged and no existing fixture data is removed.
func repair828Database(t *testing.T, scenario string) (*sql.DB, func() *sql.DB) {
	t.Helper()
	raw := os.Getenv("PERPS_DATA_TEST_DATABASE_URL")
	if raw == "" {
		t.Fatal("PERPS_DATA_TEST_DATABASE_URL is required")
	}
	endpoint, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	admin, err := store.Open(raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	name := "repair828_" + scenario + "_" + hex.EncodeToString(suffix[:])
	if _, err := admin.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(name)); err != nil {
		t.Fatal(err)
	}
	var db *sql.DB
	t.Cleanup(func() {
		if db != nil {
			if err := db.Close(); err != nil {
				t.Error(err)
			}
		}
		if _, err := admin.Exec(`DROP DATABASE ` + pq.QuoteIdentifier(name) + ` WITH (FORCE)`); err != nil {
			t.Error(err)
		}
	})
	endpoint.Path = "/" + name
	open := func() *sql.DB {
		t.Helper()
		next, err := store.Open(endpoint.String())
		if err != nil {
			t.Fatal(err)
		}
		return next
	}
	db = open()
	if err := store.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	spotBoardMigrate(t, db)
	return db, func() *sql.DB {
		t.Helper()
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
		db = open()
		if err := store.Migrate(context.Background(), db); err != nil {
			t.Fatal(err)
		}
		return db
	}
}
