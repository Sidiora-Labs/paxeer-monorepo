from pathlib import Path
import re


def test_out_of_scope_omits_consensus_test_dockerfile():
    script_path = Path(__file__).resolve().with_name("check.sh")
    script = script_path.read_text(encoding="utf-8")
    match = re.search(r"^OUT_OF_SCOPE='\n(.*?)\n'$", script, re.MULTILINE | re.DOTALL)
    assert match is not None, "check.sh must declare the OUT_OF_SCOPE list"
    out_of_scope = match.group(1).splitlines()

    assert "consensus/test/docker/Dockerfile" not in out_of_scope
    assert out_of_scope == [
        ".devcontainer/Dockerfile",
        "consensus/networks/local/localnode/Dockerfile",
        "consensus/spec/ivy-proofs/Dockerfile",
        "consensus/test/e2e/docker/Dockerfile",
        "services/explorer/backend/.devcontainer/Dockerfile",
        "services/platform/hosted/human/Dockerfile",
        "services/platform/hosted/human/Dockerfile.dockerignore",
        "sdk/contrib/devtools/dockerfile",
        "sdk/contrib/images/simd-dlv/Dockerfile",
        "sdk/contrib/images/simd-env/Dockerfile",
        "sdk/contrib/rosetta/node/Dockerfile",
        "sdk/contrib/rosetta/rosetta-cli/Dockerfile",
        "storage/db_engine/litt/util/testdata/ssh-test.Dockerfile",
        "wasm-runtime/builders/Dockerfile.alpine",
        "wasm-runtime/builders/Dockerfile.centos7",
        "wasm-runtime/builders/Dockerfile.cross",
        "wasm/contrib/prototools-docker/Dockerfile",
    ]
