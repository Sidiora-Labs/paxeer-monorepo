import importlib.util
from pathlib import Path

SCRIPT = Path(__file__).resolve().parent / "paxeer-x" / "native-program-human-peer.py"
SPEC = importlib.util.spec_from_file_location("native_program_human_peer", SCRIPT)
driver = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(driver)


def test_sources_bind_the_frame_codec():
    assert "services/agent/crates/layerx-agentd/src/human_frame.rs" in driver.SOURCES
    assert "services/agent/crates/layerx-agentd/tests/human_frame.rs" in driver.SOURCES


def test_build_lists_both_targets():
    target_dir = Path("/tmp/native-program-human-peer-target")
    assert driver.build_command(target_dir) == [
        "/root/.cargo/bin/cargo",
        "test",
        "--locked",
        "--manifest-path",
        str(driver.ROOT / "services/agent/Cargo.toml"),
        "-p",
        "layerx-agentd",
        "--target-dir",
        str(target_dir),
        "--test",
        "native_program_human_peer",
        "--test",
        "human_frame",
        "--no-run",
    ]
    assert driver.TESTS == (
        (
            "native_program_human_peer",
            "real_programs_authenticated_peer_preserves_material_budget_and_owner_decisions",
        ),
        ("human_frame", "human_peer_still_answers_lxhagt01_frames"),
    )
