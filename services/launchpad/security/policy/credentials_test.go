package policy

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
	"time"
)

func TestRealStatsRoleDeniedIdentityInsert(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	ctx := context.Background()
	role := schema + "_stats"
	roleID := pgx.Identifier{role}.Sanitize()
	schemaID := pgx.Identifier{schema}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE ROLE "+roleID+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;GRANT USAGE ON SCHEMA "+schemaID+" TO "+roleID+";CREATE TABLE "+schemaID+".identity_records(id integer PRIMARY KEY);GRANT SELECT ON "+schemaID+".identity_records TO "+roleID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanup, "DROP OWNED BY "+roleID+";DROP ROLE "+roleID); err != nil {
			t.Error(err)
		}
	})
	config := pool.Config().Copy()
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+roleID)
		return err
	}
	limited, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	if err = ValidateDatabaseRole(ctx, limited, schema); err != nil {
		t.Fatal("nonowner rejected", err)
	}
	if _, err = limited.Exec(ctx, "INSERT INTO "+schemaID+".identity_records VALUES(1)"); err == nil {
		t.Fatal("stats role crossed identity boundary")
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM "+schemaID+".identity_records").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	if ValidateDatabaseRole(ctx, pool, schema) == nil {
		t.Fatal("schema owner accepted as application role")
	}
}

func TestCredentialInventoryRejectsUnboundedAuthority(t *testing.T) {
	now := time.Now()
	base := Credential{ID: "stats-runtime", Owner: "stats", Audience: "stats", Kind: "database", Expires: now.Add(time.Minute), Schemas: []string{"stats"}, Subjects: []string{"stats.outbox"}, ObjectPrefixes: []string{"stats/reports/"}}
	if ValidateCredentials([]Credential{base}, now) != nil {
		t.Fatal("bounded credential rejected")
	}
	for _, which := range []string{"wildcard-subject", "traversal-prefix", "upload-delete", "expired", "migration"} {
		value := base
		switch which {
		case "wildcard-subject":
			value.Subjects = []string{">"}
		case "traversal-prefix":
			value.ObjectPrefixes = []string{"stats/../identity/"}
		case "upload-delete":
			value.Kind = "upload"
			value.DeleteObjects = true
		case "expired":
			value.Expires = now
		case "migration":
			value.Migration = true
			value.Expires = now.Add(16 * time.Minute)
		}
		if ValidateCredentials([]Credential{value}, now) == nil {
			t.Fatal("unbounded credential accepted", which)
		}
	}
}

func TestRepairColumnGrantsAndOwnerViews(t *testing.T) {
	pool, owned := realSecurityDatabase(t)
	ctx := context.Background()
	foreign := owned + "_foreign"
	role, inherited := owned+"_reader", owned+"_writer"
	f, r, parent := pgx.Identifier{foreign}.Sanitize(), pgx.Identifier{role}.Sanitize(), pgx.Identifier{inherited}.Sanitize()
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+f+";CREATE TABLE "+f+".private_records(id integer,secret text);INSERT INTO "+f+".private_records VALUES(1,'private');CREATE VIEW "+f+".owner_read AS SELECT id FROM "+f+".private_records;CREATE ROLE "+r+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS INHERIT;CREATE ROLE "+parent+" NOLOGIN;GRANT USAGE ON SCHEMA "+f+" TO "+r+";GRANT SELECT ON "+f+".owner_read TO "+r); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA "+f+" CASCADE;DROP OWNED BY "+r+", "+parent+";DROP ROLE "+r+", "+parent); err != nil {
			t.Error(err)
		}
	})
	config := pool.Config().Copy()
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+r)
		return err
	}
	limited, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	defer limited.Close()
	if err := ValidateDatabaseRole(ctx, limited, owned); err != nil {
		t.Fatal("owner view reader rejected", err)
	}
	var id int
	if err := limited.QueryRow(ctx, "SELECT id FROM "+f+".owner_read").Scan(&id); err != nil || id != 1 {
		t.Fatal("permitted owner-view read", id, err)
	}
	for _, privilege := range []string{"INSERT", "UPDATE"} {
		for _, indirect := range []bool{false, true} {
			recipient := r
			if indirect {
				recipient = parent
				if _, err := pool.Exec(ctx, "GRANT "+parent+" TO "+r); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := pool.Exec(ctx, "GRANT "+privilege+"(secret) ON "+f+".private_records TO "+recipient); err != nil {
				t.Fatal(err)
			}
			var tableRight, columnRight bool
			if err := limited.QueryRow(ctx, "SELECT has_table_privilege(current_user,$1,$2),has_any_column_privilege(current_user,$1,$2)", f+".private_records", privilege).Scan(&tableRight, &columnRight); err != nil || tableRight || !columnRight {
				t.Fatal("fixture must exercise effective column-only rights", tableRight, columnRight, err)
			}
			if err := ValidateDatabaseRole(ctx, limited, owned); err != ErrDenied {
				t.Fatal("foreign column authority accepted", privilege, indirect, err)
			}
			if _, err := pool.Exec(ctx, "REVOKE "+privilege+"(secret) ON "+f+".private_records FROM "+recipient); err != nil {
				t.Fatal(err)
			}
			if indirect {
				if _, err := pool.Exec(ctx, "REVOKE "+parent+" FROM "+r); err != nil {
					t.Fatal(err)
				}
			}
			if err := ValidateDatabaseRole(ctx, limited, owned); err != nil {
				t.Fatal("revoked column grant remained effective", err)
			}
		}
	}
	if _, err := pool.Exec(ctx, "ALTER TABLE "+f+".private_records OWNER TO "+parent+";GRANT "+parent+" TO "+r); err != nil {
		t.Fatal(err)
	}
	if err := ValidateDatabaseRole(ctx, limited, owned); err != ErrDenied {
		t.Fatal("inherited foreign table ownership accepted", err)
	}
}
