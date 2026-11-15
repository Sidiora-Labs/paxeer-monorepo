"""Keep the shipped SDK quickstarts on the supported RPC surface."""

import json
import re
from pathlib import Path

import pytest


ROOT = Path(__file__).resolve().parents[2]
QUICKSTARTS = (
    "SdkQuickstartGo.md",
    "SdkQuickstartJvm.md",
    "SdkQuickstartPython.md",
    "SdkQuickstartRust.md",
    "SdkQuickstartSwift.md",
    "SdkQuickstarts.md",
)


@pytest.mark.parametrize("page", QUICKSTARTS)
def test_quickstarts_name_no_retired_lx_methods(page):
    text = (ROOT / "docs/wiki" / page).read_text(encoding="utf-8")
    retired = re.findall(r"\blx_[A-Za-z0-9_]*", text)
    assert not retired, f"{page} names retired RPC methods: {retired}"


@pytest.mark.parametrize("page", QUICKSTARTS)
def test_quickstarts_name_px_request_funds_for_faucet(page):
    text = (ROOT / "docs/wiki" / page).read_text(encoding="utf-8")
    sections = re.split(r"(?m)^#{1,6} (.+)\n", text)
    faucets = [
        (sections[index], sections[index + 1])
        for index in range(1, len(sections), 2)
        if "faucet" in sections[index].lower()
    ]
    assert faucets, f"{page} must document a faucet section"
    for heading, body in faucets:
        assert re.search(r"\bpx_requestFunds\b", body), (
            f"{page}: {heading} must name the hosted px_requestFunds operation"
        )


def test_hosted_gateway_rpc_methods_match_contract():
    document = json.loads(
        (ROOT / "services/platform/hosted/gateway/openrpc.json").read_text(
            encoding="utf-8"
        )
    )
    names = {
        method["name"]
        for method in document["methods"]
        if not method["name"].startswith("px_getInference")
    }
    assert names == {
        "pax_sequencerReceipt",
        "px_getAccount",
        "px_getBalances",
        "px_getCapabilities",
        "px_getCommit",
        "px_getHistory",
        "px_getNetwork",
        "px_getReceiptKeys",
        "px_getRouteCatalogue",
        "px_getSequenceBatch",
        "px_getUnifiedHistory",
        "px_getValidators",
        "px_listAssets",
        "px_listHolderRecords",
        "px_openSession",
        "px_register",
        "px_requestFunds",
        "px_resolveAccount",
    }
