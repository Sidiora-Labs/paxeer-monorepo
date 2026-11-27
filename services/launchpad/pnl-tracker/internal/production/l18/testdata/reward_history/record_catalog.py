"""Record the reward runtime's registered schema dependencies from PostgreSQL.

Changes registering migrations in the runtime role's schema, its granted schemas except migration_control, or their predecessors must renew this transcript with this recorder in the same change and verify the cases of internal/production/l18.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
from urllib.parse import urlsplit, urlunsplit
import uuid

ROOT = Path(__file__).resolve().parents[8]
OUTPUT = Path(__file__).with_name("catalog.json")


def checksum(raw):
    return hashlib.sha256(raw).hexdigest()


def source_path(relative):
    for prefix in ("core/", "media/"):
        if relative.startswith(prefix):
            return "services/launchpad/" + relative[len(prefix):]
    if relative.startswith("spec/production-system/"):
        return "launchpad/" + relative
    return relative


def dependency_plan(registry, roles):
    runtime = next(role for role in roles if role["runtime_role"] == "pnl_runtime")
    schemas = {runtime["schema"], *(grant["schema"] for grant in runtime["grants"])} - {"migration_control"}
    catalog = {item["migration_id"]: item for item in registry["registrations"]}
    needed = set()

    def include(identity):
        if identity in needed:
            return
        needed.add(identity)
        for predecessor in catalog[identity]["predecessor_ids"]:
            include(predecessor)

    for item in catalog.values():
        if item["schema"] in schemas:
            include(item["migration_id"])
    plan = [catalog[identity] for identity in registry["application_order"] if identity in needed]
    assert len(plan) == len(needed)
    seen = set()
    for item in plan:
        assert set(item["predecessor_ids"]) <= seen
        seen.add(item["migration_id"])
    return plan


CATALOG_SQL = """
SELECT coalesce(json_object_agg(name, definition), '{}') FROM (
 SELECT n.nspname||'.'||c.relname AS name,
 json_build_object('kind',CASE c.relkind WHEN 'S' THEN 'sequence' WHEN 'v' THEN 'view' ELSE 'table' END,
 'columns',coalesce((SELECT json_agg(a.attname ORDER BY a.attnum) FROM pg_attribute a
 WHERE a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped),'[]')) AS definition
 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
 WHERE c.relkind IN ('r','p','v','S') AND n.nspname NOT IN ('pg_catalog','information_schema','public')
 AND n.nspname NOT LIKE 'pg_%'
 UNION ALL
 SELECT n.nspname||'.'||p.proname,json_build_object('kind','function','columns','[]'::json)
 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
 WHERE n.nspname NOT IN ('pg_catalog','information_schema','public') AND n.nspname NOT LIKE 'pg_%'
) objects;
"""


def record(dsn):
    def sql(endpoint, statement):
        result = subprocess.run(["psql", "--no-psqlrc", "--no-align", "--tuples-only", "--set", "ON_ERROR_STOP=1", endpoint],
                                input=statement, text=True, capture_output=True)
        if result.returncode:
            raise RuntimeError(result.stderr)
        return result.stdout.strip()

    registry_raw = (ROOT / "migrations/registry.json").read_bytes()
    registry = json.loads(registry_raw)
    roles = json.loads((ROOT / "migrations/grants.json").read_bytes())["roles"]
    plan = dependency_plan(registry, roles)
    schemas = {item["schema"] for item in plan}
    services = {item["service"] for item in plan}
    tag = "_catalog_" + uuid.uuid4().hex[:12]
    database = "reward" + tag
    parts = urlsplit(dsn)
    endpoint = urlunsplit(parts._replace(path="/" + database))
    names = {role[key] for role in roles for key in ("owner_role", "runtime_role")}
    names.add("migration_control_owner")
    for role in roles:
        names.update(scope["owner_role"] for scope in role.get("schema_capabilities", []))
    names = sorted(names, key=lambda value: (-len(value), value))
    assert all(re.fullmatch(r"[a-z0-9_]+", name) for name in names)
    references = re.compile(r"\b(?:" + "|".join(map(re.escape, names)) + r")\b")
    created = []
    sources, objects, previous, applied = {}, {}, {}, {}

    def apply(relative, raw, statement, transactional=True):
        nonlocal previous
        sources[relative] = checksum(raw)
        statement = references.sub(lambda match: match[0] + tag, statement)
        sql(endpoint, "BEGIN;\n" + statement + "\nCOMMIT;" if transactional else statement)
        current = json.loads(sql(endpoint, CATALOG_SQL))
        for name, definition in current.items():
            if previous.get(name) != definition:
                objects[name] = {**definition, "source": relative}
        for name in previous.keys() - current.keys():
            objects.pop(name, None)
        previous = current

    sql(dsn, f'CREATE DATABASE "{database}";')
    try:
        for name in names:
            role = name + tag
            sql(dsn, f'CREATE ROLE "{role}" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS;')
            created.append(role)
        for extension in registry.get("extension_prerequisites", []):
            assert re.fullmatch(r"[a-z0-9_]+", extension["extension"])
            sql(endpoint, f'CREATE EXTENSION IF NOT EXISTS "{extension["extension"]}" WITH SCHEMA "{extension["schema"]}";')
        control_path = "sdk/launchpad/shared/db/production/migrations.go"
        control_raw = (ROOT / control_path).read_bytes()
        control = re.search(r"const migrationControlDDL = `([^`]+)`", control_raw.decode())[1]
        apply(control_path, control_raw, control.split(";", 1)[1])
        legacy = json.loads((ROOT / "migrations/legacy-history-aliases.json").read_bytes())
        for history in legacy["histories"]:
            if history["service"] not in services:
                continue
            for source in history["sources"]:
                relative = source_path(source["path"])
                raw = (ROOT / relative).read_bytes()
                assert checksum(raw) == source["source_sha256"], relative
                up = raw.decode().split("-- +goose Down", 1)[0]
                assert checksum(up.encode()) == source["up_sha256"], relative
                apply(relative, raw, up)
        sql(endpoint, "\n".join(f'CREATE SCHEMA IF NOT EXISTS "{schema}";' for schema in sorted(schemas)))
        for item in plan:
            relative = source_path(item["path"])
            raw = (ROOT / relative).read_bytes()
            assert checksum(raw) == item["sha256"], relative
            apply(relative, raw, raw.decode(), item["transaction_mode"] == "transactional")
            applied[str(item["migration_id"])] = item["sha256"]
        return {"description": "Reward runtime catalog from the registered migration dependency plan and legacy sources.",
                "applied": applied,
                "sources": sources, "objects": dict(sorted(objects.items()))}
    finally:
        sql(dsn, f'DROP DATABASE "{database}" WITH (FORCE);')
        for role in reversed(created):
            sql(dsn, f'DROP ROLE "{role}";')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dsn-env", default="PNL_TEST_POSTGRES_DSN")
    args = parser.parse_args()
    result = record(os.environ[args.dsn_env])
    OUTPUT.write_text(json.dumps(result, indent=2) + "\n")


if __name__ == "__main__":
    main()
