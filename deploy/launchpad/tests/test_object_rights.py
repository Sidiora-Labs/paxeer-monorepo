from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[3]
DOCUMENT = ROOT / "deploy/launchpad/object-store/accounts.json"
POLICY = ROOT / "deploy/launchpad/object-store/policy.py"
VARIABLES = ROOT / "deploy/launchpad/railway/variables.json"

# These are the production source trees. Development forms, fixtures and cases
# are intentionally outside the account inventory.
DEVELOPMENT_FILES = {"legacy_publication.go", "legacy_reads.go"}
OBJECT_METHODS = {
    "Delete",
    "Get",
    "GetBytes",
    "Head",
    "Put",
    "PutImmutable",
    "read",
    "put",
}
RAW_OBJECT_METHODS = {
    "DeleteObject",
    "GetBucketAcl",
    "GetBucketPolicyStatus",
    "GetObject",
    "HeadBucket",
    "ListObjectsV2",
    "PresignGetObject",
    "PutObject",
}
CALL_RE = re.compile(r"(?P<receiver>[A-Za-z_][A-Za-z0-9_.\[\]]*)\.(?P<method>[A-Za-z_][A-Za-z0-9_]*)\s*\(")


def _document() -> dict:
    return json.loads(DOCUMENT.read_bytes())


def _contract_buckets() -> dict[str, tuple[str, ...]]:
    # The names are the service contract fields, rather than physical example
    # values. They are kept here so the case checks both the platform contract
    # and the account document.
    return {
        "gateway": ("METADATA_BUCKET", "USER_BUCKET", "OG_BUCKET"),
        "user": ("S3_BUCKET", "ACCOUNT_EXPORT_BUCKET"),
        "metadata": ("S3_BUCKET", "S3_APPROVED_BUCKET", "S3_QUARANTINE_BUCKET"),
    }


def _source_files(entry: dict) -> list[Path]:
    files: list[Path] = []
    for relative in entry["sourceDirectories"]:
        directory = ROOT / relative
        for path in sorted(directory.rglob("*.go")):
            if path.name.endswith("_test.go") or path.name in DEVELOPMENT_FILES:
                continue
            if path.name == "handlers.go" and "metadata/internal/metadata" in path.relative_to(ROOT).as_posix():
                continue
            if path.name.startswith("development") or path.name.startswith("dev_"):
                continue
            if "testdata" in path.parts or "internaltest" in path.parts:
                continue
            files.append(path)
    return files


def _is_object_call(receiver: str, method: str) -> bool:
    if method in RAW_OBJECT_METHODS:
        return True
    if method not in OBJECT_METHODS:
        return False
    if receiver in {"reader", "hr", "store", "pub", "o"}:
        return True
    return any(token in receiver for token in ("Objects", "Exports", "Stores", ".store"))


def _object_calls(entry: dict) -> list[dict[str, object]]:
    calls: list[dict[str, object]] = []
    for path in _source_files(entry):
        relative = path.relative_to(ROOT).as_posix()
        for line_number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
            for match in CALL_RE.finditer(line):
                receiver, method = match.group("receiver"), match.group("method")
                if _is_object_call(receiver, method):
                    calls.append({"source": relative, "line": line_number, "call": f"{receiver}.{method}"})
    return calls


def _bucket(entry: dict, name: str) -> dict:
    return next(bucket for bucket in entry["buckets"] if bucket["name"] == name)


def _policy(service: str, buckets: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        [sys.executable, str(POLICY), service, *buckets],
        cwd=ROOT,
        text=True,
        capture_output=True,
        check=False,
    )


