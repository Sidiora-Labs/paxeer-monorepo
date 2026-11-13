package storage_test

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/serve"
	storage "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage/testdata/objects"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed testdata/repair_51_metadata.go
var repair51MetadataSource []byte

//go:embed testdata/objects/objects.go
var repair51ObjectsSource []byte

//go:embed testdata/repair_51_database.mjs
var repair51DatabaseSource []byte

//go:embed testdata/repair_51.package.json
var repair51Package []byte

//go:embed testdata/repair_51.package-lock.json
var repair51PackageLock []byte

type repair51Request struct {
	Operation, DSN, Metadata, Safety, ObjectRoot string
	TargetType, TargetID, Status                 string
	Revision                                     int64
}
type repair51Response struct {
	Error                      string
	AssetID, Hash, Key, Status string
	Revision                   int64
	Body                       []byte
}
type repair51Fixture struct {
	t      *testing.T
	ctx    context.Context
	a      *storage.Authority
	config repair51Request
	input  io.WriteCloser
	output *json.Decoder
	stderr *bytes.Buffer
	router http.Handler
	asset  repair51Response
}

func newRepair51Fixture(t *testing.T) *repair51Fixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	t.Cleanup(cancel)
	dsn := repair51Database(t, ctx)
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	base := "repair51_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	names := []string{base, base + "_metadata", base + "_safety"}
	for _, name := range names {
		if _, err = db.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{name}.Sanitize()); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		for _, name := range names {
			if _, err := db.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{name}.Sanitize()+" CASCADE"); err != nil {
				t.Error(err)
			}
		}
	})
	a, err := storage.NewConnectedAuthority(db, names[0], names[1], names[2])
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, a.SchemaDDL()); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, a.RestrictionReceiptSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, migration := range []struct{ name, from, to string }{
		{"390000.sql", "metadata", names[1]}, {"390001.sql", "metadata", names[1]},
		{"380000.sql", "chat", names[2]}, {"370001.sql", "chat", names[2]},
	} {
		raw, err := os.ReadFile(filepath.Join(root, "migrations", migration.name))
		if err != nil {
			t.Fatal(err)
		}
		// Load the reviewed media schema and its cross-service views without
		// pulling in the unrelated publication/indexer and chat interaction schemas.
		source := string(raw)
		if migration.name == "390001.sql" {
			media, _, found := strings.Cut(source, `CREATE TABLE "metadata"."metadata_publication_intents_v3"`)
			if !found {
				t.Fatal("media migration boundary missing")
			}
			source = media
		}
		if migration.name == "370001.sql" {
			view := ""
			for _, statement := range strings.Split(source, ";") {
				if strings.HasPrefix(strings.TrimSpace(statement), "CREATE VIEW chat.safety_visibility_v2 ") {
					view = statement + ";"
				}
			}
			if view == "" {
				t.Fatal("reviewed safety visibility view missing")
			}
			source = view
		}
		// The reviewed migrations contain both quoted and unquoted schema names.
		sql := strings.ReplaceAll(source, migration.from+".", pgx.Identifier{migration.to}.Sanitize()+".")
		sql = strings.ReplaceAll(sql, `"`+migration.from+`"`, pgx.Identifier{migration.to}.Sanitize())
		tx, err := db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, sql); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
	}
	objectRoot := t.TempDir()
	reader := objects.Store{Root: objectRoot}
	handler, err := serve.NewConnectedApproved(serve.Deps{Buckets: map[string]serve.Reader{"approved": reader}}, a)
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err = handler.RegisterConnectedRoutes(router, "approved"); err != nil {
		t.Fatal(err)
	}
	// A temporary module under metadata's import namespace gives this test access
	// to the actual internal producer without changing either service's module.
	dir := t.TempDir()
	metadataDir := filepath.Join(root, "services/launchpad/metadata")
	module, err := os.ReadFile(filepath.Join(metadataDir, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	moduleName := "github.com/Sidiora-Technologies/KindleLaunch/media/metadata"
	mod := strings.Replace(string(module), "module "+moduleName, "module "+moduleName+"/repair51", 1)
	mod = strings.ReplaceAll(mod, "../../../sdk/launchpad/", filepath.Join(root, "sdk/launchpad")+"/")
	mod += "\nrequire " + moduleName + " v0.0.0\nreplace " + moduleName + " => " + strconv.Quote(metadataDir) + "\n"
	sum, err := os.ReadFile(filepath.Join(metadataDir, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(filepath.Join(dir, "objects"), 0700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"go.mod": []byte(mod), "go.sum": sum, "main.go": repair51MetadataSource, "objects/objects.go": repair51ObjectsSource} {
		if err = os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	command := exec.CommandContext(ctx, "go", "run", "-mod=mod", ".")
	command.Dir = dir
	command.Env = append(os.Environ(), "GOWORK=off")
	var stderr bytes.Buffer
	command.Stderr = &stderr
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		if err := command.Wait(); err != nil {
			t.Errorf("metadata producer: %v\n%s", err, stderr.String())
		}
	})
	f := &repair51Fixture{t: t, ctx: ctx, a: a, input: input, output: json.NewDecoder(bufio.NewReader(output)), stderr: &stderr, router: router,
		config: repair51Request{DSN: dsn, Metadata: names[1], Safety: names[2], ObjectRoot: objectRoot}}
	f.asset = f.call("promote", "", "")
	return f
}
func (f *repair51Fixture) call(operation, targetType, targetID string) repair51Response {
	f.t.Helper()
	request := f.config
	request.Operation = operation
	request.TargetType = targetType
	request.TargetID = targetID
	if err := json.NewEncoder(f.input).Encode(request); err != nil {
		f.t.Fatal(err)
	}
	var result repair51Response
	if err := f.output.Decode(&result); err != nil {
		f.t.Fatalf("metadata producer: %v\n%s", err, f.stderr.String())
	}
	if result.Error != "" {
		f.t.Fatalf("%s: %s\n%s", operation, result.Error, f.stderr.String())
	}
	return result
}
func (f *repair51Fixture) delivery(want int) {
	f.t.Helper()
	result := httptest.NewRecorder()
	f.router.ServeHTTP(result, httptest.NewRequest(http.MethodGet, "/"+f.asset.Key, nil))
	if result.Code != want {
		f.t.Fatalf("delivery status=%d want=%d: %s", result.Code, want, result.Body.String())
	}
	if want == http.StatusOK && !bytes.Equal(result.Body.Bytes(), f.asset.Body) {
		f.t.Fatal("delivery changed metadata's published bytes")
	}
}
func (f *repair51Fixture) decision(targetType, targetID, status string, revision int64) repair51Response {
	f.t.Helper()
	cases := pgx.Identifier{f.config.Safety, "safety_cases"}.Sanitize()
	visibility := pgx.Identifier{f.config.Safety, "safety_visibility"}.Sanitize()
	caseID := uuid.NewString()
	if _, err := f.a.Pool.Exec(f.ctx, "INSERT INTO "+cases+"(id,target_type,target_id,reporter,category,description) VALUES($1,$2,$3,$4,'spam','owned repair qualification') ON CONFLICT(reporter,target_type,target_id) DO UPDATE SET description=EXCLUDED.description RETURNING id", caseID, targetType, targetID, "0x"+strings.Repeat("2", 40)); err != nil {
		f.t.Fatal(err)
	}
	if err := f.a.Pool.QueryRow(f.ctx, "SELECT id::text FROM "+cases+" WHERE target_type=$1 AND target_id=$2", targetType, targetID).Scan(&caseID); err != nil {
		f.t.Fatal(err)
	}
	if _, err := f.a.Pool.Exec(f.ctx, "INSERT INTO "+visibility+"(target_type,target_id,status,revision,case_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(target_type,target_id) DO UPDATE SET status=EXCLUDED.status,revision=EXCLUDED.revision", targetType, targetID, status, revision, caseID); err != nil {
		f.t.Fatal(err)
	}
	v := f.call("visibility", targetType, targetID)
	if v.Status != status || v.Revision != revision {
		f.t.Fatalf("metadata visibility=%+v", v)
	}
	return v
}

func TestRepair_51_initial_lineage_visibility(t *testing.T) {
	f := newRepair51Fixture(t)
	if f.asset.Status != "allowed" || f.asset.Revision != 0 {
		t.Fatalf("initial visibility=%+v", f.asset)
	}
	if err := f.a.ImportApprovedLineage(f.ctx, f.asset.AssetID); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.a.Pool.QueryRow(f.ctx, "SELECT count(*) FROM "+pgx.Identifier{f.a.Schema, "lineage"}.Sanitize()+" WHERE hash=$1", f.asset.Hash).Scan(&count); err != nil || count != 4 {
		t.Fatalf("lineage=%d err=%v", count, err)
	}
	var initial bool
	if err := f.a.Pool.QueryRow(f.ctx, "SELECT initial_visibility FROM "+pgx.Identifier{f.a.Schema, "restrictions"}.Sanitize()+" WHERE hash=$1", f.asset.Hash).Scan(&initial); err != nil || !initial {
		t.Fatalf("initial approval=%t err=%v", initial, err)
	}
	f.delivery(http.StatusOK)
	// Revision one is the first real moderation event, not a replay conflict
	// against the positive FK anchor inserted for initial lineage.
	pool := "0x" + strings.Repeat("1", 40)
	v := f.decision("token", pool, "removed", 1)
	if _, err := f.a.Restrict(f.ctx, f.asset.Hash, v.Revision, "blocked"); err != nil {
		t.Fatal(err)
	}
	var revision int64
	var state string
	if err := f.a.Pool.QueryRow(f.ctx, "SELECT revision,state,initial_visibility FROM "+pgx.Identifier{f.a.Schema, "restrictions"}.Sanitize()+" WHERE hash=$1", f.asset.Hash).Scan(&revision, &state, &initial); err != nil || revision != 1 || state != "blocked" || initial {
		t.Fatalf("first moderation: revision=%d state=%s initial=%t err=%v", revision, state, initial, err)
	}
	f.call("sync", "token", pool)
	f.delivery(http.StatusNotFound)
	v = f.decision("token", pool, "restored", 2)
	f.call("sync", "token", pool)
	if _, err := f.a.Restrict(f.ctx, f.asset.Hash, v.Revision, "approved"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.Restrict(f.ctx, f.asset.Hash, 1, "blocked"); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("stale moderation accepted: %v", err)
	}
	f.delivery(http.StatusOK)

	// Dependency failures retain fail-closed behavior, even for cached bytes.
	if _, err := f.a.Pool.Exec(f.ctx, "DROP VIEW "+pgx.Identifier{f.config.Safety, "safety_visibility_v2"}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if err := f.a.Allowed(f.ctx, f.asset.Key); !errors.Is(err, storage.ErrUnavailable) {
		t.Fatalf("authority outage: %v", err)
	}
	// A lost restriction backup cannot conceal a durable tombstone. Replaying
	// must clear the initial marker before the deleted state is persisted.
	if _, err := f.a.Restrict(f.ctx, f.asset.Hash, 3, "deleted"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.a.Pool.Exec(f.ctx, "UPDATE "+pgx.Identifier{f.a.Schema, "restrictions"}.Sanitize()+" SET revision=1,state='approved',initial_visibility=true WHERE hash=$1", f.asset.Hash); err != nil {
		t.Fatal(err)
	}
	if err := f.a.ReplayTombstones(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.a.Pool.QueryRow(f.ctx, "SELECT revision,state,initial_visibility FROM "+pgx.Identifier{f.a.Schema, "restrictions"}.Sanitize()+" WHERE hash=$1", f.asset.Hash).Scan(&revision, &state, &initial); err != nil || revision != 3 || state != "deleted" || initial {
		t.Fatalf("tombstone replay: revision=%d state=%s initial=%t err=%v", revision, state, initial, err)
	}
	f.delivery(http.StatusNotFound)
}

func TestRepair_51_initial_delivery_visibility(t *testing.T) {
	f := newRepair51Fixture(t)
	// Delivery itself imports real producer lineage; no gateway rows are seeded.
	f.delivery(http.StatusOK)
	pool := "0x" + strings.Repeat("1", 40)
	f.decision("token", pool, "removed", 1)
	f.delivery(http.StatusNotFound)
	f.call("sync", "token", pool)
	f.decision("token", pool, "restored", 2)
	// The authoritative restore cannot bypass the metadata revision fence.
	f.delivery(http.StatusNotFound)
	f.call("sync", "token", pool)
	f.delivery(http.StatusOK)
	f.decision("asset", f.asset.AssetID, "removed", 3)
	f.delivery(http.StatusNotFound)
	f.decision("asset", f.asset.AssetID, "restored", 4)
	f.delivery(http.StatusOK)
	// A token restore must never override an independent asset removal.
	f.decision("asset", f.asset.AssetID, "removed", 5)
	f.decision("token", pool, "restored", 6)
	f.call("sync", "token", pool)
	f.delivery(http.StatusNotFound)
	f.decision("asset", f.asset.AssetID, "restored", 7)
	f.delivery(http.StatusOK)
	if _, err := f.a.Restrict(f.ctx, f.asset.Hash, 8, "deleted"); err != nil {
		t.Fatal(err)
	}
	f.delivery(http.StatusNotFound)
	if err := f.a.ImportApprovedLineage(f.ctx, f.asset.AssetID); !errors.Is(err, storage.ErrRestricted) {
		t.Fatalf("tombstone resurrected: %v", err)
	}
	if _, err := f.a.Restrict(f.ctx, f.asset.Hash, 9, "approved"); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("tombstone reversed: %v", err)
	}
}

func TestRepair_51_initial_explicit_denials(t *testing.T) {
	f := newRepair51Fixture(t)
	pool := "0x" + strings.Repeat("1", 40)
	f.decision("asset", f.asset.AssetID, "removed", 1)
	f.decision("token", pool, "restored", 2)
	f.call("sync", "token", pool)
	// The latest restore on another target cannot conceal an explicit removal.
	if err := f.a.ImportApprovedLineage(f.ctx, f.asset.AssetID); !errors.Is(err, storage.ErrRestricted) {
		t.Fatalf("removed asset imported: %v", err)
	}
	f.delivery(http.StatusNotFound)
	f.decision("asset", f.asset.AssetID, "restored", 3)
	if err := f.a.ImportApprovedLineage(f.ctx, f.asset.AssetID); err != nil {
		t.Fatal(err)
	}
	f.delivery(http.StatusOK)
	// Losing the moderation row cannot turn an already moderated token into
	// an initial approval at revision zero.
	if _, err := f.a.Pool.Exec(f.ctx, "DELETE FROM "+pgx.Identifier{f.config.Safety, "safety_visibility"}.Sanitize()+" WHERE target_type='token' AND target_id=$1", pool); err != nil {
		t.Fatal(err)
	}
	v := f.call("visibility", "token", pool)
	if v.Status != "allowed" || v.Revision != 0 {
		t.Fatalf("metadata no-row contract: %+v", v)
	}
	f.delivery(http.StatusNotFound)
}

func repair51Database(t *testing.T, ctx context.Context) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string][]byte{"package.json": repair51Package, "package-lock.json": repair51PackageLock, "database.mjs": repair51DatabaseSource} {
		if err := os.WriteFile(filepath.Join(dir, name), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	install := exec.CommandContext(ctx, "npm", "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--workspaces=false")
	install.Dir = dir
	if output, err := install.CombinedOutput(); err != nil {
		t.Fatalf("embedded SQL runtime install: %v\n%s", err, output)
	}
	command := exec.CommandContext(ctx, "node", "database.mjs")
	command.Dir = dir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		if err := command.Wait(); err != nil {
			t.Errorf("embedded SQL runtime: %v\n%s", err, stderr.String())
		}
	})
	var dsn string
	if err = json.NewDecoder(output).Decode(&dsn); err != nil {
		t.Fatalf("embedded SQL startup: %v\n%s", err, stderr.String())
	}
	return dsn
}
