from pathlib import Path

from retired_tests import assert_paths_absent, assert_symbols_unreferenced


def test_checkpoint_and_library_suites_retired():
    retired_paths = [
        "tests/solidity/CheckpointChallengeManager.t.sol",
        "tests/solidity/CheckpointRegistry.t.sol",
        "tests/solidity/GuarantorBond.t.sol",
        "tests/solidity/ContractIntegration.t.sol",
        "tests/solidity/Ed25519.t.sol",
        "tests/solidity/Ed25519Signer.sol",
        "tests/solidity/NativeStateProof.t.sol",
        "tests/solidity/MerkleDomainVector.t.sol",
        "tests/solidity/ContractPrimitives.t.sol",
        "tests/solidity/ContractExecutionSafety.t.sol",
    ]
    assert_paths_absent(retired_paths)
    retired_files = [Path(path).name for path in retired_paths]
    assert_symbols_unreferenced(retired_files, ["tests"])
    assert_symbols_unreferenced(retired_files, ["scripts"])
