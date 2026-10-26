import os
import posixpath
import re
import subprocess
from collections.abc import Iterable
from pathlib import Path
from typing import Final


REPO = Path(__file__).resolve().parents[3]

RETIRED: Final[frozenset[str]] = frozenset(
    {
        "contracts/CheckpointRegistry.sol",
        "contracts/EmergencyExit.sol",
        "contracts/GuarantorBond.sol",
        "contracts/LayerXCustody.sol",
        "contracts/WithdrawalClaims.sol",
        "contracts/challenge/CheckpointChallengeManager.sol",
        "contracts/config/Features.sol",
        "contracts/config/StaticConfig.sol",
        "contracts/config/checkpoint-settlement.json",
        "contracts/config/ed25519-vectors.json",
        "contracts/config/native-withdrawal-proof.json",
        "contracts/crypto/Ed25519.sol",
        "contracts/custody/AssetRegistry.sol",
        "contracts/custody/LayerXVault.sol",
        "contracts/custody/ReserveReconciler.sol",
        "contracts/deployment/Blueprint.sol",
        "contracts/deployment/PaxeerBetaDeploymentValidator.sol",
        "contracts/deployment/Predeploys.sol",
        "contracts/deployment/Preinstalls.sol",
        "contracts/governance/LayerXBetaTimelock.sol",
        "contracts/governance/LayerXTimelock.sol",
        "contracts/interfaces/IERC20Minimal.sol",
        "contracts/interfaces/IGuarantorEligibility.sol",
        "contracts/interfaces/ILayerXAssetRegistry.sol",
        "contracts/libraries/Burn.sol",
        "contracts/libraries/Bytes.sol",
        "contracts/libraries/CallerChecker.sol",
        "contracts/libraries/CanonicalCheckpoint.sol",
        "contracts/libraries/CryptographyPrimitives.sol",
        "contracts/libraries/DecimalsConverterHelper.sol",
        "contracts/libraries/EOA.sol",
        "contracts/libraries/Encoding.sol",
        "contracts/libraries/Error.sol",
        "contracts/libraries/Hashing.sol",
        "contracts/libraries/MerkleLib.sol",
        "contracts/libraries/MessageTypes.sol",
        "contracts/libraries/NativePublicationVerifier.sol",
        "contracts/libraries/NativeStateProof.sol",
        "contracts/libraries/PaxeerWithdrawalCodec.sol",
        "contracts/libraries/SafeTransfer.sol",
        "contracts/libraries/SemverComp.sol",
        "contracts/libraries/Storage.sol",
        "contracts/libraries/TransientContext.sol",
        "contracts/manager/BlockErrors.sol",
        "contracts/manager/ManagerContainer.sol",
        "contracts/manager/ManagerMigrator.sol",
        "contracts/manager/StandardValidatorUtils.sol",
        "contracts/security/DelegateCallAware.sol",
        "contracts/security/Governed.sol",
        "contracts/security/LayerXComponent.sol",
        "contracts/security/UUPSNotUpgradeable.sol",
        "contracts/storage/WithdrawalNullifierRegistry.sol",
        "tests/bridge/comet_credit.py",
        "tests/bridge/custody_credit.py",
        "tests/bridge/custody_genesis.py",
        "tests/bridge/deploy_local_custody.py",
        "tests/bridge/local_credit_evidence.py",
        "tests/bridge/owner-custody.py",
        "tests/bridge/paxeer_x_advanced_trust_credit.py",
        "tests/bridge/paxeer_x_multiasset_credit.py",
        "tests/bridge/qualify_credit.py",
        "tests/bridge/sign_credit.c",
        "tests/bridge/sole_writer.sh",
        "tests/bridge/test_credit.c",
        "tests/bridge/test_credit_admission.c",
        "tests/bridge/test_evidence.py",
        "tests/bridge/test_light_credit.c",
        # Retirement assertions name the inventory without consuming retired code.
        "tests/bridge/retired/kernel_retirement.py",
        "tests/bridge/retired/test_kernel_retirement_harness.py",
        "tests/bridge/retired/test_retired_asset_registry.py",
        "tests/bridge/retired/test_retired_block_errors.py",
        "tests/bridge/retired/test_retired_blueprint.py",
        "tests/bridge/retired/test_retired_burn.py",
        "tests/bridge/retired/test_retired_bytes.py",
        "tests/bridge/retired/test_retired_caller_checker.py",
        "tests/bridge/retired/test_retired_canonical_checkpoint.py",
        "tests/bridge/retired/test_retired_checkpoint_challenge_manager.py",
        "tests/bridge/retired/test_retired_checkpoint_registry.py",
        "tests/bridge/retired/test_retired_checkpoint_settlement.py",
        "tests/bridge/retired/test_retired_credit_c_harness.py",
        "tests/bridge/retired/test_retired_credit_qualification.py",
        "tests/bridge/retired/test_retired_cryptography_primitives.py",
        "tests/bridge/retired/test_retired_custody_deploy_scripts.py",
        "tests/bridge/retired/test_retired_decimals_converter_helper.py",
        "tests/bridge/retired/test_retired_delegate_call_aware.py",
        "tests/bridge/retired/test_retired_e_o_a.py",
        "tests/bridge/retired/test_retired_ed25519.py",
        "tests/bridge/retired/test_retired_emergency_exit.py",
        "tests/bridge/retired/test_retired_encoding.py",
        "tests/bridge/retired/test_retired_error.py",
        "tests/bridge/retired/test_retired_features.py",
        "tests/bridge/retired/test_retired_governed.py",
        "tests/bridge/retired/test_retired_guarantor_bond.py",
        "tests/bridge/retired/test_retired_hashing.py",
        "tests/bridge/retired/test_retired_i_e_r_c20_minimal.py",
        "tests/bridge/retired/test_retired_i_guarantor_eligibility.py",
        "tests/bridge/retired/test_retired_i_layer_x_asset_registry.py",
        "tests/bridge/retired/test_retired_layer_x_beta_timelock.py",
        "tests/bridge/retired/test_retired_layer_x_component.py",
        "tests/bridge/retired/test_retired_layer_x_custody.py",
        "tests/bridge/retired/test_retired_layer_x_timelock.py",
        "tests/bridge/retired/test_retired_layer_x_vault.py",
        "tests/bridge/retired/test_retired_light_credit.py",
        "tests/bridge/retired/test_retired_manager_container.py",
        "tests/bridge/retired/test_retired_manager_migrator.py",
        "tests/bridge/retired/test_retired_merkle_lib.py",
        "tests/bridge/retired/test_retired_message_types.py",
        "tests/bridge/retired/test_retired_native_publication_verifier.py",
        "tests/bridge/retired/test_retired_native_state_proof.py",
        "tests/bridge/retired/test_retired_native_withdrawal_proof.py",
        "tests/bridge/retired/test_retired_paxeer_beta_deployment_validator.py",
        "tests/bridge/retired/test_retired_paxeer_withdrawal_codec.py",
        "tests/bridge/retired/test_retired_paxeer_x_credit_profiles.py",
        "tests/bridge/retired/test_retired_predeploys.py",
        "tests/bridge/retired/test_retired_preinstalls.py",
        "tests/bridge/retired/test_retired_reserve_reconciler.py",
        "tests/bridge/retired/test_retired_safe_transfer.py",
        "tests/bridge/retired/test_retired_semver_comp.py",
        "tests/bridge/retired/test_retired_standard_validator_utils.py",
        "tests/bridge/retired/test_retired_staticconfig.py",
        "tests/bridge/retired/test_retired_storage.py",
        "tests/bridge/retired/test_retired_transient_context.py",
        "tests/bridge/retired/test_retired_uups_not_upgradeable.py",
        "tests/bridge/retired/test_retired_withdrawal_claims.py",
        "tests/bridge/retired/test_retired_withdrawal_nullifier_registry.py",
    }
)

