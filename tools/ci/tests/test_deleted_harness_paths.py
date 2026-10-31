from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[3]
WORKFLOW = ROOT / "scripts/ci/xweb-workflow-check.sh"


def _gate_and_qualification_scripts():
    scripts = []
    for directory in ("tools/paxeer-x/gates", "tools/qualification"):
        path = ROOT / directory
        assert path.is_dir(), f"missing script directory: {directory}"
        found = sorted(
            script
            for script in path.rglob("*")
            if script.is_file() and script.suffix in (".py", ".sh")
        )
        assert found, f"no scripts checked in {directory}"
        scripts.extend(found)
    return scripts


def test_no_gate_runs_reset_recovery():
    references = [
        str(script.relative_to(ROOT))
        for script in _gate_and_qualification_scripts()
        if "reset_recovery.py" in script.read_text()
    ]
    assert not references, f"deleted reset recovery harness referenced by: {references}"


def test_workflow_list_names_no_gateway_harness():
    workflow = WORKFLOW.read_text()
    references = [
        name
        for name in ("custody_chain.py", "events.rs", "funding.rs", "lifecycle.rs")
        if f"services/platform/hosted/gateway/tests/local/{name}" in workflow
    ]
    assert not references, f"deleted gateway harness paths in workflow: {references}"


def test_no_gate_runs_core_kernel_tests():
    deleted_tests = (
        "boundary.rs",
        "custody_asset_send.rs",
        "core_chain.py",
        "qualify.py",
        "receipt_retention.py",
    )
    references = []
    for script in [*_gate_and_qualification_scripts(), WORKFLOW]:
        source = script.read_text()
        for name in deleted_tests:
            if f"tests/{name}" in source:
                references.append(f"{script.relative_to(ROOT)}: {name}")
            if name.endswith(".rs") and "layerx-platform-core" in source:
                selector = rf"--test(?:\s+|=)['\"]?{re.escape(Path(name).stem)}\b"
                if re.search(selector, source):
                    references.append(
                        f"{script.relative_to(ROOT)}: --test {Path(name).stem}"
                    )
    assert not references, f"deleted core kernel tests referenced by: {references}"
