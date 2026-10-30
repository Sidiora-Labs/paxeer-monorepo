from pathlib import Path


def test_frontend_env_names_kernel_archive():
    env_example = (
        Path(__file__).resolve().parents[1] / "deploy/env/frontend.example.env"
    )
    lines = env_example.read_text(encoding="utf-8").splitlines()
    flag_index = lines.index("NEXT_PUBLIC_PAXEER_X_ENABLED=true")
    comment_start = flag_index
    while comment_start > 0 and lines[comment_start - 1].startswith("#"):
        comment_start -= 1
    comment = " ".join(
        line.removeprefix("#").strip().lower()
        for line in lines[comment_start:flag_index]
    )

    assert "kernel archive pages" in comment
    assert "transaction status ladder" in comment
    for line_number, line in enumerate(lines, start=1):
        assert "custody" not in line.lower(), f"{env_example}:{line_number} names custody"
