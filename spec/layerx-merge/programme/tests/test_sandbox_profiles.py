from pathlib import Path

from programme.kvx import load

MANIFEST = Path(__file__).resolve().parents[1] / "data/sandbox-profiles.kvx"


def test_manifest_lists_eleven_profiles() -> None:
    profiles = load(MANIFEST)
    assert set(profiles) == {
        "profile.go",
        "profile.go-cgo",
        "profile.rust",
        "profile.node",
        "profile.elixir",
        "profile.solidity",
        "profile.python",
        "profile.jvm",
        "profile.dotnet",
        "profile.swift",
        "profile.mixed",
    }
    for name, profile in profiles.items():
        assert set(profile) == {"toolchains", "network", "prebuild"}, name
        toolchains = profile["toolchains"]
        assert isinstance(toolchains, list) and toolchains, name
        assert all(isinstance(tool, str) and tool.strip() for tool in toolchains), name
        network = profile["network"]
        assert isinstance(network, str) and network.strip(), name
        prebuild = profile["prebuild"]
        assert isinstance(prebuild, list), name
        assert all(
            isinstance(command, str) and command.strip() for command in prebuild
        ), name

    assert profiles["profile.rust"]["network"] == "locked crate registry only"
    assert profiles["profile.node"]["network"] == (
        "pnpm store of services/wallet/pnpm-lock.yaml only"
    )
    assert "forge v1.7.1" in profiles["profile.solidity"]["toolchains"]
    assert profiles["profile.solidity"]["prebuild"] == [
        "bash contracts/bridge/evm/bootstrap-libs.sh"
    ]


def test_go_cgo_prebuilds_kernel_archives() -> None:
    profile = load(MANIFEST)["profile.go-cgo"]
    assert profile["prebuild"] == [
        "make build/liblayerx.a programs/target/debug/liblayerx_programs_sandbox.a"
    ]
    assert "go 1.25.6 with CGO_ENABLED=1" in profile["toolchains"]
    assert "C17 compiler 13.3.0" in profile["toolchains"]
    assert "rust 1.91.1" in profile["toolchains"]
