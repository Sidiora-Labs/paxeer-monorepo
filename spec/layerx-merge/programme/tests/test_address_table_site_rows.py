from pathlib import Path


ROOT = Path(__file__).resolve().parents[4]
SITE_ORIGINS = {
    "LXP_BANK_STATICCALL": "precompiles/bank/bank.go:218,262,338",
    "LXP_BANK_DELEGATECALL": "precompiles/bank/bank.go:341",
    "LXP_BANK_ZERO_VALUE": "precompiles/bank/bank.go:347",
    "LXP_AGENT_NOT_ACTIVE": "precompiles/layerxagent/common/common.go:66",
    "LXP_PRICEFEED_DELEGATECALL": "precompiles/pricefeed/pricefeed.go:157",
    "LXP_PRICEFEED_STATICCALL": "precompiles/pricefeed/pricefeed.go:160",
    "LXP_PRICEFEED_KEEPER_MISSING": "precompiles/pricefeed/pricefeed.go:163",
    "LXP_PRICEFEED_BATCH_TOO_LARGE": "precompiles/pricefeed/views.go:89",
}


def test_site_rows_present_not_retryable() -> None:
    document = (ROOT / "spec/layerx-merge/contracts/address-table.md").read_text(
        encoding="utf-8"
    )
    registry = document.split("## (b) Error code registry (D6)\n", 1)[1].split(
        "\n## (c) Activation", 1
    )[0]
    document_rows = [
        [cell.strip() for cell in line.strip().strip("|").split("|")]
        for line in document.splitlines()
        if line.strip().startswith("|")
    ]
    for code, origin in SITE_ORIGINS.items():
        matches = [row for row in document_rows if row[0] == code]
        assert len(matches) == 1, f"{code}: expected exactly one row, got {len(matches)}"
        row = matches[0]
        assert len(row) == 5, f"{code}: expected the five registry columns"
        assert f"| {code} |" in registry, f"{code}: missing from section (b)"
        assert row[1], f"{code}: missing message"
        assert row[2], f"{code}: missing emitting construct"
        assert origin in row[3], f"{code}: missing source site {origin}"
        assert row[4] == "no", f"{code}: Retryable must be no, got {row[4]!r}"
