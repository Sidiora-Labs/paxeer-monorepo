package rolecheck

import "strings"

func words(value string) []string { return strings.Fields(value) }

type Object struct {
	Kind    string
	Name    string
	Columns []string
	Rights  []string
}

type Statement struct {
	ID       string
	File     string
	Function string
	Call     int
	Kind     string
	Role     string
	Objects  []Object
	Bindings map[string]string
}

const runtimeRole = "ranking_runtime"
const selectRight = "SELECT"
const executeRight = "EXECUTE"
const functionKind = "function"
const storePath = "services/launchpad/ranking-algo/internal/store/"
const productionPath = "sdk/launchpad/shared/db/production/migrations.go"

func relation(name, columns, rights string) Object {
	return Object{Kind: "relation", Name: name, Columns: words(columns), Rights: words(rights)}
}
func functions(names string) []Object {
	namesList := words(names)
	out := make([]Object, 0, len(namesList))
	for _, name := range namesList {
		out = append(out, Object{Kind: functionKind, Name: name, Rights: []string{executeRight}})
	}
	return out
}
func statement(id, file, function string, call int, kind string, objects []Object, builtins string) Statement {
	return Statement{ID: id, File: file, Function: function, Call: call, Kind: kind, Role: runtimeRole, Objects: append(objects, functions(builtins)...)}
}

