import re
from pathlib import Path

from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
TABLE = ROOT / "spec/layerx-merge/programme/data/precompile-activation.kvx"
UPGRADES = ROOT / "node/upgrades.go"
MODULE_PREFIX = "github.com/Sidiora-Labs/Paxeer-X-Network/"


def _address(value: str) -> str:
    return f"0x{int(value, 16):04X}"


def _go_string(expression: str, source: Path) -> str:
    expression = expression.strip()
    if re.fullmatch(r'"[^"]*"', expression):
        return expression[1:-1]
    text = source.read_text(encoding="utf-8")
    if "." in expression:
        alias, symbol = expression.split(".")
        imports = re.findall(r'^\s*(\w+)?\s*"([^"]+)"\s*$', text, re.M)
        packages = [
            path
            for name, path in imports
            if (name or path.rsplit("/", 1)[-1]) == alias
        ]
        assert len(packages) == 1, expression
        assert packages[0].startswith(MODULE_PREFIX), expression
        directory = ROOT / packages[0].removeprefix(MODULE_PREFIX)
        candidates = sorted(
            path for path in directory.glob("*.go") if not path.name.endswith("_test.go")
        )
    else:
        symbol = expression
        candidates = [source]
    definitions = []
    for path in candidates:
        pattern = rf"^\s*(?:const\s+)?{re.escape(symbol)}\s*=\s*([^\n]+)"
        for value in re.findall(pattern, path.read_text(encoding="utf-8"), re.M):
            definitions.append((value.split("//", 1)[0].strip(), path))
    assert len(definitions) == 1, expression
    value, path = definitions[0]
    return _go_string(value, path)


def _gate(name: str) -> tuple[str, dict[str, list[str]]]:
    source = UPGRADES.read_text(encoding="utf-8")
    match = re.search(rf"^func {name}\(\)[^\n]*\{{\n(.*?)^\}}", source, re.M | re.S)
    assert match is not None, name
    body = match.group(1)
    upgrade = re.search(r"Upgrade:\s*([^,]+),", body)
    assert upgrade is not None, name
    entries = re.findall(r"common\.HexToAddress\(([^)]+)\):\s*\{([^}]+)\}", body)
    assert len(entries) == body.count("common.HexToAddress("), name
    addresses = {}
    for expression, modules in entries:
        address = _address(_go_string(expression, UPGRADES))
        assert address not in addresses, address
        addresses[address] = [
            _go_string(module, UPGRADES) for module in modules.split(",")
        ]
    return _go_string(upgrade.group(1), UPGRADES), addresses


def test_merge_gate_lists_twelve_addresses() -> None:
    rows = load(TABLE)
    assert rows.pop("meta") == {"format_version": 1}
    expected_rows = {}
    for gate, expected_plan, count in (
        ("activationPrecompileGate", "v6.9", 8),
        ("mergePrecompileGate", "v7.0", 12),
    ):
        plan, addresses = _gate(gate)
        assert plan == expected_plan
        assert len(addresses) == count
        for address, modules in addresses.items():
            section = f"address.{address}"
            assert section not in expected_rows
            row = {"plan": plan, "modules": modules}
            if plan == "v7.0":
                row["served_at_head"] = False
            expected_rows[section] = row
    assert rows == expected_rows

    merge_rows = {
        section.removeprefix("address."): row
        for section, row in rows.items()
        if row["plan"] == "v7.0"
    }
    expected = {f"0x{value:04X}" for value in range(0x101A, 0x101E)} | {
        f"0x{value:04X}" for value in range(0x0901, 0x0909)
    }
    assert len(merge_rows) == 12
    assert set(merge_rows) == expected
    setup = (ROOT / "precompiles/setup.go").read_text(encoding="utf-8")
    table = re.search(
        r"var MergeAddressTable = \[\]MergeAddress\{(.*?)^\}", setup, re.M | re.S
    )
    assert table is not None
    table_addresses = re.findall(r'HexToAddress\("(0x[0-9A-Fa-f]+)"\)', table.group(1))
    assert len(table_addresses) == 12
    assert {_address(address) for address in table_addresses} == expected
    for row in merge_rows.values():
        assert row["served_at_head"] is False


def test_agent_helpers_need_layerxagent() -> None:
    rows = load(TABLE)
    plan, modules = _gate("mergePrecompileGate")
    assert plan == "v7.0"
    for address in ("0x0901", "0x0902", "0x0904", "0x0908"):
        row = rows[f"address.{address}"]
        assert row["plan"] == plan
        assert row["modules"] == modules[address] == ["layerxagent"]
    streams = rows["address.0x0906"]
    assert streams["plan"] == plan
    assert streams["modules"] == modules["0x0906"] == ["layerxpay"]
    assert rows["address.0x0903"]["modules"] == modules["0x0903"] == [
        "layerxagent",
        "layerxexchange",
    ]
