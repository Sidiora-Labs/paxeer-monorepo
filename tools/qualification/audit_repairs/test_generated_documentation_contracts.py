"""Qualify the beta ledger and the system/SDK documentation repairs."""

import importlib.util
from pathlib import Path
import re
import subprocess
import tomllib

import pytest


ROOT = Path(__file__).resolve().parents[3]
LEDGER = ROOT / "spec/layerx-beta/qualification.kvx"
REPORT = ROOT / "tools/ci/beta-report.sh"
TEMPLATE = ROOT / "services/agent/tools/sdk-gen/templates/compatibility.md.tpl"
COMPATIBILITY = ROOT / "services/agent/sdk/COMPATIBILITY.md"


def system_generator():
    path = ROOT / "tools/docs/gen/gen_system_docs.py"
    spec = importlib.util.spec_from_file_location("gen_system_docs", path)
    assert spec is not None and spec.loader is not None
    generator = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(generator)
    return generator


def test_beta_ledger_has_metadata_and_no_unexecuted_records():
    ledger = tomllib.loads(LEDGER.read_text(encoding="utf-8"))
    assert set(ledger) == {"meta"}
    assert ledger["meta"] == {
        "name": "Paxeer X Network mainnet-beta qualification",
        "kind": "beta-qualification-ledger",
        "version": "0.1.0",
        "record_kinds": ["gate", "observation"],
    }
    header = LEDGER.read_text(encoding="utf-8")
    assert "[gate.<task>.<n>]" in header
    assert "[observation.<task>.<n>]" in header
    assert "/out/run/gates.jsonl" in header
    assert "hf-build" in header and "hf-done" in header
    contract = (ROOT / "docs/platform/content/beta.md").read_text(encoding="utf-8")
    assert "spec/layerx-beta/qualification.kvx" in contract
    assert "## Evidence ledger" in contract
    assert "Only a gate record raises a surface." in contract
    assert "Without executed checks, the report stays `no-go`" in contract
    assert "| report_status | no-go |" in contract
    assert "| evidence_ledger | spec/layerx-beta/qualification.kvx |" in contract


@pytest.mark.parametrize("arguments", [["--check"], []])
def test_beta_report_uses_empty_beta_ledger_from_any_directory(tmp_path, arguments):
    result = subprocess.run(
        ["bash", str(REPORT), *arguments],
        cwd=tmp_path,
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    summary = result.stdout if arguments else result.stderr
    assert "beta-report: no-go; 0 gate records" in summary
    assert "0 observations" in summary
    assert "candidate undeclared" in summary
    if not arguments:
        assert str(LEDGER) in result.stdout
        assert "Decision: **no-go**" in result.stdout
        assert "No gate record exists; every surface stays at rung source_present." in result.stdout


@pytest.mark.parametrize("mutation", ["duplicate", "missing", "wrong_type"])
def test_beta_report_rejects_invalid_metadata(tmp_path, mutation):
    text = LEDGER.read_text(encoding="utf-8")
    if mutation == "duplicate":
        text += '\n[meta]\nname = "duplicate"\n'
    elif mutation == "missing":
        text = text.replace('version = "0.1.0"\n', "")
    else:
        text = text.replace('["gate", "observation"]', '"gate"')
    invalid = tmp_path / "qualification.kvx"
    invalid.write_text(text, encoding="utf-8")
    result = subprocess.run(
        ["bash", str(REPORT), "--ledger", str(invalid), "--check"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        timeout=30,
    )
    assert result.returncode == 1, result.stdout + result.stderr
    assert "ledger invalid" in result.stderr
    assert "meta" in result.stderr
    assert "beta-report: go;" not in result.stdout


def test_generated_system_pages_match_sources_and_reference_existing_files():
    generator = system_generator()
    for generate in (generator.gen_index, generator.gen_kernel_modules):
        name, generated = generate()
        page = (ROOT / "docs/system" / name).read_text(encoding="utf-8")
        assert page == generated.rstrip("\n") + "\n"
        assert "Router routes" not in page
        paths = [value for value in re.findall(r"`([^`]+)`", page) if "/" in value]
        assert paths
        for path in paths:
            assert (ROOT / path).exists(), path
        for target in re.findall(r"\]\(([^)]+)\)", page):
            assert (ROOT / "docs/system" / target).is_file(), target

    _, routes = generator.gen_routes()
    assert "# Hosted route catalogue\n" in routes
    assert "Router routes" not in routes
    index = (ROOT / "docs/system/INDEX.md").read_text(encoding="utf-8")
    assert "[Hosted route catalogue](routes.md)" in index
    kernel = (ROOT / "docs/system/kernel-modules.md").read_text(encoding="utf-8")
    bridge_sources = set(re.findall(r"`(src/modules/bridge/[^`]+\.c)`", kernel))
    assert bridge_sources == {
        "src/modules/bridge/lxp_bridge_deposit.c",
        "src/modules/bridge/lxp_bridge_exit.c",
        "src/modules/bridge/lxp_bridge_withdraw.c",
        "src/modules/bridge/lxp_reserve.c",
    }
    assert "No interface registration" in kernel


def test_sdk_template_renders_committed_page_and_preserves_fixed_links():
    template = TEMPLATE.read_text(encoding="utf-8")
    committed = COMPATIBILITY.read_text(encoding="utf-8")
    schema = tomllib.loads(
        (ROOT / "services/agent/schema/agent-api/v1.kvx").read_text(encoding="utf-8")
    )
    approval = tomllib.loads(
        (ROOT / "services/agent/schema/agent-api/approval.kvx").read_text(encoding="utf-8")
    )
    matrix = schema["compatibility"]["matrix"]
    substitutions = {
        "SDK": matrix["sdk"],
        "CONTRACT": matrix["contract"],
        "NODE_INTERFACE": matrix["node_interface"],
        "DAEMON": matrix["daemon"],
        "APPROVAL_INTRODUCED": (
            f"{approval['module']['contract_major']}.{approval['module']['contract_minor']}"
        ),
    }
    rendered = template
    assert set(re.findall(r"\{\{([A-Z_]+)\}\}", template)) == set(substitutions)
    for name, value in substitutions.items():
        rendered = rendered.replace("{{" + name + "}}", value)
    assert rendered == committed
    for line in template.splitlines():
        if "{{" not in line:
            assert line in committed.splitlines()
    assert "generated" in template.splitlines()[0].lower()
    assert "committed file" in template.splitlines()[0]
    links = re.findall(r"\]\(([^)]+)\)", template)
    assert links
    for link in links:
        assert (COMPATIBILITY.parent / link.split("#", 1)[0]).is_file(), link
