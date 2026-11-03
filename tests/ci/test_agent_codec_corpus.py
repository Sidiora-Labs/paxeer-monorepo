import os
from pathlib import Path
import re
import shutil
import subprocess


ROOT = Path(__file__).resolve().parents[2]
PREPARE_TARGET = "agent-prepare-codec-corpus"


def test_agent_vectors_prepare_codec_corpus_from_clean_checkout(tmp_path):
    repository = tmp_path / "repository"
    repository.mkdir()
    with subprocess.Popen(
        ["git", "archive", "--format=tar", "HEAD"], cwd=ROOT, stdout=subprocess.PIPE
    ) as archive:
        extracted = subprocess.run(
            ["tar", "-xf", "-", "-C", str(repository)],
            stdin=archive.stdout,
            capture_output=True,
            text=True,
            check=False,
        )
        archive.stdout.close()
        assert archive.wait() == 0
        assert extracted.returncode == 0, extracted.stdout + extracted.stderr
    shutil.copy2(ROOT / "Makefile", repository / "Makefile")
    corpus = repository / "tests/vectors/codec/replay_corpus.lxb"
    assert not corpus.exists()

    environment = os.environ.copy()
    for name in ("MAKEFLAGS", "MFLAGS", "MAKELEVEL", "LAYERX_RUNTIME_CLOCK_SOCKET"):
        environment.pop(name, None)
    environment["CARGO_TARGET_DIR"] = str(tmp_path / "cargo-target")
    environment["CARGO_BUILD_JOBS"] = "4"
    environment["CARGO_TERM_COLOR"] = "never"

    def run(*command):
        result = subprocess.run(
            command,
            cwd=repository,
            env=environment,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            check=False,
        )
        print(result.stdout)
        return result

    vectors = run("make", "agent-test-vectors")
    assert vectors.returncode == 0, vectors.stdout
    assert "running 3 tests" in vectors.stdout, vectors.stdout
    for name in (
        "repository_corpora_are_loaded_and_fully_accounted",
        "unsupported_corpus_version_is_build_breaking",
        "unknown_vector_class_names_the_missing_capability",
    ):
        assert f"test {name} ... ok" in vectors.stdout, vectors.stdout
    assert "test result: ok. 3 passed; 0 failed;" in vectors.stdout, vectors.stdout
    generated = "retired budget pull excluded"
    assert generated in vectors.stdout, vectors.stdout
    assert vectors.stdout.index(generated) < vectors.stdout.index(
        "tools/runtime/run-with-clock.sh"
    ), vectors.stdout
    assert corpus.is_file()
    expected = corpus.read_bytes()
    assert expected

    (repository / PREPARE_TARGET).touch()
    regenerated = run("make", PREPARE_TARGET)
    assert regenerated.returncode == 0, regenerated.stdout
    assert generated in regenerated.stdout, regenerated.stdout
    assert corpus.read_bytes() == expected

    registry_test = (
        "activity::replay_registry_tests::"
        "replay_registry_tracks_kernel_budget_and_stream_declarations"
    )
    registry = run(
        "sh",
        "tools/runtime/run-with-clock.sh",
        "cargo",
        "test",
        "--manifest-path",
        "services/agent/Cargo.toml",
        "--locked",
        "-p",
        "layerx-wire",
        "--lib",
        registry_test,
        "--",
        "--exact",
    )
    assert registry.returncode == 0, registry.stdout
    assert f"test {registry_test} ... ok" in registry.stdout, registry.stdout
    assert "test result: ok. 1 passed; 0 failed;" in registry.stdout, registry.stdout
    assert corpus.read_bytes() == expected

    header = repository / "include/layerx/lx_budget.h"
    declarations, removed = re.subn(
        r"^\s*LX_BUDGET_DEFUND\s*=\s*0x00030008,\s*$",
        "",
        header.read_text(),
        flags=re.MULTILINE,
    )
    assert removed == 1
    header.write_text(declarations)
    refused = run("make", "agent-test-vectors")
    assert refused.returncode != 0, refused.stdout
    assert "unexpected undeclared replay activities: [196616, 196619]" in (
        refused.stdout
    ), refused.stdout
    assert "tools/runtime/run-with-clock.sh" not in refused.stdout, refused.stdout
    assert corpus.read_bytes() == expected