var Statements = []Statement{
	statement("input.watermark", storePath+"frozen_inputs.go", "ReadRankingUniverseTxDetailed", 1, selectRight, []Object{
		relation("indexer.indexer_ingest_watermarks_v1", "raw_complete_height typed_complete_height chain_id stream_id", selectRight),
		relation("indexer.indexer_ingest_leases_v1", "expires_at chain_id stream_id", selectRight),
	}, "pg_catalog.clock_timestamp"),
	statement("input.completeness", storePath+"frozen_inputs.go", "ReadRankingUniverseTxDetailed", 2, selectRight, []Object{
		relation("indexer.indexer_block_completeness_v1", "block_hash completed_at fetch_complete required_decode_complete chain_id stream_id height", selectRight),
	}, ""),
	statement("input.coherence", storePath+"frozen_inputs.go", "ReadRankingUniverseTxDetailed", 3, selectRight, []Object{
		relation("stats.accounting_pool_fences_v2", "frozen", selectRight),
		relation("indexer.indexer_accounting_fills_v1", "payload source_fill_id source_fact_digest", selectRight),
		relation("stats.accounting_market_commits_v2", "projection_version source_fill_id source_fact_digest fill", selectRight),
	}, ""),
	statement("input.universe", storePath+"frozen_inputs.go", "ReadRankingUniverseTxDetailed", 4, selectRight, []Object{
		relation("stats.accounting_market_commits_v2", "pool fill projection_version", selectRight),
		relation("indexer.indexer_canonical_creations_v1", "pool height transaction_index log_index block_timestamp", selectRight),
	}, "pg_catalog.sum pg_catalog.count pg_catalog.least"),
	statement("complete.lock_timeout", storePath+"publication.go", "Complete", 0, "SET", nil, ""),
	statement("complete.category_lock", storePath+"publication.go", "Complete", 1, selectRight, nil, "pg_catalog.pg_advisory_xact_lock pg_catalog.hashtextextended"),
	statement("complete.head", storePath+"publication.go", "Complete", 2, selectRight, []Object{
		relation("ranking.category_heads", "run_id cutoff_time revision category", "SELECT UPDATE"),
	}, ""),
	statement("complete.snapshot", storePath+"publication.go", "Complete", 3, "INSERT", []Object{
		relation("ranking.snapshots", "run_id category input_digest output_digest artifact public_header", "INSERT"),
	}, ""),
	statement("complete.identity", storePath+"publication.go", "Complete", 4, selectRight, []Object{
		relation("ranking.snapshots", "artifact run_id category", selectRight),
	}, ""),
	statement("complete.entries", storePath+"publication.go", "Complete", 5, "COPY", []Object{
		relation("ranking.snapshot_entries", "run_id category rank pool_address score", "INSERT"),
	}, ""),
	statement("complete.advance", storePath+"publication.go", "Complete", 6, "INSERT", []Object{
		relation("ranking.category_heads", "category run_id cutoff_time revision", "INSERT UPDATE SELECT"),
	}, ""),
	statement("complete.outbox", storePath+"publication.go", "Complete", 7, "INSERT", []Object{
		relation("ranking.publication_outbox", "run_id category artifact category_sequence", "INSERT"),
		{Kind: "sequence", Name: "ranking.publication_outbox_sequence_seq", Rights: []string{"USAGE"}},
	}, ""),
	statement("snapshot.header", storePath+"publication.go", "header", 0, selectRight, []Object{
		relation("ranking.snapshots", "run_id category input_digest output_digest public_header completed_at", selectRight),
		relation("ranking.publication_outbox", "run_id category category_sequence", selectRight),
	}, ""),
	statement("snapshot.current", storePath+"publication.go", "Current", 0, selectRight, []Object{
		relation("ranking.category_heads", "run_id category", selectRight),
	}, ""),
	statement("snapshot.page", storePath+"publication.go", "Page", 0, selectRight, []Object{
		relation("ranking.snapshot_entries", "pool_address score rank run_id category", selectRight),
	}, ""),
	statement("snapshot.boundary", storePath+"publication.go", "ValidateBoundary", 0, selectRight, []Object{
		relation("ranking.snapshot_entries", "pool_address score run_id category rank", selectRight),
	}, ""),
	statement("publication.next", storePath+"publication.go", "nextEvent", 0, selectRight, []Object{
		relation("ranking.publication_outbox", "sequence run_id category category_sequence published_at attempts", "SELECT UPDATE"),
	}, ""),
	statement("publication.entries", storePath+"publication.go", "nextEvent", 1, selectRight, []Object{
		relation("ranking.snapshot_entries", "pool_address score rank run_id category", selectRight),
	}, ""),
	statement("publication.commit", storePath+"publication.go", "CommitStreamOnceWithStatus", 0, "UPDATE", []Object{
		relation("ranking.publication_outbox", "published_at attempts broker_stream broker_sequence sequence", "UPDATE SELECT"),
	}, "pg_catalog.clock_timestamp"),
	statement("publication.failure", storePath+"publication.go", "recordFailure", 0, "UPDATE", []Object{
		relation("ranking.publication_outbox", "attempts sequence published_at", "UPDATE SELECT"),
	}, ""),
	statement("health.categories", storePath+"publication.go", "PublicationHealth", 0, selectRight, []Object{
		relation("ranking.category_heads", "category cutoff_time", selectRight),
	}, ""),
	statement("health.pending", storePath+"publication.go", "PublicationHealth", 1, selectRight, []Object{
		relation("ranking.publication_outbox", "attempts created_at published_at", selectRight),
	}, "pg_catalog.count pg_catalog.min pg_catalog.greatest pg_catalog.extract"),
	statement("prune.retained", storePath+"publication.go", "Prune", 0, selectRight, []Object{
		relation("ranking.snapshots", "run_id category public_header completed_at", "SELECT UPDATE"),
		relation("ranking.publication_outbox", "run_id category published_at", selectRight),
		relation("ranking.category_heads", "run_id category", selectRight),
	}, ""),
	pruneStatement("snapshot_entries"), pruneStatement("publication_outbox"), pruneStatement("snapshots"),
	statement("readiness.search_path", productionPath, "CheckReadiness", 0, "SET", nil, ""),
	statement("readiness.role", productionPath, "CheckReadiness", 1, selectRight, []Object{
		relation("pg_catalog.pg_roles", "rolsuper rolcreatedb rolcreaterole rolbypassrls oid rolname", selectRight),
		relation("pg_catalog.pg_class", "relowner relnamespace", selectRight),
		relation("pg_catalog.pg_namespace", "oid nspname nspowner", selectRight),
	}, "pg_catalog.pg_has_role pg_catalog.has_schema_privilege"),
	statement("readiness.attestation", productionPath, "CheckReadiness", 2, selectRight, []Object{
		{Kind: functionKind, Name: "migration_control.readiness", Columns: words("registry_digest interface_digest reader_min_version migrations"), Rights: []string{executeRight}},
	}, ""),
	statement("readiness.schema", productionPath, "CheckReadiness", 3, selectRight, []Object{
		relation("pg_catalog.pg_namespace", "nspname nspowner", selectRight),
	}, "pg_catalog.has_schema_privilege pg_catalog.pg_has_role"),
	statement("readiness.epoch", productionPath, "CheckReadiness", 4, selectRight, []Object{
		{Kind: functionKind, Name: "recovery.current_epoch", Columns: words("epoch region"), Rights: []string{executeRight}},
	}, ""),
	statement("readiness.fingerprint", productionPath, "CatalogFingerprint", 0, selectRight, []Object{
		relation("pg_catalog.pg_class", "oid relname relkind relowner relrowsecurity relforcerowsecurity relnamespace", selectRight),
		relation("pg_catalog.pg_namespace", "oid nspname", selectRight),
		relation("pg_catalog.pg_attribute", "attrelid attname attnum atttypid atttypmod attnotnull attidentity attisdropped", selectRight),
		relation("pg_catalog.pg_attrdef", "adbin adrelid adnum", selectRight),
		relation("pg_catalog.pg_constraint", "oid conrelid conname", selectRight),
		relation("pg_catalog.pg_trigger", "oid tgrelid tgname tgisinternal", selectRight),
		relation("pg_catalog.pg_proc", "oid proname proowner pronamespace", selectRight),
	}, "pg_catalog.concat_ws pg_catalog.pg_get_userbyid pg_catalog.format_type pg_catalog.pg_get_expr pg_catalog.pg_get_constraintdef pg_catalog.pg_get_indexdef pg_catalog.pg_get_triggerdef pg_catalog.pg_get_function_identity_arguments pg_catalog.pg_get_functiondef pg_catalog.pg_get_viewdef"),
	statement("privilege.function", productionPath, "checkPrivilege", 0, selectRight, nil, "pg_catalog.has_function_privilege"),
	statement("privilege.sequence", productionPath, "checkPrivilege", 1, selectRight, nil, "pg_catalog.has_sequence_privilege"),
	statement("privilege.column", productionPath, "checkPrivilege", 2, selectRight, nil, "pg_catalog.has_column_privilege"),
	statement("privilege.table", productionPath, "checkPrivilege", 3, selectRight, nil, "pg_catalog.has_table_privilege"),
	statement("privileges.tables", productionPath, "checkUnexpectedPrivileges", 0, selectRight, []Object{
		relation("pg_catalog.pg_class", "oid relname relnamespace relkind", selectRight),
		relation("pg_catalog.pg_namespace", "oid nspname", selectRight),
	}, "pg_catalog.has_table_privilege"),
	statement("privileges.columns", productionPath, "checkUnexpectedPrivileges", 1, selectRight, []Object{
		relation("pg_catalog.pg_attribute", "attrelid attname attnum attisdropped", selectRight),
		relation("pg_catalog.pg_class", "oid relname relnamespace relkind", selectRight),
		relation("pg_catalog.pg_namespace", "oid nspname", selectRight),
	}, "pg_catalog.has_column_privilege"),
	statement("privileges.functions", productionPath, "checkUnexpectedPrivileges", 2, selectRight, []Object{
		relation("pg_catalog.pg_proc", "oid proname proargtypes pronamespace", selectRight),
		relation("pg_catalog.pg_namespace", "oid nspname", selectRight),
	}, "pg_catalog.oidvectortypes pg_catalog.has_function_privilege"),
	statement("privileges.sequences", productionPath, "checkUnexpectedPrivileges", 3, selectRight, []Object{
		relation("pg_catalog.pg_class", "oid relname relnamespace relkind", selectRight),
		relation("pg_catalog.pg_namespace", "oid nspname", selectRight),
	}, "pg_catalog.has_sequence_privilege"),
	statement("recovery.assert", "sdk/launchpad/shared/queue/queue.go", "Assert", 0, selectRight, nil, "recovery.assert_epoch"),
	statement("stream.append", "sdk/launchpad/shared/delivery/stream.go", "AppendTx", 0, selectRight, nil, "api_stream.append_ranking"),
	statement("input.date_trunc", storePath+"frozen_inputs.go", "ReadRankingUniverseTxDetailed", 0, selectRight, nil, "pg_catalog.date_trunc pg_catalog.clock_timestamp"),
}

func pruneStatement(table string) Statement {
	s := statement("prune."+table, storePath+"publication.go", "Prune", 1, "DELETE", []Object{relation("ranking."+table, "run_id category", "DELETE SELECT")}, "")
	s.Bindings = map[string]string{"table": table}
	return s
}
