"""Exercise the production inventory writer and reader without a Go toolchain."""

import importlib.util
import json
import os
from pathlib import Path
import shlex

import pytest


ROOT = Path(__file__).resolve().parents[3]
HELPER = ROOT / "tools/launchpad/coverage_inventory.py"
GATE = ROOT / "tools/launchpad/coverage-gate.sh"
MAKEFILE = ROOT / "services/launchpad/Makefile"
RECORD = ROOT / "tools/qualification/audit_repairs/repair_22_33_1017.kvx"
spec = importlib.util.spec_from_file_location("coverage_inventory", HELPER)
inventory = importlib.util.module_from_spec(spec)
spec.loader.exec_module(inventory)

MODULES = ("../../sdk/launchpad/shared", "indexer", "api")
# Committed Go coverage-format inputs: every statement is covered, so these
# profiles exceed both production thresholds before inventory checks apply.
PASSING_PROFILE = "mode: atomic\nexample.org/launchpad/value.go:3.18,3.30 1 1\n"
EMPTY_PROFILE = "mode: atomic\n"


def new_run(tmp_path, modules=MODULES):
    cover = tmp_path / ".cover"
    manifest = cover / "run.json"
    token = inventory.begin_run(cover, manifest, modules)
    return cover, manifest, token


def write_profile(cover, module, contents=PASSING_PROFILE):
    path = cover / inventory.profile_name(module)
    path.write_text(contents)
    return path


def complete_profiles(cover, manifest, token, modules=MODULES):
    for module in modules:
        write_profile(cover, module)
        inventory.record_result(cover, manifest, token, module)
    inventory.finish_run(cover, manifest, token)


def test_coverage_module_inventory_rejects_missing_money_module(tmp_path):
    cover, manifest, token = new_run(tmp_path)
    for module in MODULES[1:]:
        profile = write_profile(cover, module)
        rows = profile.read_text().splitlines()[1:]
        assert all(int(row.split()[-1]) > 0 for row in rows)
        inventory.record_result(cover, manifest, token, module)
    with pytest.raises(inventory.InventoryError, match="missing.*module results"):
        inventory.finish_run(cover, manifest, token)
    assert "completed_ns" not in json.loads(manifest.read_text())
    with pytest.raises(inventory.InventoryError, match="incomplete"):
        inventory.check_run(cover, manifest, token, MODULES)


def test_real_manifest_producer_transport_consumer(tmp_path, capsys):
    cover = tmp_path / ".cover"
    manifest = cover / "run.json"
    args = [str(cover), str(manifest)]
    assert inventory.main(["begin", *args, "--modules", " ".join(MODULES)]) == 0
    token = capsys.readouterr().out.strip()
    assert token
    for module in MODULES:
        write_profile(cover, module)
        assert inventory.main(["record", *args, "--run-id", token, "--module", module]) == 0
    assert inventory.main(["finish", *args, "--run-id", token]) == 0
    assert inventory.main([
        "check", *args, "--run-id", token, "--modules", " ".join(MODULES)
    ]) == 0
    stored = json.loads(manifest.read_text())
    assert stored["run_id"] == token
    assert set(stored["results"]) == set(MODULES)
    for result in stored["results"].values():
        assert result["status"] == "profile"
        assert len(result["profile"]["sha256"]) == 64
    assert inventory.check_run(cover, manifest, token, reversed(MODULES)) == []


@pytest.mark.parametrize("empty_reason", ["no-packages", "no-statements"])
def test_empty_modules_require_explicit_results(tmp_path, empty_reason, capsys):
    cover, manifest, token = new_run(tmp_path, ("api",))
    if empty_reason == "no-statements":
        write_profile(cover, "api", EMPTY_PROFILE)
        with pytest.raises(inventory.InventoryError, match="explicit empty"):
            inventory.record_result(cover, manifest, token, "api")
    inventory.record_result(cover, manifest, token, "api", empty_reason)
    inventory.finish_run(cover, manifest, token)
    expected = "no packages" if empty_reason == "no-packages" else "no measurable statements"
    assert inventory.check_run(cover, manifest, token, ("api",)) == [("api", expected)]
    assert inventory.main([
        "check", str(cover), str(manifest), "--modules", "api", "--run-id", token
    ]) == 0
    assert f"api: EMPTY ({expected})" in capsys.readouterr().out


