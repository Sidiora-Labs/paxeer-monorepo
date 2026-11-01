import sys
from pathlib import Path

from programme.evidence import evidence_line, run
from programme.kvx import load


ROOT = Path(__file__).resolve().parents[4]


def test_evidence_line_shape() -> None:
    revision = load(ROOT / "spec/layerx-merge/spec.kvx")["meta"]["revision"]
    assert revision == "2a1033aa2"
    assert evidence_line(revision, "make -n build", 0, Path("logs/w0.log")) == (
        "revision=2a1033aa2 command=make -n build exit=0 log=logs/w0.log"
    )
    assert evidence_line("R", "C", 3, "L") == "revision=R command=C exit=3 log=L"


def test_run_records_nonzero_exit(tmp_path: Path) -> None:
    source = ROOT / "spec/workflow.kvx"
    expected = source.read_bytes()
    log = tmp_path / "logs" / "child.log"
    literal = "argument with spaces; $(literal) # untouched"
    command = [
        sys.executable,
        "-c",
        "import sys; from pathlib import Path; "
        "sys.stdout.buffer.write(Path(sys.argv[1]).read_bytes()); "
        "sys.stdout.buffer.flush(); "
        "sys.stderr.write(sys.argv[2] + '\\n'); sys.exit(3)",
        str(source),
        literal,
    ]
    assert run(command, log) == 3
    assert log.read_bytes() == expected + (literal + "\n").encode()
    assert command[-1] == literal
