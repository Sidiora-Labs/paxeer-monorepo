"""Migration manifest/source qualification, without a Go or PostgreSQL runtime.

Reopening manifests checks durable configuration only. Coordinator import and
restart execution remain explicit owner requests, never simulated in this gate.
"""

import ast
import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import tomllib


ROOT = Path(__file__).resolve().parents[3]
MIGRATIONS = ROOT / "migrations"
PRODUCTION = ROOT / "sdk/launchpad/shared/db/production"
RECORD = Path(__file__).with_name("repair_22_33_815.kvx")
DIGEST = re.compile(r"[0-9a-f]{64}\Z")


def document(name):
    with (MIGRATIONS / name).open(encoding="utf-8") as stream:
        return json.load(stream)


def record():
    return tomllib.loads(RECORD.read_text(encoding="utf-8"))


def checksum(data):
    return hashlib.sha256(data).hexdigest()


def inventory_digest(rows):
    return checksum(json.dumps(sorted(rows), separators=(",", ":")).encode())


def monorepo_path(relative):
    path = PurePosixPath(relative)
    assert not path.is_absolute()
    assert path.as_posix() == relative
    assert ".." not in path.parts and "." not in path.parts
    assert path.parts[0] in {"migrations", "launchpad", "services", "sdk", "deploy"}
    result = ROOT / relative
    assert result.resolve().is_relative_to(ROOT)
    assert not result.is_symlink()
    return result


def go_function(path, name):
    source = path.read_text(encoding="utf-8")
    declaration = re.search(
        rf"^func (?:\([^\n]*\) )?{re.escape(name)}\(", source, re.M
    )
    assert declaration is not None, f"missing real Go function {name} in {path}"
    following = re.search(r"^func ", source[declaration.end():], re.M)
    end = declaration.end() + following.start() if following else len(source)
    return source[declaration.start():end]


def test_registered_sql_paths_digests_and_application_order():
    registry = document("registry.json")
    assert registry["schema_version"] == 2
    assert registry["path_base"] == "monorepo-root"
    entries = registry["registrations"]
    assert len(entries) == record()["inventory"]["registration_count"] == 76
    by_id = {entry["migration_id"]: entry for entry in entries}
    assert len(by_id) == len(entries)
    assert len(registry["application_order"]) == len(entries)
    seen = set()
    for migration_id in registry["application_order"]:
        assert migration_id in by_id and migration_id not in seen
        migration = by_id[migration_id]
        assert set(migration["predecessor_ids"]) <= seen
        assert len(migration["predecessor_ids"]) == len(set(migration["predecessor_ids"]))
        seen.add(migration_id)
        path = monorepo_path(migration["path"])
        assert path.parent == MIGRATIONS
        assert path.name == f"{migration_id}.sql"
        sql = path.read_bytes()
        assert sql and DIGEST.fullmatch(migration["sha256"])
        assert checksum(sql) == migration["sha256"], str(path)
    assert seen == set(by_id)
    assert {path.name for path in MIGRATIONS.glob("*.sql")} == {
        f"{migration_id}.sql" for migration_id in seen
    } | {"0001_genesis_sections.sql", "0001_projection.sql", "0007_history_index.sql"}
    assert inventory_digest(
        [(entry["migration_id"], entry["sha256"]) for entry in entries]
    ) == record()["inventory"]["sql_digest_inventory_sha256"]


def test_source_inputs_relocated_without_reapproving_drift():
    registry = document("registry.json")
    inputs = {}
    for migration in registry["registrations"]:
        assert migration["source_inputs"]
        for source in migration["source_inputs"]:
            assert DIGEST.fullmatch(source["sha256"])
            monorepo_path(source["path"])
            if source["path"] in inputs:
                assert inputs[source["path"]] == source["sha256"]
            inputs[source["path"]] = source["sha256"]
    assert inventory_digest(list(inputs.items())) == record()["inventory"][
        "source_input_inventory_sha256"
    ]
    missing, drifted = [], []
    for relative, digest in sorted(inputs.items()):
        path = monorepo_path(relative)
        if not path.is_file():
            missing.append(relative)
        elif checksum(path.read_bytes()) != digest:
            drifted.append(relative)
    # These inputs are explicitly unresolved, never accepted as qualified.
    assert missing == record()["inventory"]["missing_source_inputs"]
    assert drifted == record()["inventory"]["drifted_source_inputs"]
    requests = record()["wiring"]["requests"]
    for relative in missing + drifted:
        assert any(item.startswith(relative + " | owning ") for item in requests)
    assert registry["state"] == "implemented-not-qualified"
    assert registry["approval_digest"] is None


