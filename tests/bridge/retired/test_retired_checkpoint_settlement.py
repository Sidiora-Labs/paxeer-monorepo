from pathlib import PurePosixPath

from kernel_retirement import checkpoint_settlement, live_references


def test_checkpoint_settlement_is_retired():
    document = checkpoint_settlement()
    assert document["schema"] == "layerx/checkpoint-settlement/1"

    references = live_references(["contracts/config/checkpoint-settlement.json"])
    readers = {reference.split(":", 1)[0] for reference in references}
    source_readers = {reader for reader in readers if "src" in PurePosixPath(reader).parts}
    assert source_readers == {
        "services/agent/crates/layerx-types/src/historical/settlement.rs",
    }, references
    assert readers == {
        "docker/kernel/Dockerfile.dockerignore",
        "docker/layerx/Dockerfile",
        "docker/layerx/Dockerfile.dockerignore",
        "docker/paxeer/Dockerfile.paxd-node.dockerignore",
        "services/agent/crates/layerx-proof/tests/support/historical_checkpoint.rs",
        "services/agent/crates/layerx-types/src/historical/settlement.rs",
        "services/agent/crates/layerx-types/tests/repair_22_33_1542.rs",
        "services/agent/crates/layerx-types/tests/vectors.rs",
        "services/platform/hosted/paxeer/tests/migrations/v1/settlement_domains.py",
        "services/platform/hosted/paxeer/tests/support/node_tls.rs",
        "tests/ci/test_image_compile_inputs_not_ignored.py",
        "tests/vectors/checkpoint/generate.py",
        "tools/bringup/tests/paxeer-x-runtime-contract.py",
        "tools/jev/tests/fixtures/ledger/beta-qualification.kvx",
        "tools/paxeer-x/build/104.30.2.mk",
        "tools/paxeer-x/tests/retired_callers_test.py",
        "tools/qualification/test_retired_callers_17.py",
    }, references
