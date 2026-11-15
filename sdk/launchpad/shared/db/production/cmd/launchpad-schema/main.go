package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"syscall"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const usage = "usage: launchpad-schema apply --migrations DIR | expected --service SERVICE --runtime-role ROLE --region REGION --out FILE [--migrations DIR] | check --expected FILE --sha256 DIGEST"
const readerVersion int64 = 1

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var regionName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

type registryDocument struct {
	production.MigrationRegistry
	Extensions []struct {
		Extension string `json:"extension"`
		Schema    string `json:"schema"`
	} `json:"extension_prerequisites"`
}

type legacySource struct {
	OriginalVersion       int64  `json:"original_version"`
	Path                  string `json:"path"`
	SourceSHA256          string `json:"source_sha256"`
	UpSHA256              string `json:"up_sha256"`
	StructuralFingerprint string `json:"structural_fingerprint"`
}

type legacyHistory struct {
	Service string         `json:"service"`
	Schema  string         `json:"schema"`
	History string         `json:"history"`
	Sources []legacySource `json:"sources"`
}

func (h legacyHistory) alias(s legacySource) production.LegacyMigrationAlias {
	return production.LegacyMigrationAlias{Service: h.Service, OriginalVersion: s.OriginalVersion, SourceSHA256: s.SourceSHA256, UpSHA256: s.UpSHA256, StructuralFingerprint: s.StructuralFingerprint}
}

type migrationBundle struct {
	registry        registryDocument
	catalog         map[int64]production.MigrationManifest
	roles           []production.RoleManifest
	legacy          []legacyHistory
	sql             map[int64][]byte
	sources         map[string][]byte
	owners          map[string]string
	registryDigest  string
	interfaceDigest string
}

func validDigest(value string) bool {
	raw, err := hex.DecodeString(value)
	return err == nil && len(raw) == 32 && value == strings.ToLower(value)
}

// failure reports a stable operation and SQLSTATE without server messages,
// connection configuration, SQL parameters or authentication details.
func failure(reason string, err error) error {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && regexp.MustCompile(`^[0-9A-Z]{5}$`).MatchString(pgerr.Code) {
		return fmt.Errorf("%s (SQLSTATE %s)", reason, pgerr.Code)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("%s: operation cancelled", reason)
	}
	if errors.Is(err, production.ErrMigrationInProgress) {
		return fmt.Errorf("%s: migration in progress", reason)
	}
	return errors.New(reason)
}

