"""Offline source and HTTP-transcript proof for rollout audit findings 016–018.

The production parser, planner, GraphQL transport and hosted apply functions run
in process. Shell deployment, systemd recovery and live wake/probing do not run.
"""

import ast
from contextlib import contextmanager, redirect_stderr, redirect_stdout
import copy
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import threading
import tomllib
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest


ROOT = Path(__file__).resolve().parents[3]
TESTS = ROOT / "tools/rollout/tests"
RECORD = Path(__file__).with_name("repair_22_33_734.kvx")
SHELL = (TESTS / "rollout.test.sh").read_text()
REPLAY = json.loads((TESTS / "replay.json").read_text())
RETIRED = {"router", "sequencer-primary", "sequencer-standby"}
SHA = REPLAY["sha"]
TOKEN = "sentinel-railway-token"
PROJECT = "25709f39-612a-5373-86a3-2bc9e4478de1"


def load_rollout():
    spec = importlib.util.spec_from_file_location(
        "rollout_repair_22_33_734", ROOT / "tools/rollout/rollout.py"
    )
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def replay_python():
    blocks = re.findall(r"<<'PY'\n(.*?)\nPY", SHELL, re.S)
    assert len(blocks) == 2
    return ast.parse(blocks[1])


def wake_validator():
    function = next(
        node for node in replay_python().body
        if isinstance(node, ast.FunctionDef) and node.name == "validate_wake_health"
    )
    namespace = {}
    exec(compile(ast.Module(body=[function], type_ignores=[]), str(TESTS / "rollout.test.sh"), "exec"), namespace)
    return namespace[function.name]


@contextmanager
def environment(values):
    previous = dict(os.environ)
    try:
        for name, value in values.items():
            if value is None:
                os.environ.pop(name, None)
            else:
                os.environ[name] = value
        yield
    finally:
        os.environ.clear()
        os.environ.update(previous)


@contextmanager
def transcript(rollout, calls):
    """Serve committed GraphQL replies and check every real HTTP request."""
    pending = copy.deepcopy(calls)
    seen = []
    errors = []

    class Railway(BaseHTTPRequestHandler):
        def log_message(self, *args):
            pass

        def do_POST(self):
            body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            match = re.match(r"\s*(?:query|mutation)\s+(\w+)", body["query"])
            request = {"op": match and match.group(1), "variables": body["variables"]}
            seen.append(request)
            want = pending.pop(0) if pending else None
            if self.headers.get("Authorization") != "Bearer " + TOKEN:
                errors.append("wrong authorization")
            if want is None or request != {"op": want["op"], "variables": want["variables"]}:
                errors.append(f"unexpected request {request}; expected {want}")
                reply = {"errors": [{"message": "transcript mismatch"}]}
            elif "errors" in want:
                reply = {"errors": want["errors"]}
            else:
                reply = {"data": want["data"]}
            data = json.dumps(reply).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(data)))
            self.end_headers()
            self.wfile.write(data)

    server = ThreadingHTTPServer(("127.0.0.1", 0), Railway)
    worker = threading.Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01})
    old_api = rollout.API
    rollout.API = f"http://127.0.0.1:{server.server_port}/graphql/v2"
    worker.start()
    try:
        with environment({"RAILWAY_API_TOKEN": TOKEN, "RAILWAY_PROJECT_ID": PROJECT,
                          "ROLLOUT_POLL": "0", "ROLLOUT_TIMEOUT": "2"}):
            yield seen
        assert not errors, errors
        assert not pending, pending
        assert len(seen) == len(calls)
    finally:
        rollout.API = old_api
        server.shutdown()
        server.server_close()
        worker.join(timeout=2)
        assert not worker.is_alive()


def manifest_for(tmp_path, case):
    path = tmp_path / "replay.kvx"
    path.write_text("\n".join(case["manifest"]) + "\n")
    return path


def drive_hosted_case(tmp_path, name):
    """Use the actual project/instance/apply/wait transport without shell ledger IO."""
    rollout = load_rollout()
    case = REPLAY["scenarios"][name]
    path = manifest_for(tmp_path, case)
    units = rollout.services(path)
    meta = rollout.manifest_meta(path)
    selected = case["argv"][case["argv"].index("--only") + 1] if "--only" in case["argv"] else None
    units = [unit for unit in units if selected is None or unit["name"] in selected.split(",")]
    outcomes = []
    log = io.StringIO()
    with transcript(rollout, case["calls"]) as seen:
        names, env = rollout.railway_ids(meta["environment"], meta["project"])
        for unit in units:
            try:
                if unit["target"] == "railway-image":
                    outcome = rollout.apply_railway_image(
                        unit, rollout.image_ref(unit, REPLAY["digests"]), names, env, log
                    )
                else:
                    outcome = rollout.apply_railway(unit, SHA, names, env, log)
            except RuntimeError as error:
                log.write("error: " + rollout.redact(error) + "\n")
                outcome = "fail"
            outcomes.append(f"{outcome} {unit['name']}")
            if outcome != "pass":
                break
    assert outcomes == case["out"]
    assert (0 if all(row.startswith("pass ") for row in outcomes) else 1) == case["exit"]
    return log.getvalue(), seen


