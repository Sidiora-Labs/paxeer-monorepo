import hashlib
import pathlib
import shutil
import subprocess
import time


ROOT = pathlib.Path(__file__).resolve().parents[2]


def _run(repo, *command):
    result = subprocess.run(
        command,
        cwd=repo,
        capture_output=True,
        text=True,
        check=False,
    )
    assert result.returncode == 0, result.stdout + result.stderr
    return result.stdout


def _defined_symbols(repo, path):
    output = _run(repo, "nm", "-g", "--defined-only", "--format=posix", str(path))
    return {fields[0] for line in output.splitlines() if len(fields := line.split()) >= 3}


def _archive_state(path):
    return path.stat().st_mtime_ns, hashlib.sha256(path.read_bytes()).hexdigest()


def test_native_archives_track_source_membership(tmp_path):
    repo = tmp_path / "repo"
    repo.mkdir()
    for directory in ("src", "include"):
        shutil.copytree(ROOT / directory, repo / directory)
    for name in (
        "Makefile",
        "tools/build/sanitizers.mk",
        "services/platform/Makefile.inc",
        "tools/paxeer-x/build/104.35.11.mk",
        "tools/paxeer-x/build/6.6.mk",
        "tools/paxeer-x/build/proof.mk",
    ):
        destination = repo / name
        destination.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(ROOT / name, destination)
        assert destination.read_bytes() == (ROOT / name).read_bytes()

    build = repo / "out" / "native-archives"
    archives = (build / "liblayerx.a", build / "liblayerx-testing.a")
    targets = tuple(str(path.relative_to(repo)) for path in archives)

    def make(*requested):
        _run(
            repo,
            "make",
            "--no-print-directory",
            "-j2",
            "BUILD_DIR=out/native-archives",
            *requested,
        )

    make(*targets)
    assert not (repo / "build").exists()
    removed_source = repo / "src/protocol/lxp_result.c"
    removed_member = "lxp_result.o"
    retained_member = "lxp_arena.o"
    members_before = {}
    symbols_before = {}
    removed_symbols = {}
    retained_symbols = {}
    for archive, object_dir in zip(archives, ("obj", "test-obj")):
        members_before[archive] = _run(repo, "ar", "t", str(archive)).splitlines()
        assert members_before[archive].count(removed_member) == 1
        assert retained_member in members_before[archive]
        symbols_before[archive] = _defined_symbols(repo, archive)
        removed_symbols[archive] = _defined_symbols(
            repo, build / object_dir / "src/protocol/lxp_result.o"
        )
        retained_symbols[archive] = _defined_symbols(
            repo, build / object_dir / "src/protocol/lxp_arena.o"
        )
        assert "lxp_result_name" in removed_symbols[archive]
        assert "lxp_arena_init" in retained_symbols[archive]
        assert removed_symbols[archive] <= symbols_before[archive]
        assert retained_symbols[archive] <= symbols_before[archive]
        objects = list((build / object_dir).rglob("*.o"))
        assert len(objects) == len(members_before[archive])
        assert all(path.stat().st_mtime_ns < archive.stat().st_mtime_ns for path in objects)

    object_times = {path: path.stat().st_mtime_ns for path in build.rglob("*.o")}
    archives_before = {path: _archive_state(path) for path in archives}
    make(*targets)
    assert {path: _archive_state(path) for path in archives} == archives_before
    assert {path: path.stat().st_mtime_ns for path in object_times} == object_times

    time.sleep(1.1)
    removed_source.unlink()
    assert {path: _archive_state(path) for path in archives} == archives_before
    assert {path: path.stat().st_mtime_ns for path in object_times} == object_times
    for target in targets:
        make(target)

    for archive in archives:
        members_after = _run(repo, "ar", "t", str(archive)).splitlines()
        assert removed_member not in members_after
        assert members_after == [
            member for member in members_before[archive] if member != removed_member
        ]
        symbols_after = _defined_symbols(repo, archive)
        assert removed_symbols[archive].isdisjoint(symbols_after)
        assert retained_symbols[archive] <= symbols_after
        assert symbols_after == symbols_before[archive] - removed_symbols[archive]
        modified, digest = _archive_state(archive)
        assert modified > archives_before[archive][0]
        assert digest != archives_before[archive][1]
    assert {path: path.stat().st_mtime_ns for path in object_times} == object_times

    archives_after = {path: _archive_state(path) for path in archives}
    for _ in range(2):
        make(*targets)
        assert {path: _archive_state(path) for path in archives} == archives_after
        assert {path: path.stat().st_mtime_ns for path in object_times} == object_times
