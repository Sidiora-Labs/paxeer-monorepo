package move

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/migrate"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/ceremony/internal/rehearsal"
)

const walletsTable = "wallets"

var (
	ErrConfig     = errors.New("move: invalid configuration")
	ErrNotMoved   = errors.New("move: target does not hold the moved ledger")
	ErrDeltaClash = errors.New("move: delta rows clash with the target")
)

type Options struct {
	SourceURL     string
	TargetURL     string
	MigrationsDir string
}

func LoadOptions(getenv func(string) string) (Options, error) {
	get := func(name string) string { return strings.TrimSpace(getenv(name)) }
	o := Options{SourceURL: get(rehearsal.EnvSourceURL), TargetURL: get(migrate.EnvDatabaseURL), MigrationsDir: get(rehearsal.EnvGatewayMigrationsDir)}
	for _, v := range []struct{ name, value string }{
		{rehearsal.EnvSourceURL, o.SourceURL},
		{migrate.EnvDatabaseURL, o.TargetURL},
		{rehearsal.EnvGatewayMigrationsDir, o.MigrationsDir},
	} {
		if v.value == "" {
			return Options{}, fmt.Errorf("%w: %s is not set", ErrConfig, v.name)
		}
	}
	return o, nil
}

func Move(ctx context.Context, o Options) (rehearsal.CopyReport, error) {
	if o.SourceURL == "" || o.TargetURL == "" || o.MigrationsDir == "" {
		return rehearsal.CopyReport{}, ErrConfig
	}
	return rehearsal.CopySource(ctx, o.SourceURL, o.TargetURL, o.MigrationsDir)
}

type DeltaTable struct {
	Table        string
	SourceRows   int64
	TargetBefore int64
	Moved        int64
	Conflicts    int64
	Collisions   int64
	SourceDigest [32]byte
	TargetDigest [32]byte
}

type DroppedTable struct {
	Table      string
	SourceRows int64
}

type DeltaReport struct {
	Tables  []DeltaTable
	Dropped []DroppedTable
}

func (r DeltaReport) Moved() int64 {
	var n int64
	for _, t := range r.Tables {
		n += t.Moved
	}
	return n
}

func (r DeltaReport) Lines() []string {
	out := make([]string, 0, len(r.Tables)+1)
	for _, t := range r.Tables {
		out = append(out, fmt.Sprintf("table=%s source_rows=%d target_rows_before=%d moved=%d conflicts=%d collisions=%d moved_source_sha256=%x moved_target_sha256=%x",
			t.Table, t.SourceRows, t.TargetBefore, t.Moved, t.Conflicts, t.Collisions, t.SourceDigest, t.TargetDigest))
	}
	for _, t := range r.Dropped {
		out = append(out, fmt.Sprintf("table=%s source_rows=%d dropped_by_gateway_migration=true", t.Table, t.SourceRows))
	}
	return append(out, fmt.Sprintf("tables=%d dropped=%d moved=%d", len(r.Tables), len(r.Dropped), r.Moved()))
}

func (r DeltaReport) clashes() []string {
	var out []string
	for _, t := range r.Tables {
		if t.Conflicts > 0 || t.Collisions > 0 {
			out = append(out, fmt.Sprintf("%s conflicts %d collisions %d", t.Table, t.Conflicts, t.Collisions))
		}
	}
	return out
}