def test_016_retained_manifest_and_real_plan(tmp_path):
    rollout = load_rollout()
    path = TESTS / "retained.kvx"
    units = {unit["name"]: unit for unit in rollout.services(path)}
    assert len(units) == 24
    assert not RETIRED.intersection(units)
    assert not {"kernel", "intent-ingester", "oracle-feeder", "perps-keeper"}.intersection(units)
    assert units["paxd"] == {
        "name": "paxd", "target": "box", "image": "paxd", "box_role": "paxd",
        "box_unit": "paxd.service", "health": "tcp:8545", "order": 3,
    }
    assert units["gateway"] == {
        "name": "gateway", "target": "railway", "image": "layerx-gateway",
        "railway_service": "gateway", "health": "/healthz", "order": 5,
    }
    assert units["archive"]["image"] == "paxd"
    assert units["archive"]["box_unit"] == "paxd.service"
    assert units["archive"]["health"] == "tcp:8545"
    for first, second in [("paxd", "program-registry"), ("paxd", "archive"),
                          ("wallet-postgres", "wallet-gateway"),
                          ("webhooks-ingress", "webhooks-public"), ("gas-station", "ai-evaluator")]:
        assert units[first]["order"] < units[second]["order"]
        assert f'plan_precedes {first} {second} "$tmp/plan" || fail ' in SHELL
    output = io.StringIO()
    with environment({"ROLLOUT_MANIFEST": str(path), "ROLLOUT_DRY_RUN": "1"}), redirect_stdout(output):
        assert rollout.main(["plan", "--sha", SHA]) == 0
    rows = [line.split() for line in output.getvalue().splitlines()[1:]]
    assert {row[1] for row in rows} == set(units)
    assert [int(row[0]) for row in rows] == sorted(unit["order"] for unit in units.values())
    assert any(row[1:5] == ["paxd", "box", "paxd", f"{rollout.GHCR}/paxd:{SHA[:12]}"] for row in rows)
    assert 'host=$PAXD_HOST health=tcp:8545' in output.getvalue()
    assert f'identity commit={SHA} health=/healthz' in output.getvalue()


def test_016_shell_guards_keep_image_dependency_and_recovery_checks():
    # bash -n is executed by the single declared hf-build, never a second subprocess here.
    assert 'manifest="$here/retained.kvx"\nexport ROLLOUT_MANIFEST="$manifest"' in SHELL
    for retired in RETIRED:
        assert f'$2 == "{retired}"' in SHELL
    assert 'for s in router sequencer-primary sequencer-standby kernel ' in SHELL
    for guard in [
        '[ "${#want[@]}" -eq 24 ] || fail',
        '[ "$status" -eq 2 ] || fail',
        '[ ! -s "$tmp/retired.out" ] || fail',
        'grep -qxF "unknown services: $s" "$tmp/retired.err" || fail',
        'unit=paxd\\.service host=\\\\\\$PAXD_HOST health=tcp:8545$',
        'if plan_precedes paxd program-registry "$tmp/paxd-last.plan"; then',
        'if plan_precedes paxd archive "$tmp/archive-$order.plan"; then',
        'for order in 2 3; do',
        'for name in unquoted missing target extra duplicate order health orphan section empty;',
        'digest map has no digest for: layerx-gateway',
        '--digests is required for railway-image services',
        'ledger accepted evidence outside /root/lx-ops',
        'if outcomes != want_outcomes:',
        'if case.get("secrets") and "[redacted]" not in evidence:',
        'if code == 2 and any(re.match',
        'validate_wake_health(case)',
    ]:
        assert guard in SHELL, guard
    tree = replay_python()
    assert any(isinstance(node, ast.ClassDef) and node.name == "Railway" for node in tree.body)
    calls = [ast.unparse(node.func) for node in ast.walk(tree) if isinstance(node, ast.Call)]
    assert "rollout.main" in calls
    assert "validate_wake_health" in calls
    assert "server.shutdown" in calls
    for name in ["rerun-after-crash-redeploys", "rerun-after-crash-crashes-again", "rerun-healthy-is-unchanged"]:
        assert name in REPLAY["scenarios"]


def test_017_all_replay_manifests_and_project_targets_are_retained(tmp_path):
    rollout = load_rollout()
    for name, case in REPLAY["scenarios"].items():
        units = rollout.services(manifest_for(tmp_path, case))
        assert not RETIRED.intersection(unit["name"] for unit in units), name
        assert not RETIRED.intersection(unit.get("railway_service") for unit in units), name
        assert all(unit["image"] != "layerx-router" for unit in units)
        for call in case["calls"]:
            if call["op"] == "Project" and call.get("data", {}).get("project"):
                names = {edge["node"]["name"] for edge in call["data"]["project"]["services"]["edges"]}
                assert not RETIRED.intersection(names), name
        assert not any(outcome.split()[1] in RETIRED for outcome in case.get("out", []))
    case = REPLAY["scenarios"]["first-apply"]
    assert case["out"] == ["pass gateway", "pass ai-evaluator"]
    assert any(call["op"] == "DeployCommit" and call["variables"]["c"] == SHA for call in case["calls"])
    drive_hosted_case(tmp_path, "first-apply")


