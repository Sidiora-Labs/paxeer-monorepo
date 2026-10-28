from pathlib import Path
import re

import yaml


def test_every_paxd_build_job_prepares_native_toolchain():
    root = Path(__file__).resolve().parents[3]
    paxd_build = re.compile(
        r"\bmake\s+-f\s+chain\.mk\s+"
        r"(?:install-mock-balances|install|build)(?=\s|[;&|]|$)"
    )
    openssl_install = re.compile(
        r"\b(?:apt-get|apt)\s+install\b[^\n;&|]*\blibssl-dev\b"
        r"|\bbrew\s+install\b[^\n;&|]*\bopenssl@3\b"
    )
    rust_install = re.compile(
        r"\brustup\s+toolchain\s+install\s+1\.91\.1(?=\s|[;&|]|$)"
    )
    native_build = re.compile(r"\bmake\s+paxd-native-libs(?=\s|[;&|]|$)")
    builds_checked = 0

    for path in sorted((root / ".github/workflows").glob("*.yml")):
        workflow = yaml.safe_load(path.read_text())
        for job_name, job in workflow.get("jobs", {}).items():
            runs = [
                "\n".join(
                    line
                    for line in step.get("run", "").splitlines()
                    if not line.lstrip().startswith("#")
                )
                for step in job.get("steps", [])
            ]
            for index, run in enumerate(runs):
                if not paxd_build.search(run):
                    continue
                builds_checked += 1
                context = f"{path.name}:{job_name}:step {index + 1}"
                earlier = runs[:index]
                assert any(openssl_install.search(s) for s in earlier), (
                    f"{context} must install OpenSSL headers in an earlier step"
                )
                assert any(rust_install.search(s) for s in earlier), (
                    f"{context} must install Rust 1.91.1 in an earlier step"
                )
                assert any(native_build.search(s) for s in earlier), (
                    f"{context} must run make paxd-native-libs in an earlier step"
                )
                if "macos" in str(job.get("runs-on", "")).lower():
                    assert any(
                        re.search(r"\bbrew\s+install\s+openssl@3\b", s)
                        for s in earlier
                    ), f"{context} must install openssl@3 with Homebrew"
                    for flag, option, directory in (
                        ("CFLAGS", "-I", "include"),
                        ("LDFLAGS", "-L", "lib"),
                    ):
                        export = re.compile(
                            rf"echo\s+[\"']{flag}={option}"
                            rf"\$\(brew --prefix openssl@3\)/{directory}"
                            r"[\"']\s*>>\s*[\"']?\$GITHUB_ENV\b"
                        )
                        assert any(export.search(s) for s in earlier), (
                            f"{context} must export OpenSSL {flag} through GITHUB_ENV"
                        )

    assert builds_checked > 0, "No paxd build steps were checked"