func loadBundle(directory string) (*migrationBundle, error) {
	abs, err := filepath.Abs(directory)
	if err != nil {
		return nil, failure("migrations directory is invalid", err)
	}
	readJSON := func(name string, into any) ([]byte, error) {
		raw, err := os.ReadFile(filepath.Join(abs, name))
		if err != nil {
			return nil, failure(name+" is unavailable", err)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			return nil, failure(name+" is invalid", err)
		}
		return raw, nil
	}
	b := &migrationBundle{sql: map[int64][]byte{}, sources: map[string][]byte{}, owners: map[string]string{}}
	raw, err := readJSON("registry.json", &b.registry)
	if err != nil {
		return nil, err
	}
	b.registryDigest = production.Checksum(raw)
	if _, err := production.LoadMigrationRegistry(raw, b.registryDigest); err != nil {
		return nil, failure("registry.json requires an approved digest and complete dependency order", err)
	}
	b.catalog, err = b.registry.ValidateOrder()
	if err != nil {
		return nil, failure("migration registry order is invalid", err)
	}
	var grants struct {
		SchemaVersion int                       `json:"schema_version"`
		Roles         []production.RoleManifest `json:"roles"`
	}
	raw, err = readJSON("grants.json", &grants)
	if err != nil {
		return nil, err
	}
	if grants.SchemaVersion != 2 || len(grants.Roles) == 0 {
		return nil, failure("grants.json requires version 2 and runtime roles", nil)
	}
	b.roles, b.interfaceDigest = grants.Roles, production.Checksum(raw)
	if _, err := production.ProvisionRolesSQL(b.roles); err != nil {
		return nil, failure("grants.json contains invalid roles or ownership", err)
	}
	for _, m := range b.roles {
		b.owners[m.Schema] = m.OwnerRole
		for _, c := range m.SchemaCapabilities {
			b.owners[c.Schema] = c.OwnerRole
		}
	}
	var legacy struct {
		SchemaVersion int             `json:"schema_version"`
		Histories     []legacyHistory `json:"histories"`
	}
	if _, err := readJSON("legacy-history-aliases.json", &legacy); err != nil {
		return nil, err
	}
	if legacy.SchemaVersion != 2 {
		return nil, failure("legacy aliases require version 2", nil)
	}
	b.legacy = legacy.Histories
	aliases := map[string]production.LegacyMigrationAlias{}
	for _, h := range b.legacy {
		history, err := production.HistoryTable(h.Service, h.Schema)
		if err != nil || history != h.History || b.owners[h.Schema] != h.Schema+"_owner" {
			return nil, failure("legacy history has no reviewed service and owner", err)
		}
		var previous int64
		for _, s := range h.Sources {
			a := h.alias(s)
			key, err := a.Key()
			if err != nil || s.OriginalVersion <= previous {
				return nil, failure("legacy aliases require exact reviewed fingerprints and ordered versions", err)
			}
			if _, exists := aliases[key]; exists {
				return nil, failure("duplicate legacy alias", nil)
			}
			aliases[key], previous = a, s.OriginalVersion
			raw, err := coordinatortest.ReadMigrationSource(filepath.Dir(abs), s.Path, s.SourceSHA256)
			if err != nil {
				return nil, failure("legacy source is unavailable or its checksum differs", err)
			}
			up := []byte(strings.SplitN(string(raw), "-- +goose Down", 2)[0])
			if production.Checksum(up) != s.UpSHA256 {
				return nil, failure("legacy up SQL checksum differs", nil)
			}
			b.sources[s.Path] = raw
		}
	}
	applied := map[int64]string{}
	for _, id := range b.registry.ApplicationOrder {
		m := b.catalog[id]
		if b.owners[m.Schema] != m.Schema+"_owner" {
			return nil, failure("registered schema has no reviewed owner", nil)
		}
		raw, err := coordinatortest.ReadMigrationSource(filepath.Dir(abs), m.Path, m.SHA256)
		if err != nil {
			return nil, failure(fmt.Sprintf("migration %d source is unavailable or its checksum differs", id), err)
		}
		if err := m.Validate(raw, b.catalog, applied, b.interfaceDigest, b.registry.ApprovalDigest, readerVersion); err != nil {
			return nil, failure(fmt.Sprintf("migration %d requires matching approval, interface, reader and safety metadata", id), err)
		}
		if m.TransactionMode != "transactional" {
			return nil, failure("registered migration requires a separately reviewed nontransactional runner", nil)
		}
		for _, a := range m.LegacyPredecessors {
			key, err := a.Key()
			if err != nil || aliases[key] != a {
				return nil, failure("registered legacy predecessor differs from aliases", err)
			}
		}
		b.sql[id], applied[id] = raw, m.SHA256
	}
	for _, extension := range b.registry.Extensions {
		if !identifier.MatchString(extension.Extension) || !identifier.MatchString(extension.Schema) {
			return nil, failure("invalid extension prerequisite", nil)
		}
	}
	return b, nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := run(ctx, os.Args[1:], os.Getenv, os.Stdout, os.Stderr)
	stop()
	os.Exit(code)
}

