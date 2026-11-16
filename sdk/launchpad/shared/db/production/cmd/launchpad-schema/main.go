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

const usage = "usage: launchpad-schema approve --migrations DIR | apply --migrations DIR | region --region REGION [--migrations DIR] | expected --service SERVICE --runtime-role ROLE --region REGION --out FILE [--migrations DIR] | check --expected FILE --sha256 DIGEST"
const readerVersion int64 = 1

var identifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
var regionName = regexp.MustCompile(`^[a-z][a-z0-9-]{1,62}$`)

type registryDocument struct {
	production.MigrationRegistry
	State                 string   `json:"state"`
	QualificationRequired []string `json:"qualification_required"`
	PathBase              string   `json:"path_base"`
	Extensions            []struct {
		Extension      string  `json:"extension"`
		Schema         string  `json:"schema"`
		Version        *string `json:"version"`
		RuntimeInstall bool    `json:"runtime_install"`
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
	grantsRaw       []byte
	legacyRaw       []byte
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

func validateRegistryShape(r production.MigrationRegistry) (map[int64]production.MigrationManifest, error) {
	if r.SchemaVersion != 2 || len(r.ApplicationOrder) == 0 || len(r.ApplicationOrder) != len(r.Registrations) {
		return nil, errors.New("invalid migration registry")
	}
	catalog := map[int64]production.MigrationManifest{}
	for _, m := range r.Registrations {
		if _, exists := catalog[m.MigrationID]; exists {
			return nil, errors.New("duplicate migration")
		}
		catalog[m.MigrationID] = m
	}
	seen := map[int64]bool{}
	for _, id := range r.ApplicationOrder {
		m, ok := catalog[id]
		if !ok || seen[id] {
			return nil, errors.New("invalid application order")
		}
		for _, predecessor := range m.PredecessorIDs {
			if !seen[predecessor] {
				return nil, fmt.Errorf("unapplied DAG predecessor %d", predecessor)
			}
		}
		seen[id] = true
	}
	return catalog, nil
}

func loadBundle(directory string) (*migrationBundle, error) {
	return loadBundleMode(directory, false)
}

func loadBundleMode(directory string, approval bool) (*migrationBundle, error) {
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
	if !approval {
		if _, err := production.LoadMigrationRegistry(raw, b.registryDigest); err != nil {
			return nil, failure("registry.json requires an approved digest and complete dependency order", err)
		}
	} else if _, err := validateRegistryShape(b.registry.MigrationRegistry); err != nil {
		return nil, failure("registry.json requires an approved digest and complete dependency order", err)
	}
	if approval {
		b.catalog, err = validateRegistryShape(b.registry.MigrationRegistry)
	} else {
		b.catalog, err = b.registry.ValidateOrder()
	}
	if err != nil {
		return nil, failure("migration registry order is invalid", err)
	}
	if !approval && b.registry.State == "approved" && approvalManifestDigest(b.registry) != b.registry.ApprovalDigest {
		return nil, failure("registry.json approval digest differs from its registrations", nil)
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
	b.grantsRaw = append([]byte(nil), raw...)
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
	b.legacyRaw, err = readJSON("legacy-history-aliases.json", &legacy)
	if err != nil {
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
			if approval {
				key = fmt.Sprintf("%s:%d", a.Service, a.OriginalVersion)
			}
			if (!approval && err != nil) || s.OriginalVersion <= previous {
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
		if !approval {
			if err := m.Validate(raw, b.catalog, applied, b.interfaceDigest, b.registry.ApprovalDigest, readerVersion); err != nil {
				return nil, failure(fmt.Sprintf("migration %d requires matching approval, interface, reader and safety metadata", id), err)
			}
		}
		if m.TransactionMode != "transactional" {
			if m.TransactionMode != "nontransactional" || m.OnlineIndex == nil || m.OnlineIndex.Schema != m.Schema {
				return nil, failure("registered migration requires an exact online index plan", nil)
			}
			statement, err := m.OnlineIndex.CreateSQL()
			if err != nil || statement != string(raw) {
				return nil, failure("registered online index differs from its source", err)
			}
		}
		if approval && (m.Path == "" || !validDigest(m.SHA256) || len(raw) == 0 || production.Checksum(raw) != m.SHA256) {
			return nil, failure(fmt.Sprintf("migration %d source is unavailable or its checksum differs", id), nil)
		}
		for _, a := range m.LegacyPredecessors {
			key, err := a.Key()
			if (!approval && err != nil) || (!approval && aliases[key] != a) {
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
	case "approve", "apply":
		flags.StringVar(&migrations, "migrations", "migrations", "migration directory")
	case "region":
		flags.StringVar(&migrations, "migrations", "migrations", "migration directory")
		flags.StringVar(&region, "region", "", "recovery region")
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
	if (args[0] == "approve" || args[0] == "apply" || args[0] == "region") && migrations == "" || args[0] == "region" && !regionName.MatchString(region) || args[0] == "expected" && (migrations == "" || service == "" || len(service) > 128 || !identifier.MatchString(role) || !regionName.MatchString(region) || out == "") || args[0] == "check" && (expected == "" || !validDigest(digest)) {
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
		if args[0] == "approve" {
			bundle, err = loadBundleMode(migrations, true)
		} else {
			bundle, err = loadBundle(migrations)
		}
		if err == nil {
			var conn *pgx.Conn
			conn, err = pgx.Connect(ctx, dsn)
			if err != nil {
				err = failure("administrator connection failed", err)
			} else {
				defer conn.Close(context.Background())
				if args[0] == "approve" {
					err = bundle.approve(ctx, conn, migrations)
				} else if args[0] == "apply" {
					err = bundle.apply(ctx, conn)
				} else if args[0] == "region" {
					err = bundle.region(ctx, conn, region)
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

func serverMajorVersion(ctx context.Context, conn *pgx.Conn) (int, error) {
	var number int64
	if err := conn.QueryRow(ctx, "SELECT current_setting('server_version_num')::bigint").Scan(&number); err != nil || number < 10000 {
		return 0, failure("server version lookup failed", err)
	}
	return int(number / 10000), nil
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
	major, err := serverMajorVersion(ctx, conn)
	if err != nil {
		return err
	}
	if b.registry.ServerMajorVersion > 0 && major != b.registry.ServerMajorVersion {
		return failure("database server major version differs from the approved registry", nil)
	}
	return b.applyInternal(ctx, conn, false)
}

func (b *migrationBundle) applyInternal(ctx context.Context, conn *pgx.Conn, approving bool) error {
	if !approving {
		major, err := serverMajorVersion(ctx, conn)
		if err != nil {
			return err
		}
		if b.registry.ServerMajorVersion > 0 && major != b.registry.ServerMajorVersion {
			return failure("database server major version differs from the approved registry", nil)
		}
	}
	return withCoordinator(ctx, conn, func() error {
		if approving {
			names := make([]string, 0, len(b.owners)+1)
			for schema := range b.owners {
				names = append(names, schema)
			}
			names = append(names, "recovery", "migration_control")
			var present bool
			if err := conn.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_namespace WHERE nspname=ANY($1::text[]))", names).Scan(&present); err != nil {
				return failure("release schema lookup failed", err)
			}
			if present {
				return failure("approval requires an empty release database", nil)
			}
		}
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
		if err := conn.QueryRow(ctx, "SELECT to_regclass('migration_control.migration_attempts') IS NOT NULL").Scan(&present); err != nil {
			return failure("migration attempts lookup failed", err)
		}
		if !present {
			if err := execute(ctx, conn, "migration attempts creation failed", production.OnlineIndexControlDDL()); err != nil {
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
		for i := range b.legacy {
			h := &b.legacy[i]
			if err := ensureHistory(ctx, conn, h.Service, h.Schema); err != nil {
				return err
			}
			if err := b.importHistory(ctx, conn, h, approving); err != nil {
				return err
			}
		}
		if approving {
			if err := b.finalizeApproval(ctx, conn); err != nil {
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
			var migrationErr error
			if m.TransactionMode == "nontransactional" {
				migrationErr = production.ApplyOnlineIndexMigration(ctx, conn, m, *m.OnlineIndex, b.catalog, b.interfaceDigest, b.registry.ApprovalDigest, readerVersion, nil)
			} else {
				migrationErr = production.ApplyRegisteredMigration(ctx, conn, m, b.sql[id], b.catalog, b.interfaceDigest, b.registry.ApprovalDigest, readerVersion)
			}
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

func (b *migrationBundle) region(ctx context.Context, conn *pgx.Conn, region string) error {
	return withCoordinator(ctx, conn, func() error {
		if _, err := b.applied(ctx, conn, true); err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return failure("recovery region transaction failed", err)
		}
		defer tx.Rollback(context.Background())
		var epoch int64
		var current string
		err = tx.QueryRow(ctx, "SELECT epoch,region FROM recovery.recovery_control WHERE singleton FOR UPDATE").Scan(&epoch, &current)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err := tx.Exec(ctx, "INSERT INTO recovery.recovery_control(singleton,epoch,region,revision,checkpoint_sha256) VALUES(true,1,$1,1,$2)", region, b.registryDigest); err != nil {
				return failure("recovery region recording failed", err)
			}
			return tx.Commit(ctx)
		}
		if err != nil {
			return failure("recovery region lookup failed", err)
		}
		if current != region || epoch < 1 {
			return failure("recovery region differs from the recorded authority", nil)
		}
		return tx.Commit(ctx)
	})
}

func (b *migrationBundle) importHistory(ctx context.Context, conn *pgx.Conn, h *legacyHistory, discovering bool) error {
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
		if discovering && s.StructuralFingerprint == "" {
			tx, err := conn.Begin(ctx)
			if err != nil {
				return failure("legacy structure fingerprint transaction failed", err)
			}
			actual, err := production.CatalogFingerprint(ctx, tx, h.Schema)
			_ = tx.Rollback(context.Background())
			if err != nil {
				return failure("legacy structure fingerprint unavailable", err)
			}
			s.StructuralFingerprint = actual
			h.Sources[index].StructuralFingerprint = actual
			a.StructuralFingerprint = actual
		}
		if err := production.ImportLegacyBaseline(ctx, conn, a, h.Schema, raw, upSQL); err != nil {
			return failure("legacy baseline import refused", err)
		}
	}
	return nil
}

func approvalManifestDigest(doc registryDocument) string {
	copy := doc
	copy.ApprovalDigest = ""
	copy.ServerMajorVersion = 0
	copy.Registrations = append([]production.MigrationManifest(nil), doc.Registrations...)
	for i := range copy.Registrations {
		copy.Registrations[i].ApprovalManifestDigest = ""
	}
	raw, err := json.Marshal(copy)
	if err != nil {
		return ""
	}
	return production.Checksum(raw)
}

func (b *migrationBundle) finalizeApproval(ctx context.Context, conn *pgx.Conn) error {
	major, err := serverMajorVersion(ctx, conn)
	if err != nil {
		return err
	}
	b.registry.ServerMajorVersion = major
	b.registry.State = "approved"
	b.registry.QualificationRequired = []string{}
	byAlias := map[string]string{}
	for _, history := range b.legacy {
		for _, source := range history.Sources {
			byAlias[fmt.Sprintf("%s:%d", history.Service, source.OriginalVersion)] = source.StructuralFingerprint
		}
	}
	for i := range b.registry.Registrations {
		m := &b.registry.Registrations[i]
		m.InterfaceDigest = b.interfaceDigest
		m.ReaderMinVersion = readerVersion
		m.EstimatedLockMS = 2000
		for j := range m.LegacyPredecessors {
			key := fmt.Sprintf("%s:%d", m.LegacyPredecessors[j].Service, m.LegacyPredecessors[j].OriginalVersion)
			fingerprint, ok := byAlias[key]
			if !ok || !validDigest(fingerprint) {
				return failure("legacy structure fingerprint unavailable", nil)
			}
			m.LegacyPredecessors[j].StructuralFingerprint = fingerprint
		}
	}
	b.registry.ApprovalDigest = approvalManifestDigest(b.registry)
	if !validDigest(b.registry.ApprovalDigest) {
		return failure("approval manifest encoding failed", nil)
	}
	for i := range b.registry.Registrations {
		b.registry.Registrations[i].ApprovalManifestDigest = b.registry.ApprovalDigest
	}
	b.catalog, err = validateRegistryShape(b.registry.MigrationRegistry)
	if err != nil {
		return failure("approved migration registry order is invalid", err)
	}
	raw, err := json.Marshal(b.registry)
	if err != nil {
		return failure("approved registry encoding failed", err)
	}
	b.registryDigest = production.Checksum(raw)
	return nil
}

func (b *migrationBundle) approve(ctx context.Context, conn *pgx.Conn, directory string) error {
	if err := b.applyInternal(ctx, conn, true); err != nil {
		return err
	}
	registryRaw, err := json.MarshalIndent(b.registry, "", "  ")
	if err != nil {
		return failure("approved registry encoding failed", err)
	}
	registryRaw = append(registryRaw, '\n')
	var aliases map[string]json.RawMessage
	var histories []map[string]json.RawMessage
	if err := json.Unmarshal(b.legacyRaw, &aliases); err != nil {
		return failure("legacy aliases encoding failed", err)
	}
	if err := json.Unmarshal(aliases["histories"], &histories); err != nil {
		return failure("legacy aliases encoding failed", err)
	}
	for i, history := range b.legacy {
		var sources []map[string]json.RawMessage
		if err := json.Unmarshal(histories[i]["sources"], &sources); err != nil {
			return failure("legacy aliases encoding failed", err)
		}
		for j, source := range history.Sources {
			sources[j]["structural_fingerprint"], err = json.Marshal(source.StructuralFingerprint)
			if err != nil {
				return failure("legacy aliases encoding failed", err)
			}
		}
		histories[i]["sources"], err = json.Marshal(sources)
		if err != nil {
			return failure("legacy aliases encoding failed", err)
		}
	}
	aliases["histories"], err = json.Marshal(histories)
	if err != nil {
		return failure("legacy aliases encoding failed", err)
	}
	legacyRaw, err := json.MarshalIndent(aliases, "", "  ")
	if err != nil {
		return failure("legacy aliases encoding failed", err)
	}
	legacyRaw = append(legacyRaw, '\n')
	return writeDocumentsAtomically([]struct {
		path string
		raw  []byte
	}{{filepath.Join(directory, "registry.json"), registryRaw}, {filepath.Join(directory, "grants.json"), b.grantsRaw}, {filepath.Join(directory, "legacy-history-aliases.json"), legacyRaw}})
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

func writeDocumentsAtomically(documents []struct {
	path string
	raw  []byte
}) error {
	type pending struct{ path, temp string }
	created := make([]pending, 0, len(documents))
	for _, document := range documents {
		file, err := os.CreateTemp(filepath.Dir(document.path), ".launchpad-schema-*")
		if err != nil {
			for _, item := range created {
				_ = os.Remove(item.temp)
			}
			return failure("schema document output cannot be created", err)
		}
		name := file.Name()
		if _, err = file.Write(document.raw); err == nil {
			err = file.Close()
		} else {
			_ = file.Close()
		}
		if err != nil {
			_ = os.Remove(name)
			for _, item := range created {
				_ = os.Remove(item.temp)
			}
			return failure("schema document output failed", err)
		}
		created = append(created, pending{document.path, name})
	}
	for _, item := range created {
		if err := os.Rename(item.temp, item.path); err != nil {
			for _, rest := range created {
				_ = os.Remove(rest.temp)
			}
			return failure("schema document publication failed", err)
		}
	}
	return nil
}
