"""Keep the beta surface inventory aligned with the merged chain."""

import re
from pathlib import Path


def test_beta_surfaces_name_no_retired_kernel_surface():
    beta = (Path(__file__).resolve().parents[1] / "content" / "beta.md").read_text(
        encoding="utf-8"
    )
    surfaces = re.search(r"^## Surfaces\n(.*?)(?=^## |\Z)", beta, re.M | re.S)
    assert surfaces is not None, "Missing beta surface inventory"
    table = [
        [cell.strip() for cell in line.strip().strip("|").split("|")]
        for line in surfaces.group(1).splitlines()
        if line.strip().startswith("|")
    ]
    assert table[0] == [
        "Surface",
        "Scope",
        "Class",
        "Required rung",
        "Reached rung",
        "Source",
    ]
    assert all(re.fullmatch(r":?-+:?", cell) for cell in table[1])
    rows = table[2:]
    assert rows, "The beta surface inventory must not be empty"
    for row in rows:
        assert len(row) == 6, f"Malformed beta surface row: {row}"
        for retired in (
            "native-core",
            "native-daemon",
            "guarantor",
            "layerxd",
            "custody credit",
        ):
            assert retired not in " ".join(row).lower(), row

    chain_rows = [row for row in rows if row[0] == "paxeer-chain"]
    assert len(chain_rows) == 1, "Expected one merged-chain surface"
    chain_scope = chain_rows[0][1].lower()
    assert "paxd on chain 125" in chain_scope
    assert "merged precompiles" in chain_scope
    for retired in ("custody", "anchor", "kernel"):
        assert retired not in chain_scope

    funding = re.search(r"^Accounts are funded\b[^.]*\.", beta, re.M)
    assert funding is not None, "Missing account funding sentence"
    funding_sentence = " ".join(funding.group(0).lower().split())
    assert "chain transfers" in funding_sentence
    assert "chain 125" in funding_sentence
    assert "custody credit" not in funding_sentence