func run(ctx context.Context, args []string, getenv func(string) string, stdout, stderr io.Writer) int {
	badUsage := func() int { _, _ = fmt.Fprintln(stderr, usage); return 2 }
	if len(args) == 0 {
		return badUsage()
	}
	flags := flag.NewFlagSet("launchpad-schema", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var migrations, service, role, region, out, expected, digest string
	switch args[0] {
	case "apply":
		flags.StringVar(&migrations, "migrations", "", "migration directory")
	case "expected":
		flags.StringVar(&migrations, "migrations", "migrations", "migration directory")
		flags.StringVar(&service, "service", "", "service")
		flags.StringVar(&role, "runtime-role", "", "runtime role")
		flags.StringVar(&region, "region", "", "recovery region")
		flags.StringVar(&out, "out", "", "output file")
	case "check":
		flags.StringVar(&expected, "expected", "", "expected schema file")
		flags.StringVar(&digest, "sha256", "", "document digest")
	default:
		return badUsage()
	}
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		return badUsage()
	}
	if args[0] == "apply" && migrations == "" || args[0] == "expected" && (migrations == "" || service == "" || len(service) > 128 || !identifier.MatchString(role) || !regionName.MatchString(region) || out == "") || args[0] == "check" && (expected == "" || !validDigest(digest)) {
		return badUsage()
	}
	env := "LAUNCHPAD_ADMIN_DATABASE_URL"
	if args[0] == "check" {
		env = "LAUNCHPAD_RUNTIME_DATABASE_URL"
	}
	dsn := getenv(env)
	if dsn == "" {
		_, _ = fmt.Fprintln(stderr, env+" is required")
		return 2
	}
	var err error
	if args[0] == "check" {
		err = check(ctx, dsn, expected, digest)
	} else {
		var bundle *migrationBundle
		bundle, err = loadBundle(migrations)
		if err == nil {
			var conn *pgx.Conn
			conn, err = pgx.Connect(ctx, dsn)
			if err != nil {
				err = failure("administrator connection failed", err)
			} else {
				defer conn.Close(context.Background())
				if args[0] == "apply" {
					err = bundle.apply(ctx, conn)
				} else {
					var sum string
					sum, err = bundle.expected(ctx, conn, service, role, region, out)
					if err == nil {
						_, err = fmt.Fprintln(stdout, sum)
						if err != nil {
							err = failure("digest output failed", err)
						}
					}
				}
			}
		}
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func check(ctx context.Context, dsn, path, digest string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return failure("expected schema file is unavailable", err)
	}
	want, err := production.LoadExpectedSchema(raw, digest)
	if err != nil {
		return failure("expected schema document or digest is invalid", err)
	}
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return failure("runtime connection configuration is invalid", err)
	}
	config.MaxConns, config.MinConns = 1, 0
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return failure("runtime connection failed", err)
	}
	defer pool.Close()
	if err := production.CheckReadiness(ctx, pool, want); err != nil {
		return readinessFailure(err)
	}
	return nil
}

func readinessFailure(err error) error {
	// Only fixed product reasons can be displayed; PostgreSQL errors may contain
	// operator supplied data or credentials in their message and detail fields.
	for _, reason := range []string{"runtime role authority mismatch", "schema attestation mismatch", "applied migration checksum mismatch", "runtime schema ownership forbidden", "structural fingerprint mismatch", "restoration authority mismatch", "required runtime privilege absent", "unexpected whole-table runtime privilege", "unexpected runtime column privilege", "unexpected runtime function privilege", "unexpected runtime sequence privilege", "expected schema absent"} {
		if err.Error() == reason {
			return errors.New(reason)
		}
	}
	return failure("database readiness refused", err)
}

func withCoordinator(ctx context.Context, conn *pgx.Conn, action func() error) error {
	var locked bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", production.CoordinatorKey).Scan(&locked); err != nil {
		return failure("coordinator lock failed", err)
	}
	if !locked {
		return failure("migration in progress", nil)
	}
	// The dedicated connection is closed by the command after this call, which
	// also releases the session lock if a cancelled operation lost its session.
	defer conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", production.CoordinatorKey)
	return action()
}

func execute(ctx context.Context, conn *pgx.Conn, step, sql string, args ...any) error {
	if _, err := conn.Exec(ctx, sql, args...); err != nil {
		return failure(step, err)
	}
	return nil
}

func ensureRole(ctx context.Context, conn *pgx.Conn, role string, owner bool) error {
	var exists, unsafe bool
	err := conn.QueryRow(ctx, "SELECT true,rolsuper OR rolcreatedb OR rolcreaterole OR rolinherit OR rolreplication OR rolbypassrls OR ($2 AND rolcanlogin) FROM pg_roles WHERE rolname=$1", role, owner).Scan(&exists, &unsafe)
	if errors.Is(err, pgx.ErrNoRows) {
		return execute(ctx, conn, "role creation failed", "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS")
	}
	if err != nil {
		return failure("role authority lookup failed", err)
	}
	if unsafe {
		return failure("existing role has unreviewed authority", nil)
	}
	return nil
}

func ensureHistory(ctx context.Context, conn *pgx.Conn, service, schema string) error {
	history, err := production.HistoryTable(service, schema)
	if err != nil {
		return failure("service history is unregistered", err)
	}
	var exists bool
	if err := conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", history).Scan(&exists); err != nil {
		return failure("service history lookup failed", err)
	}
	if exists {
		return nil
	}
	ddl, err := production.HistoryDDL(service, schema)
	if err != nil {
		return failure("service history is unregistered", err)
	}
	return execute(ctx, conn, "service history creation failed", ddl)
}

