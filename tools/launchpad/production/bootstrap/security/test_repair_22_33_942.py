"""Exercise the inventory producer and check the map against committed sources.

These documentation regressions do not execute the inventoried Go suites or
qualify a scanner deployment. The real generator's stdout is the JSON interface
under test; no scanner implementation or transport is replaced in Python.
"""

from contextlib import redirect_stdout
import hashlib
from io import StringIO
import json
from pathlib import Path
import re
import runpy
import shlex

import pytest


DIRECTORY = Path(__file__).resolve().parent
ROOT = DIRECTORY.parents[4]


@pytest.fixture(scope="module")
def inventory():
    documentation = (DIRECTORY / "GAPS.md").read_text()
    command = re.search(r"```sh\n([^\n]+)\n```", documentation)
    assert command is not None, "the map must provide a repository-root command"
    argv = shlex.split(command.group(1))
    assert argv == [
        "python3",
        "tools/launchpad/production/bootstrap/security/build_manifest.py",
    ]
    manifest = DIRECTORY / "manifest.json"
    before = manifest.read_bytes()
    output = StringIO()
    with redirect_stdout(output):
        runpy.run_path(str(ROOT / argv[1]), run_name="__main__")
    assert manifest.read_bytes() == before, "inventory generation must not publish"
    return documentation, json.loads(output.getvalue())


def test_security_map_generator_path_emits_reconcilable_inventory(inventory):
    documentation, emitted = inventory
    assert emitted["schema_version"] == 1
    assert "not a production security certificate" in emitted["scope"]
    checks = emitted["checks"]
    assert len(checks) == 13
    assert len({row["id"] for row in checks}) == len(checks)
    for row in checks:
        assert (ROOT / row["cwd"] / "go.mod").is_file()
        assert row["execution_status"] == "not_executed_by_inventory"
        assert row["requires"]
        assert row["skip_policy"] == (
            "Any skip, zero executed test cases, build failure or nonzero exit "
            "fails qualification."
        )
        command = shlex.split(row["command"])
        assert command[:8] == [
            "GOWORK=off", "GOMAXPROCS=2", "go", "test", "-p", "1",
            "-count=1", "-json",
        ]
        packages = [arg for arg in command if arg.startswith("./")]
        assert packages
        assert row["sources"], row["id"]
        for source in row["sources"]:
            path = ROOT / source["path"]
            assert path.parent in {ROOT / row["cwd"] / p for p in packages}
            assert source["sha256"] == hashlib.sha256(path.read_bytes()).hexdigest()
            assert source["tests"] == re.findall(
                r"^func (Test\w+)\(t \*testing.T\)", path.read_text(), re.M
            )
        assert any(source["tests"] for source in row["sources"])
    assert "tools/launchpad/production/bootstrap/security/manifest.json" in documentation
    for claim in (
        "Capture stdout as a candidate", "Before replacing", "SHA-256 hashes",
        "dependency prerequisites", "separate execution evidence",
        "not_executed_by_inventory", "does not update the checked-in manifest",
    ):
        assert claim in documentation
    assert "tools/production/bootstrap/security/build_manifest.py" not in documentation


def test_security_map_scanner_behaviour_matches_current_assertions(inventory):
    documentation, emitted = inventory
    scanner = (ROOT / "sdk/launchpad/shared/http/virusscan.go").read_text()
    http_tests = (ROOT / "sdk/launchpad/shared/http/http_test.go").read_text()
    upload_tests = (
        ROOT / "services/launchpad/gateway/internal/upload/scan_unavailable_test.go"
    ).read_text()
    indexed_tests = {
        name for row in emitted["checks"] for source in row["sources"]
        for name in source["tests"]
    }
    for name in (
        "TestScanBufferNotConfigured", "TestScanBufferUnreachable",
        "TestScanBufferClean", "TestScanBufferInfected",
        "TestUpload_ScannerUnavailableFailsClosed503",
    ):
        assert name in documentation
        assert name in indexed_tests, "the current producer must inventory the assertion"
    for reason in ("clamav-not-configured", "clamav-unreachable"):
        assert f'return fail(ScanUnavailable, "{reason}")' in scanner
        assert f'res.Reason != "{reason}"' in http_tests
        assert reason in documentation
    assert http_tests.count(
        'res.Clean || res.Verdict != ScanUnavailable || res.Reason != '
    ) == 2
    assert http_tests.count('res.ContentHash != "" || res.Size != 0') >= 2
    for claim in ("`Clean: false`", "empty content hash", "zero size"):
        assert claim in documentation
    assert 'host := opts.Host' in scanner
    assert 'host = os.Getenv("CLAMAV_HOST")' in scanner
    assert "uses `ScanOptions.Host`, then `CLAMAV_HOST`" in documentation
    assert 'if reply == "stream: OK" {' in scanner
    assert scanner.count("r.Clean = true") == 1
    assert "Only the complete ClamAV\n`stream: OK` verdict" in documentation
    assert "rec.Code != http.StatusServiceUnavailable" in upload_tests
    assert 'strings.Contains(rec.Body.String(), tc.reason)' in upload_tests
    assert "if up.called {" in upload_tests
    assert 'logs.String()' in upload_tests
    for claim in ("HTTP 503", "masked client errors", "logged reasons", "no forwarding"):
        assert claim in documentation
    verified = scanner.split("func ScanVerifiedBuffer(", 1)[1]
    assert verified.index("ScannerVersion(ctx, opts)") < verified.index("ScanBuffer(")
    assert verified.index("if result.Verdict != ScanClean") < verified.index("after, e :=")
    assert "after.Engine != verified.Engine" in verified
    assert "after.SignatureRevision != verified.SignatureRevision" in verified
    for reason in ("clamav-version-unavailable", "clamav-version-changed"):
        assert f'ScanResult{{Verdict: ScanUnavailable, Reason: "{reason}"}}' in verified
        assert reason in documentation
    assert "Only after a `CLEAN` result" in documentation


def test_security_map_scanner_qualification_remains_explicit(inventory):
    documentation, emitted = inventory
    prose = " ".join(documentation.split())
    for claim in (
        "fixed protocol responses", "do not execute live ClamAV/EICAR scanning",
        "real-scanner version and EICAR evidence", "timeout/malformed-response",
        "failed-dependency evidence", "failed-scan storage-denial evidence",
        "no object-store writes or metadata side effects",
        "forwarding assertion alone does not prove these storage-side-effect bounds",
        "reject skips, unavailable fixtures, missing dependencies, stale hashes",
        "zero matching tests",
    ):
        assert claim in prose
    for obsolete in ("Scanner fails open", "Clean:true", "Clean: true when unconfigured"):
        assert obsolete not in documentation
    for ident in ("http-security", "gateway-session"):
        row = next(row for row in emitted["checks"] if row["id"] == ident)
        assert "Docker daemon and redis:7-alpine image for Redis-backed cases" in row["requires"]
    assert "Every inventory row starts `not_executed_by_inventory`" in documentation
