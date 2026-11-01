import importlib.util
import json
import subprocess
import sys
from dataclasses import asdict
from pathlib import Path

import pytest


def test_portable_generator_reads_o_keyed_manifest(tmp_path):
    repo = Path(__file__).resolve().parents[4]
    generator_path = repo / "sdk/platform/generators/generate_portable.py"
    spec = importlib.util.spec_from_file_location("generate_portable", generator_path)
    assert spec is not None and spec.loader is not None
    generator = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = generator
    spec.loader.exec_module(generator)

    manifest_path = Path("sdk/platform/conformance/operations.json")
    manifest = json.loads((repo / manifest_path).read_text(encoding="utf-8"))
    records = [record for group in manifest["operations"].values() for record in group]
    items = generator.operations(repo)
    keys = [(item.plane, item.name) for item in items]
    assert len(items) == len(records)
    assert len(set(keys)) == len(keys)
    assert keys == sorted((record["plane"], record["name"]) for record in records)
    by_key = {(item.plane, item.name): item for item in items}
    for record in records:
        expected = dict(record)
        expected["idempotent"] = expected.pop("idempotency")
        expected.setdefault("transport", "http")
        expected.setdefault("to", None)
        expected.setdefault("selector", None)
        expected.setdefault("per_leg", False)
        assert asdict(by_key[record["plane"], record["name"]]) == expected

    rendered = generator.render(repo)
    assert set(rendered) == {
        repo / "sdk/platform/dotnet/Generated/OperationCatalog.cs",
        repo / "sdk/platform/swift/Sources/LayerXSDK/Generated/OperationCatalog.swift",
    }
    for path, content in rendered.items():
        assert path.read_bytes() == content.encode("utf-8")
    subprocess.run(
        [sys.executable, str(generator_path), "--check", str(repo)],
        check=True,
        capture_output=True,
        text=True,
    )

    temporary_manifest = tmp_path / manifest_path
    temporary_manifest.parent.mkdir(parents=True)
    record = records[0]
    invalid_operations = [
        [],
        None,
        "O1",
        1,
        *(
            {key: [record]}
            for key in ("prepare", "O0", "O01", "O-1", "o1", "O1x", "O1\n")
        ),
        *({"O1": group} for group in ([], {}, None, record, [None], [record, []])),
        {"O1": [record], "O2": [record]},
    ]
    for invalid in invalid_operations:
        temporary_manifest.write_text(
            json.dumps({"schema": 1, "operations": invalid}), encoding="utf-8"
        )
        with pytest.raises(ValueError):
            generator.operations(tmp_path)

    invalid_transports = [
        {"transport": "evm", "method": "eth_call", "path": "/invalid"},
        {"transport": "evm", "method": "POST", "path": ""},
        {"transport": "http", "method": "eth_call"},
        {"transport": "unknown", "method": "POST"},
    ]
    for transport in invalid_transports:
        temporary_manifest.write_text(
            json.dumps({"schema": 1, "operations": {"O1": [dict(record, **transport)]}}),
            encoding="utf-8",
        )
        with pytest.raises(ValueError):
            generator.operations(tmp_path)