func (b *migrationBundle) applied(ctx context.Context, conn *pgx.Conn, complete bool) (map[int64]string, error) {
	rows, err := conn.Query(ctx, "SELECT migration_id,service,schema_name,checksum,registry_digest FROM migration_control.applied_migrations")
	if err != nil {
		return nil, failure("migration ledger is unavailable", err)
	}
	defer rows.Close()
	applied := map[int64]string{}
	for rows.Next() {
		var id int64
		var service, schema, checksum, approval string
		if err := rows.Scan(&id, &service, &schema, &checksum, &approval); err != nil {
			return nil, failure("migration ledger is invalid", err)
		}
		m, ok := b.catalog[id]
		if !ok || service != m.Service || schema != m.Schema || checksum != m.SHA256 || !validDigest(approval) {
			return nil, failure("migration ledger differs from the approved registry", nil)
		}
		applied[id] = checksum
	}
	if err := rows.Err(); err != nil {
		return nil, failure("migration ledger read failed", err)
	}
	for id := range applied {
		for _, predecessor := range b.catalog[id].PredecessorIDs {
			if applied[predecessor] != b.catalog[predecessor].SHA256 {
				return nil, failure("migration ledger has an unapplied predecessor", nil)
			}
		}
	}
	if complete && len(applied) != len(b.catalog) {
		return nil, failure("registered migrations are not all applied", nil)
	}
	return applied, nil
}

func (b *migrationBundle) ownerInterfaces(ctx context.Context, conn *pgx.Conn, owner string) error {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return failure("owner interface transaction failed", err)
	}
	defer tx.Rollback(context.Background())
	if err := coordinatortest.GrantPresentOwnerInterfaces(ctx, tx, b.roles, owner); err != nil {
		return failure("owner interface grants failed", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return failure("owner interface commit failed", err)
	}
	return nil
}

