"""Keep every interop workspace member on the CI test path."""

import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys
import tomllib

import yaml

import interop_workspace_test as workspace_runner


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ".github/workflows/interop-x402.yml"
MANIFEST = "services/interop/Cargo.toml"


def interop_workflow():
    return yaml.load((ROOT / WORKFLOW).read_text(), Loader=yaml.BaseLoader)


def interop_test_command():
    makefile = (ROOT / "Makefile").read_text()
    (manifest,) = re.findall(r"^INTEROP_MANIFEST := (.+)$", makefile, re.MULTILINE)
    assert manifest == MANIFEST
    assert re.search(r"^INTEROP_CARGO \?= cargo$", makefile, re.MULTILINE)
    (recipe,) = re.findall(r"^interop-test:\n((?:\t[^\n]*\n)+)", makefile, re.MULTILINE)
    assert len(recipe.splitlines()) == 1, recipe
    command = shlex.split(
        recipe.strip().replace("$(INTEROP_CARGO)", "cargo").replace(
            "$(INTEROP_MANIFEST)", manifest
        )
    )
    assert command == [
        "python3", "tests/ci/interop_workspace_test.py", "--cargo", "cargo",
        "--manifest-path", MANIFEST,
    ]
    return workspace_runner.workspace_test_command(
        cargo=command[3], manifest_path=command[5]
    )


def test_interop_workspace_test_job_exists():
    workflow = interop_workflow()
    for event in ("push", "pull_request"):
        trigger = workflow["on"][event]
        assert {
            "services/interop/**", "Makefile", WORKFLOW,
            "tests/ci/test_interop_workspace_tests_wired.py",
            "tests/ci/interop_workspace_test.py",
            "tests/ci/data/interop_test_baseline.json",
        } <= set(trigger["paths"])
        assert "paths-ignore" not in trigger
        assert "branches-ignore" not in trigger
    assert workflow["on"]["push"]["branches"] == ["main"]
    assert "branches" not in workflow["on"]["pull_request"]
    assert "types" not in workflow["on"]["pull_request"]
    assert workflow["permissions"] == {"contents": "read"}

    job = workflow["jobs"]["workspace-tests"]
    for key in ("if", "needs", "continue-on-error", "strategy", "defaults"):
        assert key not in job
    assert "ubuntu-24.04" in job["runs-on"]
    assert int(job["timeout-minutes"]) >= 20
    steps = job["steps"]
    for step in steps:
        for key in ("if", "continue-on-error", "working-directory"):
            assert key not in step
    (checkout,) = [
        step for step in steps
        if step.get("uses", "").startswith("actions/checkout@")
    ]
    assert re.fullmatch(r"actions/checkout@[0-9a-f]{40}", checkout["uses"])
    assert checkout["with"]["persist-credentials"] == "false"
    (install,) = [step for step in steps if step["name"] == "Install pinned Rust toolchain"]
    version = tomllib.loads((ROOT / MANIFEST).read_text())["workspace"]["package"]["rust-version"]
    assert shlex.split(install["run"]) == [
        "rustup", "toolchain", "install", version,
        "--profile", "minimal", "--no-self-update",
    ]
    (run,) = [step for step in steps if step["name"] == "Run every interop workspace test"]
    assert shlex.split(run["run"]) == [
        "make", "interop-test", f"INTEROP_CARGO=cargo +{version}",
    ]
    assert steps.index(checkout) < steps.index(install) < steps.index(run)


def test_interop_test_target_uses_workspace():
    commands = interop_test_command()
    assert commands
    entries = workspace_runner.read_baseline()
    for command in commands:
        assert command[:5] == [
            "cargo", "test", "--locked", "--manifest-path", MANIFEST,
        ]
        assert command[5] == "--package"
        assert command[7] == "--no-fail-fast"
        assert "--workspace" not in command and "--exclude" not in command
        if command[8] == "--doc":
            assert command[8:] == ["--doc"]
            continue
        boundary = command.index("--")
        selector = shlex.join(command[8:boundary])
        assert command[boundary:] == [
            "--", "--exact",
            *(argument for entry in entries
              if (entry["member"], entry["target"]) == (command[6], selector)
              for argument in ("--skip", entry["qualified_test_name"])),
        ]
    pinned_commands = workspace_runner.workspace_test_command("cargo +1.91.1")
    assert pinned_commands
    assert all(command[:2] == ["cargo", "+1.91.1"] for command in pinned_commands)
    assert [command[2:] for command in pinned_commands] == [command[1:] for command in commands]
    manifest = tomllib.loads((ROOT / MANIFEST).read_text())
    assert manifest["profile"]["test"]["debug"] == 0


