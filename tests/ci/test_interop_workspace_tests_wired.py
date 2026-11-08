"""Keep every interop workspace member on the CI test path."""

import json
from pathlib import Path
import re
import shlex
import subprocess
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
    command = interop_test_command()
    assert command[:7] == [
        "cargo", "test", "--locked", "--manifest-path", MANIFEST,
        "--workspace", "--no-fail-fast",
    ]
    assert command[7:9] == ["--", "--exact"]
    entries = workspace_runner.read_baseline()
    assert command[9:] == [
        argument for entry in entries
        for argument in ("--skip", entry["qualified_test_name"])
    ]
    assert workspace_runner.workspace_test_command("cargo +1.91.1")[:2] == [
        "cargo", "+1.91.1",
    ]
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
    command = interop_test_command()[:7]
    assert "--workspace" in command
    assert not {"-p", "--package", "--exclude", "--lib", "--bins", "--test", "--"}.intersection(command)


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