def test_stale_profile_is_rejected_at_production_recording(tmp_path):
    cover, manifest, token = new_run(tmp_path, ("indexer",))
    path = write_profile(cover, "indexer")
    start = json.loads(manifest.read_text())["started_ns"]
    os.utime(path, ns=(start - 1_000_000_000, start - 1_000_000_000))
    with pytest.raises(inventory.InventoryError, match="stale profile: indexer"):
        inventory.record_result(cover, manifest, token, "indexer")


def test_zero_statement_blocks_are_explicitly_empty(tmp_path):
    cover, manifest, token = new_run(tmp_path, ("api",))
    write_profile(cover, "api", "mode: atomic\nexample.org/empty.go:3.18,3.20 0 1\n")
    inventory.record_result(cover, manifest, token, "api", "no-statements")
    inventory.finish_run(cover, manifest, token)
    assert inventory.check_run(cover, manifest, token, ("api",)) == [
        ("api", "no measurable statements")
    ]


def test_prior_complete_manifest_cannot_substitute_for_new_run(tmp_path):
    cover, manifest, old_token = new_run(tmp_path)
    complete_profiles(cover, manifest, old_token)
    old_manifest = manifest.read_bytes()
    new_token = inventory.begin_run(cover, manifest, MODULES)
    assert new_token != old_token
    manifest.write_bytes(old_manifest)
    with pytest.raises(inventory.InventoryError, match="run id mismatch"):
        inventory.check_run(cover, manifest, new_token, MODULES)


@pytest.mark.parametrize("change", ["missing", "modified", "retained", "inventory", "incomplete"])
def test_complete_manifest_rejects_invalid_consumption(tmp_path, change):
    cover, manifest, token = new_run(tmp_path)
    complete_profiles(cover, manifest, token)
    expected = MODULES
    if change == "missing":
        (cover / inventory.profile_name(MODULES[0])).unlink()
        message = "missing regular profile"
    elif change == "modified":
        write_profile(cover, MODULES[0], PASSING_PROFILE.replace(" 1 1", " 1 0"))
        message = "changed since recording"
    elif change == "retained":
        write_profile(cover, "old-module")
        message = "unrecorded or retained"
    elif change == "inventory":
        expected = (*MODULES, "pnl-tracker")
        message = "differs from expected"
    else:
        stored = json.loads(manifest.read_text())
        del stored["completed_ns"]
        manifest.write_text(json.dumps(stored))
        message = "incomplete"
    with pytest.raises(inventory.InventoryError, match=message):
        inventory.check_run(cover, manifest, token, expected)


def test_empty_declaration_cannot_hide_code_or_retained_profile(tmp_path):
    cover, manifest, token = new_run(tmp_path, ("indexer",))
    write_profile(cover, "indexer")
    with pytest.raises(inventory.InventoryError, match="retained profile"):
        inventory.record_result(cover, manifest, token, "indexer", "no-packages")
    with pytest.raises(inventory.InventoryError, match="accurate explicit empty"):
        inventory.record_result(cover, manifest, token, "indexer", "no-statements")


def test_profile_added_after_empty_result_is_rejected(tmp_path):
    cover, manifest, token = new_run(tmp_path, ("indexer",))
    inventory.record_result(cover, manifest, token, "indexer", "no-packages")
    inventory.finish_run(cover, manifest, token)
    write_profile(cover, "indexer")
    with pytest.raises(inventory.InventoryError, match="unrecorded or retained"):
        inventory.check_run(cover, manifest, token, ("indexer",))


@pytest.mark.parametrize("contents", ["", "mode: nope\n", "mode: atomic\ninvalid\n"])
def test_malformed_profiles_fail_closed(tmp_path, contents):
    cover, manifest, token = new_run(tmp_path, ("api",))
    write_profile(cover, "api", contents)
    with pytest.raises(inventory.InventoryError, match="invalid coverage"):
        inventory.record_result(cover, manifest, token, "api")