func MoveDelta(ctx context.Context, o Options) (DeltaReport, error) {
	var report DeltaReport
	if o.SourceURL == "" || o.TargetURL == "" {
		return report, ErrConfig
	}
	src, err := rehearsal.OpenSource(ctx, o.SourceURL)
	if err != nil {
		return report, err
	}
	defer src.Close()
	counts, err := src.Counts(ctx)
	if err != nil {
		return report, fmt.Errorf("%w: %v", rehearsal.ErrSource, err)
	}
	tgt, err := rehearsal.Connect(ctx, o.TargetURL)
	if err != nil {
		return report, fmt.Errorf("%w: connect: %v", rehearsal.ErrTarget, err)
	}
	defer tgt.Close(context.Background())
	later, err := checkMoved(ctx, tgt, src.Ledger)
	if err != nil {
		return report, err
	}
	present, err := targetTables(ctx, tgt, src.Tables, later)
	if err != nil {
		return report, err
	}
	kept := make([]rehearsal.Table, 0, len(src.Tables))
	for _, table := range src.Tables {
		if present[table.Name] {
			kept = append(kept, table)
		} else {
			report.Dropped = append(report.Dropped, DroppedTable{Table: table.Name, SourceRows: counts[table.Name]})
		}
	}
	if err := rehearsal.CheckCounterparts(ctx, tgt, kept); err != nil {
		return report, err
	}
	order, err := rehearsal.ForeignKeyOrder(ctx, tgt, kept)
	if err != nil {
		return report, err
	}
	tx, err := tgt.Begin(ctx)
	if err != nil {
		return report, fmt.Errorf("%w: begin: %v", rehearsal.ErrTarget, err)
	}
	defer tx.Rollback(context.Background())
	if err := rehearsal.SetTriggers(ctx, tx, order, false); err != nil {
		return report, err
	}
	for i, t := range order {
		d, err := deltaTable(ctx, src, tx, t, i, counts[t.Name])
		if err != nil {
			return report, err
		}
		report.Tables = append(report.Tables, d)
	}
	if clashes := report.clashes(); len(clashes) > 0 {
		return report, fmt.Errorf("%w: %s", ErrDeltaClash, strings.Join(clashes, ", "))
	}
	if err := rehearsal.SetSequences(ctx, tx, order); err != nil {
		return report, err
	}
	if err := rehearsal.SetTriggers(ctx, tx, order, true); err != nil {
		return report, err
	}
	if err := tx.Commit(ctx); err != nil {
		return report, fmt.Errorf("%w: commit: %v", rehearsal.ErrTarget, err)
	}
	return report, nil
}

func checkMoved(ctx context.Context, q rehearsal.Querier, ledger []string) ([]string, error) {
	var exists bool
	if err := q.QueryRow(ctx, "select to_regclass($1) is not null", rehearsal.LedgerTable).Scan(&exists); err != nil {
		return nil, fmt.Errorf("%w: inspect ledger: %v", rehearsal.ErrTarget, err)
	}
	if !exists {
		return nil, fmt.Errorf("%w: no ledger", ErrNotMoved)
	}
	rows, err := q.Query(ctx, "select filename from "+pgx.Identifier{rehearsal.LedgerTable}.Sanitize())
	if err != nil {
		return nil, fmt.Errorf("%w: read ledger: %v", rehearsal.ErrTarget, err)
	}
	have, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, fmt.Errorf("%w: read ledger: %v", rehearsal.ErrTarget, err)
	}
	return migrationDifference(ledger, have)
}

func migrationDifference(source, target []string) ([]string, error) {
	set := make(map[string]bool, len(target))
	for _, f := range target {
		set[f] = true
	}
	for _, f := range source {
		if !set[f] {
			return nil, fmt.Errorf("%w: %s missing", ErrNotMoved, f)
		}
	}
	for _, f := range source {
		delete(set, f)
	}
	var later []string
	for _, f := range target {
		if set[f] {
			later = append(later, f)
		}
	}
	return later, nil
}

func targetTables(ctx context.Context, q rehearsal.Querier, tables []rehearsal.Table, laterMigrations []string) (map[string]bool, error) {
	target, err := rehearsal.ReadTables(ctx, q)
	if err != nil {
		return nil, err
	}
	return checkTargetTables(tables, target, laterMigrations)
}

func checkTargetTables(source, target []rehearsal.Table, laterMigrations []string) (map[string]bool, error) {
	applied := make(map[string]bool, len(laterMigrations))
	for _, f := range laterMigrations {
		applied[f] = true
	}
	present := make(map[string]bool, len(target))
	for _, t := range target {
		present[t.Name] = true
	}
	var unexplained []string
	for _, t := range source {
		if present[t.Name] {
			continue
		}
		var retirement string
		switch t.Name {
		case "whitelist_entries", "funded_tiers":
			retirement = "006_archive_funded.sql"
		case "layerx_accounts", "wallet_custody_submissions", "wallet_custody_authorizations":
			retirement = "019_drop_retired_layerx_custody.sql"
		}
		if retirement == "" || !applied[retirement] {
			unexplained = append(unexplained, t.Name)
		}
	}
	if len(unexplained) > 0 {
		return nil, fmt.Errorf("%w: no counterpart for %s and no applied gateway migration retires it", rehearsal.ErrSchema, strings.Join(unexplained, ", "))
	}
	return present, nil
}

