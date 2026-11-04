"""Adapter contract proofs using committed provider transcripts and owned files.

The production recovery validator and provider adapters run unchanged. Only
their native command peers use the transcript transport; this is not PITR or
provider qualification.
"""
import base64
import copy
from datetime import datetime, timedelta, timezone
import hashlib
from pathlib import Path
import re
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent
sys.path.insert(0, str(ROOT))
import recovery as r
from phase2.providers import PostgreSQL, ProviderFailure, S3Archive, file_sha
import test_recovery as contracts


class TranscriptTransport:
    """Strict command-peer transcript, with object bytes kept in memory."""

    def __init__(self, root):
        self.root = root
        self.fixture = r.load((ROOT / "testdata/repair_m718/providers.json").read_bytes())
        self.objects = {}
        self.calls = []
        self.version = self.fixture["versions"][0]
        self.kms = self.fixture["kms"]
        # The WAL executable is never executed. Its exact source identity is
        # pinned by the real restore adapter, as it would be for a native tool.
        self.tools = {"wal-reader": {"path": str(ROOT / "phase2/providers.py"),
                                      "sha256": file_sha(ROOT / "phase2/providers.py")}}

    def run(self, tool, args, *, env=None, data=None, timeout=120):
        self.calls.append((tool, args, env, timeout))
        if tool == "aws":
            return self.archive(args)
        command = [tool, *args]
        backup = str(self.root / "backup")
        restored = str(self.root / "restored")
        replacements = {"<backup>": backup, "<restored>": restored}
        expected = []
        for token in self.fixture["postgres_commands"][len([c for c in self.calls if c[0] != "aws"]) - 1]:
            for key, value in replacements.items():
                token = token.replace(key, value)
            expected.append(token)
        if command != expected:
            raise AssertionError((command, expected))
        if tool == "pg_basebackup":
            if timeout != 1800 or env["PGSERVICE"] != "qualification":
                raise AssertionError("backup authority/deadline drift")
            for relative, content in self.fixture["plain_directory"].items():
                path = self.root / "backup" / relative
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_bytes(content.encode())
        elif tool == "pg_verifybackup":
            if timeout != 300:
                raise AssertionError("unbounded backup verification")
            for relative, content in self.fixture["plain_directory"].items():
                if (Path(args[-1]) / relative).read_bytes() != content.encode():
                    raise ProviderFailure("transcript backup content changed")
        elif tool == "pg_controldata":
            return self.fixture["controldata"].encode()
        elif tool == "pg_ctl":
            if timeout != 90:
                raise AssertionError("unbounded restore startup")
        else:
            raise AssertionError("unrecorded native command: " + tool)
        return b""

    def archive(self, args):
        if args[:8] != ["--endpoint-url", "https://archive.invalid", "--region", "region-one",
                        "--output", "json", "--no-cli-pager", "s3api"]:
            raise AssertionError("archive command identity drift")
        operation = args[8]
        if args[9:11] != ["--bucket", "qualification-backups"]:
            raise AssertionError("archive bucket drift")
        options = args[11:]
        value = lambda flag: options[options.index(flag) + 1]
        if operation in self.fixture["policy"]:
            return r.canonical(self.fixture["policy"][operation])
        key = value("--key")
        if operation == "put-object":
            payload = Path(value("--body")).read_bytes()
            checksum = hashlib.sha256(payload).hexdigest()
            expected = ["--key", key, "--body", value("--body"), "--if-none-match", "*",
                        "--checksum-sha256", base64.b64encode(bytes.fromhex(checksum)).decode(),
                        "--server-side-encryption", "aws:kms", "--ssekms-key-id", self.kms,
                        "--object-lock-mode", "COMPLIANCE", "--object-lock-retain-until-date",
                        value("--object-lock-retain-until-date"), "--metadata", "sha256=" + checksum]
            if options != expected or (key, self.version) in self.objects:
                raise AssertionError("immutable upload command drift")
            metadata = {"VersionId": self.version, "ContentLength": len(payload),
                        "ChecksumSHA256": base64.b64encode(bytes.fromhex(checksum)).decode(),
                        "Metadata": {"sha256": checksum}, "ServerSideEncryption": "aws:kms",
                        "SSEKMSKeyId": self.kms, "ObjectLockMode": "COMPLIANCE",
                        "ObjectLockRetainUntilDate": value("--object-lock-retain-until-date")}
            self.objects[key, self.version] = (payload, metadata)
            return r.canonical({"VersionId": self.version})
        version = value("--version-id")
        payload, metadata = self.objects[key, version]
        expected = ["--key", key, "--version-id", version, "--checksum-mode", "ENABLED"]
        if operation == "get-object":
            expected.append(options[-1])
        if options != expected:
            raise AssertionError("pinned download command drift")
        if operation == "head-object":
            return r.canonical(metadata)
        if operation == "get-object":
            Path(options[-1]).write_bytes(payload)
            return b"{}"
        raise AssertionError("unrecorded archive operation: " + operation)