func (b *migrationBundle) apply(ctx context.Context, conn *pgx.Conn) error {
	return withCoordinator(ctx, conn, func() error {
		if err := execute(ctx, conn, "public schema revocation failed", "REVOKE CREATE ON SCHEMA public FROM PUBLIC"); err != nil {
			return err
		}
		for _, x := range b.registry.Extensions {
			if err := execute(ctx, conn, "extension prerequisite failed", "CREATE EXTENSION IF NOT EXISTS "+pgx.Identifier{x.Extension}.Sanitize()+" WITH SCHEMA "+pgx.Identifier{x.Schema}.Sanitize()); err != nil {
				return err
			}
		}
		if err := ensureRole(ctx, conn, "migration_control_owner", true); err != nil {
			return err
		}
		var present bool
		if err := conn.QueryRow(ctx, "SELECT to_regnamespace('migration_control') IS NOT NULL").Scan(&present); err != nil {
			return failure("control schema lookup failed", err)
		}
		if !present {
			ddl := production.MigrationControlDDL()
			if err := execute(ctx, conn, "migration control creation failed", ddl[strings.Index(ddl, ";")+1:]); err != nil {
				return err
			}
		}
		applied, err := b.applied(ctx, conn, false)
		if err != nil {
			return err
		}
		schemas := make([]string, 0, len(b.owners))
		for schema := range b.owners {
			schemas = append(schemas, schema)
		}
		sort.Strings(schemas)
		for _, schema := range schemas {
			if err := ensureRole(ctx, conn, b.owners[schema], true); err != nil {
				return err
			}
		}
		for _, h := range b.legacy {
			if err := ensureHistory(ctx, conn, h.Service, h.Schema); err != nil {
				return err
			}
			if err := b.importHistory(ctx, conn, h); err != nil {
				return err
			}
		}
		statements, err := production.ProvisionRolesSQL(b.roles)
		if err != nil {
			return failure("runtime provisioning is invalid", err)
		}
		for _, m := range b.roles {
			if err := ensureRole(ctx, conn, m.RuntimeRole, false); err != nil {
				return err
			}
		}
		for _, statement := range statements {
			if strings.HasPrefix(statement, "CREATE ROLE ") {
				continue
			}
			if err := execute(ctx, conn, "runtime schema provisioning failed", statement); err != nil {
				return err
			}
		}
		database := pgx.Identifier{conn.Config().Database}.Sanitize()
		for _, id := range b.registry.ApplicationOrder {
			m := b.catalog[id]
			if _, done := applied[id]; done {
				continue
			}
			if err := ensureHistory(ctx, conn, m.Service, m.Schema); err != nil {
				return err
			}
			if err := b.ownerInterfaces(ctx, conn, b.owners[m.Schema]); err != nil {
				return err
			}
			owner := pgx.Identifier{b.owners[m.Schema]}.Sanitize()
			if err := execute(ctx, conn, "migration database privilege failed", "GRANT CREATE ON DATABASE "+database+" TO "+owner); err != nil {
				return err
			}
			migrationErr := production.ApplyRegisteredMigration(ctx, conn, m, b.sql[id], b.catalog, b.interfaceDigest, b.registry.ApprovalDigest, readerVersion)
			revokeErr := execute(context.Background(), conn, "migration database privilege revocation failed", "REVOKE CREATE ON DATABASE "+database+" FROM "+owner)
			if migrationErr != nil {
				return failure(fmt.Sprintf("migration %d refused", id), migrationErr)
			}
			if revokeErr != nil {
				return revokeErr
			}
		}
		for _, schema := range schemas {
			quoted := pgx.Identifier{schema}.Sanitize()
			if err := execute(ctx, conn, "public privilege revocation failed", "REVOKE ALL ON ALL TABLES IN SCHEMA "+quoted+" FROM PUBLIC; REVOKE ALL ON ALL SEQUENCES IN SCHEMA "+quoted+" FROM PUBLIC; REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA "+quoted+" FROM PUBLIC; ALTER DEFAULT PRIVILEGES FOR ROLE "+pgx.Identifier{b.owners[schema]}.Sanitize()+" REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC"); err != nil {
				return err
			}
		}
		if err := b.ownerInterfaces(ctx, conn, ""); err != nil {
			return err
		}
		for _, m := range b.roles {
			m.OwnerReadGrants = nil
			grants, err := m.GrantSQL()
			if err != nil {
				return failure("runtime grants are invalid", err)
			}
			for _, sql := range grants {
				if err := execute(ctx, conn, "runtime grants failed", sql); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (b *migrationBundle) importHistory(ctx context.Context, conn *pgx.Conn, h legacyHistory) error {
	for index, s := range h.Sources {
		a := h.alias(s)
		var source, up, fingerprint string
		err := conn.QueryRow(ctx, "SELECT source_checksum,up_checksum,structural_fingerprint FROM migration_control.legacy_baselines WHERE service=$1 AND original_version=$2", h.Service, s.OriginalVersion).Scan(&source, &up, &fingerprint)
		if err == nil {
			if source != s.SourceSHA256 || up != s.UpSHA256 || fingerprint != s.StructuralFingerprint {
				return failure("recorded legacy baseline differs from its alias", nil)
			}
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return failure("legacy baseline lookup failed", err)
		}
		var recorded bool
		if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM "+h.History+" WHERE version_id=$1 AND is_applied)", s.OriginalVersion).Scan(&recorded); err != nil {
			return failure("legacy history lookup failed", err)
		}
		raw := b.sources[s.Path]
		upSQL := []byte(strings.SplitN(string(raw), "-- +goose Down", 2)[0])
		if !recorded {
			tx, err := conn.Begin(ctx)
			if err != nil {
				return failure("legacy transaction failed", err)
			}
			err = func() error {
				defer tx.Rollback(context.Background())
				if index == 0 {
					var nonempty bool
					if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname=$1) OR EXISTS(SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname=$1)", h.Schema).Scan(&nonempty); err != nil {
						return err
					}
					if nonempty {
						return failure("legacy schema requires its original history proof", nil)
					}
				} else {
					actual, err := production.CatalogFingerprint(ctx, tx, h.Schema)
					if err != nil {
						return err
					}
					if actual != h.Sources[index-1].StructuralFingerprint {
						return failure("legacy predecessor structure differs from its alias", nil)
					}
				}
				owner := pgx.Identifier{b.owners[h.Schema]}.Sanitize()
				for _, sql := range []string{"GRANT CREATE ON DATABASE " + pgx.Identifier{conn.Config().Database}.Sanitize() + " TO " + owner, "SET LOCAL lock_timeout='2s'; SET LOCAL ROLE " + owner, string(upSQL), "RESET ROLE", "REVOKE CREATE ON DATABASE " + pgx.Identifier{conn.Config().Database}.Sanitize() + " FROM " + owner} {
					if _, err := tx.Exec(ctx, sql); err != nil {
						return err
					}
				}
				if _, err := tx.Exec(ctx, "INSERT INTO "+h.History+"(version_id,is_applied) VALUES($1,true)", s.OriginalVersion); err != nil {
					return err
				}
				return tx.Commit(ctx)
			}()
			if err != nil {
				return failure("legacy initialization refused", err)
			}
		}
		if err := production.ImportLegacyBaseline(ctx, conn, a, h.Schema, raw, upSQL); err != nil {
			return failure("legacy baseline import refused", err)
		}
	}
	return nil
}

func (b *migrationBundle) expected(ctx context.Context, conn *pgx.Conn, service, role, region, out string) (string, error) {
	var digest string
	err := withCoordinator(ctx, conn, func() error {
		var manifest *production.RoleManifest
		for i := range b.roles {
			if b.roles[i].Service == service && b.roles[i].RuntimeRole == role {
				manifest = &b.roles[i]
				break
			}
		}
		if manifest == nil {
			return failure("service and runtime role are not registered together", nil)
		}
		applied, err := b.applied(ctx, conn, true)
		if err != nil {
			return err
		}
		want := production.ExpectedSchema{Service: service, RuntimeRole: role, RegistryDigest: b.registryDigest, InterfaceDigest: b.interfaceDigest, ReaderVersion: readerVersion, Migrations: applied, Schemas: map[string]string{}, Region: region, Privileges: manifest.Grants}
		tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
		if err != nil {
			return failure("catalog snapshot failed", err)
		}
		err = func() error {
			defer tx.Rollback(context.Background())
			if _, err := tx.Exec(ctx, "SET LOCAL search_path TO public"); err != nil {
				return err
			}
			var actualRegion string
			if err := tx.QueryRow(ctx, "SELECT epoch,region FROM recovery.current_epoch()").Scan(&want.Epoch, &actualRegion); err != nil {
				return err
			}
			if actualRegion != region || want.Epoch < 1 {
				return failure("recovery authority does not match the requested region", nil)
			}
			schemas := map[string]bool{manifest.Schema: true, "recovery": true}
			for _, c := range manifest.SchemaCapabilities {
				schemas[c.Schema] = true
			}
			for _, g := range manifest.Grants {
				if g.Schema != "migration_control" {
					schemas[g.Schema] = true
				}
			}
			for schema := range schemas {
				fingerprint, err := production.CatalogFingerprint(ctx, tx, schema)
				if err != nil {
					return err
				}
				want.Schemas[schema] = fingerprint
			}
			return tx.Commit(ctx)
		}()
		if err != nil {
			return failure("catalog or recovery authority is unavailable or mismatched", err)
		}
		raw, err := json.Marshal(want)
		if err != nil {
			return failure("expected schema encoding failed", err)
		}
		digest = production.Checksum(raw)
		if _, err := production.LoadExpectedSchema(raw, digest); err != nil {
			return failure("expected schema requirements are incomplete", err)
		}
		if err := production.RecordRuntimeReadiness(ctx, conn, want); err != nil {
			return failure("runtime readiness recording refused", err)
		}
		config, err := pgxpool.ParseConfig(conn.Config().ConnString())
		if err != nil {
			return failure("readiness connection configuration is invalid", err)
		}
		config.MaxConns, config.MinConns = 1, 0
		config.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
			_, err := c.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
			return err
		}
		pool, err := pgxpool.NewWithConfig(ctx, config)
		if err != nil {
			return failure("readiness connection failed", err)
		}
		defer pool.Close()
		if err := production.CheckReadiness(ctx, pool, want); err != nil {
			return readinessFailure(err)
		}
		return writeDocument(out, raw)
	})
	return digest, err
}

func writeDocument(path string, raw []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".expected-schema-*")
	if err != nil {
		return failure("expected schema output cannot be created", err)
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(raw); err != nil {
		_ = file.Close()
		return failure("expected schema output failed", err)
	}
	if err := file.Close(); err != nil {
		return failure("expected schema output close failed", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return failure("expected schema output publication failed", err)
	}
	return nil
}