def test_duplicate_and_colliding_inventory_is_rejected(tmp_path):
    for modules in ((), ("api", "api"), ("a/b", "a_b")):
        with pytest.raises(inventory.InventoryError):
            new_run(tmp_path, modules)


def test_duplicate_manifest_keys_are_rejected(tmp_path):
    cover, manifest, token = new_run(tmp_path)
    manifest.write_text('{"version":1,"version":1}')
    with pytest.raises(inventory.InventoryError, match="duplicate manifest key"):
        inventory.check_run(cover, manifest, token, MODULES)


def test_failed_cli_check_is_an_error(tmp_path, capsys):
    with pytest.raises(SystemExit) as error:
        inventory.main(["check", str(tmp_path), str(tmp_path / "absent.json"), "--modules", "api"])
    assert error.value.code == 1
    assert "cannot read run manifest" in capsys.readouterr().err


def test_shell_guards_measurement_with_complete_inventory():
    source = GATE.read_text()
    logical = source.replace("\\\n", " ")
    command = next(line for line in logical.splitlines() if line.startswith("python3 "))
    words = shlex.split(command)
    assert words[:3] == ["python3", "$SCRIPT_DIR/coverage_inventory.py", "check"]
    assert words[-3:] == ["||", "exit", "1"]
    for key in ("RUN_MANIFEST", "COVER_RUN_ID", "MODULES"):
        assert any("${" + key + ":?" in word for word in words)
    assert words[3] == "$COVER_DIR"
    assert "--modules" in words and "--run-id" in words
    assert source.index(command.splitlines()[0][:30]) < source.index("for mod in $MODULES;")
    assert source.index("for mod in $MODULES;") < source.index("go tool cover")
    assert 'prof="$COVER_DIR/${mod//\\//_}.out"' in source
    assert 'MONEY_MIN="${MONEY_MIN:-90.0}"' in source
    assert 'DEFAULT_MIN="${DEFAULT_MIN:-85.0}"' in source
    assert 'if is_money "$mod"; then min="$MONEY_MIN"; else min="$DEFAULT_MIN"; fi' in source
    assert 'pct="$(go tool cover -func="$fprof"' in source
    assert 'fprof="$(filter_generated "$prof" "$mod")"' in source
    assert "exit !(p+0 >= m+0)" in source
    assert 'if [ "$fail" -ne 0 ]; then' in source
    assert "NR > 1 && $2 > 0 {found=1} END {exit !found}" in source


def test_makefile_boundary_and_wiring_request_are_explicit():
    source = MAKEFILE.read_text().replace("\\\n", " ")
    modules = next(line for line in source.splitlines() if line.startswith("MODULES :=")).split(":=", 1)[1].split()
    money = next(line for line in source.splitlines() if line.startswith("MONEY_MODULES :=")).split(":=", 1)[1].split()
    assert {"indexer", "pnl-tracker", "../../sdk/launchpad/shared"} <= set(money) <= set(modules)
    assert "cover-check: cover" in source
    assert 'TEST_RUNNER := $(abspath ../../tests/ci/data/launchpad_go_tests.py)' in source
    assert 'python3 $(TEST_RUNNER) --module "$$m" --go "$(GO)" --flags="$(GOFLAGS)" --profile "$(CURDIR)/.cover/$$name.out";' in source
    runner = (ROOT / 'tests/ci/data/launchpad_go_tests.py').read_text()
    assert '"-covermode=atomic"' in runner
    assert 'f"-coverprofile={part}"' in runner
    assert 'Path(profile).write_text("\\n".join(lines) + "\\n")' in runner
    record = RECORD.read_text()
    assert '[finding.coverage-module-inventory]' in record
    assert 'test_coverage_module_inventory_rejects_missing_money_module' in record
    assert 'services/launchpad/Makefile' in record
    assert 'owner_domain = 4' in record
    assert 'baseline_not_run' in record
