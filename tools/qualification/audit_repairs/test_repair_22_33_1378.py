"""Offline source proofs for perf-db workflow configuration and owner wiring."""

import ast
from fnmatch import fnmatchcase
from pathlib import Path
import re
import shlex
import tomllib

import yaml


ROOT = Path(__file__).resolve().parents[3]
CRATE = "services/platform/perf-db"
WORKFLOWS = ROOT / CRATE / ".github/workflows"
RECORD = Path(__file__).with_name("repair_22_33_1378.kvx")
AUDIT = ROOT / "spec/layerx-merge/programme/audit/records/22.30.75.kvx"


def load_workflow(name):
    # BaseLoader preserves GitHub's `on` key instead of YAML 1.1's boolean True.
    workflow = yaml.load(
        (WORKFLOWS / f"{name}.yml").read_text(encoding="utf-8"),
        Loader=yaml.BaseLoader,
    )
    assert isinstance(workflow, dict)
    assert set(workflow["jobs"])
    return workflow


def assert_cargo_context(workflow, expected_commands, cache_jobs):
    assert workflow["defaults"]["run"]["working-directory"] == CRATE
    manifest_path = ROOT / CRATE / "Cargo.toml"
    manifest = tomllib.loads(manifest_path.read_text(encoding="utf-8"))
    assert manifest["package"]["name"] == "perfdb"
    assert manifest["workspace"] == {}
    assert (manifest_path.parent / manifest["bin"][0]["path"]).is_file()
    lock = tomllib.loads((manifest_path.parent / "Cargo.lock").read_text())
    assert any(package["name"] == "perfdb" for package in lock["package"])
    actual_commands = {}
    actual_cache_jobs = set()
    for job_name, job in workflow["jobs"].items():
        assert "defaults" not in job
        steps = job["steps"]
        assert steps[0]["uses"] == "actions/checkout@v4"
        commands = []
        for step in steps:
            if "run" in step:
                assert "working-directory" not in step
                command = shlex.split(step["run"])
                assert command[0] == "cargo"
                commands.append(command)
            if step.get("uses") == "Swatinem/rust-cache@v2":
                assert step["with"]["workspaces"] == f"{CRATE} -> target"
                actual_cache_jobs.add(job_name)
        actual_commands[job_name] = commands
    assert actual_commands == expected_commands
    assert actual_cache_jobs == set(cache_jobs)
    return manifest


def assert_filters(workflow, name, events, expected_patterns, required_paths):
    assert set(workflow["on"]) == set(events)
    for event in events:
        if event not in {"push", "pull_request"}:
            continue
        trigger = workflow["on"][event]
        assert trigger["branches"] == ["main"]
        assert trigger["paths"] == expected_patterns
        assert "paths-ignore" not in trigger
        for path in required_paths:
            assert any(fnmatchcase(path, pattern) for pattern in trigger["paths"])
        for unrelated in ("src/main.rs", "Cargo.toml", "services/agent/Cargo.toml"):
            assert not any(fnmatchcase(unrelated, pattern) for pattern in trigger["paths"])
        assert f".github/workflows/perf-db-{name}.yml" in trigger["paths"]


def test_perf_benchmark_workflow_location():
    workflow = load_workflow("benchmark")
    assert workflow["name"] == "Benchmarks"
    assert_cargo_context(
        workflow, {"bench": [["cargo", "bench", "--no-fail-fast"]]}, {"bench"}
    )
    patterns = [
        f"{CRATE}/src/**",
        f"{CRATE}/benches/**",
        f"{CRATE}/Cargo.toml",
        f"{CRATE}/Cargo.lock",
        f"{CRATE}/.github/workflows/benchmark.yml",
        ".github/workflows/perf-db-benchmark.yml",
    ]
    assert_filters(
        workflow,
        "benchmark",
        {"workflow_dispatch", "push"},
        patterns,
        [
            f"{CRATE}/src/main.rs",
            f"{CRATE}/src/storage/wal.rs",
            f"{CRATE}/benches/tick_ingestion.rs",
            *patterns[2:],
        ],
    )
    manifest = tomllib.loads((ROOT / CRATE / "Cargo.toml").read_text())
    for bench in manifest["bench"]:
        assert bench["harness"] is False
        path = f"{CRATE}/benches/{bench['name']}.rs"
        assert (ROOT / path).is_file()
        assert any(fnmatchcase(path, pattern) for pattern in patterns)


