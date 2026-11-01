from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


def test_build_fragment_names_no_balance_read():
    fragment = REPO_ROOT / "tools/paxeer-x/build/104.31.2.mk"
    assert "tests/programs/test_balance_read.c" not in fragment.read_text(
        encoding="utf-8"
    )


def test_interface_producer_names_no_interface_suite():
    producer = REPO_ROOT / "tools/qualification/paxeer-x/program-interface-producer.py"
    source = producer.read_text(encoding="utf-8")
    assert "tests/programs/test_interface_protocol.c" not in source
    compile(source, str(producer), "exec")
