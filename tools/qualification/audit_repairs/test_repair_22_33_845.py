"""Prove f006's proposal text and its production wiring without a toolchain."""

import ast
import json
from pathlib import Path
import re
import shlex
import tomllib


ROOT = Path(__file__).resolve().parents[3]
GOV = ROOT / "tools/paxeer-x/gov"
TEMPLATE = GOV / "templates/01-software-upgrade.json"
RECORD = Path(__file__).with_name("repair_22_33_845.kvx")


def test_f006_description_identifies_pre_merge_plan_and_post_merge_refusal():
    proposal = json.loads(TEMPLATE.read_text(encoding="utf-8"))
    assert proposal["description"].split(". ") == [
        "Upgrade the chain to paxd v6.12.0 at the plan height",
        "This is the pre-merge v6.12.0 plan; the merged-network upgrade uses a separate plan",
        "After the merge, the custody executor is retained only to refuse custody governance messages",
        "The release asset and its SHA-256 are named in the plan info.",
    ]
    assert "activates the custody governance route" not in proposal["description"]


def test_f006_preserves_plan_identity_schema_and_render_inputs():
    proposal = json.loads(TEMPLATE.read_text(encoding="utf-8"))
    assert {key: value for key, value in proposal.items() if key != "description"} == {
        "name": "v6.12.0",
        "height": "@HEIGHT@",
        "info": "@UPGRADE_INFO@",
        "title": "Paxeer X Network v6.12.0",
        "deposit": "@DEPOSIT@",
        "is_expedited": False,
    }
    upgrade = (ROOT / "node/upgrades_v6120.go").read_text(encoding="utf-8")
    names = re.findall(r'^const V6120Upgrade = "([^"]+)"$', upgrade, re.M)
    assert names == [proposal["name"]]
    assert set(re.findall(r"@([A-Z0-9_]+)@", TEMPLATE.read_text())) == {
        "HEIGHT", "UPGRADE_INFO", "DEPOSIT",
    }


def test_f006_real_template_description_reaches_guarded_upgrade_consumer():
    renderer = (GOV / "render.sh").read_text(encoding="utf-8")
    logical = renderer.replace("\\\n", " ")
    assert 'TEMPLATES="$ROOT/tools/paxeer-x/gov/templates"' in renderer
    assert 'for template in "$TEMPLATES"/*.json; do' in renderer
    assert 'else . end)\' "$template" >"$OUT/$(basename "$template")"' in renderer
    assert 'test("^@[A-Z0-9_]+@$")' in renderer
    assert not re.fullmatch(
        r"@[A-Z0-9_]+@",
        json.loads(TEMPLATE.read_text(encoding="utf-8"))["description"],
    )
    assert 'up="$OUT/01-software-upgrade.json"' in renderer
    start = logical.index('dry_run "$up" tx gov submit-proposal software-upgrade')
    command = shlex.split(logical[start:].split("\n", 1)[0])
    assert command == [
        "dry_run", "$up", "tx", "gov", "submit-proposal", "software-upgrade",
        "$(jq -r .name $up)",
        "--upgrade-height", "$(jq -r .height $up)",
        "--upgrade-info", "$(jq -r .info $up)",
        "--title", "$(jq -r .title $up)",
        "--description", "$(jq -r .description $up)",
        "--deposit", "$(jq -r .deposit $up)",
    ]
    for guard in (
        'set -euo pipefail',
        'if ! "$PAXD" "$@" --generate-only --offline',
        'die "paxd refused $(basename "$file")"',
        "jq -e '.body.messages | length == 1'",
    ):
        assert guard in logical
    consumer = (ROOT / "sdk/x/upgrade/client/cli/tx.go").read_text(encoding="utf-8")
    parse = consumer.split("func parseArgsToContent(", 1)[1]
    assert 'cmd.Flags().String(cli.FlagDescription, "", "description of proposal")' in consumer
    assert "description, err := cmd.Flags().GetString(cli.FlagDescription)" in parse
    assert "plan := types.Plan{Name: name, Height: height, Info: info}" in parse
    assert "content := types.NewSoftwareUpgradeProposal(title, description, plan)" in parse
    assert "content, err := parseArgsToContent(cmd, name)" in consumer
    assert "gov.NewMsgSubmitProposalWithExpedite(content, deposit, from, isExpedited)" in consumer
    assert "tx.GenerateOrBroadcastTxCLI(cmd.Context(), clientCtx, cmd.Flags(), msg)" in consumer


def test_record_maps_f006_to_declared_proofs_and_evidence():
    record = tomllib.loads(RECORD.read_text(encoding="utf-8"))
    audit = tomllib.loads((
        ROOT / "spec/layerx-merge/programme/audit/records/22.30.181.kvx"
    ).read_text(encoding="utf-8"))
    assert set(record["finding"]) == {"f006"}
    finding = record["finding"]["f006"]
    for key in ("file", "line", "class", "severity", "owner_domain"):
        assert finding[key] == audit["finding"]["f006"][key]
    assert finding["audit"] == "22.30.181"
    declared = {
        node.name for node in ast.parse(Path(__file__).read_text()).body
        if isinstance(node, ast.FunctionDef) and node.name.startswith("test_f006_")
    }
    assert set(finding["tests"]) == declared
    assert len(finding["tests"]) == len(declared) == 3
    task = record["task"]["22"]["33"]["845"]
    assert task["build_cmd"] == "python3 --version"
    assert task["verify_cmd"] == (
        "python3 -m pytest -q tools/qualification/audit_repairs/test_repair_22_33_845.py"
    )
    assert task["evidence_ledger"] == "/out/run/gates.jsonl"
    assert task["wiring_requests"] == []
    assert "No pre-change verification was run" in task["baseline"]
    assert "not executed" in task["review_only"]