def test_object_rights_document() -> None:
    document = _document()
    accounts = document["accounts"]
    contracts = _contract_buckets()
    platform = json.loads(VARIABLES.read_bytes())
    config_files = {
        "gateway": ROOT / "services/launchpad/gateway/internal/config/config.go",
        "user": ROOT / "services/launchpad/user/internal/config/config.go",
        "metadata": ROOT / "services/launchpad/metadata/internal/config/config.go",
    }

    assert document["schemaVersion"] == 1
    assert set(accounts) == set(contracts) == {"gateway", "user", "metadata"}
    for service, names in contracts.items():
        contract = platform[service]
        required = set(contract["required"])
        assert {"S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} <= required
        env_names = set(re.findall(r'env:"([A-Z0-9_]+)', config_files[service].read_text(encoding="utf-8")))
        entry = accounts[service]
        assert entry["accessKey"] == "S3_ACCESS_KEY_ID"
        assert entry["secretKey"] == "S3_SECRET_ACCESS_KEY"
        assert tuple(bucket["name"] for bucket in entry["buckets"]) == names
        assert set(names) <= env_names, f"bucket contract names missing from {service} config"

    calls = document["calls"]
    discovered: list[dict[str, object]] = []
    for service, entry in accounts.items():
        found = _object_calls(entry)
        discovered.extend({"service": service, **call} for call in found)
        for call in found:
            rows = [
                row for row in calls
                if row["service"] == service
                and row["source"] == call["source"]
                and row["line"] == call["line"]
                and row["call"] == call["call"]
            ]
            assert rows, f"object call not named: {call['source']}:{call['line']} {call['call']}"

    # Every table row points at a discovered source call and a contract bucket.
    for row in calls:
        entry = accounts[row["service"]]
        names = {bucket["name"] for bucket in entry["buckets"]}
        assert row["bucket"] in names, f"{row['service']} call uses unknown bucket {row['bucket']}"
        assert any(
            call["service"] == row["service"]
            and call["source"] == row["source"]
            and call["line"] == row["line"]
            and call["call"] == row["call"]
            for call in discovered
        ), f"table names a missing call: {row['source']}:{row['line']} {row['call']}"
        bucket = _bucket(entry, row["bucket"])
        assert row["action"] in bucket["actions"], f"{row['source']}:{row['line']} action {row['action']}"
        assert row["prefix"] in bucket["prefixes"], f"{row['source']}:{row['line']} prefix {row['prefix']}"

    required_rights: dict[tuple[str, str], set[tuple[str, str]]] = {}
    for row in calls:
        required_rights.setdefault((row["service"], row["bucket"]), set()).add((row["action"], row["prefix"]))
    for service, entry in accounts.items():
        for bucket in entry["buckets"]:
            actual_actions = set(bucket["actions"])
            actual_prefixes = set(bucket["prefixes"])
            required = required_rights.get((service, bucket["name"]), set())
            assert actual_actions == {action for action, _ in required}, f"unused action in {service}/{bucket['name']}"
            assert actual_prefixes == {prefix for _, prefix in required}, f"unused prefix in {service}/{bucket['name']}"

    examples = {
        "gateway": ["media-approved", "media-user", "media-og"],
        "user": ["user-assets", "account-exports"],
        "metadata": ["metadata-legacy", "metadata-approved", "metadata-quarantine"],
    }
    for service, names in examples.items():
        result = _policy(service, names)
        assert result.returncode == 0, result.stderr
        policy = json.loads(result.stdout)
        assert all(statement["Effect"] == "Allow" for statement in policy["Statement"])
        assert all(statement["Action"] != ["*"] for statement in policy["Statement"])
        resources = [resource for statement in policy["Statement"] for resource in statement["Resource"]]
        assert all(resource != "*" for resource in resources)
        assert all(any(f":::{bucket}" in resource for bucket in names) for resource in resources)
        assert all(sum(f":::{bucket}" in resource for bucket in names) == 1 for resource in resources)
        assert all("*" not in action for statement in policy["Statement"] for action in statement["Action"])

    for service, names in examples.items():
        for supplied in (names[1:], names + ["surplus-bucket"]):
            result = _policy(service, supplied)
            assert result.returncode != 0, f"accepted wrong bucket count for {service}: {supplied}"
        result = _policy(service, ["INVALID_BUCKET", *names[1:]])
        assert result.returncode != 0, f"accepted invalid bucket for {service}"

    unknown = _policy("unknown", [])
    assert unknown.returncode != 0 and "unknown service" in unknown.stderr
