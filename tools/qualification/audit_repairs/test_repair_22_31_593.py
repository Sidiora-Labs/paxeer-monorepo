"""Audit the 7.0.0 documentation against the registered node plans and stores."""

from pathlib import Path
import re

import pytest


ROOT = Path(__file__).resolve().parents[3]
NODE_DOCS = (
    "services/platform/hosted/gateway/README.md",
    "docs/site/docs/agents/running.md",
    "services/explorer/README.md",
    "docs/paxeer-network.md",
    "docs/runbooks/paxeer-x-fork.md",
)
RUNBOOK = "spec/layerx-merge/docs/activation-runbook.md"


def read(relative):
    return (ROOT / relative).read_text(encoding="utf-8")


def prose(relative):
    return " ".join(read(relative).replace("`", "").split())


def go_function(source, name):
    match = re.search(
        rf"^func (?:\([^\n]*\) )?{re.escape(name)}\([^\n]*\{{(.*?)^\}}",
        source,
        re.MULTILINE | re.DOTALL,
    )
    assert match is not None, f"missing Go function: {name}"
    return match.group(1)


def plan_value(name):
    declarations = read("node/upgrades.go") + "\n" + read("node/upgrades_v6120.go")
    expressions = re.findall(
        rf"^const {re.escape(name)} = (\S+)", declarations, re.MULTILINE
    )
    assert len(expressions) == 1, f"expected one declaration for {name}"
    expression = expressions[0]
    if expression.startswith("pcommon."):
        shared = expression.removeprefix("pcommon.")
        expressions = re.findall(
            rf'^const {re.escape(shared)} = ("[^"\n]+")',
            read("precompiles/common/precompiles.go"),
            re.MULTILINE,
        )
        assert len(expressions) == 1, f"missing shared plan constant: {shared}"
        expression = expressions[0]
    assert re.fullmatch(r'"[^"\n]+"', expression), expression
    return expression[1:-1]


def plan_rows():
    section = read(RUNBOOK).split("## Plans\n", 1)[1].split("\n## ", 1)[0]
    rows = [
        [cell.strip() for cell in line.strip("|").split("|")]
        for line in section.splitlines()
        if line.startswith("| ")
    ]
    assert rows[0] == [
        "Plan",
        "Handler",
        "Stores added",
        "Stores deleted",
        "Precompile gate",
        "Pending rule",
    ]
    assert all(len(row) == 6 for row in rows)
    return rows[2:]


@pytest.mark.parametrize("relative", NODE_DOCS)
def test_public_node_docs_serve_px_at_root_with_only_two_hosted_operations(
    relative,
):
    text = prose(relative)
    retired_claims = (
        "The gateway's px_* methods are a hosted contract",
        "public node does not deploy those APIs",
        "Do not assume a public node implements px_*",
        "they are not public node methods",
        "The hosted gateway's px_ methods",
        "Hosted px_ methods are specified separately",
    )
    for claim in retired_claims:
        assert claim.lower() not in text.lower(), (relative, claim)
    assert "The px namespace is served at the node root" in text, relative
    assert re.search(
        r"only px_register and px_requestFunds are hosted operations\.", text
    ), relative


def test_release_documents_700_package_without_retired_daemon_option():
    text = prose("RELEASE.md")
    assert "PAXD_RELEASE_LAYERXD" not in text
    assert "enable the retired daemon option" not in text
    assert "version = v7.0.0" in text
    assert "upgrade = v7.0" in text
    assert "MergeUpgrade" in text
    assert "mergeUpgradePlan" in text


def test_consensus_ledger_opens_700_with_merge_and_later_anchor_deletion():
    text = prose("docs/release/consensus-commits.md")
    assert "The next release is **v7.0.0**" in text
    assert "The next release is **v6.12.0**" not in text
    ledger = read("docs/release/consensus-commits.md").split(
        "## v7.0.0 ledger", 1
    )[1].split("## Historical", 1)[0]
    for fact in (
        plan_value("MergeUpgrade"),
        "MergeUpgrade",
        "mergeUpgradePlan",
        plan_value("AnchorStoreDeleteUpgrade"),
        "AnchorStoreDeleteUpgrade",
        "app.mm.RunMigrations",
        "layerxanchor",
        "later plan",
    ):
        assert fact in ledger, fact
    assert re.search(r"\| `[0-9a-f]{40}` \|", ledger)


