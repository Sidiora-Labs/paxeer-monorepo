import json
import os
import shlex
import subprocess
from pathlib import Path

from programme.evidence import evidence_line
from programme.kvx import load
from programme.parity import (
    PARITY_PATH,
    SELECTOR_ENVIRONMENT,
    TASK_PATHS,
    command_for,
    run_all,
)

ROOT = Path(__file__).resolve().parents[4]


def test_parity_rows_pass_on_merged_revision() -> None:
    rows = load(PARITY_PATH)
    constructs = {
        name: row for name, row in rows.items() if name.startswith("construct.")
    }
    assert constructs
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    directory = Path(os.environ["HF_OUT_DIR"]) / "logs" / "parity" / revision
    wallet_types = ROOT / "apps/wallet/next-env.d.ts"
    wallet_types_before = wallet_types.read_bytes()
    results = run_all(rows, ROOT, directory)
    assert wallet_types.read_bytes() == wallet_types_before
    lines = (directory / "evidence.log").read_text(encoding="utf-8").splitlines()
    assert set(results) == set(constructs)
    assert len(lines) == len(constructs)
    reports = [
        json.loads(line)
        for line in (directory / "report.jsonl").read_text(encoding="utf-8").splitlines()
    ]
    assert [report["row"] for report in reports] == list(constructs)
    assert len(reports) == len(constructs)
    assert not list(directory.glob("construct.*.log"))
    run_record = json.loads((directory / "run.json").read_text(encoding="utf-8"))
    assert run_record["revision"] == revision
    go_rows = [
        name
        for name, row in constructs.items()
        if any(
            selector.partition("::")[0].endswith(".go") for selector in row["selectors"]
        )
    ]
    assert go_rows
    assert run_record["go_cache_cleanup"] == {
        "after_row": go_rows[-1],
        "evidence": evidence_line(
            revision, "go clean -cache", 0, directory / "go-cache-clean.log"
        ),
    }
    node = run_record["selector_environment"]
    assert node["starts"] == node["stops"] == 1
    assert len(node["homes"]) == 1
    assert node["removed"]
    assert all(not Path(home).exists() for home in node["homes"])
    assert not (directory / "node-tmp").exists()
    environment_rows = [report for report in reports if report["environment_sources"]]
    assert any(
        "apps/wallet/next-env.d.ts" in report["restored_files"]
        for report in environment_rows
    )
    assert {report["row"] for report in environment_rows} == {
        "construct.existing-wallet-human-service-and-explorer-playwright",
        "construct.new-signed-receipt-logs-new-case",
    }
    for report in environment_rows:
        assert report["environment_sources"] == {
            selector: SELECTOR_ENVIRONMENT[selector]
            for selector in constructs[report["row"]]["selectors"]
            if selector in SELECTOR_ENVIRONMENT
        }
    for stage in ("free_space_start", "free_space_end"):
        assert "Available" in run_record[stage]
        assert len(run_record[stage].splitlines()) >= 2
    for report in reports:
        assert len(report["output"]) <= 200
        assert report["omitted_lines"] >= 0
        assert "Available" in report["free_space"]
        assert report["passed"] == results[report["row"]]
        assert not report["missing_go_tests"], (report["row"], report["missing_go_tests"])
        assert not report["missing_rust_tests"], (report["row"], report["missing_rust_tests"])
        assert not report["unpassed_go_tests"], (report["row"], report["unpassed_go_tests"])
        assert not report["unpassed_rust_tests"], (
            report["row"],
            report["unpassed_rust_tests"],
        )
        assert report["restored_files"] == sorted(set(report["restored_files"]))
        assert TASK_PATHS.isdisjoint(report["restored_files"])
    failed = [name for name, passed in results.items() if not passed]
    evidence = dict(zip(constructs, lines, strict=True))
    report_by_row = {report["row"]: report for report in reports}
    assert not failed, "Parity rows failed:\n" + "\n".join(
        f"{name}: {evidence[name]}\n"
        + f" selectors={constructs[name]['selectors']} report={directory / 'report.jsonl'}\n"
        + f"omitted_lines={report_by_row[name]['omitted_lines']}\n"
        + "\n".join(report_by_row[name]["output"])
        for name in failed
    )
    for (name, row), line in zip(constructs.items(), lines, strict=True):
        log_path = directory / f"{name}.log"
        report = report_by_row[name]
        assert report["revision"] == revision
        assert report["selectors"] == row["selectors"]
        assert report["log"] == str(log_path)
        assert report["exit"] == 0
        assert report["evidence"] == line
        command = command_for(row)
        if shlex.split(command)[:2] == ["go", "test"]:
            command = shlex.join([*shlex.split(command), "-json"])
            executed = set(report["go_tests_run"])
            expected = {selector.split("::")[1] for selector in row["selectors"]}
            assert executed == expected, (name, log_path)
            assert expected.issubset(set(report["go_tests_passed"])), (name, log_path)
        elif all(
            selector.partition("::")[0].endswith(".rs") for selector in row["selectors"]
        ):
            executed = set(report["rust_tests_run"])
            expected = {selector.partition("::")[2] for selector in row["selectors"]}
            assert executed == expected, (name, log_path)
            assert expected.issubset(set(report["rust_tests_passed"])), (name, log_path)
        assert report["command"] == command
        prefix, separator, count = line.rpartition(" tests=")
        assert separator and int(count) > 0, (name, line)
        assert report["tests"] == int(count)
        assert int(count) >= len(set(row["selectors"])), (name, line)
        assert prefix == evidence_line(revision, command, 0, log_path)
    node_rows = [
        row
        for row in constructs.values()
        if any(selector.startswith("node/") for selector in row["selectors"])
    ]
    assert node_rows
    prerequisites = (
        (directory / "prerequisites.log").read_text(encoding="utf-8").splitlines()
    )
    assert prerequisites == [
        evidence_line(
            revision,
            "make build/liblayerx.a programs/target/debug/liblayerx_programs_sandbox.a",
            0,
            directory / "native-libraries.log",
        ),
        evidence_line(revision, "make evmone-lib", 0, directory / "evmone-lib.log"),
    ]