class RepairM718(unittest.TestCase):
    def test_repair_m718_opaque_version_round_trip(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            credentials = root / "credentials"
            credentials.write_text("[qualification]\n")
            credentials.chmod(0o600)
            transport = TranscriptTransport(root)
            archive = S3Archive(transport, endpoint="https://archive.invalid", region="region-one",
                                bucket="qualification-backups", profile="qualification",
                                credentials_file=credentials, kms_key_arn=transport.kms)
            source = root / "source"
            source.write_bytes(b"immutable physical backup object\x00\xff")
            retained = (datetime.now(timezone.utc) + timedelta(days=400)).isoformat()
            versions = transport.fixture["versions"] + ["A", "9" + "A_" * 100, "V" * 1024]
            hashes = set()
            for number, version in enumerate(versions):
                with self.subTest(version=version):
                    transport.version = version
                    objects = []
                    for kind in ("base", "wal"):
                        uploaded = archive.upload(source, f"backup/{number}/{kind}", retained,
                                                  kind="daily-base" if kind == "base" else "wal")
                        self.assertEqual(uploaded["version_id"].encode(), version.encode())
                        objects.append({**uploaded, "kind": kind})
                    m = contracts.manifest()
                    m["objects"] = objects
                    m["inventory_sha256"] = r.sha({"tables": m["tables"], "objects": objects})
                    trusted = r.sha(m)
                    decoded = r.load(r.canonical(m))
                    r.manifest(decoded)
                    plan = r.restore_plan(contracts.config(), decoded, trusted, 150, "12345")
                    self.assertEqual(plan["manifest_sha256"], trusted)
                    self.assertFalse(plan["traffic_cutover_allowed"])
                    for index, obj in enumerate(decoded["objects"]):
                        output = archive.download(obj, root / f"download-{number}-{index}")
                        self.assertEqual(output.read_bytes(), source.read_bytes())
                    hashes.add(m["inventory_sha256"])
                    tampered = copy.deepcopy(decoded)
                    tampered["objects"][0]["version_id"] = "different-version"
                    with self.assertRaisesRegex(r.Invalid, "inventory digest mismatch"):
                        r.manifest(tampered)
                    tampered["inventory_sha256"] = r.sha({"tables": tampered["tables"], "objects": tampered["objects"]})
                    with self.assertRaisesRegex(r.Invalid, "untrusted manifest"):
                        r.restore_plan(contracts.config(), tampered, trusted, 150, "12345")
            self.assertEqual(len(hashes), len(versions))
        # Run the production validator's negative boundaries too.
        contracts.Contracts("test_opaque_versions_remain_bound_to_inventory_and_trusted_manifest").debug()

    def test_repair_m718_plain_directory_round_trip(self):
        declaration = r.load((ROOT / "resources.json").read_bytes())
        r.resources(declaration)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            service = root / "service"
            service.write_text("[qualification]\n")
            service.chmod(0o600)
            transport = TranscriptTransport(root)
            pg = PostgreSQL(transport, service, "qualification", "coordinator")
            backup = pg.physical_backup(root / "backup")
            self.assertEqual(backup["manifest_sha256"], file_sha(root / "backup/backup_manifest"))
            # Reopen the provider with the persisted directory and manifest
            # identity. The restore consumes the actual producer's return value.
            del pg
            pg = PostgreSQL(transport, service, "qualification", "coordinator")
            result = pg.restore_physical(backup["directory"], root / "restored",
                                         target_time="2026-01-01T00:00:00Z", timeline=1, port=15432,
                                         wal_command=["wal-reader"], expected_system_id="12345",
                                         expected_manifest_sha256=backup["manifest_sha256"])
            self.assertEqual(result["system_identifier"], "12345")
            self.assertEqual(result["recovery_target_action"], "pause")
            self.assertFalse(result["traffic_cutover_allowed"])
            for relative, content in transport.fixture["plain_directory"].items():
                self.assertEqual((root / "restored" / relative).read_bytes(), content.encode())
            self.assertEqual((root / "restored").stat().st_mode & 0o777, 0o700)
            self.assertTrue((root / "restored/recovery.signal").is_file())
            configuration = (root / "restored/postgresql.conf").read_text()
            for setting in ("listen_addresses=''", "archive_mode=off", "recovery_target_action='pause'",
                            "recovery_target_timeline=1", "recovery_target_inclusive=on", "port=15432"):
                self.assertIn(setting + "\n", configuration)
            commands = declaration["command_contracts"]
            replacements = {"<isolated-owned-backup-directory>": str(root / "backup"),
                            "<isolated-owned-data-directory>": str(root / "restored")}
            expanded = {}
            for name in ("base_backup", "verify_base", "restore"):
                expanded[name] = []
                for token in commands[name]:
                    for placeholder, value in replacements.items():
                        token = token.replace(placeholder, value)
                    expanded[name].append(token)
            actual = [[tool, *args] for tool, args, _, _ in transport.calls]
            self.assertEqual(actual[0], expanded["base_backup"])
            self.assertEqual(actual[1], expanded["verify_base"])
            self.assertEqual(actual[2], expanded["verify_base"])
            self.assertEqual(actual[-1], expanded["restore"])
            # Identity failure must prevent both directory copying and startup.
            (root / "backup/backup_manifest").write_text("corrupt")
            calls = len(transport.calls)
            with self.assertRaisesRegex(ProviderFailure, "physical backup manifest changed"):
                pg.restore_physical(backup["directory"], root / "rejected",
                                    target_time="2026-01-01T00:00:00Z", timeline=1, port=15433,
                                    wal_command=["wal-reader"], expected_system_id="12345",
                                    expected_manifest_sha256=backup["manifest_sha256"])
            self.assertFalse((root / "rejected").exists())
            self.assertEqual(len(transport.calls), calls)
        contracts.Contracts("test_declarative_resources_cover_owned_requirements_without_claiming_activation").debug()
        contracts.Contracts("test_resource_validation_fails_closed_for_every_requirement_group").debug()

    def test_repair_m718_command_paths(self):
        text = (ROOT / "README.md").read_text()
        for obsolete in ("deploy/production/", "tools/production/"):
            self.assertNotIn(obsolete, text)
        paths = re.findall(
            r"(?:deploy|tools)/launchpad/production/[A-Za-z0-9_./-]+", text
        )
        self.assertGreaterEqual(len(paths), 3)
        for path in paths:
            with self.subTest(path=path):
                self.assertTrue((ROOT.parents[3] / path).exists(), path)
        self.assertIn(
            "python3 deploy/launchpad/production/recovery/check.py\n", text
        )
        checker = (ROOT / "check.py").read_text()
        for condition in (
            'discover(str(root),pattern="test_*.py")',
            "result.wasSuccessful() and result.testsRun>=15 and not result.skipped",
        ):
            self.assertIn(condition, checker)
        for safety in (
            "Compose project/service labels", "schema ownership token",
            "exclusively assigned namespace", "Each subprocess has a deadline",
            "independently trusted inventory", "traffic cutover disabled",
            "No extraction", "--format=plain", "--manifest-checksums=SHA256",
            "--timeout=60",
        ):
            with self.subTest(safety=safety):
                self.assertIn(safety, text)