def test_perf_ci_workflow_location():
    workflow = load_workflow("ci")
    assert workflow["name"] == "CI"
    manifest = assert_cargo_context(
        workflow,
        {
            "fmt": [["cargo", "fmt", "--all", "--", "--check"]],
            "clippy": [
                ["cargo", "clippy", "--all-targets", "--all-features", "--", "-D", "warnings"]
            ],
            "test": [
                ["cargo", "build", "--all-targets"],
                ["cargo", "test", "--all-features"],
            ],
            "msrv": [["cargo", "check", "--all-targets"]],
            "coverage": [
                ["cargo", "llvm-cov", "--all-features", "--workspace", "--lcov", "--output-path", "lcov.info"]
            ],
        },
        {"clippy", "test", "msrv", "coverage"},
    )
    assert_filters(
        workflow,
        "ci",
        {"push", "pull_request"},
        [f"{CRATE}/**", ".github/workflows/perf-db-ci.yml"],
        [f"{CRATE}/src/engine.rs", f"{CRATE}/Cargo.toml", f"{CRATE}/Cargo.lock",
         f"{CRATE}/.github/workflows/ci.yml"],
    )
    assert workflow["env"] == {"CARGO_TERM_COLOR": "always", "RUSTFLAGS": "-D warnings"}
    assert workflow["concurrency"] == {
        "group": "${{ github.workflow }}-${{ github.ref }}", "cancel-in-progress": "true"
    }
    jobs = workflow["jobs"]
    assert jobs["test"]["strategy"] == {
        "fail-fast": "false", "matrix": {"os": ["ubuntu-latest", "macos-latest"]}
    }
    assert jobs["test"]["runs-on"] == "${{ matrix.os }}"
    assert manifest["package"]["rust-version"] == "1.85"
    assert manifest["package"]["edition"] == "2024"
    for name, job in jobs.items():
        toolchains = [step for step in job["steps"]
                      if step.get("uses", "").startswith("dtolnay/rust-toolchain@")]
        assert len(toolchains) == 1
        expected = manifest["package"]["rust-version"] if name == "msrv" else "stable"
        assert toolchains[0]["uses"] == f"dtolnay/rust-toolchain@{expected}"
    assert jobs["fmt"]["steps"][1]["with"]["components"] == "rustfmt"
    assert jobs["clippy"]["steps"][1]["with"]["components"] == "clippy"
    coverage = jobs["coverage"]["steps"]
    assert coverage[1]["with"]["components"] == "llvm-tools-preview"
    assert coverage[2]["uses"] == "taiki-e/install-action@cargo-llvm-cov"
    upload = coverage[-1]
    assert upload["uses"] == "codecov/codecov-action@v7"
    producer = shlex.split(coverage[-2]["run"])
    produced_path = ROOT / CRATE / producer[producer.index("--output-path") + 1]
    assert ROOT / upload["with"]["files"] == produced_path
    assert upload["with"]["fail_ci_if_error"] == "false"


def test_perf_security_workflow_location():
    workflow = load_workflow("security")
    assert workflow["name"] == "Security"
    assert_cargo_context(workflow, {"audit": [["cargo", "audit"]], "deny": []}, set())
    assert_filters(
        workflow,
        "security",
        {"push", "pull_request", "schedule"},
        [f"{CRATE}/**", ".github/workflows/perf-db-security.yml"],
        [f"{CRATE}/Cargo.toml", f"{CRATE}/Cargo.lock", f"{CRATE}/deny.toml",
         f"{CRATE}/.github/workflows/security.yml"],
    )
    assert workflow["on"]["schedule"] == [{"cron": "0 6 * * 1"}]
    audit = workflow["jobs"]["audit"]["steps"]
    assert audit[1]["uses"] == "dtolnay/rust-toolchain@stable"
    assert audit[2]["uses"] == "taiki-e/install-action@cargo-audit"
    deny = workflow["jobs"]["deny"]["steps"][1]
    assert deny["uses"] == "EmbarkStudios/cargo-deny-action@v2"
    assert deny["with"] == {"command": "check", "manifest-path": f"{CRATE}/Cargo.toml"}
    manifest_path = ROOT / deny["with"]["manifest-path"]
    policy = tomllib.loads((manifest_path.parent / "deny.toml").read_text())
    assert policy["advisories"]["ignore"] == []
    assert policy["bans"]["wildcards"] == "deny"
    assert policy["sources"] == {"unknown-registry": "deny", "unknown-git": "deny"}


def test_perf_workflow_record_maps_findings_and_root_wiring():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    repair = record["repair"]
    assert repair["task"] == "22.33.1378"
    assert repair["base_revision"] == "04c1e9407e75f7ac92c260452f2232ab07bf3ef1"
    assert "not run" in repair["baseline"]
    assert "not executed" in repair["review_only"]
    audit = AUDIT.read_text(encoding="utf-8")
    declared = {node.name for node in ast.parse(Path(__file__).read_text()).body
                if isinstance(node, ast.FunctionDef) and node.name.startswith("test_")}
    proving_tests = set()
    assert set(record["finding"]) == {
        f"perf-{name}-workflow-location" for name in ("benchmark", "ci", "security")
    }
    for name in ("benchmark", "ci", "security"):
        finding_id = f"perf-{name}-workflow-location"
        finding = record["finding"][finding_id]
        block = re.search(
            rf"^\[finding\.{re.escape(finding_id)}\]\n(.*?)(?=^\[|\Z)",
            audit, re.M | re.S,
        )
        assert block is not None
        original = tomllib.loads(block.group(1))
        for key in ("file", "line", "class", "severity", "observed", "expected", "proposal", "owner_domain"):
            assert finding[key] == original[key]
        assert finding["audit"] == "22.30.75"
        assert finding["tests"] == [f"test_perf_{name}_workflow_location"]
        proving_tests.update(finding["tests"])
        wiring = record["wiring"][name]
        assert wiring["path"] == f".github/workflows/perf-db-{name}.yml"
        assert wiring["source"] == finding["file"]
        assert wiring["owner_domain"] == 16
        assert wiring["status"] == "requested; outside task scope"
        assert wiring["path"] in wiring["request"]
        assert "domain 16" in wiring["request"]
    assert proving_tests == declared - {"test_perf_workflow_record_maps_findings_and_root_wiring"}
    evidence = record["evidence"]
    assert evidence["build_cmd"] == "python3 --version"
    assert evidence["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_1378.py"
    )
    assert evidence["ledger"] == "/out/run/gates.jsonl"
    assert "same candidate revision" in evidence["verified_revision"]
