import json
import re
import subprocess
import sys
from pathlib import Path


def test_generator_lists_nested_and_embedded_abis(tmp_path):
    generator = Path(__file__).with_name("gen_system_docs.py")
    root = generator.resolve().parents[3]
    subprocess.run(
        [sys.executable, str(generator), str(tmp_path)],
        cwd=root,
        check=True,
        capture_output=True,
        text=True,
    )
    text = (tmp_path / "precompiles.md").read_text(encoding="utf-8")
    summary = text.split("\n## Summary\n\n", 1)[1].split("\n## ", 1)[0]
    rows = {}
    for line in summary.splitlines()[2:]:
        name, source, addresses, count = (value.strip() for value in line.strip("|").split("|"))
        assert source not in rows, f"duplicate ABI row: {source}"
        rows[source] = (name, addresses, int(count))

    spot_addresses = ", ".join(f"`0x{address:040x}`" for address in range(0x1021, 0x1026))
    for name, filename, addresses in (
        ("perps", "abi/abi.json", "`0x0000000000000000000000000000000000001015`"),
        ("pricefeed", "abi/abi.json", "`0x0000000000000000000000000000000000001020`"),
        ("spot", "abi/pecor.json", spot_addresses),
        ("spot", "abi/orders.json", spot_addresses),
        ("spot", "abi/stoporders.json", spot_addresses),
        ("spot", "abi/quoter.json", spot_addresses),
        ("spot", "abi/vault.json", spot_addresses),
        ("spot", "abi/share.json", spot_addresses),
        ("layerxexchange", "abi.json", "`0x0000000000000000000000000000000000001015`"),
        ("addr", "abi.json", "`0x0000000000000000000000000000000000001004`"),
    ):
        source = f"precompiles/{name}/{filename}"
        entries = json.loads((root / source).read_text(encoding="utf-8"))
        functions = [entry for entry in entries if entry.get("type") == "function"]
        assert functions, source
        assert rows[f"`{source}`"] == (name, addresses, len(functions))
        section = text.split(f"\n## {name} (`{Path(filename).name}`)\n\n", 1)[1].split("\n## ", 1)[0]
        assert f"Address: {addresses}\n" in section
        for entry in entries:
            if entry.get("type") in ("function", "event", "error"):
                assert f"- `{entry['name']}(" in section

    for name, address, method in (
        ("orob", 0x0901, "resolveOffset"),
        ("clearing", 0x0902, "computeClearing"),
        ("oracle", 0x0903, "aggregate"),
        ("pofq", 0x0904, "scoreBatch"),
        ("scheduler", 0x0905, "schedule"),
        ("streams", 0x0906, "open"),
        ("teeattestor", 0x0907, "verify"),
        ("eip712", 0x0908, "domainSeparator"),
    ):
        source = f"precompiles/layerxagent/{name}/{name}.go"
        raw = re.search(r"\babiJSON\s*=\s*`([^`]*)`", (root / source).read_text(encoding="utf-8"))
        assert raw is not None, source
        entries = json.loads(raw.group(1))
        count = sum(entry.get("type") == "function" for entry in entries)
        assert count > 0
        assert rows[f"`{source}:abiJSON`"] == (f"layerxagent/{name}", f"`0x{address:040x}`", count)
        section = text.split(f"\n## layerxagent/{name} (`abiJSON`)\n\n", 1)[1].split("\n## ", 1)[0]
        assert f"Address: `0x{address:040x}`\n" in section
        assert f"- `{method}(" in section
        for entry in entries:
            if entry.get("type") in ("function", "event", "error"):
                assert f"- `{entry['name']}(" in section

    assert (
        "- `resolveOffset(int256 oraclePrice, int16 offsetBps) returns (int256 absolutePrice)` `pure`"
        in text
    )
    assert not any("/legacy/" in source or "/testdata/" in source for source in rows)
