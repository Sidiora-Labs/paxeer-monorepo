"""Keep planned ask-stream declarations aligned with the native pay ABI."""

import json
import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
CATALOGUE = ROOT / "spec/layerx-merge/contracts/inference-market.md"
PROTOCOL = ROOT / "docs/agent/inference-market/protocol.md"


def _parameters(declaration):
    parameters = []
    for field in declaration.split(","):
        match = re.fullmatch(
            r"\s*([a-z][a-z0-9]*)(\s+indexed)?(?:\s+[A-Za-z_][A-Za-z0-9_]*)?\s*",
            field,
        )
        assert match is not None, f"Invalid documented parameter: {field!r}"
        parameters.append((match.group(1), match.group(2) is not None))
    return parameters


def test_ask_stream_documentation_matches_abi():
    abi = json.loads(
        (ROOT / "precompiles/layerxpay/abi.json").read_text(encoding="utf-8")
    )
    catalogue = CATALOGUE.read_text(encoding="utf-8")
    protocol = PROTOCOL.read_text(encoding="utf-8")
    methods = ("streamOpenForAsk", "askStreamOf")
    event_name = "AskStreamOpened"
    event_fields = [("bytes32", True), ("bytes32", True), ("address", False)]

    entries = {}
    for name in (*methods, event_name):
        matches = [entry for entry in abi if entry.get("name") == name]
        assert len(matches) == 1, f"Expected one ABI entry for {name}"
        entries[name] = matches[0]

    for path, document in ((CATALOGUE, catalogue), (PROTOCOL, protocol)):
        for name in methods:
            entry = entries[name]
            assert entry["type"] == "function"
            assert [field["type"] for field in entry["outputs"]] == ["bytes32"]
            declarations = re.findall(
                rf"\b{name}\s*\(([^()]*)\)\s*(view\s+)?returns\s*\(([^()]*)\)",
                document,
            )
            assert len(declarations) == 1, f"{path}: expected one {name} declaration"
            inputs, view, outputs = declarations[0]
            assert _parameters(inputs) == [
                (field["type"], False) for field in entry["inputs"]
            ], f"{path}: {name} inputs differ from the ABI"
            assert _parameters(outputs) == [
                (field["type"], False) for field in entry["outputs"]
            ], f"{path}: {name} outputs differ from the ABI"
            assert bool(view) == (entry["stateMutability"] == "view"), (
                f"{path}: {name} view modifier differs from the ABI"
            )

        event = entries[event_name]
        assert event["type"] == "event"
        assert event["anonymous"] is False
        abi_fields = [(field["type"], field["indexed"]) for field in event["inputs"]]
        assert abi_fields == event_fields
        declarations = re.findall(rf"\b{event_name}\s*\(([^()]*)\)", document)
        assert len(declarations) == 1, f"{path}: expected one {event_name} declaration"
        assert _parameters(declarations[0]) == abi_fields, (
            f"{path}: {event_name} types or indexing differ from the ABI"
        )

    assert "Every row is status new at the revision" in catalogue
    rows = [
        [cell.strip() for cell in line.strip("|").split("|")]
        for line in catalogue.splitlines()
        if line.startswith("|")
    ]
    for name in (*methods, event_name):
        matches = [
            row for row in rows if len(row) == 6 and re.match(rf"{name}\s*\(", row[3])
        ]
        assert len(matches) == 1, f"Expected one planned catalogue row for {name}"
        row = matches[0]
        assert row[2].endswith("(planned)"), f"{name} implementation must stay planned"
        assert row[3].endswith("(planned)"), f"{name} signature must stay planned"
        assert row[5] == "new", f"{name} catalogue status must stay new"

    assert "The inference market is not yet available." in protocol
    assert "This page describes the planned F11" in protocol
    assert "The planned pay methods at `0x101B` are:" in protocol