func deltaTable(ctx context.Context, src *rehearsal.Source, tx pgx.Tx, t rehearsal.Table, i int, counted int64) (DeltaTable, error) {
	d := DeltaTable{Table: t.Name, SourceRows: counted}
	stage := pgx.Identifier{fmt.Sprintf("ceremony_delta_%d", i)}.Sanitize()
	moved := pgx.Identifier{fmt.Sprintf("ceremony_delta_moved_%d", i)}.Sanitize()
	if _, err := tx.Exec(ctx, "create temporary table "+stage+" on commit drop as select "+t.ColumnList("")+" from "+t.Ident()+" with no data"); err != nil {
		return d, fmt.Errorf("%w: stage %s: %v", rehearsal.ErrTarget, t.Name, err)
	}
	streamed, loaded, _, err := rehearsal.Stream(ctx, src, tx.Conn().PgConn(), t, t.CopyIn(stage))
	if err != nil {
		return d, err
	}
	if streamed != counted || loaded != streamed {
		return d, fmt.Errorf("%w: %s counted %d, streamed %d, staged %d", rehearsal.ErrCountMismatch, t.Name, counted, streamed, loaded)
	}
	if err := tx.QueryRow(ctx, "select count(*) from "+t.Ident()).Scan(&d.TargetBefore); err != nil {
		return d, fmt.Errorf("%w: count %s: %v", rehearsal.ErrTarget, t.Name, err)
	}
	differs := make([]string, len(t.Columns))
	for j, c := range t.Columns {
		col := pgx.Identifier{c.Name}.Sanitize()
		differs[j] = "d." + col + "::text is distinct from t." + col + "::text"
	}
	if err := tx.QueryRow(ctx, "select count(*) from "+stage+" d join "+t.Ident()+" t on "+t.KeyMatch("d", "t")+
		" where "+strings.Join(differs, " or ")).Scan(&d.Conflicts); err != nil {
		return d, fmt.Errorf("%w: compare %s: %v", rehearsal.ErrTarget, t.Name, err)
	}
	absent := "not exists (select 1 from " + t.Ident() + " t where " + t.KeyMatch("t", "d") + ")"
	if t.Name == walletsTable {
		for _, need := range []string{"address", "user_id", "kind"} {
			if !hasColumn(t, need) {
				return d, fmt.Errorf("%w: wallets.%s", rehearsal.ErrSchema, need)
			}
		}
		if err := tx.QueryRow(ctx, "select count(*) from "+stage+" d where "+absent+
			" and exists (select 1 from wallets w where lower(w.address) = lower(d.address) or (w.user_id = d.user_id and w.kind = d.kind))").Scan(&d.Collisions); err != nil {
			return d, fmt.Errorf("%w: collisions %s: %v", rehearsal.ErrTarget, t.Name, err)
		}
	}
	if d.Conflicts > 0 || d.Collisions > 0 {
		return d, nil
	}
	if _, err := tx.Exec(ctx, "create temporary table "+moved+" on commit drop as select * from "+stage+" d where "+absent); err != nil {
		return d, fmt.Errorf("%w: select delta %s: %v", rehearsal.ErrTarget, t.Name, err)
	}
	sourceDigest, n, err := rehearsal.Digest(ctx, tx.Conn().PgConn(), t.CopyOut(moved, "m"))
	if err != nil {
		return d, fmt.Errorf("%w: digest delta %s: %v", rehearsal.ErrTarget, t.Name, err)
	}
	tag, err := tx.Exec(ctx, "insert into "+t.Ident()+" ("+t.ColumnList("")+") select "+t.ColumnList("m")+" from "+moved+" m order by "+t.OrderBy("m"))
	if err != nil {
		return d, fmt.Errorf("%w: insert delta %s: %v", rehearsal.ErrTarget, t.Name, err)
	}
	inTarget := "(select t.* from " + t.Ident() + " t where exists (select 1 from " + moved + " m where " + t.KeyMatch("m", "t") + "))"
	targetDigest, back, err := rehearsal.Digest(ctx, tx.Conn().PgConn(), t.CopyOut(inTarget, "r"))
	if err != nil {
		return d, fmt.Errorf("%w: re-read delta %s: %v", rehearsal.ErrTarget, t.Name, err)
	}
	if tag.RowsAffected() != n || back != n {
		return d, fmt.Errorf("%w: %s delta %d, inserted %d, re-read %d", rehearsal.ErrCountMismatch, t.Name, n, tag.RowsAffected(), back)
	}
	if sourceDigest != targetDigest {
		return d, fmt.Errorf("%w: %s delta", rehearsal.ErrDigestMismatch, t.Name)
	}
	d.Moved, d.SourceDigest, d.TargetDigest = n, sourceDigest, targetDigest
	return d, nil
}

func hasColumn(t rehearsal.Table, name string) bool {
	for _, c := range t.Columns {
		if c.Name == name {
			return true
		}
	}
	return false
}