def test_every_interop_member_covered():
    workspace_root = ROOT / "services/interop"
    workspace = tomllib.loads((ROOT / MANIFEST).read_text())["workspace"]
    inventory = {path.parent.resolve() for path in workspace_root.glob("crates/*/Cargo.toml")}
    declared = {
        path.resolve()
        for member in workspace["members"]
        for path in workspace_root.glob(member)
    }
    assert inventory
    assert declared == inventory
    assert not workspace.get("exclude")
    metadata = subprocess.run(
        ["cargo", "metadata", "--locked", "--no-deps", "--format-version", "1",
         "--manifest-path", MANIFEST],
        cwd=ROOT, capture_output=True, text=True, check=False,
    )
    assert metadata.returncode == 0, metadata.stderr
    packages = json.loads(metadata.stdout)
    member_ids = set(packages["workspace_members"])
    members = [package for package in packages["packages"] if package["id"] in member_ids]
    assert {Path(package["manifest_path"]).parent.resolve() for package in members} == inventory
    for package in members:
        assert any(target["test"] for target in package["targets"]), package["name"]
    expected = set()
    for package in members:
        for target in package["targets"]:
            if target["test"]:
                kind = target["kind"]
                if kind[0] in ("bin", "test", "example", "bench"):
                    selector = (f"--{kind[0]}", target["name"])
                else:
                    assert set(kind) <= {"lib", "rlib", "dylib", "cdylib", "staticlib", "proc-macro"}
                    selector = ("--lib",)
                expected.add((package["name"], selector))
            if target["doctest"]:
                expected.add((package["name"], ("--doc",)))
    commands = interop_test_command()
    actual = [
        (command[6], tuple(command[8:command.index("--")] if "--" in command else command[8:]))
        for command in commands
    ]
    assert set(actual) == expected
    assert len(actual) == len(expected)


def test_interop_baseline_entries_name_retiring_tasks():
    baseline = ROOT / "tests/ci/data/interop_test_baseline.json"
    entries = json.loads(baseline.read_text())
    assert entries == workspace_runner.read_baseline()
    retiring_tasks = {
        "layerx-bridge-relayer": "22.39.124",
        "layerx-committed-proof": "22.39.125",
        "layerx-fiat": "22.39.125",
        "layerx-portable": "22.39.125",
        "layerx-mirror": "22.39.125",
        "layerx-gas-station": "22.39.126",
        "layerx-interop-gateway": "22.39.126",
        "layerx-interop-service": "22.39.126",
        "layerx-migrate": "22.39.126",
        "layerx-ucp": "22.39.127",
        "layerx-x402": "22.39.127",
        "x-websearch": "22.39.127",
    }
    seen = set()
    for entry in entries:
        member = entry["member"]
        assert member in retiring_tasks
        assert entry["retiring_task"] == retiring_tasks[member]
        assert entry["first_error_line"].strip()
        assert len(entry["first_error_line"].splitlines()) == 1
        name = entry["qualified_test_name"]
        assert re.fullmatch(r"\w+(?:::\w+)*", name), entry
        key = (member, entry["target"], name)
        assert key not in seen, entry
        seen.add(key)
        member_root = ROOT / "services/interop/crates" / member
        manifest = tomllib.loads((member_root / "Cargo.toml").read_text())
        assert manifest["package"]["name"] == member
        flag, *target = shlex.split(entry["target"])
        if flag == "--lib":
            assert not target
            target_source = member_root / manifest.get("lib", {}).get("path", "src/lib.rs")
            assert target_source.is_file()
        else:
            assert flag in ("--test", "--bin") and len(target) == 1
            kind = flag[2:]
            declared = {
                item["name"]: item.get("path", f"tests/{item['name']}.rs")
                for item in manifest.get(kind, [])
            }
            path = declared.get(target[0], f"tests/{target[0]}.rs")
            target_source = member_root / path
            assert target_source.is_file(), entry
        leaf = name.rsplit("::", 1)[-1]
        assert any(
            re.search(r"\bfn\s+" + re.escape(leaf) + r"\s*\(", source.read_text())
            for source in {target_source.resolve(), *member_root.rglob("*.rs")}
        ), entry


def cargo_marker_source(marker, *, fail=False, name="same_name", suffix=False):
    assertion = 'assert_eq!(1, 2, "intentional unlisted failure");' if fail else ""
    extra = ""
    if suffix:
        extra = cargo_marker_source("suffix", name="same_name_suffix").split("mod shared {\n", 1)[1].rsplit("}", 1)[0]
    return f'''#[cfg(test)]
mod shared {{
    #[test]
    fn {name}() {{
        let directory = std::env::var("INTEROP_MARKER_DIR").unwrap();
        std::fs::write(std::path::Path::new(&directory).join("{marker}"), "executed").unwrap();
        {assertion}
    }}
{extra}}}
'''


def write_cargo_file(workspace, path, source):
    destination = workspace / path
    destination.parent.mkdir(parents=True, exist_ok=True)
    destination.write_text(source)


