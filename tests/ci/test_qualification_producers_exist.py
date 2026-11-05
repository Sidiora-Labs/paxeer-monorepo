import re
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
QUALIFICATION = ROOT / "tools/qualification/paxeer-x"
BUILD = ROOT / "tools/paxeer-x/build"
DAEMON_SOURCE = re.compile(r"tests/daemon/[A-Za-z0-9_]+\.c")
RETIRED_PRODUCERS = (
    "lxp_test_arbiter_admission.c",
    "lxp_test_arbiter_native_authority.c",
    "lxp_test_arbiter_prestate.c",
    "lxp_test_arbiter_prestate_transport.c",
    "lxp_test_asset_execution_prestate.c",
    "lxp_test_asset_inventory.c",
    "lxp_test_program_replay.c",
    "lxp_test_replay_authority.c",
    "lxp_test_replay_catalogue.c",
)


def test_qualification_daemon_producers_exist():
    scripts = sorted(QUALIFICATION.glob("*.py"))
    fragments = sorted(BUILD.glob("*.mk"))
    assert scripts, "No qualification scripts found"
    assert fragments, "No build fragments found"
    references = [
        (path.relative_to(ROOT), number, producer)
        for path in [*scripts, *fragments, ROOT / "Makefile"]
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1)
        for producer in DAEMON_SOURCE.findall(line)
    ]
    missing = [
        f"{path}:{number}: {producer}"
        for path, number, producer in references
        if not (ROOT / producer).is_file()
    ]
    assert not missing, "Missing daemon producers:\n" + "\n".join(missing)


def test_qualification_names_no_retired_daemon_producers():
    references = []
    for directory in (QUALIFICATION, BUILD):
        files = sorted(path for path in directory.rglob("*") if path.is_file())
        assert files, f"No qualification files found under {directory}"
        for path in files:
            contents = path.read_bytes()
            for producer in RETIRED_PRODUCERS:
                if producer.encode("ascii") in contents:
                    references.append(f"{path.relative_to(ROOT)}: {producer}")
    assert not references, "Retired daemon producers remain:\n" + "\n".join(references)