def test_unqualified_registry_has_no_approval_authority():
    registry = document("registry.json")
    assert registry["state"] == "implemented-not-qualified"
    assert registry["approval_digest"] is None
    for migration in registry["registrations"]:
        for field in (
            "interface_digest", "reader_min_version", "estimated_lock_ms",
            "approval_manifest_digest",
        ):
            assert migration[field] is None, (migration["migration_id"], field)
        for predecessor in migration["legacy_predecessors"]:
            assert predecessor["structural_fingerprint"] is None
    assert registry["extension_prerequisites"] == [{
        "extension": "pg_trgm", "schema": "public", "version": None,
        "runtime_install": False,
    }]
    proposal = document("pnl-reward-approval.json")
    retained = record()["provisional_migration_180001"]
    reward = next(m for m in registry["registrations"] if m["migration_id"] == 180001)
    assert proposal["migration_id"] == reward["migration_id"]
    assert proposal["source_sha256"] == reward["sha256"]
    for field in ("interface_digest", "reader_min_version", "estimated_lock_ms"):
        assert proposal[field] == retained[field]
    assert DIGEST.fullmatch(retained["approval_manifest_digest"])
    assert "task 22.33.917" in retained["wiring_request"]
    required = registry["qualification_required"]
    for fact in (
        "measured lock <=2000ms", "deployed reader minimum versions",
        "actual structural fingerprints", "frozen output interface digest",
        "pg_trgm version", "approval_digest is null", "source_inputs",
        "import and restart refusal", "least-privilege runtime readiness",
    ):
        assert any(fact in item for item in required), fact


def test_real_coordinator_checks_admission_before_ddl():
    source = PRODUCTION / "migrations.go"
    order = go_function(source, "ValidateOrder")
    assert "!validDigest(r.ApprovalDigest)" in order
    assert order.index("!validDigest(r.ApprovalDigest)") < order.index("catalog :=")
    loader = go_function(source, "LoadMigrationRegistry")
    assert "Checksum(data) != expectedDigest" in loader
    assert "registry.ValidateOrder()" in loader
    validate = go_function(source, "Validate")
    for guard in (
        "!validDigest(interfaceDigest)", "m.InterfaceDigest != interfaceDigest",
        "!validDigest(approvalDigest)", "m.ApprovalManifestDigest != approvalDigest",
        "m.ReaderMinVersion < 1", "deployedReaderMin < m.ReaderMinVersion",
        "m.EstimatedLockMS <= 0", "m.EstimatedLockMS > 2000",
        "m.SHA256 != Checksum(sql)", "!reflect.DeepEqual(candidate, approved)",
    ):
        assert guard in validate, guard
    apply = go_function(source, "ApplyRegisteredMigration")
    assert apply.index("m.Validate(") < apply.index("tx.Exec(ctx, string(sql))")
    assert "SELECT migration_id,checksum FROM migration_control.applied_migrations" in apply
    assert "legacy.Key()" in apply
    assert "fingerprint != legacy.StructuralFingerprint" in apply
    assert "SET LOCAL lock_timeout='2s'" in apply
    assert "pg_try_advisory_xact_lock" in apply


