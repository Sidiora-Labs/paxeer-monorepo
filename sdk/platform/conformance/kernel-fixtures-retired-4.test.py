from pathlib import Path


def test_kernel_fixture_group_3_readers_gone():
    root = Path(__file__).resolve().parents[3]
    retired = tuple(
        f"receipt-programs-{case}-v4".encode("utf-8")
        for case in ("executed", "principal", "mutated-leg")
    ) + (b"receipt-programs-positive-v3.json",)
    readers = (
        root / "services/agent/crates/layerx-sdk/src",
        root / "services/agent/crates/layerx-agentd/src",
        root / "tools/platform/cli/src",
    )
    references = []
    for directory in readers:
        assert directory.is_dir(), f"Missing reader source directory: {directory}"
        for path in sorted(directory.rglob("*")):
            if not path.is_file():
                continue
            source = path.read_bytes()
            for name in retired:
                if name in source:
                    references.append(f"{path.relative_to(root)}: {name.decode('utf-8')}")
    assert not references, "Retired kernel fixture readers remain:\n" + "\n".join(
        references
    )