_QUOTED_PATH = re.compile(r'''"([^"\r\n]+)"|'([^'\r\n]+)'|<([^<>\r\n]+)>''')


def assert_retired(paths: Iterable[str | Path], root: Path = REPO) -> None:
    root = Path(root).resolve()
    targets = set()
    for path in paths:
        path = Path(path)
        name = (
            path.relative_to(root).as_posix()
            if path.is_absolute()
            else path.as_posix()
        )
        name = posixpath.normpath(name)
        if name in {".", ".."} or name.startswith("../"):
            raise ValueError(f"Retired path must be inside the repository: {path}")
        targets.add(name)

    result = subprocess.run(
        ["git", "ls-files", "--cached", "-z"],
        cwd=root,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
    )
    tracked = {os.fsdecode(name) for name in result.stdout.split(b"\0") if name}
    still_tracked = sorted(targets & tracked)
    patterns = {
        target: re.compile(r"(?<![\w./-])" + re.escape(target) + r"(?![\w./-])")
        for target in targets
    }
    references = []
    for name in sorted(tracked - RETIRED - targets):
        path = Path(name)
        if (
            path.parts[0] in {"docs", "spec"}
            or path.suffix == ".md"
            or name == "GOTCHA.kvx"
        ):
            continue
        source = root / path
        if not source.is_file():
            continue
        content = source.read_bytes()
        if b"\0" in content:
            continue
        lines = content.decode("utf-8", "surrogateescape").splitlines()
        for number, line in enumerate(lines, 1):
            matches = {
                target for target, pattern in patterns.items() if pattern.search(line)
            }
            for match in _QUOTED_PATH.finditer(line):
                literal = next(value for value in match.groups() if value is not None)
                resolved = posixpath.normpath(
                    posixpath.join(path.parent.as_posix(), literal)
                )
                if resolved in targets:
                    matches.add(resolved)
            for target in sorted(matches):
                references.append(f"{name}:{number}: {target}")

    if still_tracked or references:
        findings = ["Retirement check failed:"]
        if still_tracked:
            findings.append("Tracked retired files:")
            findings.extend(f"  {name}" for name in still_tracked)
        if references:
            findings.append("Live references:")
            findings.extend(f"  {reference}" for reference in references)
        raise AssertionError("\n".join(findings))