def test_legacy_source_checksums_and_import_refusal_contract():
    legacy = document("legacy-history-aliases.json")
    assert legacy["schema_version"] == 2
    assert legacy["state"] == "implemented-not-qualified"
    assert legacy["path_base"] == "monorepo-root"
    assert len(legacy["histories"]) == 8
    aliases = {}
    evidence = []
    for history in legacy["histories"]:
        assert history["history"].startswith("migration_control.")
        assert history["existing_database_import"] == (
            "requires independently approved structural fingerprint and exact "
            "source/history checksums; never max(version)"
        )
        versions = []
        for source in history["sources"]:
            assert source["alias"] == f"{history['service']}:{source['original_version']}"
            assert source["alias"] not in aliases
            aliases[source["alias"]] = source
            versions.append(source["original_version"])
            raw = monorepo_path(source["path"]).read_bytes()
            up = raw.split(b"-- +goose Down", 1)[0]
            assert checksum(raw) == source["source_sha256"]
            assert checksum(up) == source["up_sha256"]
            assert len(up) == source["up_bytes"]
            assert source["structural_fingerprint"] is None
            evidence.append((
                source["alias"], source["original_version"], source["source_sha256"],
                source["up_sha256"], source["up_bytes"], source["structural_fingerprint"],
            ))
        assert versions == list(range(1, len(versions) + 1))
    assert len(aliases) == 11
    assert inventory_digest(evidence) == record()["inventory"][
        "legacy_evidence_inventory_sha256"
    ]
    for migration in document("registry.json")["registrations"]:
        for predecessor in migration["legacy_predecessors"]:
            source = aliases[f"{predecessor['service']}:{predecessor['original_version']}"]
            for field in ("source_sha256", "up_sha256", "structural_fingerprint"):
                assert predecessor[field] == source[field]
    key = go_function(PRODUCTION / "migrations.go", "Key")
    assert "!validDigest(a.StructuralFingerprint)" in key
    importer = go_function(PRODUCTION / "migrations.go", "ImportLegacyBaseline")
    assert importer.index("a.Key()") < importer.index("conn.Begin(ctx)")
    for guard in (
        "Checksum(source) != a.SourceSHA256", "Checksum(up) != a.UpSHA256",
        "CatalogFingerprint(ctx, tx, schema)", "actual != a.StructuralFingerprint",
        "WHERE version_id=$1 AND is_applied", "if !recorded",
        "original per-service history proof absent",
    ):
        assert guard in importer, guard
    assert importer.index("if !recorded") < importer.index(
        "INSERT INTO migration_control.legacy_baselines"
    )
    assert "max(version" not in importer.lower()


def test_legacy_evidence_survives_manifest_close_and_reopen():
    # The only in-scope component is a manifest. This does not restart Go/PG.
    paths = [MIGRATIONS / name for name in (
        "legacy-history-aliases.json", "registry.json", "readiness.json",
    )]
    before = []
    for path in paths:
        with path.open("rb") as stream:
            before.append(stream.read())
        assert stream.closed
    # Drop parsed state and open new handles against the persisted source.
    for path, expected in zip(paths, before, strict=True):
        with path.open("rb") as reopened:
            recovered = reopened.read()
        assert reopened.closed
        assert recovered == expected
        assert json.loads(recovered) == json.loads(expected)
    legacy = document("legacy-history-aliases.json")
    assert all(source["structural_fingerprint"] is None
               for history in legacy["histories"] for source in history["sources"])
    assert document("registry.json")["approval_digest"] is None
    assert document("readiness.json")["registry_sha256"] is None


def test_readiness_revokes_stale_binding_until_qualification():
    readiness = document("readiness.json")
    assert readiness["schema_version"] == 2
    assert readiness["registry_sha256"] is None
    assert readiness["state"] == "requires_real_combined_postgres_qualification"
    for field in (
        "expected_catalog_fingerprints", "expected_reader_versions",
        "required_recovery_epoch", "required_recovery_region",
    ):
        assert readiness[field] is None
    assert readiness["mode"] == "trusted_expected_schema_readonly"
    assert readiness["runtime_can_read_history"] is False
    assert readiness["denies_unexpected_effective_privileges"] is True
    assert readiness["readiness_function"] == "migration_control.readiness(text)"
    assert readiness["current_epoch_function"] == "recovery.current_epoch()"
    assert readiness["expected_schema_file"] == (
        "operator-supplied JSON bound by EXPECTED_SCHEMA_SHA256"
    )
    assert document("registry.json")["approval_digest"] is None