@pytest.mark.parametrize("mode", ["plan", "apply"])
@pytest.mark.parametrize("retired", sorted(RETIRED))
@pytest.mark.parametrize("mixed", [False, True])
def test_017_real_retired_selection_refuses_before_transport(tmp_path, mode, retired, mixed):
    selection = retired + (",gateway" if mixed else "")
    name = f"retired-{mode}-{selection.replace(',', '-')}"
    case = REPLAY["scenarios"][name]
    assert case["exit"] == 2 and case["calls"] == []
    assert case["err"] == "unknown services: " + retired
    path = manifest_for(tmp_path, case)
    rollout = load_rollout()
    out, err = io.StringIO(), io.StringIO()
    # No token, project, digest file or endpoint is needed: refusal precedes IO.
    with environment({"ROLLOUT_MANIFEST": str(path), "RAILWAY_API_TOKEN": None,
                      "RAILWAY_PROJECT_ID": None, "ROLLOUT_DRY_RUN": None}), redirect_stdout(out), redirect_stderr(err):
        assert rollout.main(case["argv"]) == 2
    assert out.getvalue() == ""
    assert err.getvalue().splitlines() == [case["err"]]


@pytest.mark.parametrize("name", [
    "rerun-after-crash-redeploys", "rerun-after-crash-crashes-again",
    "rerun-healthy-is-unchanged", "rerun-removing-redeploys",
    "rerun-waits-for-running-deployment", "rerun-deployment-runs-old-image",
    "rerun-deployment-predates-health", "health-only-change", "needs-approval-blocks",
    "service-missing-blocks", "invalid-input-error-redacted", "long-error-cut-redacted",
])
def test_017_real_recovery_idempotence_and_secret_redaction(tmp_path, name):
    log, seen = drive_hosted_case(tmp_path, name)
    case = REPLAY["scenarios"][name]
    for secret in case.get("secrets", []):
        assert secret not in log
    for secret, marker in case.get("marked", {}).items():
        if marker == "cut":
            assert re.search(r"\.\.\. \(\d+ more characters\)", log)
        else:
            assert marker in log
    if case.get("secrets"):
        assert set(case["marked"]) == set(case["secrets"])
        assert "[redacted]" in log
    if name == "rerun-healthy-is-unchanged":
        assert [call["op"] for call in seen] == ["Project", "Instance", "Deployment"]
        assert "unchanged image " in log
    if name == "rerun-after-crash-redeploys":
        assert [call["op"] for call in seen] == ["Project", "Instance", "Deploy", "Deployment"]
        assert "redeploying" in log


@pytest.mark.parametrize("name", [
    "rerun-sleeping-is-unchanged", "rerun-sleeping-woke-healthy",
    "rerun-sleeping-wake-unavailable", "rerun-sleeping-wake-timed-out",
])
def test_018_wake_health_contract_and_real_terminal_policy(tmp_path, name):
    case = REPLAY["scenarios"][name]
    validate = wake_validator()
    validate(case)
    proof = case["wake_health"]
    assert proof["path"] == "/healthz" and proof["required_for_pass"] is True
    if name.endswith("woke-healthy"):
        assert proof["state"] == "ready"
        for broken_observations in [[], [{"event": "wake", "result": "success"}],
                                    [{"event": "health", "method": "GET", "path": "/healthz", "status": 200}]]:
            broken = copy.deepcopy(case)
            broken["wake_health"]["observations"] = broken_observations
            with pytest.raises(AssertionError):
                validate(broken)
    else:
        assert case["exit"] == 1 and case["out"] == ["blocked gateway"]
        assert proof["state"] in {"unavailable", "timed-out"}
        broken = copy.deepcopy(case)
        broken["exit"], broken["out"] = 0, ["pass gateway"]
        with pytest.raises(AssertionError):
            validate(broken)
    log, seen = drive_hosted_case(tmp_path, name)
    assert [call["op"] for call in seen] == ["Project", "Instance", "Deployment"]
    assert "SLEEPING" in log
    if case["exit"] == 0:
        assert "SUCCESS" in log
    assert load_rollout().TERMINAL["SLEEPING"] == "blocked"


def test_record_maps_each_finding_and_states_scope_and_proof_limits():
    record = tomllib.loads(RECORD.read_text())
    assert set(record["finding"]) == {"016", "017", "018"}
    functions = {node.name for node in ast.parse(Path(__file__).read_text()).body if isinstance(node, ast.FunctionDef)}
    for finding in record["finding"].values():
        assert finding["tests"]
        assert set(finding["tests"]).issubset(functions)
    task = record["task"]["22"]["33"]["734"]
    assert task["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert "No pre-change verification" in task["baseline"]
    assert "review" in task["proof_limits"]
    assert len(task["wiring_requests"]) == 2
    assert all("domain 16" in request for request in task["wiring_requests"])
