import hashlib
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parents[3]
LIBRARY_DIRECTORIES = {
    "libwasmvm": "wasm-runtime/internal/api",
    "libwasmvm152": "wasm/x/wasm/artifacts/v152/api",
    "libwasmvm155": "wasm/x/wasm/artifacts/v155/api",
}


def test_builder_copies_libwasmvm_from_context():
    dockerfile = (ROOT / "docker/layerx/Dockerfile").read_text()
    assert "node.hyperpaxeer.com" not in dockerfile
    builder = re.search(
        r"^FROM [^\n]+ AS builder\n(.*?)(?=^FROM )",
        dockerfile,
        re.MULTILINE | re.DOTALL,
    )
    assert builder is not None
    instructions = re.sub(r"\\\n\s*", " ", builder.group(1))
    checksum_run = re.search(
        r"^RUN set -eu; .*sha256sum --check --strict libwasmvm.sha256\)\s*$",
        instructions,
        re.MULTILINE,
    )
    assert checksum_run is not None
    run = checksum_run.group()
    assert not re.search(r"\b(?:curl|wget|https?://)\b", run)
    assert 'case "${TARGETARCH}" in' in run
    assert 'amd64) ARCH_SUFFIX="x86_64" ;;' in run
    assert 'arm64) ARCH_SUFFIX="aarch64" ;;' in run
    assert '*) echo "Unsupported architecture: ${TARGETARCH}" && exit 1 ;;' in run
    library_loop = re.search(r"for library in (.*?); do", run)
    assert library_loop is not None

    for library, directory in LIBRARY_DIRECTORIES.items():
        copy = f"COPY {directory}/ {directory}/\n"
        assert copy in instructions
        assert instructions.index(copy) < checksum_run.start()
        cp = f'cp "{directory}/{library}.${{ARCH_SUFFIX}}.so" /go/lib/;'
        assert cp in run
        assert run.index(cp) < run.index("for library in")
        assert f'"{library}.${{ARCH_SUFFIX}}.so"' in library_loop.group(1)

    digest_copy = (
        "COPY wasm-runtime/libwasmvm-linux.sha256 "
        "wasm-runtime/libwasmvm-linux.sha256\n"
    )
    assert digest_copy in instructions
    assert instructions.index(digest_copy) < checksum_run.start()
    assert (
        "expected=$(awk -v name=\"${library}\" '$2 == name { print $1 }' "
        "wasm-runtime/libwasmvm-linux.sha256);"
    ) in run
    assert 'test "${#expected}" -eq 64;' in run
    assert (
        "printf '%s  %s\\n' \"${expected}\" \"${library}\" "
        ">> /go/lib/libwasmvm.sha256;"
    ) in run
    assert "(cd /go/lib && sha256sum --check --strict libwasmvm.sha256)" in run
    assert "COPY --from=builder /go/lib/*.so /usr/lib/\n" in dockerfile


def test_tree_libraries_match_pinned_digests():
    digests = {}
    digest_file = ROOT / "wasm-runtime/libwasmvm-linux.sha256"
    for line in digest_file.read_text().splitlines():
        digest, filename = line.split()
        assert re.fullmatch(r"[0-9a-f]{64}", digest)
        assert filename not in digests
        digests[filename] = digest

    for library, directory in LIBRARY_DIRECTORIES.items():
        filename = f"{library}.x86_64.so"
        assert filename in digests
        path = ROOT / directory / filename
        assert hashlib.sha256(path.read_bytes()).hexdigest() == digests[filename], (
            f"{path.relative_to(ROOT)} does not match its pinned digest"
        )
