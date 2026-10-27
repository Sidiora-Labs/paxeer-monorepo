from pathlib import Path

import pytest

from programme.kvx import KvxError, load
from programme.sandbox import prebuild_for, profiles, unknown_profiles

ROOT = Path(__file__).resolve().parents[4]
MANIFEST = Path(__file__).resolve().parents[1] / "data/sandbox-profiles.kvx"


def test_unknown_profile_is_reported(tmp_path: Path) -> None:
    known = profiles()
    assert known == {
        name.removeprefix("profile."): profile
        for name, profile in load(MANIFEST).items()
        if name.startswith("profile.")
    }
    assert "go-cgo" in known
    assert "cobol" not in known

    spec = tmp_path / "spec.kvx"
    valid_tasks = "".join(
        f'[task.1.1.{number}]\nsandbox_profile = "{profile}"\n\n'
        for number, profile in enumerate(known, start=1)
    )
    spec.write_text(
        '[meta]\nsandbox_profile = "cobol"\n\n' + valid_tasks,
        encoding="utf-8",
    )
    assert unknown_profiles(spec) == []

    spec.write_text(
        spec.read_text(encoding="utf-8")
        + '[task.17.6.99]\nsandbox_profile = "cobol"\n',
        encoding="utf-8",
    )
    assert unknown_profiles(spec) == ["17.6.99"]

    with spec.open("a", encoding="utf-8") as source:
        source.write(
            '\n[task.17.6.100]\nsandbox_profile = "cobol"\n'
            '\n[task.17.6.101]\ntitle = "Missing profile"\n'
            '\n[task.17.6.102]\nsandbox_profile = ["go"]\n'
        )
    assert unknown_profiles(spec) == [
        "17.6.99", "17.6.100", "17.6.101", "17.6.102"
    ]


def test_prebuild_for_go_cgo_task() -> None:
    spec = load(ROOT / "spec/layerx-merge/spec.kvx")
    task = next(
        fields
        for name, fields in spec.items()
        if name.startswith("task.") and fields.get("sandbox_profile") == "go-cgo"
    )
    assert isinstance(task["build_cmd"], str) and task["build_cmd"]
    assert prebuild_for(task) == [
        "make build/liblayerx.a programs/target/debug/liblayerx_programs_sandbox.a"
    ]
    assert prebuild_for({"sandbox_profile": "go"}) == []
    assert prebuild_for({"sandbox_profile": "solidity"}) == [
        "bash contracts/bridge/evm/bootstrap-libs.sh"
    ]

    with pytest.raises(KvxError, match="unknown sandbox_profile"):
        prebuild_for({"sandbox_profile": "cobol"})
    with pytest.raises(KvxError, match="unknown sandbox_profile"):
        prebuild_for({})
