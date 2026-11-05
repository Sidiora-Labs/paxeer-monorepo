import subprocess
from pathlib import Path

from programme.gates import list_wave, run_wave
from programme.kvx import load

ROOT = Path(__file__).resolve().parents[4]
MANIFEST = ROOT / "spec/layerx-merge/programme/data/wave-gates.kvx"
WAVES = [*map(str, range(7)), "programme"]


def test_manifest_names_every_wave() -> None:
    manifest = load(MANIFEST)
    assert set(manifest) == {f"wave.{n}" for n in WAVES}
    closing_anchors = {"0": 1, "2": 2, "4": 3, "5": 4}
    for n in WAVES:
        wave = manifest[f"wave.{n}"]
        assert wave["script"] == f"scripts/gates/wave-{n}.sh"
        assert Path(wave["script"]).parent == Path("scripts/gates")
        assert wave["evidence_dir"] == f"logs/waves/{n}"
        expected_anchors = (
            [f"{domain}.0.{closing_anchors[n]}" for domain in range(1, 18)]
            if n in closing_anchors
            else []
        )
        assert wave["anchors"] == expected_anchors
        commands = list_wave(n)
        assert commands
        assert all(command.strip() == command and command for command in commands)


def test_run_wave_records_evidence_for_missing_script(tmp_path: Path, capsys) -> None:
    root = tmp_path / "root with spaces; $literal"
    root.mkdir()
    revision = subprocess.run(
        ["git", "rev-parse", "HEAD"],
        cwd=ROOT,
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
    for n in WAVES:
        script = f"scripts/gates/wave-{n}.sh"
        assert not (root / script).exists()
        assert run_wave(n, root) == 127
        directory = root / f"logs/waves/{n}"
        log_path = directory / f"wave-{n}.log"
        lines = (directory / "evidence.log").read_text(encoding="utf-8").splitlines()
        assert len(lines) == 1
        assert lines[0].startswith(f"revision={revision} command=")
        assert script in lines[0]
        assert f" exit=127 log={log_path}" in lines[0]
        assert capsys.readouterr().out == lines[0] + "\n"
        assert script in log_path.read_text(encoding="utf-8")
