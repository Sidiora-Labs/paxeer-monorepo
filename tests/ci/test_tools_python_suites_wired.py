"""Keep the tools Python suites on the main CI execution path."""

import ast
import json
import os
from pathlib import Path
import shlex
import subprocess
import sys

import yaml


ROOT = Path(__file__).resolve().parents[2]
BASELINE = "tests/ci/data/tools_python_suite_baseline.json"
SUITES = (
    "tools/release/tests",
    "tools/release/rehearsal",
    "tools/bringup/tests",
    "tools/containers",
    "tools/deps/tests",
    "tools/rollout",
    "tools/deploy",
    "tools/docs/gen",
    "tools/spec",
    "tools/benchmark",
    "tools/jev/tests",
    "tools/jev/checks",
    "tools/paxeer-x/tests",
)


def test_tools_python_target_lists_suites():
    result = subprocess.run(
        ["make", "--no-print-directory", "-n", "test-tools-python"],
        cwd=ROOT,
        env={key: value for key, value in os.environ.items()
             if key not in {"MAKEFLAGS", "MFLAGS", "MAKEOVERRIDES"}},
        capture_output=True, text=True, timeout=30,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    commands = [shlex.split(line) for line in result.stdout.splitlines() if line.strip()]
    assert len(commands) == 1
    command = commands[0]
    assert command[:2] == ["python3", "-c"]
    assert command[3:] == list(SUITES)
    runner = ast.parse(command[2])
    assert any(isinstance(node, ast.Call) and isinstance(node.func, ast.Name)
               and node.func.id == "open" and node.args[0].value == BASELINE
               for node in ast.walk(runner))
    assert '["python3", "-m", "pytest", "-q", *sys.argv[1:]' in command[2]
    assert '"--deselect=" + case["nodeid"] for case in cases' in command[2]
    for suite in SUITES:
        assert any(
            path.name.startswith("test_") or path.name.endswith("_test.py")
            for path in (ROOT / suite).rglob("*.py")
        ), suite
    makefile = (ROOT / "Makefile").read_text()
    assert ".PHONY: test-tools-python" in makefile


def test_tools_python_baseline_tracks_exact_cases():
    cases = json.loads((ROOT / BASELINE).read_text())
    expected = set()
    for module, cls in (("ai_evidence_test.py", "EvidenceTests"),
                        ("gate_dispatch_test.py", "GateDispatchTest")):
        path = ROOT / "tools/paxeer-x/tests" / module
        tree = ast.parse(path.read_text())
        declaration = next(node for node in tree.body
                           if isinstance(node, ast.ClassDef) and node.name == cls)
        expected.update(f"tools/paxeer-x/tests/{module}::{cls}::{node.name}"
                        for node in declaration.body
                        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_"))
    expected.update({
        "tools/paxeer-x/tests/candidate_test.py::CandidateTest::test_production_cli_preserves_absent_pending_and_null_inventory",
        "tools/paxeer-x/tests/candidate_test.py::CandidateTest::test_real_shell_selector_and_unknown_input_refusal",
        "tools/paxeer-x/tests/test_gov_templates.py::GovTemplatesTest::test_gov_suite_passes_without_retired_templates",
        "tools/paxeer-x/tests/test_retired_catalogue_deleted.py::RetiredCatalogueDeletedTest::test_deleted_trees_absent_and_catalogue_valid",
        "tools/paxeer-x/tests/test_signature_route_catalogued.py::test_explorer_gate_runs_sig_provider",
    })
    paxeer_cases = [case for case in cases
                    if case["nodeid"].startswith("tools/paxeer-x/tests/")]
    remaining = {
        "tools/paxeer-x/tests/test_retired_catalogue_deleted.py::RetiredCatalogueDeletedTest::test_deleted_trees_absent_and_catalogue_valid",
    }
    retired = expected - remaining
    assert not retired & {case["nodeid"] for case in cases}
    assert {case["nodeid"] for case in paxeer_cases} <= remaining
    assert len(paxeer_cases) == sum(case["retiring_task"] == "22.39.144" for case in cases)
    assert len(cases) == len({case["nodeid"] for case in cases})
    families = {
        "22.39.144": lambda nodeid: nodeid in expected,
        "22.39.156": lambda nodeid: nodeid.startswith("tools/jev/tests/"),
        "22.39.157": lambda nodeid: any(nodeid.startswith(prefix) for prefix in (
            "tools/release/tests/",
            "tools/release/rehearsal/",
        )),
        "22.39.158": lambda nodeid: any(nodeid.startswith(prefix) for prefix in (
            "tools/containers/test_check_all_services_scope.py::test_check_all_services_scope",
            "tools/containers/test_check_all_services_scope.py::test_every_tracked_dockerfile_has_scope",
            "tools/containers/test_kernel_init.py::",
            "tools/containers/test_human_service_image.py::",
            "tools/containers/test_kernel_image.py::",
            "tools/containers/test_paxd_images.py::",
            "tools/containers/test_retired_images.py::",
        )),
        "22.39.159": lambda nodeid: nodeid in {
            "tools/docs/gen/test_gen_system_docs.py::test_precompiles_lists_nested_abi_dirs",
            "tools/docs/gen/test_gen_system_docs_abis.py::test_generator_lists_nested_and_embedded_abis",
        },
        "22.39.160": lambda nodeid: nodeid ==
            "tools/deps/tests/test_go_root_checksums.py::test_go_root_nested_checksums",
        "22.39.164": lambda nodeid: any(nodeid.startswith(prefix) for prefix in (
            "tools/bringup/tests/",
            "tools/benchmark/",
        )),
    }
    for case in cases:
        assert set(case) == {"nodeid", "reason", "retiring_task"}
        assert case["reason"].strip()
        assert case["retiring_task"] in families
        assert families[case["retiring_task"]](case["nodeid"]), case
        module, *symbols = case["nodeid"].split("::")
        declarations = ast.parse((ROOT / module).read_text()).body
        for symbol in symbols:
            symbol = symbol.split("[", 1)[0]
            declaration = next(node for node in declarations
                               if isinstance(node, (ast.ClassDef, ast.FunctionDef))
                               and node.name == symbol)
            declarations = declaration.body


def test_bringup_and_benchmark_baseline_is_retired():
    cases = json.loads((ROOT / BASELINE).read_text())
    assert not [case for case in cases if case["nodeid"].startswith(
        ("tools/bringup/", "tools/benchmark/")
    )]
    assert not [case for case in cases if case["retiring_task"] == "22.39.164"]


def test_ci_runs_tools_python_target():
    workflow = yaml.load(
        (ROOT / ".github/workflows/ci.yml").read_text(), Loader=yaml.BaseLoader,
    )
    assert workflow["on"]["push"]["branches"] == ["main"]
    assert "pull_request" in workflow["on"]
    for event in ("push", "pull_request"):
        settings = workflow["on"][event]
        if isinstance(settings, dict):
            assert not {"paths", "paths-ignore", "branches-ignore"} & settings.keys()
    jobs = [
        job for job in workflow["jobs"].values()
        if any(step.get("run", "").strip() == "make test-tools-python"
               for step in job.get("steps", []))
    ]
    assert len(jobs) == 1
    job = jobs[0]
    assert not {"if", "needs", "continue-on-error"} & job.keys()
    assert job.get("defaults", {}).get("run", {}).get("working-directory", ".") == "."
    steps = job["steps"]
    run = next(step for step in steps if step.get("run", "").strip() == "make test-tools-python")
    assert run.get("working-directory", ".") == "."
    for step in steps:
        assert not {"if", "continue-on-error"} & step.keys()
    checkout = next(step for step in steps
                    if step.get("uses", "").startswith("actions/checkout@"))
    go = next(step for step in steps
              if step.get("uses", "").startswith("actions/setup-go@"))
    assert checkout["with"]["persist-credentials"] == "false"
    assert go["with"]["go-version-file"] == "go.mod"
    assert go["with"]["cache-dependency-path"] == "go.sum"
    install = next(step for step in steps
                   if "-m pip install pytest PyYAML" in step.get("run", ""))
    assert "requests" in shlex.split(install["run"])
    assert 'echo "$RUNNER_TEMP/tools-python/bin" >> "$GITHUB_PATH"' in install["run"]
    assert steps.index(checkout) < steps.index(go) < steps.index(run)
    assert steps.index(install) < steps.index(run)
    foundry = next(step for step in steps
                   if step.get("uses", "").startswith("foundry-rs/foundry-toolchain@"))
    assert foundry["uses"] == "foundry-rs/foundry-toolchain@908c540300062bd5a7e473851cdb4282204cee09"
    assert foundry["with"] == {"version": "v1.8.1", "cache": "false"}
    buildx = next(step for step in steps
                  if step.get("uses", "").startswith("docker/setup-buildx-action@"))
    assert buildx["uses"] == "docker/setup-buildx-action@8d2750c68a42422c14e847fe6c8ac0403b4cbd6f"
    native = next(step for step in steps
                  if "apt-get install" in step.get("run", ""))
    assert {"zstd", "openssh-server", "python3-venv", "nginx", "libnginx-mod-stream"} <= set(shlex.split(native["run"]))
    rust = next(step for step in steps
                if "rustup toolchain install 1.91.1 --profile minimal" in step.get("run", ""))
    build = next(step for step in steps
                 if "make build/liblayerx.a" in step.get("run", ""))
    for command in (
        "make build/liblayerx.a",
        "make build-loadtest",
        "forge build --root tools/benchmark/loadtest/contracts/evm",
        'go build -o "$RUNNER_TEMP/tools-python-bin/bringup-inspect" ./tools/bringup/cmd/bringup-inspect',
        'echo "$RUNNER_TEMP/tools-python-bin" >> "$GITHUB_PATH"',
        "cargo build --locked --manifest-path tools/bringup/tests/xweb-loader/Cargo.toml",
    ):
        assert command in build["run"].splitlines()
    ssh = next(step for step in steps
               if "sudo service ssh start" in step.get("run", ""))
    for command in (
        'install -d -m 700 "$HOME/.ssh"',
        "ssh-keygen -q -t ed25519 -N '' -f \"$HOME/.ssh/id_ed25519\"",
        'cat "$HOME/.ssh/id_ed25519.pub" >> "$HOME/.ssh/authorized_keys"',
        'chmod 600 "$HOME/.ssh/authorized_keys"',
        'ssh-keyscan -H localhost 127.0.0.1 >> "$HOME/.ssh/known_hosts"',
        "ssh -o BatchMode=yes localhost true",
    ):
        assert command in ssh["run"].splitlines()
    assert steps.index(native) < steps.index(build)
    assert steps.index(rust) < steps.index(build)
    assert steps.index(foundry) < steps.index(build)
    assert steps.index(buildx) < steps.index(build)
    for prerequisite in (foundry, buildx, build, ssh):
        assert steps.index(go) < steps.index(prerequisite) < steps.index(run)


def test_paxeer_x_collects_without_go(tmp_path):
    environment = dict(os.environ, PATH=str(tmp_path), PYTEST_DISABLE_PLUGIN_AUTOLOAD="1")
    environment.pop("PYTEST_ADDOPTS", None)
    result = subprocess.run(
        [sys.executable, "-m", "pytest", "--collect-only", "-q", "tools/paxeer-x/tests"],
        cwd=ROOT, env=environment, capture_output=True, text=True, timeout=60,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    collected = result.stdout.splitlines()
    for path in (ROOT / "tools/paxeer-x/tests").glob("*.py"):
        tree = ast.parse(path.read_text())
        if any(isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef))
               and node.name.startswith("test_") for node in ast.walk(tree)):
            assert any(line.startswith(path.relative_to(ROOT).as_posix() + "::")
                       for line in collected), path
    assert "tests collected" in result.stdout
    assert not any(line.startswith("ERROR")
                   for line in (result.stdout + result.stderr).splitlines())
    probe = subprocess.run(
        [sys.executable, "-c", """
import runpy
import sys

candidate = runpy.run_path(sys.argv[1])
assert candidate['SCHEMA'] == 'paxeer-x.candidate.v1'
try:
    candidate['foundation']()
except candidate['Invalid'] as error:
    assert str(error) == 'candidate policy unavailable or timed out'
else:
    raise AssertionError('Policy evaluation must require the real Go toolchain')
""", str(ROOT / "tools/paxeer-x/candidate.py")],
        cwd=ROOT, env=environment, capture_output=True, text=True, timeout=30,
    )
    assert probe.returncode == 0, probe.stdout + probe.stderr