def test_interop_documents_archive_attestation_bindings_and_sequencer_receipts():
    text = prose("services/interop/README.md")
    assert "mirror archive" in text
    assert "Attestation bindings and sequencer receipts" in text
    for fact in (
        "GET /v1/deposit/{chainId}/{txHash}/{logIndex}",
        "GET /v1/release/{chainId}/{paxeerTxHash}/{nonce}",
        "distinct-attestor threshold",
        "pax_sequencerReceipt",
        "submitted",
        "receipted",
        "committed",
        "ProtocolReceipt v2",
        "inclusive height windows",
        "layerx-receipt-proof-v2",
        "does not prove BFT commit",
        "no vault or custody",
    ):
        assert fact in text, fact
    assert "402LXP" not in text
    assert "Custody and withdrawal guarantees stay on the Paxeer X chain" not in text
    assert (ROOT / "docs/interop/PORTABILITY.md").is_file()
    assert (ROOT / "spec/layerx-merge/contracts/interop.md").is_file()


def test_activation_runbook_lists_every_explicitly_registered_plan_in_node_order():
    source = read("node/upgrades.go")
    known = re.search(
        r"return \[\]string\{([^}]+)\}", go_function(source, "knownUpgradePlans")
    )
    assert known is not None
    symbols = [name.strip() for name in known.group(1).split(",") if name.strip()]
    registered = re.findall(
        r"SetUpgradeHandler\((\w+),", go_function(source, "RegisterUpgradeHandlers")
    )
    # upgradeName is the separate loop over historical embedded tags.
    explicit = [name for name in registered if name != "upgradeName"]
    assert symbols and explicit == symbols
    rows = plan_rows()
    assert [row[0] for row in rows] == [plan_value(name) for name in symbols]
    for symbol, row in zip(symbols, rows, strict=True):
        handler = re.search(
            rf"SetUpgradeHandler\({re.escape(symbol)}, app\.(\w+)\)", source
        )
        if symbol == "AnchorStoreDeleteUpgrade":
            assert re.search(
                r"SetUpgradeHandler\(AnchorStoreDeleteUpgrade, func\([^\n]+\{\s*"
                r"return app\.mm\.RunMigrations\(ctx, app\.configurator, fromVM\)",
                source,
            )
            assert row[1] == "inline: app.mm.RunMigrations"
        else:
            assert handler is not None, symbol
            assert row[1] == handler.group(1)
        pending = "done_height == 0"
        if symbol == "ActivationUpgrade":
            pending += " and not activation_modules_present"
        assert row[5] == pending


def test_activation_runbook_merge_stores_match_on_disk_keys_and_delete_plan():
    source = read("node/upgrades.go")
    constants = dict(
        re.findall(
            r'^\s*(\w+StoreKey)\s*=\s*"([^"\n]+)"',
            read("storage/common/keys/store_keys.go"),
            re.MULTILINE,
        )
    )
    names = re.findall(
        r"storekeys\.(\w+StoreKey)", go_function(source, "mergeStoreUpgrades")
    )
    assert names and len(names) == len(set(names))
    expected_stores = [constants[name] for name in names]
    rows = {row[0]: row for row in plan_rows()}
    merge = rows[plan_value("MergeUpgrade")]
    assert merge[2].split(", ") == expected_stores
    assert merge[3] == "none"
    assert merge[4] == "mergePrecompileGate"
    table = read(RUNBOOK).split("## Merge stores\n", 1)[1].split("\n## ", 1)[0]
    documented_stores = re.findall(
        r"^\| `storekeys\.(\w+)` \| `([^`]+)` \|$", table, re.MULTILINE
    )
    assert documented_stores == list(zip(names, expected_stores, strict=True))
    assert "layerxhost" not in prose(RUNBOOK)
    app = read("node/app.go")
    deleted = re.search(r'^const retiredAnchorStoreName = "([^"\n]+)"', app, re.M)
    assert deleted is not None
    assert re.search(
        r"case AnchorStoreDeleteUpgrade:\s*return storetypes\.StoreUpgrades"
        r"\{Deleted: \[\]string\{retiredAnchorStoreName\}\}",
        go_function(app, "layerxStoreUpgrades"),
    )
    deletion = rows[plan_value("AnchorStoreDeleteUpgrade")]
    assert deletion[2] == "none"
    assert deletion[3] == deleted.group(1)
    assert deletion[4] == "none"
