package cli_test

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func TestChainModulesCompletionRetiredSurfacesAbsent(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatalf("locate repository root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("locate repository go.mod: %v", err)
	}

	retiredSurfaces := []struct {
		id     string
		path   string
		kind   string
		symbol string
	}{
		{"interface.15.cli-interchain-scripts-protoc-swagger-gen-j57", "interchain/scripts/protoc-swagger-gen.sh", "file", ""},
		{"interface.15.cli-interchain-scripts-protocgen-j57", "interchain/scripts/protocgen.sh", "file", ""},
		{"interface.6.cli-tests-bridge-custody-credit-py-j36", "tests/bridge/custody_credit.py", "file", ""},
		{"interface.6.cli-tests-bridge-custody-genesis-py-j36", "tests/bridge/custody_genesis.py", "file", ""},
		{"interface.6.cli-tests-bridge-deploy-local-custody-py-j36", "tests/bridge/deploy_local_custody.py", "file", ""},
		{"interface.6.cli-tests-bridge-local-credit-evidence-py-j36", "tests/bridge/local_credit_evidence.py", "file", ""},
		{"interface.6.cli-tests-bridge-owner-custody-py-j36", "tests/bridge/owner-custody.py", "file", ""},
		{"interface.6.cli-tests-bridge-paxeer-x-advanced-trust-credit-py-j36", "tests/bridge/paxeer_x_advanced_trust_credit.py", "file", ""},
		{"interface.6.cli-tests-bridge-paxeer-x-multiasset-credit-py-j36", "tests/bridge/paxeer_x_multiasset_credit.py", "file", ""},
		{"interface.6.cli-tests-bridge-qualify-credit-py-j36", "tests/bridge/qualify_credit.py", "file", ""},
		{"interface.6.cli-tests-bridge-sign-credit-c-j36", "tests/bridge/sign_credit.c", "file", ""},
		{"interface.6.cli-tests-bridge-sole-writer-sh-j36", "tests/bridge/sole_writer.sh", "file", ""},
		{"interface.6.cli-tests-bridge-test-credit-admission-c-j36", "tests/bridge/test_credit_admission.c", "file", ""},
		{"interface.6.cli-tests-bridge-test-credit-c-j36", "tests/bridge/test_credit.c", "file", ""},
		{"interface.6.cli-tests-bridge-test-evidence-py-j36", "tests/bridge/test_evidence.py", "file", ""},
		{"interface.6.cli-tests-bridge-test-light-credit-c-j36", "tests/bridge/test_light_credit.c", "file", ""},
		{"interface.6.err-assetregistry-assetalreadyregistered-4db515c4", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.err-assetregistry-assetnotregistered-1c6754cc", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.err-assetregistry-invalidasset-c891add2", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.err-blockerrors-duplicatecomponent-07b126aa", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-invalidcomponent-bc81836e", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-invalidmanagerconfiguration-18fd766c", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-invalidmigrationoperation-e8414a4f", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-invalidmigrationstate-706e3d45", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-invalidmigrationversion-ce33d54c", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-invalidmigrationwindow-7d2485e5", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-manageralreadyinitialized-00bf0e3a", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-managernotinitialized-613e17d6", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-managerunauthorized-adc45603", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-migrationcallfailed-d66823f1", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-migrationcodehashmismatch-61996fce", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-migrationcommitmentmismatch-6b73957a", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-migrationstoragelayoutmismatch-91800106", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blockerrors-selectornotallowed-9948a254", "contracts/manager/BlockErrors.sol", "file", ""},
		{"interface.6.err-blueprint-blueprintsealed-3c9bffa2", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.err-blueprint-deploymentcollision-1844405b", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.err-blueprint-deploymentfailed-83d2279b", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.err-blueprint-invalidblueprint-76d3c58e", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.err-blueprint-manageronly-20a1c0fc", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.err-burn-invalidburnamount-2075cc10", "contracts/libraries/Burn.sol", "file", ""},
		{"interface.6.err-bytes-bytesoutofbounds-f3663dee", "contracts/libraries/Bytes.sol", "file", ""},
		{"interface.6.err-bytes-trailingbytes-c3e44a73", "contracts/libraries/Bytes.sol", "file", ""},
		{"interface.6.err-callerchecker-contractcalleronly-d076f1a3", "contracts/libraries/CallerChecker.sol", "file", ""},
		{"interface.6.err-callerchecker-delegatedaccountnotallowed-ed7de61e", "contracts/libraries/CallerChecker.sol", "file", ""},
		{"interface.6.err-callerchecker-directcodelessoriginonly-5d056f31", "contracts/libraries/CallerChecker.sol", "file", ""},
		{"interface.6.err-canonicalcheckpoint-validityprooftoolarge-8b3c1cbd", "contracts/libraries/CanonicalCheckpoint.sol", "file", ""},
		{"interface.6.err-checkpointchallengemanager-challengepending-f0dacd68", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.err-checkpointchallengemanager-challengewindowclosed-2b11bdc2", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.err-checkpointchallengemanager-invalidchallenge-92fa6fd3", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.err-checkpointchallengemanager-nobondowed-d16f72ce", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.err-checkpointchallengemanager-transferfailed-90b8ec18", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.err-checkpointregistry-canonicalchaininvalidated-c2bcecd4", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-challengeauthorityonly-d5f2f4c7", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-checkpointproposeronly-9bf0bda4", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-invalidcertificate-1d39f946", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-invalidconfiguration-c52a9bd3", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-invalidheader-babb01dd", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-invalidwitnesses-b6c2a1b1", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-staterootdiscontinuity-a778c6a6", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-checkpointregistry-witnessesalreadypublished-8c646659", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.err-cryptographyprimitives-invalidsignature-8baa579f", "contracts/libraries/CryptographyPrimitives.sol", "file", ""},
		{"interface.6.err-cryptographyprimitives-invalidsignaturelength-2c33b568", "contracts/libraries/CryptographyPrimitives.sol", "file", ""},
		{"interface.6.err-cryptographyprimitives-invalidsignatures-5682f3bf", "contracts/libraries/CryptographyPrimitives.sol", "file", ""},
		{"interface.6.err-cryptographyprimitives-invalidsignaturev-ba76ff02", "contracts/libraries/CryptographyPrimitives.sol", "file", ""},
		{"interface.6.err-decimalsconverterhelper-inexactconversion-d2d235a3", "contracts/libraries/DecimalsConverterHelper.sol", "file", ""},
		{"interface.6.err-decimalsconverterhelper-unsupporteddecimals-fd91cbed", "contracts/libraries/DecimalsConverterHelper.sol", "file", ""},
		{"interface.6.err-delegatecallaware-delegatecallonly-52cafe32", "contracts/security/DelegateCallAware.sol", "file", ""},
		{"interface.6.err-delegatecallaware-directcallonly-cba87daa", "contracts/security/DelegateCallAware.sol", "file", ""},
		{"interface.6.err-emergencyexit-exitalreadyconsumed-d91b090b", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.err-emergencyexit-exitnoteligible-8dd1f339", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.err-emergencyexit-invalidexitclaim-df6fb140", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.err-encoding-canonicalvaluetoolarge-a7ba0067", "contracts/libraries/Encoding.sol", "file", ""},
		{"interface.6.err-features-foreignchainfacility-6f123303", "contracts/config/Features.sol", "file", ""},
		{"interface.6.err-features-requiredfeaturedisabled-f6b52f41", "contracts/config/Features.sol", "file", ""},
		{"interface.6.err-features-unsupportedfeature-68a6833c", "contracts/config/Features.sol", "file", ""},
		{"interface.6.err-guarantorbond-invalidbondaction-3e5dac69", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.err-guarantorbond-invalidconfiguration-c52a9bd3", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.err-guarantorbond-invalidequivocationevidence-43c22b84", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.err-guarantorbond-reentrancy-ab143c06", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.err-guarantorbond-transferfailed-90b8ec18", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.err-guarantorbond-unauthorized-82b42900", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.err-hashing-hashinputtoolarge-2d71c4b6", "contracts/libraries/Hashing.sol", "file", ""},
		{"interface.6.err-layerxcomponent-invalidcomponentattestation-d93dcc55", "contracts/security/LayerXComponent.sol", "file", ""},
		{"interface.6.err-layerxcustody-invalidcustodytopology-0b9702dd", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.err-layerxtimelockcore-callfailed-8c6081f3", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.err-layerxtimelockcore-invalidoperation-398d4d32", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.err-layerxtimelockcore-operationnotready-f800799b", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.err-layerxtimelockcore-unauthorized-82b42900", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.err-layerxvault-depositrootalreadyregistered-aa2e362e", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.err-layerxvault-depositrootproposeronly-1abb3555", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.err-layerxvault-invalidbondaccounting-cdfb3e58", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.err-layerxvault-invaliddeposit-b2e532de", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.err-layerxvault-invaliddepositroot-aced36ac", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.err-layerxvault-invalidrelease-1cbb0709", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.err-layerxvault-settlementmoduleonly-9462dc65", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.err-merklelib-merkleindexoutofrange-38de686d", "contracts/libraries/MerkleLib.sol", "file", ""},
		{"interface.6.err-merklelib-merkleprooftoodeep-da21365c", "contracts/libraries/MerkleLib.sol", "file", ""},
		{"interface.6.err-messagetypes-invalidmessageenvelope-0e1ec66f", "contracts/libraries/MessageTypes.sol", "file", ""},
		{"interface.6.err-nativepublicationverifier-invalidwitnesses-b6c2a1b1", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.err-nativestateproof-invalidencoding-ad4d102a", "contracts/libraries/NativeStateProof.sol", "file", ""},
		{"interface.6.err-nativestateproof-invalidpath-20db8267", "contracts/libraries/NativeStateProof.sol", "file", ""},
		{"interface.6.err-nativestateproof-rootmismatch-5ade0455", "contracts/libraries/NativeStateProof.sol", "file", ""},
		{"interface.6.err-nativestateproof-wrongmodule-8290c0ea", "contracts/libraries/NativeStateProof.sol", "file", ""},
		{"interface.6.err-nativestateproof-wrongversion-bcb184aa", "contracts/libraries/NativeStateProof.sol", "file", ""},
		{"interface.6.err-paxeerbetadeploymentvalidator-invalidbetadeploymentinput-59f0acf6", "contracts/deployment/PaxeerBetaDeploymentValidator.sol", "file", ""},
		{"interface.6.err-paxeerbetadeploymentvalidator-invalidgenesisartifacts-db77ffb3", "contracts/deployment/PaxeerBetaDeploymentValidator.sol", "file", ""},
		{"interface.6.err-paxeerbetadeploymentvalidator-invalidguarantor-02b613b9", "contracts/deployment/PaxeerBetaDeploymentValidator.sol", "file", ""},
		{"interface.6.err-paxeerbetadeploymentvalidator-invalidusdl-bc9e8e99", "contracts/deployment/PaxeerBetaDeploymentValidator.sol", "file", ""},
		{"interface.6.err-paxeerbetadeploymentvalidator-unfundedguarantor-67290a9a", "contracts/deployment/PaxeerBetaDeploymentValidator.sol", "file", ""},
		{"interface.6.err-paxeerbetadeploymentvalidator-wrongpaxeerchain-fd7279ba", "contracts/deployment/PaxeerBetaDeploymentValidator.sol", "file", ""},
		{"interface.6.err-predeploys-predeployindexoutofrange-7c6a92a9", "contracts/deployment/Predeploys.sol", "file", ""},
		{"interface.6.err-predeploys-unknownpredeployrole-84c764ed", "contracts/deployment/Predeploys.sol", "file", ""},
		{"interface.6.err-preinstalls-componentattestationfailed-75f1d887", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.err-preinstalls-duplicatecomponentaddress-635ed4e1", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.err-preinstalls-incompletecomponentmanifest-b2bb0a2e", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.err-preinstalls-invalidcomponentmanifest-4a2ef030", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.err-reservereconciler-invalidreserveproof-397c495b", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.err-reservereconciler-reservedeficit-8c406fff", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.err-safetransfer-tokenbalancereadfailed-5bf31db9", "contracts/libraries/SafeTransfer.sol", "file", ""},
		{"interface.6.err-safetransfer-tokencallfailed-3f409f9a", "contracts/libraries/SafeTransfer.sol", "file", ""},
		{"interface.6.err-semvercomp-invalidsemanticversion-4e44dcf5", "contracts/libraries/SemverComp.sol", "file", ""},
		{"interface.6.err-semvercomp-semanticversionoverflow-25462cc5", "contracts/libraries/SemverComp.sol", "file", ""},
		{"interface.6.err-staticconfig-assetdefinitionsnotordered-cf167dda", "contracts/config/StaticConfig.sol", "file", ""},
		{"interface.6.err-staticconfig-invalidassetdefinition-59524b08", "contracts/config/StaticConfig.sol", "file", ""},
		{"interface.6.err-staticconfig-invalidstaticconfig-d794ef95", "contracts/config/StaticConfig.sol", "file", ""},
		{"interface.6.err-staticconfig-staticconfigwrongchain-d6a3db70", "contracts/config/StaticConfig.sol", "file", ""},
		{"interface.6.err-storage-invalidstoragenamespace-f8c80136", "contracts/libraries/Storage.sol", "file", ""},
		{"interface.6.err-storage-reservedstorageslot-fdd7ac5d", "contracts/libraries/Storage.sol", "file", ""},
		{"interface.6.err-transientcontext-contextalreadyactive-9b7ee338", "contracts/libraries/TransientContext.sol", "file", ""},
		{"interface.6.err-transientcontext-invalidcontext-271a26f1", "contracts/libraries/TransientContext.sol", "file", ""},
		{"interface.6.err-uupsnotupgradeable-upgradespermanentlydisabled-a41f05b0", "contracts/security/UUPSNotUpgradeable.sol", "file", ""},
		{"interface.6.err-withdrawalclaims-challengedcheckpoint-070b9cc3", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.err-withdrawalclaims-claimnotready-57155ae3", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.err-withdrawalclaims-invalidclaim-ed3c247c", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.err-withdrawalclaims-nullifieralreadyused-cad2ae02", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.err-withdrawalnullifierregistry-consumeronly-b40e21ac", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.err-withdrawalnullifierregistry-invalidtransition-a6532e5d", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.err-withdrawalnullifierregistry-nullifieralreadyused-cad2ae02", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.fmt-ed25519-vectors-json", "contracts/config/ed25519-vectors.json", "file", ""},
		{"interface.6.fmt-native-withdrawal-proof-json", "contracts/config/native-withdrawal-proof.json", "file", ""},
		{"interface.6.go-layerxanchor-anchorreader-j35", "modules/layerxanchor/keeper/keeper.go", "go_type", "AnchorReader"},
		{"interface.6.pc-0x1012-verifydiscoveryproof", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.pc-0x1012-verifyed25519", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.pc-0x1012-verifyreceipt", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.pc-0x1012-verifyreceiptinclusion", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.pc-0x1012-verifystateproof", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.sevt-assetregistry-assetpauseset-6e474d37", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sevt-assetregistry-assetregistered-51a95834", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sevt-assetregistry-assetriskupdated-a7e86d28", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sevt-blueprint-componentdeployed-f00ad9aa", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sevt-blueprint-deploymentssealed-2581095e", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sevt-burn-nativeburned-65647045", "contracts/libraries/Burn.sol", "file", ""},
		{"interface.6.sevt-checkpointchallengemanager-bondowed-11c6481d", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sevt-checkpointchallengemanager-bondwithdrawn-0d41118e", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sevt-checkpointchallengemanager-challengeraised-7f77f864", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sevt-checkpointchallengemanager-challengeresolved-6e14ed77", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sevt-checkpointregistry-checkpointinvalidated-811a8c0d", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sevt-checkpointregistry-checkpointregistered-094d0613", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sevt-checkpointregistry-checkpointwitnessespublished-8e3fa17f", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sevt-emergencyexit-emergencydeclarationset-db898f74", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sevt-emergencyexit-emergencyexitexecuted-4f804cfb", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-bonddeposited-9be06186", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-custodiedvaluesynchronized-25c0bbd1", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-genesisbondedsetsealed-857f8e8c", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-guarantoractivated-726c5656", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-guarantorjailstatusset-0c272464", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-guarantorremoved-14846c89", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-guarantorsignerrotated-797169e0", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-guarantorslashed-6d5a906b", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-slashingauthorityset-0f8a2b32", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-guarantorbond-unbondingstarted-6553f6fe", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sevt-layerxtimelockcore-callpermissionset-851894fb", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sevt-layerxtimelockcore-operationcancelled-cf0f63b9", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sevt-layerxtimelockcore-operationexecuted-1277662f", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sevt-layerxtimelockcore-operationscheduled-6ed6a9d3", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sevt-layerxvault-custodydeposit-7edb71c9", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sevt-layerxvault-custodyrelease-37567a5b", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sevt-layerxvault-depositrootauthorityset-d9fd1f38", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sevt-layerxvault-depositrootregistered-dc7b7dbc", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sevt-layerxvault-guarantorbondset-72225434", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sevt-layerxvault-settlementmoduleset-4c7eb316", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sevt-managercontainer-genesisdeploymentfinalized-e806e8e2", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sevt-managercontainer-managerinitialized-a2655dd9", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sevt-managercontainer-migratorset-3ba47589", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sevt-managercontainer-systemreleaseadvanced-1a847d5b", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sevt-managermigrator-migrationcancelled-a4391902", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sevt-managermigrator-migrationcompleted-461cfa92", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sevt-managermigrator-migrationoperationexecuted-18230d81", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sevt-managermigrator-migrationstaged-a6f898c3", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sevt-reservereconciler-reservereconciled-896573c9", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.sevt-withdrawalclaims-claimcancelled-be665ea0", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sevt-withdrawalclaims-claimfinalised-c01cc728", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sevt-withdrawalclaims-claimqueued-c732a87b", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sevt-withdrawalnullifierregistry-consumerset-8253cfa2", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sevt-withdrawalnullifierregistry-nullifiertransition-e598c62f", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-assetregistry-asset-8539ccf4", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sol-assetregistry-emergencypause-68fd6a9c", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sol-assetregistry-governanceunpause-65d1d97b", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sol-assetregistry-registerasset-ea249288", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sol-assetregistry-updaterisk-67a3114b", "contracts/custody/AssetRegistry.sol", "file", ""},
		{"interface.6.sol-blueprint-blueprinthash-8390f8c9", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-deploy-0add6670", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-deploymentforrole-1dcb3518", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-deploymentssealed-b94e6296", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-deploytimelock-1c3e66c2", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-emergencycouncil-7778960e", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-governancetimelock-9a1fdd79", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-manager-481c6a75", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-predict-a2e8bb1b", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-predicttimelock-38a9df5f", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-protocolversion-2ae9c600", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-releaseversion-afc492c8", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-salt-df3ab1f8", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-seal-3fb27b85", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-staticconfighash-75c0982c", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-timelockblueprinthash-c6a588f1", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-blueprint-timelockinitcodehash-f9397946", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-challenge-cffd46dc", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-challengeperiod-f3f480d9", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-claimable-9b58e0a1", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-guarantorbond-0cd8c9eb", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-minimumchallengebond-bc809d39", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-owedbond-5fb71407", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-raisechallenge-0cfcd92c", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-registry-7b103999", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-resolvechallenge-7d89122d", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-windowclosesat-6a83375e", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointchallengemanager-withdrawbond-66eb9cec", "contracts/challenge/CheckpointChallengeManager.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-canonicalheader-1327f8ca", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-certificatecommitment-2326ab7c", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpoint-order-version-a59c4763", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpointatbatch-dfbd701f", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpointbatchnumber-d1ffc589", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpointepoch-f5a57349", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpointguarantorsetversion-21f5e4e9", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpointhash-ad2f010f", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpointproposer-9627ba55", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-checkpointtimestamp-d84bf6e5", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-derivedgenesischeckpointid-289db000", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-evidence-version-56fd1e93", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-explicitlyinvalidated-80389178", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-finalisedbatchnumber-b35afeb2", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-finalisedepoch-06041e7b", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-finalisedlastsequence-543bc3b9", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-finalisedstateroot-dfd06094", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-finalisedtimestamp-327ab67b", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-firstinvalidatedbatch-a03445ac", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-genesiscanonicalstateroot-5d3ef8ca", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-genesischeckpointid-7412a590", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-genesismanifestdigest-21fbb805", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-genesisreceiptroot-784a620b", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-guarantoreligibility-9e2b11f1", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-guarantorids-0dc3090c", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-invalidatecheckpoint-633b2d12", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-iscanonicalcheckpoint-9d79aff7", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-isfinalised-7500c5a3", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-isrecordedancestor-10ee2d39", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-isrecordedcertificate-05d94f01", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-latestcanonicalcheckpointhash-082ac01d", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-latestfinalisedstateroot-3685caa7", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-maximumattestationdelay-4a10629c", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-maximumattestationdelaymilliseconds-0d686ff6", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-maximumattestations-14b95b25", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-maximumfuturetimestampdrift-958b6a76", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-maximumfuturetimestampdriftmilliseconds-6ff32d03", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-networkid-9025e64c", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-protocolversion-2ae9c600", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-publicationverifier-d0af9a8a", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-publishcheckpointwitnesses-69929738", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-registercheckpoint-c72a8843", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-registeredat-3dd8a007", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-threshold-42cde4e8", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-verifyregisteredcertificate-f5a5d6cd", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-witnessesdigest-dccb9434", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-checkpointregistry-witnessespublished-8df2516e", "contracts/CheckpointRegistry.sol", "file", ""},
		{"interface.6.sol-delegatecallaware-isdelegatecall-a65b0421", "contracts/security/DelegateCallAware.sol", "file", ""},
		{"interface.6.sol-ed25519verifier-verify-5bf48e3a", "contracts/crypto/Ed25519.sol", "file", ""},
		{"interface.6.sol-emergencyexit-balanceconsumed-7276feda", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-challengemanager-023a96fe", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-declareemergency-5ad7b430", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-eligible-d832d92f", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-emergencycouncildeclare-50d17fff", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-executeexit-2613092b", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-executeexit-61d13735", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-exitnullifier-cf44f774", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-governanceemergency-079a3051", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-latestcheckpointhash-b34eb182", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-livenessbound-c5527266", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-networkid-9025e64c", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-nullifierregistry-b870676c", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-recipientverifier-fde26bcf", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-registry-7b103999", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-requiredwithdrawalid-ce3cda10", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-emergencyexit-vault-fbfa77cf", "contracts/EmergencyExit.sol", "file", ""},
		{"interface.6.sol-eoa-library-j36", "contracts/libraries/EOA.sol", "file", ""},
		{"interface.6.sol-error-library-j36", "contracts/libraries/Error.sol", "file", ""},
		{"interface.6.sol-guarantorbond-activateguarantor-237dd04f", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-assetid-44de240a", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-beginunbond-8b570a10", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-bondedactive-a3a1d8d7", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-bondrecord-f408c04d", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-bondtoken-c28f4392", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-cancelunbond-42c8c4f2", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-custodiedvalue-47694c8d", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-custodyauthority-623f809c", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-depositbond-09e08644", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-finalizeunbond-7fac9311", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-genesisbondedsetcommitment-83dabe23", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-genesisbondedsetversion-8de5279e", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-guarantoridforsigner-ca48ef72", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-lastgovernancesequence-fa81a41c", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-membershipauthority-e1821676", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-membershipversion-adbc7b77", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-minimumbond-aa7517e1", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-minimumbondbps-32e6890e", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-networkid-9025e64c", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-protocolversion-2ae9c600", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-removeguarantor-f4350071", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-rotateguarantorsigner-ea708c30", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-sealgenesisbondedset-9ec8334b", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-setguarantorjailstatus-d5582999", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-setslashingauthority-ef455c4c", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-setunresolvedslashing-60781246", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-signerauthorization-83e505ab", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-slashedbalance-96136638", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-slashforcheckpoint-59e1c468", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-slashingauthority-db8192d6", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-submitequivocation-c5ca7dab", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-sweepslashed-10e2b182", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-synccustodiedvalue-152aad29", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-totalbonded-44d96e95", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-unbondingdelay-db7dd35a", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-guarantorbond-vault-fbfa77cf", "contracts/GuarantorBond.sol", "file", ""},
		{"interface.6.sol-iblueprintpredictor-predict-a2e8bb1b", "contracts/deployment/Predeploys.sol", "file", ""},
		{"interface.6.sol-iblueprintpredictor-predicttimelock-38a9df5f", "contracts/deployment/Predeploys.sol", "file", ""},
		{"interface.6.sol-icheckpointregistryauthority-registry-7b103999", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-ierc1271minimal-isvalidsignature-1626ba7e", "contracts/libraries/CryptographyPrimitives.sol", "file", ""},
		{"interface.6.sol-ierc20minimal-balanceof-70a08231", "contracts/interfaces/IERC20Minimal.sol", "file", ""},
		{"interface.6.sol-ierc20minimal-transfer-a9059cbb", "contracts/interfaces/IERC20Minimal.sol", "file", ""},
		{"interface.6.sol-ierc20minimal-transferfrom-23b872dd", "contracts/interfaces/IERC20Minimal.sol", "file", ""},
		{"interface.6.sol-igenesisbond-assetid-44de240a", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-bondtoken-c28f4392", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-custodiedvalue-47694c8d", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-genesisbondedsetcommitment-83dabe23", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-genesisbondedsetversion-8de5279e", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-membershipversion-adbc7b77", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-slashedbalance-96136638", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-totalbonded-44d96e95", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisbond-vault-fbfa77cf", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesischeckpointregistry-derivedgenesischeckpointid-289db000", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesischeckpointregistry-genesiscanonicalstateroot-5d3ef8ca", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesischeckpointregistry-genesischeckpointid-7412a590", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesischeckpointregistry-genesismanifestdigest-21fbb805", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesischeckpointregistry-genesisreceiptroot-784a620b", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesischeckpointregistry-protocolversion-2ae9c600", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesischeckpointregistry-staticconfighash-75c0982c", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisvault-assetregistry-979d7e86", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisvault-guarantorbond-0cd8c9eb", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-igenesisvault-totalcustodied-83d6da5f", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-iguarantorbondaccounting-assetid-44de240a", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-iguarantorbondaccounting-bondtoken-c28f4392", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-iguarantorbondaccounting-synccustodiedvalue-152aad29", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-iguarantorbondaccounting-vault-fbfa77cf", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-iguarantoreligibility-bondedactive-a3a1d8d7", "contracts/interfaces/IGuarantorEligibility.sol", "file", ""},
		{"interface.6.sol-iguarantoreligibility-membershipversion-adbc7b77", "contracts/interfaces/IGuarantorEligibility.sol", "file", ""},
		{"interface.6.sol-iguarantoreligibility-networkid-9025e64c", "contracts/interfaces/IGuarantorEligibility.sol", "file", ""},
		{"interface.6.sol-iguarantoreligibility-protocolversion-2ae9c600", "contracts/interfaces/IGuarantorEligibility.sol", "file", ""},
		{"interface.6.sol-iguarantoreligibility-slashingauthority-db8192d6", "contracts/interfaces/IGuarantorEligibility.sol", "file", ""},
		{"interface.6.sol-ilayerxassetregistry-asset-8539ccf4", "contracts/interfaces/ILayerXAssetRegistry.sol", "file", ""},
		{"interface.6.sol-ilayerxcomponent-componentrole-3d321dbf", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.sol-ilayerxcomponent-releaseversion-afc492c8", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.sol-ilayerxcomponent-staticconfighash-75c0982c", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.sol-ilayerxcomponent-storagelayoutversion-d6a1fe3b", "contracts/deployment/Preinstalls.sol", "file", ""},
		{"interface.6.sol-ilayerxverify-verifydiscoveryproof-2a5bdc60", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.sol-ilayerxverify-verifyed25519-b23cd638", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.sol-ilayerxverify-verifyreceipt-582eecb5", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.sol-ilayerxverify-verifyreceiptinclusion-3f0de084", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.sol-ilayerxverify-verifystateproof-e6038e69", "precompiles/layerxverify/layerxverify.go", "height_gate", "pcommon.RetiredAt"},
		{"interface.6.sol-imanagersealattestation-componentforrole-f5517ac2", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-imanagersealattestation-deploymentid-e16b1032", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-imanagersealattestation-emergencycouncil-7778960e", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-imanagersealattestation-genesisfinalized-4421d5f5", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-imanagersealattestation-governancetimelock-9a1fdd79", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-imanagersealattestation-initialized-158ef93e", "contracts/deployment/Blueprint.sol", "file", ""},
		{"interface.6.sol-layerxbetatimelock-library-j36", "contracts/governance/LayerXBetaTimelock.sol", "file", ""},
		{"interface.6.sol-layerxcomponent-componentrole-3d321dbf", "contracts/security/LayerXComponent.sol", "file", ""},
		{"interface.6.sol-layerxcomponent-releaseversion-afc492c8", "contracts/security/LayerXComponent.sol", "file", ""},
		{"interface.6.sol-layerxcomponent-staticconfighash-75c0982c", "contracts/security/LayerXComponent.sol", "file", ""},
		{"interface.6.sol-layerxcomponent-storagelayoutversion-d6a1fe3b", "contracts/security/LayerXComponent.sol", "file", ""},
		{"interface.6.sol-layerxcustody-challengemanager-023a96fe", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.sol-layerxcustody-checkpointregistry-33b77cf0", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.sol-layerxcustody-guarantorbond-0cd8c9eb", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.sol-layerxcustody-topologyhash-af113dd3", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.sol-layerxcustody-vault-fbfa77cf", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.sol-layerxcustody-withdrawalclaims-9127f507", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.sol-layerxcustody-withdrawalnullifiers-7caa5b68", "contracts/LayerXCustody.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-callpermission-60785aba", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-cancel-c4d252f5", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-completed-f6419d96", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-delayfloor-30f9f4a5", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-execute-774bfd21", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-executor-48c7e528", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-graceperiod-a06db7dc", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-guardian-f29e2236", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-maximumcallvalue-ce951c4e", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-mindelay-c63c4e9b", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-operationid-3f3e7af1", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-operationnonce-4e3f1002", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-proposer-054f8cbf", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-readyat-bea20801", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-receive", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-schedule-79aad5c8", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-setcallpermission-e0c2d34d", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-setrole-50c6d02e", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxtimelockcore-updatemindelay-21eee455", "contracts/governance/LayerXTimelock.sol", "file", ""},
		{"interface.6.sol-layerxvault-assetregistry-979d7e86", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-deposit-8a9e532c", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-depositnonce-6e4d5961", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-depositregistrationdigest-2b19f162", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-depositrootauthority-b4768600", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-depositrootregistered-5caaa4e8", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-depositverifier-26120c88", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-evidence-version-56fd1e93", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-guarantorbond-0cd8c9eb", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-recordeddeposit-faf96c3d", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-registerdepositroot-8ff1fac9", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-release-85a9549a", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-releasedclaim-44fd7485", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-setdepositrootauthority-e9465c84", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-setguarantorbond-05c3d9ea", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-setsettlementmodule-0f2f5f64", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-settlementmodule-f887bc30", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-totalcustodied-83d6da5f", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-layerxvault-totalreleased-c1eeb972", "contracts/custody/LayerXVault.sol", "file", ""},
		{"interface.6.sol-managercontainer-completemigration-a38d3360", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-componentforrole-f5517ac2", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-currentmanifestroot-bc982156", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-currentrelease-fcfb9a6d", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-deploymentid-e16b1032", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-emergencycouncil-7778960e", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-finalizegenesis-85758cc7", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-genesiscanonicalstateroot-5d3ef8ca", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-genesisfinalized-4421d5f5", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-genesismanifestdigest-21fbb805", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-genesisreceiptroot-784a620b", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-governancetimelock-9a1fdd79", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-initialize-4cefcee5", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-initialized-158ef93e", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-isselectorallowed-c9d02fb6", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-manifestat-26bcc857", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-manifestforrole-ecdbdff1", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-migrator-7cd07e47", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-protocolversion-2ae9c600", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-requireallowed-a4ef893e", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-rolecount-ddf96358", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-selectorsforrole-7a836966", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-setmigrator-23cf3118", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-usdlassetid-fe0ad34f", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-usdlcustodycap-b7b680b4", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-usdldecimals-eefd2cf4", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-usdlminimumdeposit-2d846fcc", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managercontainer-usdltoken-8fef276e", "contracts/manager/ManagerContainer.sol", "file", ""},
		{"interface.6.sol-managermigrator-activemigration-f1c605d4", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-cancelmigration-745518e8", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-container-90e534a7", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-executemigration-2285390e", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-executor-c34c08e5", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-governancetimelock-9a1fdd79", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-maximumcallvalue-ce951c4e", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-maximumoperationgas-2dfb59be", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-maximumvalidity-845cd6d9", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-migrationnonce-18ea6fac", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-migrationplan-711f8952", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-minimumdelay-427ad624", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-operationroot-a0f2058e", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-receive", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-managermigrator-stagemigration-b9f2e751", "contracts/manager/ManagerMigrator.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-certificatecommitment-2326ab7c", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-checkpointbatchnumber-d1ffc589", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-checkpointepoch-f5a57349", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-checkpointguarantorsetversion-21f5e4e9", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-finalisedstateroot-dfd06094", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-isrecordedancestor-10ee2d39", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-maximumattestations-14b95b25", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-networkid-9025e64c", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationregistry-threshold-42cde4e8", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationverifier-recipientverifier-fde26bcf", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-nativepublicationverifier-verify-17a9d9cb", "contracts/libraries/NativePublicationVerifier.sol", "file", ""},
		{"interface.6.sol-paxeerwithdrawalcodec-library-j36", "contracts/libraries/PaxeerWithdrawalCodec.sol", "file", ""},
		{"interface.6.sol-reservereconciler-latest-79feb107", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.sol-reservereconciler-liabilityleaf-4617d004", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.sol-reservereconciler-reconcile-45215d90", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.sol-reservereconciler-registry-7b103999", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.sol-reservereconciler-vault-fbfa77cf", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.sol-reservereconciler-withdrawalclaims-9127f507", "contracts/custody/ReserveReconciler.sol", "file", ""},
		{"interface.6.sol-standardvalidatorutils-library-j36", "contracts/manager/StandardValidatorUtils.sol", "file", ""},
		{"interface.6.sol-transientcontext-executioncontext-10df9c1a", "contracts/libraries/TransientContext.sol", "file", ""},
		{"interface.6.sol-transientcontext-transientstorageenabled-7139d5ef", "contracts/libraries/TransientContext.sol", "file", ""},
		{"interface.6.sol-uupsnotupgradeable-proxiableuuid-52d1902d", "contracts/security/UUPSNotUpgradeable.sol", "file", ""},
		{"interface.6.sol-uupsnotupgradeable-upgradeto-3659cfe6", "contracts/security/UUPSNotUpgradeable.sol", "file", ""},
		{"interface.6.sol-uupsnotupgradeable-upgradetoandcall-4f1ef286", "contracts/security/UUPSNotUpgradeable.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-cancelchallengedclaim-52c1d82c", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-challengemanager-023a96fe", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-claim-bd66528a", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-finaliseclaim-3851a861", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-networkid-9025e64c", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-nullifierregistry-b870676c", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-pendingamount-e2c14469", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-queueclaim-0d7992c8", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-queueclaim-9c6b4bce", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-registry-7b103999", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-vault-fbfa77cf", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-withdrawalleaf-5400160a", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalclaims-withdrawalnullifier-3e51d152", "contracts/WithdrawalClaims.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-cancel-ec7a4f43", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-claimfornullifier-b57ea0a6", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-consume-4da5a6d7", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-consumer-de69b6ac", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-reserve-fdd83b7a", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-setconsumer-02c9ef45", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-status-52ad0d5e", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-withdrawalidfornullifier-719cc02a", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
		{"interface.6.sol-withdrawalnullifierregistry-withdrawalidused-698db267", "contracts/storage/WithdrawalNullifierRegistry.sol", "file", ""},
	}
	if len(retiredSurfaces) != 523 {
		t.Fatalf("retired interface inventory has %d entries, want 523", len(retiredSurfaces))
	}
	seen := make(map[string]bool, len(retiredSurfaces))
	solComments := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*`)
	for _, retired := range retiredSurfaces {
		if seen[retired.id] {
			t.Fatalf("duplicate retired interface %s", retired.id)
		}
		seen[retired.id] = true
		t.Run("retired/"+retired.id, func(t *testing.T) {
			path := filepath.Join(root, filepath.FromSlash(retired.path))
			_, err := os.Lstat(path)
			if os.IsNotExist(err) && retired.kind != "height_gate" {
				return
			}
			if err != nil {
				t.Fatalf("check retired interface %s at %s: %v", retired.id, retired.path, err)
			}
			if retired.kind == "file" {
				t.Fatalf("retired interface %s remains at %s", retired.id, retired.path)
			}
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read retired interface %s at %s: %v", retired.id, retired.path, err)
			}
			switch retired.kind {
			case "height_gate":
				file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.AllErrors)
				if err != nil {
					t.Fatalf("parse retired interface %s height gate at %s: %v", retired.id, retired.path, err)
				}
				found := false
				ast.Inspect(file, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					selector, ok := call.Fun.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					pkg, ok := selector.X.(*ast.Ident)
					if !ok || pkg.Name+"."+selector.Sel.Name != retired.symbol {
						return true
					}
					for _, arg := range call.Args {
						upgrade, ok := arg.(*ast.SelectorExpr)
						if !ok || upgrade.Sel.Name != "MergeUpgrade" {
							continue
						}
						if pkg, ok := upgrade.X.(*ast.Ident); ok && pkg.Name == "pcommon" {
							found = true
						}
					}
					return true
				})
				if !found {
					t.Errorf("retired interface %s requires a %s call with pcommon.MergeUpgrade at %s", retired.id, retired.symbol, retired.path)
				}
				gateTest := "precompiles/layerxverify/retire_gate_test.go"
				info, err := os.Stat(filepath.Join(root, filepath.FromSlash(gateTest)))
				if err != nil {
					t.Fatalf("retired interface %s requires height gate test %s: %v", retired.id, gateTest, err)
				}
				if !info.Mode().IsRegular() {
					t.Errorf("retired interface %s height gate test %s is not a regular file", retired.id, gateTest)
				}
			case "go_type":
				file, err := parser.ParseFile(token.NewFileSet(), path, source, parser.AllErrors)
				if err != nil {
					t.Fatalf("parse retired interface %s at %s: %v", retired.id, retired.path, err)
				}
				ast.Inspect(file, func(n ast.Node) bool {
					if decl, ok := n.(*ast.TypeSpec); ok && decl.Name.Name == retired.symbol {
						t.Errorf("retired interface %s type %s remains at %s", retired.id, retired.symbol, retired.path)
					}
					return true
				})
			case "abi_method":
				var entries []struct {
					Type string `json:"type"`
					Name string `json:"name"`
				}
				if err := json.Unmarshal(source, &entries); err != nil {
					t.Fatalf("parse retired interface %s ABI at %s: %v", retired.id, retired.path, err)
				}
				for _, entry := range entries {
					if entry.Type == "function" && entry.Name == retired.symbol {
						t.Errorf("retired interface %s method %s remains at %s", retired.id, retired.symbol, retired.path)
					}
				}
			case "sol_function":
				declaration := regexp.MustCompile(`\bfunction\s+` + regexp.QuoteMeta(retired.symbol) + `\s*\(`)
				if declaration.Match(solComments.ReplaceAll(source, nil)) {
					t.Errorf("retired interface %s function %s remains at %s", retired.id, retired.symbol, retired.path)
				}
			default:
				t.Fatalf("unknown retired interface kind %q for %s", retired.kind, retired.id)
			}
		})
	}

	plannedPaths := []struct {
		task string
		path string
	}{
		{"6.13.1", "interchain/modules/core/types/proto_registry_test.go"},
		{"6.2.3", "precompiles/common/lxp_python_parity_test.go"},
		{"6.2.4", "precompiles/common/lxp_typescript_parity_test.go"},
		{"6.32.3", "precompiles/layerxanchor/retired_surface_test.go"},
		{"6.34.1", "precompiles/layerxanchor/unwired_test.go"},
		{"6.71.1", "precompiles/layerxverify/retire_gate_test.go"},
		{"6.59.7", "tests/bridge/retired/repair_6_59_7.kvx"},
		{"6.53.2", "tests/bridge/retired/test_retired_asset_registry.py"},
		{"6.54.1", "tests/bridge/retired/test_retired_block_errors.py"},
		{"6.59.1", "tests/bridge/retired/test_retired_blueprint.py"},
		{"6.57.1", "tests/bridge/retired/test_retired_burn.py"},
		{"6.57.2", "tests/bridge/retired/test_retired_bytes.py"},
		{"6.57.3", "tests/bridge/retired/test_retired_caller_checker.py"},
		{"6.57.4", "tests/bridge/retired/test_retired_canonical_checkpoint.py"},
		{"6.53.1", "tests/bridge/retired/test_retired_checkpoint_challenge_manager.py"},
		{"6.52.1", "tests/bridge/retired/test_retired_checkpoint_registry.py"},
		{"6.59.7", "tests/bridge/retired/test_retired_checkpoint_settlement.py"},
		{"6.60.2", "tests/bridge/retired/test_retired_credit_c_harness.py"},
		{"6.60.3", "tests/bridge/retired/test_retired_credit_qualification.py"},
		{"6.57.5", "tests/bridge/retired/test_retired_cryptography_primitives.py"},
		{"6.60.5", "tests/bridge/retired/test_retired_custody_deploy_scripts.py"},
		{"6.97.2", "tests/bridge/retired/test_retired_daemon_consumers.py"},
		{"6.57.6", "tests/bridge/retired/test_retired_decimals_converter_helper.py"},
		{"6.80.1", "tests/bridge/retired/test_retired_delegate_call_aware.py"},
		{"6.57.7", "tests/bridge/retired/test_retired_e_o_a.py"},
		{"6.56.1", "tests/bridge/retired/test_retired_ed25519.py"},
		{"6.52.2", "tests/bridge/retired/test_retired_bridge_emergency_exit.py"},
		{"6.57.8", "tests/bridge/retired/test_retired_encoding.py"},
		{"6.57.9", "tests/bridge/retired/test_retired_error.py"},
		{"6.59.5", "tests/bridge/retired/test_retired_features.py"},
		{"6.52.3", "tests/bridge/retired/test_retired_guarantor_bond.py"},
		{"6.57.10", "tests/bridge/retired/test_retired_hashing.py"},
		{"6.56.3", "tests/bridge/retired/test_retired_i_guarantor_eligibility.py"},
		{"6.56.4", "tests/bridge/retired/test_retired_i_layer_x_asset_registry.py"},
		{"6.55.2", "tests/bridge/retired/test_retired_layer_x_beta_timelock.py"},
		{"6.52.4", "tests/bridge/retired/test_retired_layer_x_custody.py"},
		{"6.55.1", "tests/bridge/retired/test_retired_layer_x_timelock.py"},
		{"6.53.3", "tests/bridge/retired/test_retired_layer_x_vault.py"},
		{"6.54.2", "tests/bridge/retired/test_retired_manager_container.py"},
		{"6.54.3", "tests/bridge/retired/test_retired_manager_migrator.py"},
		{"6.58.1", "tests/bridge/retired/test_retired_merkle_lib.py"},
		{"6.58.2", "tests/bridge/retired/test_retired_message_types.py"},
		{"6.58.4", "tests/bridge/retired/test_retired_native_state_proof.py"},
		{"6.59.8", "tests/bridge/retired/test_retired_native_withdrawal_proof.py"},
		{"6.59.2", "tests/bridge/retired/test_retired_paxeer_beta_deployment_validator.py"},
		{"6.58.5", "tests/bridge/retired/test_retired_paxeer_withdrawal_codec.py"},
		{"6.60.4", "tests/bridge/retired/test_retired_paxeer_x_credit_profiles.py"},
		{"6.59.3", "tests/bridge/retired/test_retired_predeploys.py"},
		{"6.59.4", "tests/bridge/retired/test_retired_preinstalls.py"},
		{"6.53.4", "tests/bridge/retired/test_retired_reserve_reconciler.py"},
		{"6.58.7", "tests/bridge/retired/test_retired_semver_comp.py"},
		{"6.54.4", "tests/bridge/retired/test_retired_standard_validator_utils.py"},
		{"6.59.6", "tests/bridge/retired/test_retired_staticconfig.py"},
		{"6.58.8", "tests/bridge/retired/test_retired_storage.py"},
		{"6.58.9", "tests/bridge/retired/test_retired_transient_context.py"},
		{"6.80.4", "tests/bridge/retired/test_retired_uups_not_upgradeable.py"},
		{"6.52.5", "tests/bridge/retired/test_retired_withdrawal_claims.py"},
		{"6.80.5", "tests/bridge/retired/test_retired_withdrawal_nullifier_registry.py"},
	}
	if len(plannedPaths) != 58 {
		t.Fatalf("planned path inventory has %d entries, want 58", len(plannedPaths))
	}
	seenPaths := make(map[string]bool, len(plannedPaths))
	for _, planned := range plannedPaths {
		if seenPaths[planned.path] {
			t.Fatalf("duplicate planned path %s", planned.path)
		}
		seenPaths[planned.path] = true
		t.Run("planned/"+planned.path, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(root, filepath.FromSlash(planned.path)))
			if err != nil {
				t.Fatalf("task %s required planned path %s: %v", planned.task, planned.path, err)
			}
			if !info.Mode().IsRegular() {
				t.Errorf("task %s required planned path %s is not a regular file", planned.task, planned.path)
			}
		})
	}
}
