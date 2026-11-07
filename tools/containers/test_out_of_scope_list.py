from pathlib import Path
import re


def parse_out_of_scope(script: str) -> list[str]:
    match = re.search(r"^OUT_OF_SCOPE='\n(.*?)\n'$", script, re.MULTILINE | re.DOTALL)
    assert match is not None, "check.sh must declare the OUT_OF_SCOPE list"
    # Match rows() in check.sh: discard empty lines and whole-line comments,
    # preserving path whitespace, inline hashes, order and duplicate entries.
    return [
        row
        for row in match.group(1).split("\n")
        if row and not re.match(r"^[ \t\r\v\f]*#", row)
    ]


def assert_reviewed_out_of_scope(script: str) -> None:
    out_of_scope = parse_out_of_scope(script)
    assert "consensus/test/docker/Dockerfile" not in out_of_scope
    assert out_of_scope == [
        "apps/perps/Dockerfile",
        "deploy/launchpad/Dockerfile.native",
        "deploy/launchpad/Dockerfile.svc",
        "deploy/launchpad/Dockerfile.web",
        "services/platform/hosted/feeder/Dockerfile",
        "services/platform/perps-data/deploy/Dockerfile",
        "services/platform/perps-data/deploy/nats/Dockerfile",
        "tools/deploy/charting-library.Dockerfile",
        "tools/loadtest/Dockerfile",
        "tools/paxctl/Dockerfile",
        "core/deps/db-backend/tools/Dockerfile",
        "core/deps/geth/Dockerfile",
        "core/deps/geth/Dockerfile.alltools",
        "core/deps/geth/crypto/secp256k1/libsecp256k1/ci/linux-debian.Dockerfile",
        "sdk/contrib/devtools/dockerfile",
        "sdk/contrib/images/simd-dlv/Dockerfile",
        "sdk/contrib/images/simd-env/Dockerfile",
        "sdk/contrib/rosetta/node/Dockerfile",
        "sdk/contrib/rosetta/rosetta-cli/Dockerfile",
        "wasm-runtime/builders/Dockerfile.alpine",
        "wasm-runtime/builders/Dockerfile.centos7",
        "wasm-runtime/builders/Dockerfile.cross",
        "wasm/contrib/prototools-docker/Dockerfile",
        "tools/wasm-tools/ci/docker/aarch64-linux/Dockerfile",
        "tools/wasm-tools/ci/docker/aarch64-musl/Dockerfile",
        "tools/wasm-tools/ci/docker/x86_64-linux/Dockerfile",
        "tools/wasm-tools/ci/docker/x86_64-musl/Dockerfile",
        ".devcontainer/Dockerfile",
        "consensus/spec/ivy-proofs/Dockerfile",
        "consensus/test/e2e/docker/Dockerfile",
        "services/explorer/backend/.devcontainer/Dockerfile",
        "storage/db_engine/litt/util/testdata/ssh-test.Dockerfile",
        "docker/platform-node/Dockerfile",
        "docker/redis/Dockerfile",
    ]


def test_out_of_scope_omits_consensus_test_dockerfile():
    script_path = Path(__file__).resolve().with_name("check.sh")
    assert_reviewed_out_of_scope(script_path.read_text(encoding="utf-8"))