def test_readiness_roles_and_real_expected_schema_consumer():
    readiness = document("readiness.json")
    roles = {role["runtime_role"] for role in document("grants.json")["roles"]}
    ids = {migration["migration_id"] for migration in document("registry.json")[
        "registrations"
    ]}
    assert set(readiness["additional_required_migrations_by_role"]) <= roles
    for role, migrations in readiness["additional_required_migrations_by_role"].items():
        assert migrations and len(migrations) == len(set(migrations)), role
        assert set(migrations) <= ids, role
    assert set(readiness["unused_runtime_roles"]) <= roles
    loader = go_function(PRODUCTION / "migrations.go", "LoadExpectedSchema")
    assert "Checksum(data) != expectedSHA256" in loader
    assert "decoder.DisallowUnknownFields()" in loader
    assert "validateExpectedSchema(expected)" in loader
    validation = go_function(PRODUCTION / "migrations.go", "validateExpectedSchema")
    for guard in (
        "!validDigest(want.RegistryDigest)", "!validDigest(want.InterfaceDigest)",
        "want.ReaderVersion < 1", "len(want.Migrations) == 0",
        "len(want.Schemas) == 0", "want.Epoch < 1",
        "!validDigest(fingerprint)", "runtime history access forbidden",
    ):
        assert guard in validation, guard
    check = go_function(PRODUCTION / "migrations.go", "CheckReadiness")
    for guard in (
        "registry != want.RegistryDigest", "inter != want.InterfaceDigest",
        "reader > want.ReaderVersion", "actual != fingerprint",
        "epoch != want.Epoch || region != want.Region",
        "checkUnexpectedPrivileges(",
    ):
        assert guard in check, guard


def test_coordinator_consumer_reads_monorepo_paths_and_checks_digests():
    source = PRODUCTION / "coordinatortest/coordinatortest.go"
    reader = go_function(source, "readBound")
    assert "os.ReadFile(filepath.Join(root, relative))" in reader
    assert 'want != "" && production.Checksum(raw) != want' in reader
    # Fully rooted registry paths bypass the retained old-prefix branches.
    for migration in document("registry.json")["registrations"]:
        assert not migration["path"].startswith(("spec/", "core/", "media/"))
        assert monorepo_path(migration["path"]).is_file()
    bootstrap = go_function(source, "Bootstrap")
    assert "readBound(t, root, m.Path, m.SHA256)" in bootstrap
    assert "readBound(t, root, source.Path, source.SourceSHA256)" in bootstrap
    assert "production.CatalogFingerprint(" in bootstrap
    assert "INSERT INTO migration_control.legacy_baselines" in bootstrap
    runtime_test = go_function(
        PRODUCTION / "coordinatortest/coordinatortest_test.go",
        "TestPlatformRuntimeRolesPassReadiness",
    )
    assert 'Bootstrap(t, admin, "chat_runtime", "livestream_runtime")' in runtime_test
    assert "production.LoadExpectedSchema(rt.ExpectedSchema, rt.ExpectedSchemaSHA256)" in runtime_test
    assert "pgxpool.New(ctx, rt.DSN)" in runtime_test
    assert "production.CheckReadiness(ctx, pool, want)" in runtime_test
    assert "err != nil || owns || creates" in runtime_test


def test_record_names_all_proofs_and_unexecuted_owner_obligations():
    repair = record()
    assert repair["repair"]["task"] == "22.33.815"
    assert set(repair["repair"]["finding_ids"]) == {
        "legacy-import-evidence", "readiness-registry-digest",
        "migration-admission-unqualified", "migration-paths",
    }
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body
                if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    for finding in repair["repair"]["finding_ids"]:
        proofs = repair["finding"][finding]["tests"]
        assert proofs and set(proofs) <= declared
        assert repair["finding"][finding]["closure_boundary"]
    assert "No pre-change verification was run" in repair["repair"]["baseline"]
    assert "not a coordinator process restart" in repair["repair"]["review_only"]
    assert "same revision" in repair["repair"]["verified_revision"]
    assert repair["repair"]["evidence_ledger"] == "/out/run/gates.jsonl"
    requests = repair["wiring"]["requests"]
    for needed in (
        "ImportLegacyBaseline", "coordinatortest/coordinatortest.go",
        "coordinatortest/coordinatortest_test.go", "deploy/launchpad/production",
        "stop/reopen", "measured", "least-privilege",
    ):
        assert any(needed in item for item in requests), needed