def lock_cargo_workspace(workspace):
    result = subprocess.run(
        ["cargo", "generate-lockfile", "--offline", "--manifest-path", str(workspace / "Cargo.toml")],
        cwd=workspace, capture_output=True, text=True, check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr


def run_interop_workspace(workspace, baseline, markers):
    markers.mkdir()
    result = subprocess.run(
        [sys.executable, str(ROOT / "tests/ci/interop_workspace_test.py"),
         "--manifest-path", str(workspace / "Cargo.toml"), "--baseline-path", str(baseline)],
        cwd=ROOT, capture_output=True, text=True, check=False,
        env={**os.environ, "INTEROP_MARKER_DIR": str(markers),
             "CARGO_TARGET_DIR": str(workspace / "target"), "CARGO_NET_OFFLINE": "true"},
    )
    return result


def cargo_test_totals(output):
    summaries = re.findall(
        r"test result: (?:ok|FAILED)\. (\d+) passed; (\d+) failed; \d+ ignored; \d+ measured; (\d+) filtered out;",
        output,
    )
    assert summaries, output
    return tuple(sum(int(summary[index]) for summary in summaries) for index in range(3))


def test_interop_baseline_skips_are_scoped_to_cargo_targets(tmp_path):
    workspace = tmp_path / "workspace"
    write_cargo_file(workspace, "Cargo.toml", '''[workspace]
members = ["relayer", "fiat"]
resolver = "2"
''')
    write_cargo_file(workspace, "relayer/Cargo.toml", '''[package]
name = "layerx-bridge-relayer"
version = "0.1.0"
edition = "2021"

[[example]]
name = "coverage"
test = true

[[bench]]
name = "coverage"
test = true
''')
    doctest = '''//! ```
//! let directory = std::env::var("INTEROP_MARKER_DIR").unwrap();
//! std::fs::write(std::path::Path::new(&directory).join("doc"), "executed").unwrap();
//! ```
'''
    write_cargo_file(workspace, "relayer/src/lib.rs", doctest + cargo_marker_source("retired"))
    write_cargo_file(
        workspace, "relayer/tests/retained.rs",
        cargo_marker_source("retained", suffix=True),
    )
    write_cargo_file(workspace, "relayer/examples/coverage.rs", "fn main() {}\n" + cargo_marker_source("example"))
    write_cargo_file(workspace, "relayer/benches/coverage.rs", cargo_marker_source("bench"))
    write_cargo_file(workspace, "fiat/Cargo.toml", '''[package]
name = "layerx-fiat"
version = "0.1.0"
edition = "2021"
''')
    write_cargo_file(workspace, "fiat/src/main.rs", "fn main() {}\n" + cargo_marker_source("unlisted"))
    lock_cargo_workspace(workspace)
    baseline = tmp_path / "baseline.json"
    entries = [
        {"member": "layerx-bridge-relayer", "target": target,
         "qualified_test_name": "shared::same_name", "first_error_line": "recorded assertion failure",
         "retiring_task": "22.39.124"}
        for target in ("--lib", "--test retained")
    ]
    always = {"unlisted", "suffix", "example", "bench", "doc"}
    for phase, remaining, expected_markers, totals in (
        ("before", entries, always, (5, 0, 2)),
        ("retired", entries[1:], always | {"retired"}, (6, 0, 1)),
        ("empty", [], always | {"retired", "retained"}, (7, 0, 0)),
    ):
        baseline.write_text(json.dumps(remaining))
        assert workspace_runner.read_baseline(baseline) == remaining
        commands = workspace_runner.workspace_test_command(
            manifest_path=str(workspace / "Cargo.toml"), baseline_path=baseline,
        )
        assert len(commands) == 6
        for command in commands:
            if "--skip" in command:
                assert command[6] == "layerx-bridge-relayer"
                boundary = command.index("--")
                assert command[boundary:] == ["--", "--exact", "--skip", "shared::same_name"]
                assert any(entry["target"] == shlex.join(command[8:boundary]) for entry in remaining)
        markers = tmp_path / phase
        result = run_interop_workspace(workspace, baseline, markers)
        output = result.stdout + result.stderr
        assert result.returncode == 0, output
        assert {path.name for path in markers.iterdir()} == expected_markers, output
        assert all(path.read_text() == "executed" for path in markers.iterdir())
        assert cargo_test_totals(output) == totals, output
        assert result.stdout.count("Interop baseline:") == len(remaining), output
        for entry in remaining:
            assert f"{entry['member']} {entry['target']} {entry['qualified_test_name']}" in output
            assert f"retiring task {entry['retiring_task']} | {entry['first_error_line']}" in output


def test_interop_target_failure_propagates_and_continues(tmp_path):
    workspace = tmp_path / "workspace"
    write_cargo_file(workspace, "Cargo.toml", '''[package]
name = "layerx-bridge-relayer"
version = "0.1.0"
edition = "2021"
''')
    write_cargo_file(workspace, "tests/a_failure.rs", cargo_marker_source("failure", fail=True))
    write_cargo_file(workspace, "tests/z_later.rs", cargo_marker_source("later"))
    lock_cargo_workspace(workspace)
    baseline = tmp_path / "baseline.json"
    baseline.write_text("[]")
    markers = tmp_path / "markers"
    result = run_interop_workspace(workspace, baseline, markers)
    output = result.stdout + result.stderr
    assert result.returncode == 101, output
    assert {path.name for path in markers.iterdir()} == {"failure", "later"}, output
    assert (markers / "later").read_text() == "executed"
    assert "intentional unlisted failure" in output
    assert result.stdout.index("--test a_failure") < result.stdout.index("--test z_later"), output
    assert cargo_test_totals(output) == (1, 1, 0), output
