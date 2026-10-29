import importlib.util
from pathlib import Path


def test_precompiles_lists_nested_abi_dirs():
    generator_path = Path(__file__).with_name("gen_system_docs.py")
    spec = importlib.util.spec_from_file_location("gen_system_docs", generator_path)
    assert spec is not None
    assert spec.loader is not None
    generator = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(generator)

    filename, text = generator.gen_precompiles()

    assert filename == "precompiles.md"
    summary = text.split("\n## Summary\n\n", 1)[1].split("\n## ", 1)[0]
    for name, address in (
        ("perps", "0x0000000000000000000000000000000000001015"),
        ("pricefeed", "0x0000000000000000000000000000000000001020"),
    ):
        assert f"| {name} | `precompiles/{name}/abi/abi.json` | `{address}` |" in summary
        assert f"\n## {name} (`abi.json`)\n\nAddress: `{address}`\n" in text

    assert (
        "| addr | `precompiles/addr/abi.json` | `0x0000000000000000000000000000000000001004` |"
        in summary
    )
    assert (generator.ROOT / "docs/system" / filename).read_text(
        encoding="utf-8"
    ) == text.rstrip("\n") + "\n"
