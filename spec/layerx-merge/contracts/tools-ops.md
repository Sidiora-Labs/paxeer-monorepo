# Tools ops contract

Source revision: `2a1033aa2`. This contract freezes the cited interface and acceptance blocks of the specification. Statuses include the specification overrides. A source marked planned has line 0; a positive source line is verified in the pinned revision.

The context and decision tables reproduce the rules linked by interface identifiers. An anchor without a linked context gains no additional storage, gas, activation or pause rule here. Acceptance qualification remains UNRUN until its declared gate succeeds.

## bringup

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.32.1</code> | req | 16032.1 |
| <code>acceptance.16.32.1</code> | scenario | Given docs-names.sh pointing at spec/layerx-merge/spec.kvx, when it runs from the repository root, then it exits 0 |
| <code>acceptance.16.32.1</code> | test | tools/bringup/tests/test_spec_paths.py::BringupSpecPathsTest.test_docs_names_reads_existing_spec (planned) |
| <code>acceptance.16.32.1</code> | qualification | UNRUN |
| <code>req.16032</code> | ac_1 | tools/bringup/docs-names.sh reads spec/layerx-merge/spec.kvx instead of the deleted spec at tools/bringup/docs-names.sh:6 and exits 0. |
| <code>acceptance.16.33.1</code> | req | 16033.1 |
| <code>acceptance.16.33.1</code> | scenario | Given the four bring-up scripts without guarantor, custody-proof and replica steps, when the scanner runs on them and ca.sh services runs, then both exit 0 |
| <code>acceptance.16.33.1</code> | test | tools/bringup/tests/test_kernel_genesis_retired.py::KernelGenesisRetiredTest.test_bringup_names_no_guarantor_or_custody_proof (planned) |
| <code>acceptance.16.33.1</code> | qualification | UNRUN |
| <code>req.16033</code> | ac_1 | The scanner finds no guarantor, custody-credit or replica caller in tools/bringup/kernel-genesis.sh (HEAD: :35-134), value-loop.sh (:22-53), human-state-preserve.sh (:27) or ca.sh (:173-180), and tools/bringup/ca.sh services exits 0. |
| <code>acceptance.16.34.1</code> | req | 16034.1 |
| <code>acceptance.16.34.1</code> | scenario | Given the bring-up tests without retired daemon and custody-proof checks, when the scanner and foundation-artifacts.py run, then both exit 0 |
| <code>acceptance.16.34.1</code> | test | tools/bringup/tests/test_bringup_fixtures_retired.py::BringupFixturesRetiredTest.test_fixtures_name_no_retired_daemon (planned) |
| <code>acceptance.16.34.1</code> | qualification | UNRUN |
| <code>req.16034</code> | ac_1 | The scanner finds no custody-credit or finality-authority caller in tools/bringup/check-live.test.sh (HEAD: :1471, :3328) or the two fixture tests (HEAD: tools/bringup/tests/foundation-artifacts.py:18, tools/bringup/tests/paxeer-x-fixture-foundation.py:315), and foundation-artifacts.py exits 0. |
| <code>acceptance.16.35.1</code> | req | 16035.1 |
| <code>acceptance.16.35.1</code> | scenario | Given xweb-config.sh with the lx_ methods replaced, when render serve runs into a temporary directory, then it exits 0 and no rendered file contains lx_ |
| <code>acceptance.16.35.1</code> | test | tools/bringup/tests/test_xweb_config.py::XwebConfigTest.test_render_emits_no_lx_route (planned) |
| <code>acceptance.16.35.1</code> | qualification | UNRUN |
| <code>req.16035</code> | ac_1 | tools/bringup/xweb-config.sh render serve writes configuration naming no lx_ method (HEAD: tools/bringup/xweb-config.sh:22-177) and exits 0. |
| <code>capability.16.7.1</code> | title | Point the bring-up docs and live checks at the layerx-merge spec |
| <code>capability.16.7.1</code> | reqs | 16032.1 |
| <code>capability.16.7.2</code> | title | Remove the guarantor and custody-proof steps from kernel genesis and the value loop |
| <code>capability.16.7.2</code> | reqs | 16033.1 |
| <code>capability.16.7.3</code> | title | Remove the retired daemon readiness and custody-proof checks from bring-up tests |
| <code>capability.16.7.3</code> | reqs | 16034.1 |
| <code>capability.16.7.4</code> | title | Render xweb bring-up configuration without lx_ routes |
| <code>capability.16.7.4</code> | reqs | 16035.1 |

## ci-scripts

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.26.1</code> | req | 16026.1 |
| <code>acceptance.16.26.1</code> | scenario | Given spec/layerx-merge/qualification.kvx at HEAD, when beta-ledger-check.sh and beta-report.sh --check run without a ledger flag, then both exit 0 and read the layerx-merge ledger |
| <code>acceptance.16.26.1</code> | test | tools/ci/tests/test_beta_ledger_paths.py::BetaLedgerPathsTest.test_ledger_check_reads_layerx_merge_ledger (planned) |
| <code>acceptance.16.26.1</code> | qualification | UNRUN |
| <code>req.16026</code> | ac_1 | scripts/ci/beta-ledger-check.sh, whose default ledger at scripts/ci/beta-ledger-check.sh:685 names the deleted spec/layerx-beta tree, reads spec/layerx-merge/qualification.kvx by default and exits 0. |
| <code>acceptance.16.27.1</code> | req | 16027.1 |
| <code>acceptance.16.27.1</code> | scenario | Given xweb-workflow-check.sh without retired or missing paths, when it runs from the repository root, then it exits 0 |
| <code>acceptance.16.27.1</code> | test | tools/ci/tests/test_xweb_workflow_check.py::XwebWorkflowCheckTest.test_check_passes_against_head_tree (planned) |
| <code>acceptance.16.27.1</code> | qualification | UNRUN |
| <code>req.16027</code> | ac_1 | scripts/ci/xweb-workflow-check.sh exits 0 after the retired paths at scripts/ci/xweb-workflow-check.sh:72-76, :286-311 and :317-327 and the missing paths at :167 and :270 are removed. |
| <code>acceptance.16.28.1</code> | req | 16028.1 |
| <code>acceptance.16.28.1</code> | scenario | Given the state surface list without the retired root contracts, when solidity-state-surface.sh runs, then it exits 0 |
| <code>acceptance.16.28.1</code> | test | tools/ci/tests/test_solidity_state_surface.py::SolidityStateSurfaceTest.test_surface_check_passes_on_kept_contracts (planned) |
| <code>acceptance.16.28.1</code> | qualification | UNRUN |
| <code>req.16028</code> | ac_1 | scripts/ci/solidity-state-surface.sh exits 0 with only kept contracts in the list that starts at scripts/ci/solidity-state-surface.sh:4. |
| <code>acceptance.16.29.1</code> | req | 16029.1 |
| <code>acceptance.16.29.1</code> | scenario | Given check-live.sh with the lx_ probes replaced, when the scanner runs on it and the script runs without a mode, then the scanner exits 0 and the script exits 2 with its usage |
| <code>acceptance.16.29.1</code> | test | tools/ci/tests/test_wallet_check_live.py::WalletCheckLiveTest.test_check_live_names_no_lx_method (planned) |
| <code>acceptance.16.29.1</code> | qualification | UNRUN |
| <code>req.16029</code> | ac_1 | scripts/wallet/check-live.sh probes no lx_ method after the calls at scripts/wallet/check-live.sh:19, :784 and :822 are replaced, and prints its usage with exit 2 when run without a mode. |
| <code>capability.16.5.1</code> | title | Point the beta ledger tools at the layerx-merge qualification ledger |
| <code>capability.16.5.1</code> | reqs | 16026.1 |
| <code>capability.16.5.2</code> | title | Remove the retired daemon and credit paths from the xweb workflow check |
| <code>capability.16.5.2</code> | reqs | 16027.1 |
| <code>capability.16.5.3</code> | title | Drop the retired root contracts from the Solidity state surface check |
| <code>capability.16.5.3</code> | reqs | 16028.1 |
| <code>capability.16.5.4</code> | title | Probe pax_ routes instead of lx_ routes in the wallet live check |
| <code>capability.16.5.4</code> | reqs | 16029.1 |
| <code>resolution.17.973</code> | title | decision.1602 binds: Tooling reads the layerx-merge spec and ledger |
| <code>resolution.17.973</code> | applies_to | decision.1602<br>task.16.5.1<br>task.16.7.1<br>task.16.9.2<br>task.16.10.1<br>task.16.10.2<br>task.16.10.3 |
| <code>resolution.17.973</code> | source | decision.1602 in fragments/J58/decision.kvx; tasks from affects |

## ci-workflows

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.22.1</code> | req | 16022.1 |
| <code>acceptance.16.22.1</code> | scenario | Given xweb-test.yml without the kernel web job and path filters, when the retired-callers scanner runs on it with component kernel-web, then it exits 0 with no finding |
| <code>acceptance.16.22.1</code> | test | tools/ci/tests/test_xweb_workflow.py::XwebWorkflowTest.test_workflow_has_no_kernel_web_job (planned) |
| <code>acceptance.16.22.1</code> | qualification | UNRUN |
| <code>req.16022</code> | ac_1 | The retired-callers scanner reports no kernel-web finding for .github/workflows/xweb-test.yml after the filters at .github/workflows/xweb-test.yml:25-26 are removed. |
| <code>acceptance.16.23.1</code> | req | 16023.1 |
| <code>acceptance.16.23.1</code> | scenario | Given the three CI files without the layerx-wire crate, when the scanner runs with component wire-codec on them, then it exits 0 with no finding |
| <code>acceptance.16.23.1</code> | test | tools/ci/tests/test_wire_codec_ci.py::WireCodecCiTest.test_ci_names_no_wire_crate (planned) |
| <code>acceptance.16.23.1</code> | qualification | UNRUN |
| <code>req.16023</code> | ac_1 | The retired-callers scanner reports no wire-codec finding for .github/workflows/interop-x402.yml:12, .github/workflows/platform.yml:594 or scripts/ci/release-version.py:37. |
| <code>acceptance.16.24.1</code> | req | 16024.1 |
| <code>acceptance.16.24.1</code> | scenario | Given human.yml without layerxd and sign-credit, when the scanner runs on it, then it exits 0 with no finding |
| <code>acceptance.16.24.1</code> | test | tools/ci/tests/test_human_workflow.py::HumanWorkflowTest.test_workflow_builds_no_retired_binary (planned) |
| <code>acceptance.16.24.1</code> | qualification | UNRUN |
| <code>req.16024</code> | ac_1 | The retired-callers scanner reports no finding for .github/workflows/human.yml once layerxd and sign-credit leave the list at .github/workflows/human.yml:64. |
| <code>acceptance.16.25.1</code> | req | 16025.1 |
| <code>acceptance.16.25.1</code> | scenario | Given node-build.yml testing only ./node/..., when the scanner runs on it with component custody-deposit, then it exits 0 with no finding |
| <code>acceptance.16.25.1</code> | test | tools/ci/tests/test_node_build_workflow.py::NodeBuildWorkflowTest.test_workflow_tests_no_custody_module (planned) |
| <code>acceptance.16.25.1</code> | qualification | UNRUN |
| <code>req.16025</code> | ac_1 | The retired-callers scanner reports no custody-deposit finding for .github/workflows/node-build.yml after the step at .github/workflows/node-build.yml:126 drops the custody module. |
| <code>acceptance.16.67.1</code> | req | 16067.1 |
| <code>acceptance.16.67.1</code> | scenario | Given ci.yml with the loadtest job on the routed runner, when the workflow is parsed and the runner routing check runs, then the job runs exactly the two loadtest commands and the check exits 0 |
| <code>acceptance.16.67.1</code> | test | tools/ci/tests/test_ci_loadtest_job.py::CiLoadtestJobTest.test_root_workflow_runs_loadtest_module (planned) |
| <code>acceptance.16.67.1</code> | qualification | UNRUN |
| <code>req.16067</code> | ac_1 | .github/workflows/ci.yml (jobs at .github/workflows/ci.yml:20-79) gains a loadtest job running go -C tools/loadtest build ./... and go -C tools/loadtest test -count=1 ./..., and scripts/ci/runner-routing-check.sh exits 0. |
| <code>acceptance.16.68.1</code> | req | 16068.1 |
| <code>acceptance.16.68.1</code> | scenario | Given the Makefile without the replay golden targets and CI without the replay-matrix job, when the core gate inventory check and the runner routing check run, then both exit 0 |
| <code>acceptance.16.68.1</code> | test | tools/ci/tests/test_replay_golden_ci.py::ReplayGoldenCiTest.test_core_gates_and_routing_pass_without_replay_golden (planned) |
| <code>acceptance.16.68.1</code> | qualification | UNRUN |
| <code>req.16068</code> | ac_1 | scripts/workspace/check-core-gates.sh (pattern at scripts/workspace/check-core-gates.sh:11) and scripts/ci/runner-routing-check.sh (row at :44) exit 0 with the replay-matrix job, scripts/ci/replay-matrix.sh and both replay golden inventory rows removed. |
| <code>capability.16.4.1</code> | title | Drop the kernel web adapter and root trees from the xweb workflow |
| <code>capability.16.4.1</code> | reqs | 16022.1 |
| <code>capability.16.4.2</code> | title | Drop the layerx-wire crate from the CI workflows and the release version map |
| <code>capability.16.4.2</code> | reqs | 16023.1 |
| <code>capability.16.4.3</code> | title | Drop layerxd and sign-credit from the human workflow build |
| <code>capability.16.4.3</code> | reqs | 16024.1 |
| <code>capability.16.4.4</code> | title | Drop the retired custody module from the node build workflow test step |
| <code>capability.16.4.4</code> | reqs | 16025.1 |
| <code>capability.16.4.5</code> | title | Run the loadtest module build and tests in the root CI workflow |
| <code>capability.16.4.5</code> | reqs | 16067.1 |
| <code>capability.16.5.6</code> | title | Remove the replay golden release gate from CI and the core gate inventory |
| <code>capability.16.5.6</code> | reqs | 16068.1 |

## container-images

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.17.1</code> | req | 16017.1 |
| <code>acceptance.16.17.1</code> | scenario | Given docker/relay-archive and docker/intent-ingester deleted, when tools/containers/check.sh runs over the whole layout, then it exits 0 and no tracked file names a deleted Dockerfile |
| <code>acceptance.16.17.1</code> | test | tools/containers/test_retired_images.py::RetiredImagesTest.test_container_layout_passes_without_retired_images (planned) |
| <code>acceptance.16.17.1</code> | qualification | UNRUN |
| <code>req.16017</code> | ac_1 | tools/containers/check.sh (layout check at tools/containers/check.sh:1) exits 0 after docker/relay-archive and docker/intent-ingester are deleted. |
| <code>acceptance.16.18.1</code> | req | 16018.1 |
| <code>acceptance.16.18.1</code> | scenario | Given docker/kernel/Dockerfile without the layerxd, guarantor, handover, archive codec and custody-proof stages, when tools/containers/check.sh kernel runs, then it exits 0 |
| <code>acceptance.16.18.1</code> | test | tools/containers/test_kernel_image.py::KernelImageTest.test_kernel_dockerfile_passes_build_check (planned) |
| <code>acceptance.16.18.1</code> | qualification | UNRUN |
| <code>req.16018</code> | ac_1 | tools/containers/check.sh kernel, which runs docker build --check on the kernel entry at tools/containers/check.sh:43, exits 0 with the retired stages removed from docker/kernel/Dockerfile:16-40. |
| <code>acceptance.16.19.1</code> | req | 16019.1 |
| <code>acceptance.16.19.1</code> | scenario | Given docker/kernel/init.sh with the guarantor, layerxd, layerxd-authority and relay-archive services removed, when bash -n and tools/paxeer-x/tests/retired_callers_test.py --component guarantor-runtime --component lni run over the three kernel box files, then both exit 0 |
| <code>acceptance.16.19.1</code> | test | tools/containers/test_kernel_init.py::KernelInitTest.test_init_lists_no_retired_service (planned) |
| <code>acceptance.16.19.1</code> | qualification | UNRUN |
| <code>req.16019</code> | ac_1 | docker/kernel/init.sh no longer defines or starts layerxd (docker/kernel/init.sh:1257), layerxd-authority (:1274), guarantor (:1141, :1277-1278) or relay-archive (:2140, :2171), and passes bash -n and the retired-callers check for the guarantor-runtime and lni components. |
| <code>acceptance.16.20.1</code> | req | 16020.1 |
| <code>acceptance.16.20.1</code> | scenario | Given docker/platform-node deleted and unregistered, when tools/containers/check.sh platform-node runs, then it exits 2 as an unknown service |
| <code>acceptance.16.20.1</code> | test | tools/containers/test_platform_node_retired.py::PlatformNodeRetiredTest.test_check_refuses_retired_service_name (planned) |
| <code>acceptance.16.20.1</code> | qualification | UNRUN |
| <code>req.16020</code> | ac_1 | tools/containers/check.sh platform-node exits 2 once the rows at tools/containers/check.sh:45 and :78 are gone, while tools/containers/check.sh kernel exits 0. |
| <code>acceptance.16.21.1</code> | req | 16021.1 |
| <code>acceptance.16.21.1</code> | scenario | Given the layerx and paxd-node Dockerfiles without the checkpoint settlement copy, when tools/containers/check.sh layerx paxeer runs, then it exits 0 |
| <code>acceptance.16.21.1</code> | test | tools/containers/test_paxd_images.py::PaxdImagesTest.test_layerx_and_paxeer_images_pass_build_check (planned) |
| <code>acceptance.16.21.1</code> | qualification | UNRUN |
| <code>req.16021</code> | ac_1 | tools/containers/check.sh layerx paxeer exits 0 with the checkpoint-settlement.json copies removed from docker/layerx/Dockerfile:17 and docker/paxeer/Dockerfile.paxd-node:11. |
| <code>acceptance.16.69.1</code> | req | 16069.1 |
| <code>acceptance.16.69.1</code> | scenario | Given the human service image without the movement role and binary, when the entrypoint runs with role movement and check.sh runs on human-service, then the entrypoint exits 64 and check.sh exits 0 |
| <code>acceptance.16.69.1</code> | test | tools/containers/test_human_service_image.py::HumanServiceImageTest.test_movement_role_refused_and_image_checks (planned) |
| <code>acceptance.16.69.1</code> | qualification | UNRUN |
| <code>req.16069</code> | ac_1 | docker/human-service/entrypoint.sh refuses the movement role with exit 64 once it leaves the list at docker/human-service/entrypoint.sh:15, and tools/containers/check.sh human-service exits 0 without the movement provider build at docker/human-service/Dockerfile:12. |
| <code>capability.16.3.1</code> | title | Retire the relay-archive and intent-ingester images |
| <code>capability.16.3.1</code> | reqs | 16017.1 |
| <code>capability.16.3.2</code> | title | Drop the retired daemons and checkpoint settlement file from the kernel image build |
| <code>capability.16.3.2</code> | reqs | 16018.1 |
| <code>capability.16.3.3</code> | title | Remove the layerxd, guarantor and relay archive services from the kernel init script |
| <code>capability.16.3.3</code> | reqs | 16019.1 |
| <code>capability.16.3.4</code> | title | Retire the platform-node image |
| <code>capability.16.3.4</code> | reqs | 16020.1 |
| <code>capability.16.3.5</code> | title | Drop the checkpoint settlement file from the layerx and paxd-node images |
| <code>capability.16.3.5</code> | reqs | 16021.1 |
| <code>capability.16.3.6</code> | title | Remove the movement role and provider binary from the human service image |
| <code>capability.16.3.6</code> | reqs | 16069.1 |
| <code>resolution.17.974</code> | title | decision.1603 binds: Retired daemons leave the container images |
| <code>resolution.17.974</code> | applies_to | decision.1603<br>task.16.3.1<br>task.16.3.2<br>task.16.3.3<br>task.16.3.4 |
| <code>resolution.17.974</code> | source | decision.1603 in fragments/J58/decision.kvx; tasks from affects |
| <code>resolution.17.979</code> | title | decision.1608 binds: Human service tooling stays except the movement role |
| <code>resolution.17.979</code> | applies_to | decision.1608<br>task.16.3.6 |
| <code>resolution.17.979</code> | source | decision.1608 in fragments/J58/decision.kvx; tasks from affects |

## foundry-profile

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.31.1</code> | req | 16031.1 |
| <code>acceptance.16.31.1</code> | scenario | Given the beta deploy script deleted, when make -n workspace-build and the scanner over scripts run, then both exit 0 |
| <code>acceptance.16.31.1</code> | test | tools/ci/tests/test_beta_deploy_retired.py::BetaDeployRetiredTest.test_root_forge_build_without_beta_script (planned) |
| <code>acceptance.16.31.1</code> | qualification | UNRUN |
| <code>req.16031</code> | ac_1 | scripts/PaxeerBetaDeploy.s.sol (HEAD: imports at :4-22) is deleted, make -n workspace-build (Makefile:3309) exits 0 and the scanner finds no anchor or custody-claims caller under scripts. |
| <code>acceptance.16.66.1</code> | req | 16066.1 |
| <code>acceptance.16.66.1</code> | scenario | Given foundry.toml without the checkpoint read permissions, when forge config --json and forge build --offline run at the root, then both exit 0 and no retired path is granted |
| <code>acceptance.16.66.1</code> | test | tools/ci/tests/test_foundry_profile.py::FoundryProfileTest.test_root_profile_grants_no_retired_paths (planned) |
| <code>acceptance.16.66.1</code> | qualification | UNRUN |
| <code>req.16066</code> | ac_1 | forge config --json at the repository root reports no fs_permissions entry for tests/vectors/checkpoint or contracts/config and no tests/solidity test root (HEAD: the profile read by workspace-build at Makefile:3311) and forge build --offline exits 0. |
| <code>capability.16.12.1</code> | title | Drop the checkpoint vector and settlement read permissions from the root Foundry profile |
| <code>capability.16.12.1</code> | reqs | 16066.1 |
| <code>capability.16.6.1</code> | title | Retire the beta deployment script for the retired root contracts |
| <code>capability.16.6.1</code> | reqs | 16031.1 |
| <code>resolution.17.975</code> | title | decision.1604 binds: The beta deployment script retires whole |
| <code>resolution.17.975</code> | applies_to | decision.1604<br>task.16.6.1 |
| <code>resolution.17.975</code> | source | decision.1604 in fragments/J58/decision.kvx; tasks from affects |
| <code>resolution.17.976</code> | title | decision.1605 binds: The root Foundry profile stays for the kept contracts |
| <code>resolution.17.976</code> | applies_to | decision.1605<br>task.16.12.1 |
| <code>resolution.17.976</code> | source | decision.1605 in fragments/J58/decision.kvx; tasks from affects |

## makefile-graph

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.10.1</code> | req | 16010.1 |
| <code>acceptance.16.10.1</code> | scenario | Given the member rules at Makefile:910-978, when make -n test-exchange-native runs, then it exits 0 and prints the run lines of all seven member test binaries |
| <code>acceptance.16.10.1</code> | test | tools/ci/tests/test_makefile_exchange_native.py::ExchangeNativeAggregateTest.test_aggregate_runs_every_member (planned) |
| <code>acceptance.16.10.1</code> | qualification | UNRUN |
| <code>req.16010</code> | ac_1 | make -n test-exchange-native exits 0 and its recipe runs the seven member binaries defined at Makefile:910-978. |
| <code>acceptance.16.11.1</code> | req | 16011.1 |
| <code>acceptance.16.11.1</code> | scenario | Given the retired kernel library targets at Makefile:381-900 and :2189, when make -n runs each retired target name, then make exits 2 and reports no rule for each |
| <code>acceptance.16.11.1</code> | test | tools/ci/tests/test_makefile_wave1.py::MakefileWaveOneTest.test_retired_targets_have_no_rule (planned) |
| <code>acceptance.16.11.1</code> | qualification | UNRUN |
| <code>req.16011</code> | ac_1 | make -n for each of test-replay-golden, test-replay-golden-local, test-oracle-adapter, test-oracle-root, test-web-adapter, test-web-intake, test-web-root, test-web-program-path and test-idempotency exits 2 with No rule to make target (rules at HEAD: Makefile:381, :385, :817, :855, :877, :888, :900, :3547, :2189). |
| <code>acceptance.16.11.2</code> | req | 16011.2 |
| <code>acceptance.16.11.2</code> | scenario | Given the aggregates at Makefile:292, :3276 and :1713 and the kept oracle rules at Makefile:832-865, when make -n runs them with test-journal, then it exits 0 and no printed recipe names a retired kernel library object |
| <code>acceptance.16.11.2</code> | test | tools/ci/tests/test_makefile_wave1.py::MakefileWaveOneTest.test_aggregates_dry_run_clean (planned) |
| <code>acceptance.16.11.2</code> | qualification | UNRUN |
| <code>req.16011</code> | ac_2 | make -n test core-test-all test-wave-8 test-journal test-oracle-intake test-oracle-bounds test-oracle-failclosed exits 0 and the printed recipes name none of lx_oracle_adapter, lx_web_adapter, lx_web_root, lx_oracle_root, lxp_idempotency, replay-golden or lxp_checkpoint_cert (aggregates at Makefile:292, :3276, :1713). |
| <code>acceptance.16.12.1</code> | req | 16012.1 |
| <code>acceptance.16.12.1</code> | scenario | Given the DA, replica, batch seal, gateway receive and guarantor kernel test rules at Makefile:1008-1180 and :1365-1373, when make -n runs each retired target, then make exits 2 for each |
| <code>acceptance.16.12.1</code> | test | tools/ci/tests/test_makefile_wave2.py::MakefileWaveTwoTest.test_retired_targets_have_no_rule (planned) |
| <code>acceptance.16.12.1</code> | qualification | UNRUN |
| <code>req.16012</code> | ac_1 | make -n for each of test-da, test-da-possession, test-da-verified, test-da-retrieval, test-da-challenge, test-da-unavailable, test-replica, test-replica-divergence, test-batch-seal, test-batch-distribute, test-gateway-receive, test-guarantor, test-guarantor-cert, test-guarantor-bond and test-guarantor-disagreement exits 2 (rules at HEAD: Makefile:1137, :1145, :1153, :1164, :1172, :1180, :1045, :1063, :1013, :1021, :1372, :1096, :1104, :1112, :1129). |
| <code>acceptance.16.12.2</code> | req | 16012.2 |
| <code>acceptance.16.12.2</code> | scenario | Given the aggregates at Makefile:292, :1034 and :1255 and the module registry rule at Makefile:4180, when make -n test core-test-all test-wave-10 test-wave-12 layerx-module-registry runs, then it exits 0 and no recipe names a retired object or node.c |
| <code>acceptance.16.12.2</code> | test | tools/ci/tests/test_makefile_wave2.py::MakefileWaveTwoTest.test_aggregates_dry_run_clean (planned) |
| <code>acceptance.16.12.2</code> | qualification | UNRUN |
| <code>req.16012</code> | ac_2 | make -n test core-test-all test-wave-10 test-wave-12 layerx-module-registry exits 0 and the printed recipes name none of lxp_da_, lxp_replica, lxp_batch_seal, test_batch_publish, test_gateway_receive, lxp_guarantor or cmd/layerx-module-registry/node.c (Makefile:292, :1034, :1255, :4180). |
| <code>acceptance.16.13.1</code> | req | 16013.1 |
| <code>acceptance.16.13.1</code> | scenario | Given the agent LNI and wire rules at Makefile:2696-2746 and :3056-3064, when make -n runs each retired target, then make exits 2 for each and the wire harness variable is gone |
| <code>acceptance.16.13.1</code> | test | tools/ci/tests/test_makefile_wave3.py::MakefileWaveThreeTest.test_retired_targets_have_no_rule (planned) |
| <code>acceptance.16.13.1</code> | qualification | UNRUN |
| <code>req.16013</code> | ac_1 | make -n for each of agent-test-lni-schema, agent-test-lni-transport, agent-test-lni-handshake, agent-test-lni-abi, agent-test-wire-parity and agent-qualify-wire exits 2 (rules at HEAD: Makefile:2696, :2699, :2703, :2706, :3060, :3064), and AGENT_WIRE_HARNESS at Makefile:17 is undefined in make -p output. |
| <code>acceptance.16.13.2</code> | req | 16013.2 |
| <code>acceptance.16.13.2</code> | scenario | Given the agent and programs aggregates at Makefile:2558, :3676 and :3697 and the genesis module table rule at Makefile:1559, when make -n runs them with the boundary gate, then it exits 0, the module table links the shared harness and no recipe names the wire codec, the LNI preparation state or the fee-governance program |
| <code>acceptance.16.13.2</code> | test | tools/ci/tests/test_makefile_wave3.py::MakefileWaveThreeTest.test_aggregates_dry_run_clean (planned) |
| <code>acceptance.16.13.2</code> | qualification | UNRUN |
| <code>req.16013</code> | ac_2 | make -n agent-test agent-qualify-boundary programs-core-test programs-protocol-regression test-genesis-module-table exits 0, the boundary gate recipe (Makefile:2735) names a node JSON-RPC URL variable, the test_genesis_module_table recipe (Makefile:1559) compiles tests/lxp_test_harness.c, and no recipe names layerx-wire, layerxd-lni or programs_fee_governance (Makefile:2558, :3676, :3697). |
| <code>acceptance.16.14.1</code> | req | 16014.1 |
| <code>acceptance.16.14.1</code> | scenario | Given the daemon LNI test rules at Makefile:1476-1488 and :3959-3995 and the contracts stamp at Makefile:1266-1270, when make -n runs each retired target, then make exits 2 for each |
| <code>acceptance.16.14.1</code> | test | tools/ci/tests/test_makefile_wave4.py::MakefileWaveFourTest.test_retired_targets_have_no_rule (planned) |
| <code>acceptance.16.14.1</code> | qualification | UNRUN |
| <code>req.16014</code> | ac_1 | make -n for each of test-daemon-lni-admission, test-daemon-lni-account, test-daemon-lni-module and test-daemon-lni-head-attestation, test-paxeer, test-emergency-exit and test-receipt-offline exits 2 (rules at HEAD: Makefile:1487, :3969, :3983, :3995, :1272, :1322, :1392), and no rule builds build/contracts/.paxeer-built (Makefile:1266). |
| <code>acceptance.16.14.2</code> | req | 16014.2 |
| <code>acceptance.16.14.2</code> | scenario | Given test-paxeer-bond at Makefile:1281 and the genesis builder rule at Makefile:1571, when make -n runs them with test and core-test-all, then it exits 0, the builder links the shared harness and no recipe compiles a retired root contract or a layerxd LNI source |
| <code>acceptance.16.14.2</code> | test | tools/ci/tests/test_makefile_wave4.py::MakefileWaveFourTest.test_aggregates_dry_run_clean (planned) |
| <code>acceptance.16.14.2</code> | qualification | UNRUN |
| <code>req.16014</code> | ac_2 | make -n test core-test-all test-paxeer-bond test-genesis-builder-cli exits 0, the test_genesis_builder_cli recipe (Makefile:1571) compiles tests/lxp_test_harness.c, and no printed recipe names contracts/LayerXCustody.sol, contracts/CheckpointRegistry.sol, contracts/GuarantorBond.sol, lxp_daemon_lni or lxp_daemon_modules.h (Makefile:319-324, :1272, :1281). |
| <code>acceptance.16.15.1</code> | req | 16015.1 |
| <code>acceptance.16.15.1</code> | scenario | Given the batch WAL, finality, custody-proof, credit and Paxeer membership rules at Makefile:1964-2106, :3915-3956 and :4191-4221, when make -n runs each retired target, then make exits 2 for each |
| <code>acceptance.16.15.1</code> | test | tools/ci/tests/test_makefile_wave5.py::MakefileWaveFiveTest.test_retired_targets_have_no_rule (planned) |
| <code>acceptance.16.15.1</code> | qualification | UNRUN |
| <code>req.16015</code> | ac_1 | make -n for each of test-batch-wal-recovery, test-maintenance-publication, test-terminal-rejection, test-batch-identity, test-finality-evidence, test-daemon-finality-authority, test-light-credit, test-bridge-credit, custody-proof-build, test-paxeer-membership-sync, test-paxeer-anchor-submit, test-history and test-equivocation exits 2 (rules at HEAD: Makefile:2020, :1975, :1990, :2017, :2036, :2103, :3925, :3939, :3915, :4196, :4207, :1081, :1120). |
| <code>acceptance.16.15.2</code> | req | 16015.2 |
| <code>acceptance.16.15.2</code> | scenario | Given the human, withdrawal-proof and paxd-domain rules at Makefile:2294-2398, :4172 and :3154, when make -n runs them with test and core-test-all, then it exits 0 and no recipe names a retired custody, credit, batch WAL, finality or anchor artifact |
| <code>acceptance.16.15.2</code> | test | tools/ci/tests/test_makefile_wave5.py::MakefileWaveFiveTest.test_aggregates_dry_run_clean (planned) |
| <code>acceptance.16.15.2</code> | qualification | UNRUN |
| <code>req.16015</code> | ac_2 | make -n test core-test-all human-test human-test-integration test-native-withdrawal-proof test-paxd-domain exits 0 and no printed recipe names layerx-custody-proof, sign-credit, lxp_daemon_batch_wal, lxp_daemon_finality_authority, native-withdrawal-proof.json, anchor_vectors.json, native_handover_reads or layerxcustody (Makefile:2294, :2352, :4172, :3154). |
| <code>acceptance.16.16.1</code> | req | 16016.1 |
| <code>acceptance.16.16.1</code> | scenario | Given the layerxd, guarantor, handover, verify, daemon and retired kernel test rules at Makefile:992-1289, :1400-1590 and :3812-4299, when make -n runs each retired target, then make exits 2 for each and the LNI parity script is gone |
| <code>acceptance.16.16.1</code> | test | tools/ci/tests/test_makefile_wave6.py::MakefileWaveSixTest.test_retired_targets_have_no_rule (planned) |
| <code>acceptance.16.16.1</code> | qualification | UNRUN |
| <code>req.16016</code> | ac_1 | make -n for each of layerxd, layerx-handover, layerx-guarantor, layerx-verify, layerxctl, test-layerxd, test-daemon-guarantor, test-daemon-handover, test-lni-version-parity, relay-archive-build, paxeer-x-finality-recordings-build, test-sequencer, test-sequencer-recovery, test-batch-time and test-paxeer-bond exits 2 (rules at HEAD: Makefile:1430, :1461, :4065, :1525, :1497, :1473, :4101, :3845, :3813, :4272, :4299, :997, :1031, :1004, :1281), and scripts/ci/lni-version-parity.sh is absent. |
| <code>acceptance.16.16.2</code> | req | 16016.2 |
| <code>acceptance.16.16.2</code> | scenario | Given the aggregates at Makefile:211, :292, :3276, :3294 and :4014, when make -n runs them with human-test and the new test-tests-python target, then it exits 0 and no recipe names a retired daemon tree or the movement provider |
| <code>acceptance.16.16.2</code> | test | tools/ci/tests/test_makefile_wave6.py::MakefileWaveSixTest.test_aggregates_dry_run_clean (planned) |
| <code>acceptance.16.16.2</code> | qualification | UNRUN |
| <code>req.16016</code> | ac_2 | make -n build test core-test-all core-qualification-environment beta-qualify-focused human-test test-tests-python exits 0, test-tests-python sits next to test-harness (Makefile:286) and no printed recipe names cmd/layerxd, cmd/layerx-guarantor, cmd/layerx-handover, cmd/layerx-verify, tests/daemon, layerx-archive-codec or layerx-human-movement-provider (Makefile:211, :292, :3276, :3294, :4014). |
| <code>acceptance.16.999.1</code> | req | 16999.1 |
| <code>acceptance.16.999.1</code> | scenario | Given the candidate after every domain-16 wave-6 task, when test_tools_ops_completion_retired_surfaces_absent runs, then no retired surface remains and every planned path exists |
| <code>acceptance.16.999.1</code> | test | tools/ci/tests/test_completion_gate.py::CompletionGateTest.test_tools_ops_completion_retired_surfaces_absent (planned) |
| <code>acceptance.16.999.1</code> | qualification | UNRUN |
| <code>req.16999</code> | ac_1 | test_tools_ops_completion_retired_surfaces_absent passes on the merged candidate: no retired domain-16 surface remains and every planned domain-16 wave-6 path exists. |
| <code>capability.16.0.2</code> | title | Retire the DA, replica, batch seal, gateway receive and guarantor kernel test rules |
| <code>capability.16.0.2</code> | reqs | 16012.1<br>16012.2 |
| <code>capability.16.0.3</code> | title | Retire the daemon LNI test rules and the retired root contract build stamp |
| <code>capability.16.0.3</code> | reqs | 16014.1<br>16014.2 |
| <code>capability.16.0.4</code> | title | Retire the batch WAL, finality, custody-proof, credit and anchor submit rules |
| <code>capability.16.0.4</code> | reqs | 16015.1<br>16015.2 |
| <code>capability.16.0.9</code> | title | Domain 16 completion gate |
| <code>capability.16.0.9</code> | reqs | 16999.1 |
| <code>capability.16.2.1</code> | title | Add the test-exchange-native aggregate over the perps and spot kernel tests |
| <code>capability.16.2.1</code> | reqs | 16010.1 |
| <code>capability.16.2.2</code> | title | Retire the replay-golden, oracle and web root, idempotency and checkpoint header rules |
| <code>capability.16.2.2</code> | reqs | 16011.1<br>16011.2 |
| <code>capability.16.2.3</code> | title | Retire the agent LNI and wire codec rules and the fee-governance program rule |
| <code>capability.16.2.3</code> | reqs | 16013.1<br>16013.2 |
| <code>capability.16.2.4</code> | title | Retire the layerxd, guarantor, handover, verify and daemon test rules |
| <code>capability.16.2.4</code> | reqs | 16016.1<br>16016.2 |
| <code>resolution.17.977</code> | title | decision.1606 binds: Makefile retirements run dependents first |
| <code>resolution.17.977</code> | applies_to | decision.1606<br>task.16.0.2<br>task.16.0.3<br>task.16.0.4<br>task.16.2.2<br>task.16.2.3<br>task.16.2.4 |
| <code>resolution.17.977</code> | source | decision.1606 in fragments/J58/decision.kvx; tasks from affects |
| <code>resolution.17.982</code> | title | decision.1611 binds: The replay golden release gate retires with its target |
| <code>resolution.17.982</code> | applies_to | decision.1611<br>task.16.2.2<br>task.16.5.6 |
| <code>resolution.17.982</code> | source | decision.1611 in fragments/J58/decision.kvx; tasks from affects |
| <code>resolution.17.991</code> | title | decision.1790 binds: Waves 0-6 are balanced by moving free tasks |
| <code>resolution.17.991</code> | applies_to | decision.1790<br>task.1.0.4<br>task.16.0.4 |
| <code>resolution.17.991</code> | source | decision.1790 in fragments/J79/decision.kvx; tasks from affects |

## paxeer-x-gates

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.cfg-pxx-candidate-service-catalogue-j58</code> | <code>config_key</code> | <code>new</code> | <code>tools/paxeer-x/candidate.py</code> (planned) | <code>SERVICE_CATALOGUE = 'spec/layerx-merge/spec.kvx'</code> |
| <code>interface.16.fmt-pxx-gates-registry-j58</code> | <code>file_format</code> | <code>new</code> | <code>tools/paxeer-x/gates/registry.kvx</code> (planned) | <code>[task.&lt;gate-id&gt;] gate = "tools/paxeer-x/gates/&lt;gate-id&gt;.sh"</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.16.cfg-pxx-candidate-service-catalogue-j58</code> | notes | Single service-catalogue path read by candidate.catalogue and every release and qualification script. |
| <code>interface.16.fmt-pxx-gates-registry-j58</code> | notes | Gate registration index read by tools/paxeer-x/verify-task.sh in place of the deleted spec/paxeer-x/spec.kvx. |

Contract context <code>contract_context.16.paxeer-x-gates</code>:

| Field | Frozen rule |
|---|---|
| interface | tools/paxeer-x/verify-task.sh &lt;task-id&gt; with registrations in tools/paxeer-x/gates/registry.kvx (planned) |
| storage | tools/paxeer-x/gates/registry.kvx sections [task.&lt;id&gt;]; the service catalogue in spec/layerx-merge/spec.kvx. |
| errors | verify-task.sh exit codes 2 to 12 as documented at tools/paxeer-x/verify-task.sh:10-12. |
| events | Gates print PAXEER_X_GATE tests=&lt;n&gt; skipped=&lt;m&gt;. |
| gas | none |
| activation | Registry from wave 0. |
| invariants | Every executable gate has exactly one registry section. |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.36.1</code> | req | 16036.1 |
| <code>acceptance.16.36.1</code> | scenario | Given gate 2.2 deleted and gate 24.14 without the layerxd pathspec and runtime-fixture steps, when the scanner runs on gate 24.14, then it exits 0 with no finding, bash -n accepts gate 24.14 and gate 2.2 is absent |
| <code>acceptance.16.36.1</code> | test | tools/paxeer-x/tests/test_gates_guarantor_retired.py::GatesGuarantorRetiredTest.test_gates_name_no_retired_daemon (planned) |
| <code>acceptance.16.36.1</code> | qualification | UNRUN |
| <code>req.16036</code> | ac_1 | tools/paxeer-x/gates/2.2.sh (HEAD: :18-36) is absent, and the scanner finds no guarantor-runtime or finality-authority caller in tools/paxeer-x/gates/24.14.sh (HEAD: :15-30), which still parses with bash -n. |
| <code>acceptance.16.37.1</code> | req | 16037.1 |
| <code>acceptance.16.37.1</code> | scenario | Given the three gates and the build fragment without layerxd and replica steps, when the scanner runs on them, then it exits 0 with no finding |
| <code>acceptance.16.37.1</code> | test | tools/paxeer-x/tests/test_gates_layerxd_retired.py::GatesLayerxdRetiredTest.test_gates_name_no_layerxd_or_replica (planned) |
| <code>acceptance.16.37.1</code> | qualification | UNRUN |
| <code>req.16037</code> | ac_1 | The scanner finds no replica, guarantor-runtime or finality-authority caller in tools/paxeer-x/gates/104.30.5.sh (HEAD: :82), 104.32.6.sh (:70), 111.6.11.sh (:8) or tools/paxeer-x/build/104.35.35.mk (:2). |
| <code>acceptance.16.38.1</code> | req | 16038.1 |
| <code>acceptance.16.38.1</code> | scenario | Given gates 24.16 and 5.1 without the root contracts and credit drivers, when the scanner runs on them, then it exits 0 with no finding |
| <code>acceptance.16.38.1</code> | test | tools/paxeer-x/tests/test_gates_custody_retired.py::GatesCustodyRetiredTest.test_gates_name_no_root_contract_or_credit (planned) |
| <code>acceptance.16.38.1</code> | qualification | UNRUN |
| <code>req.16038</code> | ac_1 | The scanner finds no anchor, custody-claims or custody-credit caller in tools/paxeer-x/gates/24.16.sh (HEAD: :8-18) or tools/paxeer-x/gates/5.1.sh (HEAD: :57). |
| <code>acceptance.16.39.1</code> | req | 16039.1 |
| <code>acceptance.16.39.1</code> | scenario | Given gate 103.6.11 without the layerx-wire entries and the fee-governance native test and producer without the relay case, when the test runs, then the scanner exits 0 on gate 103.6.11 and neither file names LX_PROGRAMS_FEE_GOVERNANCE or native_signed_governance_producer_consumer |
| <code>acceptance.16.39.1</code> | test | tools/paxeer-x/tests/test_gates_wire_fee_retired.py::GatesWireFeeRetiredTest.test_gates_name_no_wire_codec_or_fee_relay (planned) |
| <code>acceptance.16.39.1</code> | qualification | UNRUN |
| <code>req.16039</code> | ac_1 | The scanner finds no wire-codec caller in tools/paxeer-x/gates/103.6.11.sh (HEAD: :291, :474), and neither tests/programs/test_fee_governance.c (HEAD: :400, :534) nor tools/qualification/paxeer-x/program_fee_governance.py (HEAD: :22) names LX_PROGRAMS_FEE_GOVERNANCE or native_signed_governance_producer_consumer. |
| <code>acceptance.16.40.1</code> | req | 16040.1 |
| <code>acceptance.16.40.1</code> | scenario | Given the governance templates without the exchange, custody and anchor proposals, when tools/paxeer-x/gov/test.sh runs, then it exits 0 and no rendered proposal names a retired message |
| <code>acceptance.16.40.1</code> | test | tools/paxeer-x/tests/test_gov_templates.py::GovTemplatesTest.test_gov_suite_passes_without_retired_templates (planned) |
| <code>acceptance.16.40.1</code> | qualification | UNRUN |
| <code>req.16040</code> | ac_1 | tools/paxeer-x/gov/test.sh (HEAD: :1) exits 0 with templates 04, 05 and 06 removed and render.sh (HEAD: :183-184) no longer submitting them. |
| <code>acceptance.16.41.1</code> | req | 16041.1 |
| <code>acceptance.16.41.1</code> | scenario | Given the registry listing every gate, when verify-task.sh runs with an unknown selector, then it exits 2 as a usage refusal and every gate file has a registry section |
| <code>acceptance.16.41.1</code> | test | tools/paxeer-x/tests/test_gate_registry.py::GateRegistryTest.test_every_gate_is_registered_and_dispatches (planned) |
| <code>acceptance.16.41.1</code> | qualification | UNRUN |
| <code>req.16041</code> | ac_1 | Every executable gate has a [task.&lt;id&gt;] section in tools/paxeer-x/gates/registry.kvx (planned), and tools/paxeer-x/verify-task.sh, whose registration source at tools/paxeer-x/verify-task.sh:283 names the deleted spec, refuses an unknown selector with exit 2 (usage, tools/paxeer-x/verify-task.sh:77). |
| <code>acceptance.16.42.1</code> | req | 16042.1 |
| <code>acceptance.16.42.1</code> | scenario | Given spec/layerx-merge/spec.kvx with 36 service blocks, when candidate.catalogue(ROOT / SERVICE_CATALOGUE) runs, then it returns 36 services each with purpose, route and dependencies |
| <code>acceptance.16.42.1</code> | test | tools/paxeer-x/tests/test_service_catalogue.py::ServiceCatalogueTest.test_catalogue_reads_layerx_merge_services (planned) |
| <code>acceptance.16.42.1</code> | qualification | UNRUN |
| <code>req.16042</code> | ac_1 | candidate.catalogue (tools/paxeer-x/candidate.py:116) reads SERVICE_CATALOGUE = spec/layerx-merge/spec.kvx and returns its 36 [service.*] blocks with purpose, route and dependencies. |
| <code>acceptance.16.43.1</code> | req | 16043.1 |
| <code>acceptance.16.43.1</code> | scenario | Given the route catalogue without lx_ records, when it is loaded, then no rpc_records method starts with lx_ and the schema is unchanged |
| <code>acceptance.16.43.1</code> | test | tools/paxeer-x/tests/test_route_catalogue_lx.py::RouteCatalogueLxTest.test_catalogue_lists_no_lx_method (planned) |
| <code>acceptance.16.43.1</code> | qualification | UNRUN |
| <code>req.16043</code> | ac_1 | tools/paxeer-x/route-catalogue.json lists no lx_ method after the records starting at tools/paxeer-x/route-catalogue.json:4219 are deleted, and keeps schema paxeer-x.routes.v1. |
| <code>acceptance.16.44.1</code> | req | 16044.1 |
| <code>acceptance.16.44.1</code> | scenario | Given the catalogue listing the deleted daemon trees, when the scanner runs with --self-test and then with --deleted for those components, then both exit 0 |
| <code>acceptance.16.44.1</code> | test | tools/paxeer-x/tests/test_retired_catalogue_deleted.py::RetiredCatalogueDeletedTest.test_deleted_trees_absent_and_catalogue_valid (planned) |
| <code>acceptance.16.44.1</code> | qualification | UNRUN |
| <code>req.16044</code> | ac_1 | tools/paxeer-x/retired_callers.json validates under --self-test and every deleted tree it lists for guarantor-runtime (:73), lni (:258), replica (:134) and intent-ingester (:411) is absent under --deleted. |
| <code>acceptance.16.70.1</code> | req | 16070.1 |
| <code>acceptance.16.70.1</code> | scenario | Given the gates without the versioned boundary gate and the hosted fixture checks, when verify-task.sh runs selector 0.5 and bash -n runs on gate 104.17.4, then verify-task.sh exits 2 as a usage refusal and bash -n exits 0 |
| <code>acceptance.16.70.1</code> | test | tools/paxeer-x/tests/test_gates_deleted_sources.py::GatesDeletedSourcesTest.test_gates_name_no_deleted_source (planned) |
| <code>acceptance.16.70.1</code> | qualification | UNRUN |
| <code>req.16070</code> | ac_1 | tools/paxeer-x/verify-task.sh 0.5 exits 2 (usage, unknown task selector at tools/paxeer-x/verify-task.sh:77) once gate 0.5 (tools/paxeer-x/gates/0.5.sh:5) and versioned_boundaries.py are deleted, and tools/paxeer-x/gates/104.17.4.sh passes bash -n without the hosted fixture reads at tools/paxeer-x/gates/104.17.4.sh:26-28 and tools/paxeer-x/gates/104.17.4.sh:54-62. |
| <code>capability.16.8.1</code> | title | Remove the guarantor and finality steps from the paxeer-x gates 2.2 and 24.14 |
| <code>capability.16.8.1</code> | reqs | 16036.1 |
| <code>capability.16.8.2</code> | title | Remove the layerxd and replica steps from the paxeer-x gates 104.30.5, 104.32.6 and 111.6.11 |
| <code>capability.16.8.2</code> | reqs | 16037.1 |
| <code>capability.16.8.3</code> | title | Remove the checkpoint, custody and bridge credit steps from the paxeer-x gates 24.16 and 5.1 |
| <code>capability.16.8.3</code> | reqs | 16038.1 |
| <code>capability.16.8.4</code> | title | Remove the wire codec steps from paxeer-x gate 103.6.11 and the fee-governance relay case from the fee-governance producer |
| <code>capability.16.8.4</code> | reqs | 16039.1 |
| <code>capability.16.8.5</code> | title | Remove the exchange, custody and anchor governance templates |
| <code>capability.16.8.5</code> | reqs | 16040.1 |
| <code>capability.16.8.6</code> | title | Retire the versioned boundary gate and the hosted fixture checks of gate 104.17.4 |
| <code>capability.16.8.6</code> | reqs | 16070.1 |
| <code>capability.16.9.1</code> | title | Register paxeer-x gates in a committed registry instead of the deleted spec |
| <code>capability.16.9.1</code> | reqs | 16041.1 |
| <code>capability.16.9.2</code> | title | Read the paxeer-x service catalogue from the layerx-merge spec |
| <code>capability.16.9.2</code> | reqs | 16042.1 |
| <code>capability.16.9.3</code> | title | Delete the lx_ route entries from the paxeer-x route catalogue |
| <code>capability.16.9.3</code> | reqs | 16043.1 |
| <code>capability.16.9.4</code> | title | Record the deleted daemon trees in the retired-callers catalogue |
| <code>capability.16.9.4</code> | reqs | 16044.1 |
| <code>resolution.17.972</code> | title | decision.1601 binds: Paxeer-x gate registrations move to a committed registry |
| <code>resolution.17.972</code> | applies_to | decision.1601<br>task.16.9.1 |
| <code>resolution.17.972</code> | source | decision.1601 in fragments/J58/decision.kvx; tasks from affects |

## qualification-harness

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.30.1</code> | req | 16030.1 |
| <code>acceptance.16.30.1</code> | scenario | Given the producer without its LNI step, when the scanner runs with component lni and the producer runs with --help, then both exit 0 |
| <code>acceptance.16.30.1</code> | test | tools/ci/tests/test_program_http_producer.py::ProgramHttpProducerTest.test_producer_uses_no_lni (planned) |
| <code>acceptance.16.30.1</code> | qualification | UNRUN |
| <code>req.16030</code> | ac_1 | scripts/qualification/paxeer-x/program-http-producer.py has no LNI step (HEAD: :342) and its --help exits 0. |
| <code>acceptance.16.45.1</code> | req | 16045.1 |
| <code>acceptance.16.45.1</code> | scenario | Given the producers repointed to SERVICE_CATALOGUE, when each is loaded and run with --help, then each builds an existing catalogue path and exits 0 |
| <code>acceptance.16.45.1</code> | test | tools/qualification/test_service_catalogue_1.py::ServiceCatalogueReaders1Test.test_producers_load_existing_catalogue (planned) |
| <code>acceptance.16.45.1</code> | qualification | UNRUN |
| <code>req.16045</code> | ac_1 | The producers that read the deleted spec at tools/qualification/paxeer_x_release.py:63, tools/qualification/paxeer_x_release.py:80, tools/qualification/paxeer-x/human_native_custody.py:96, tools/qualification/paxeer-x/event-delivery-contract.py:100, tools/qualification/paxeer-x/human_attestor_intents.py:77, tools/qualification/paxeer-x/native_send_purpose.py:104 read spec/layerx-merge/spec.kvx through SERVICE_CATALOGUE and exit 0 under --help. |
| <code>acceptance.16.46.1</code> | req | 16046.1 |
| <code>acceptance.16.46.1</code> | scenario | Given the producers repointed to SERVICE_CATALOGUE, when each is loaded and run with --help, then each builds an existing catalogue path and exits 0 |
| <code>acceptance.16.46.1</code> | test | tools/qualification/test_service_catalogue_2.py::ServiceCatalogueReaders2Test.test_producers_load_existing_catalogue (planned) |
| <code>acceptance.16.46.1</code> | qualification | UNRUN |
| <code>req.16046</code> | ac_1 | The producers that read the deleted spec at tools/qualification/paxeer-x/human_deposit_initial_response.py:250, tools/qualification/paxeer-x/gas-station-contract.py:1430, tools/qualification/paxeer-x/human_program_approvals.py:95, tools/qualification/paxeer-x/interop-archive-contract.py:64, tools/qualification/paxeer-x/interop-archive-contract.py:976, tools/qualification/paxeer-x/interop-archive-contract.py:1482, tools/qualification/paxeer-x/interop-archive-contract.py:1860 read spec/layerx-merge/spec.kvx through SERVICE_CATALOGUE and exit 0 under --help. |
| <code>acceptance.16.47.1</code> | req | 16047.1 |
| <code>acceptance.16.47.1</code> | scenario | Given the producers repointed to SERVICE_CATALOGUE, when each is loaded and run with --help, then each builds an existing catalogue path and exits 0 |
| <code>acceptance.16.47.1</code> | test | tools/qualification/test_service_catalogue_3.py::ServiceCatalogueReaders3Test.test_producers_load_existing_catalogue (planned) |
| <code>acceptance.16.47.1</code> | qualification | UNRUN |
| <code>req.16047</code> | ac_1 | The producers that read the deleted spec at tools/qualification/paxeer-x/human-canonical-projections.py:183, tools/qualification/paxeer-x/registry-router-bootstrap.py:413, tools/qualification/paxeer-x/human-native-send-producer.py:89, tools/qualification/paxeer-x/fixtures/xweb_attestation.py:25 read spec/layerx-merge/spec.kvx through SERVICE_CATALOGUE and exit 0 under --help. |
| <code>acceptance.16.48.1</code> | req | 16048.1 |
| <code>acceptance.16.48.1</code> | scenario | Given tools/qualification/paxeer-x/arbiter-admission.py, tools/qualification/paxeer-x/arbiter-native-authority.py, tools/qualification/paxeer-x/arbiter-prestate-transport.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.48.1</code> | test | tools/qualification/test_retired_callers_1.py::RetiredCallers1Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.48.1</code> | qualification | UNRUN |
| <code>req.16048</code> | ac_1 | The scanner finds no wire-codec, batch-seal, finality-authority, guarantor-runtime caller in tools/qualification/paxeer-x/arbiter-admission.py, tools/qualification/paxeer-x/arbiter-native-authority.py, tools/qualification/paxeer-x/arbiter-prestate-transport.py (HEAD sites tools/qualification/paxeer-x/arbiter-admission.py:21, tools/qualification/paxeer-x/arbiter-admission.py:65, tools/qualification/paxeer-x/arbiter-admission.py:70, tools/qualification/paxeer-x/arbiter-native-authority.py:29, tools/qualification/paxeer-x/arbiter-native-authority.py:32, tools/qualification/paxeer-x/arbiter-native-authority.py:36, tools/qualification/paxeer-x/arbiter-native-authority.py:37, tools/qualification/paxeer-x/arbiter-native-authority.py:38, tools/qualification/paxeer-x/arbiter-native-authority.py:39, tools/qualification/paxeer-x/arbiter-prestate-transport.py:21, tools/qualification/paxeer-x/arbiter-prestate-transport.py:64, tools/qualification/paxeer-x/arbiter-prestate-transport.py:69) and each Python producer exits 0 under --help. |
| <code>acceptance.16.49.1</code> | req | 16049.1 |
| <code>acceptance.16.49.1</code> | scenario | Given tools/qualification/paxeer-x/arbiter_prestate.py, tools/qualification/paxeer-x/asset-execution-prestate.py, tools/qualification/paxeer-x/asset-inventory.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.49.1</code> | test | tools/qualification/test_retired_callers_2.py::RetiredCallers2Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.49.1</code> | qualification | UNRUN |
| <code>req.16049</code> | ac_1 | The scanner finds no wire-codec, batch-seal, finality-authority, guarantor-runtime caller in tools/qualification/paxeer-x/arbiter_prestate.py, tools/qualification/paxeer-x/asset-execution-prestate.py, tools/qualification/paxeer-x/asset-inventory.py (HEAD sites tools/qualification/paxeer-x/arbiter_prestate.py:20, tools/qualification/paxeer-x/arbiter_prestate.py:93, tools/qualification/paxeer-x/asset-execution-prestate.py:21, tools/qualification/paxeer-x/asset-execution-prestate.py:64, tools/qualification/paxeer-x/asset-execution-prestate.py:77, tools/qualification/paxeer-x/asset-inventory.py:21, tools/qualification/paxeer-x/asset-inventory.py:65, tools/qualification/paxeer-x/asset-inventory.py:78), none of them names tests/storage/lxp_test_maintenance_publication.c and each Python producer exits 0 under --help. |
| <code>acceptance.16.50.1</code> | req | 16050.1 |
| <code>acceptance.16.50.1</code> | scenario | Given tools/qualification/paxeer-x/authenticated-replay-step.py, tools/qualification/paxeer-x/market-sandbox-capture.py, tools/qualification/paxeer-x/market-sandbox-profile.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.50.1</code> | test | tools/qualification/test_retired_callers_3.py::RetiredCallers3Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.50.1</code> | qualification | UNRUN |
| <code>req.16050</code> | ac_1 | The scanner finds no wire-codec, batch-seal caller in tools/qualification/paxeer-x/authenticated-replay-step.py, tools/qualification/paxeer-x/market-sandbox-capture.py, tools/qualification/paxeer-x/market-sandbox-profile.py (HEAD sites tools/qualification/paxeer-x/authenticated-replay-step.py:29, tools/qualification/paxeer-x/market-sandbox-capture.py:20, tools/qualification/paxeer-x/market-sandbox-capture.py:52, tools/qualification/paxeer-x/market-sandbox-capture.py:56, tools/qualification/paxeer-x/market-sandbox-capture.py:59, tools/qualification/paxeer-x/market-sandbox-profile.py:32) and each Python producer exits 0 under --help. |
| <code>acceptance.16.51.1</code> | req | 16051.1 |
| <code>acceptance.16.51.1</code> | scenario | Given tools/qualification/paxeer-x/program-replay-record.py, tools/qualification/paxeer-x/replay-authority.py, tools/qualification/paxeer-x/replay-catalogue.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.51.1</code> | test | tools/qualification/test_retired_callers_4.py::RetiredCallers4Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.51.1</code> | qualification | UNRUN |
| <code>req.16051</code> | ac_1 | The scanner finds no wire-codec, batch-seal caller in tools/qualification/paxeer-x/program-replay-record.py, tools/qualification/paxeer-x/replay-authority.py, tools/qualification/paxeer-x/replay-catalogue.py (HEAD sites tools/qualification/paxeer-x/program-replay-record.py:14, tools/qualification/paxeer-x/program-replay-record.py:18, tools/qualification/paxeer-x/replay-authority.py:19, tools/qualification/paxeer-x/replay-authority.py:26, tools/qualification/paxeer-x/replay-catalogue.py:21, tools/qualification/paxeer-x/replay-catalogue.py:75) and each Python producer exits 0 under --help. |
| <code>acceptance.16.52.1</code> | req | 16052.1 |
| <code>acceptance.16.52.1</code> | scenario | Given tools/qualification/paxeer-x/native-programs-link.py, tools/qualification/paxeer-x/build-caps-discovery.sh without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.52.1</code> | test | tools/qualification/test_retired_callers_5.py::RetiredCallers5Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.52.1</code> | qualification | UNRUN |
| <code>req.16052</code> | ac_1 | The scanner finds no finality-authority, custody-credit, replica, batch-seal caller in tools/qualification/paxeer-x/native-programs-link.py, tools/qualification/paxeer-x/build-caps-discovery.sh (HEAD sites tools/qualification/paxeer-x/native-programs-link.py:25, tools/qualification/paxeer-x/native-programs-link.py:170, tools/qualification/paxeer-x/build-caps-discovery.sh:34) and each Python producer exits 0 under --help. |
| <code>acceptance.16.53.1</code> | req | 16053.1 |
| <code>acceptance.16.53.1</code> | scenario | Given tools/qualification/paxeer-x/interop-archive-contract.py, tools/qualification/paxeer-x/fixtures/caps_fixture.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.53.1</code> | test | tools/qualification/test_retired_callers_6.py::RetiredCallers6Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.53.1</code> | qualification | UNRUN |
| <code>req.16053</code> | ac_1 | The scanner finds no finality-authority, custody-credit, replica, batch-seal caller in tools/qualification/paxeer-x/interop-archive-contract.py, tools/qualification/paxeer-x/fixtures/caps_fixture.py (HEAD sites tools/qualification/paxeer-x/interop-archive-contract.py:2288, tools/qualification/paxeer-x/interop-archive-contract.py:2307, tools/qualification/paxeer-x/fixtures/caps_fixture.py:136) and each Python producer exits 0 under --help. |
| <code>acceptance.16.54.1</code> | req | 16054.1 |
| <code>acceptance.16.54.1</code> | scenario | Given tools/qualification/paxeer-x/authority-evidence-read.py, tools/qualification/paxeer-x/router-complete-readiness.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.54.1</code> | test | tools/qualification/test_retired_callers_7.py::RetiredCallers7Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.54.1</code> | qualification | UNRUN |
| <code>req.16054</code> | ac_1 | The scanner finds no finality-authority, replica, handover-client caller in tools/qualification/paxeer-x/authority-evidence-read.py, tools/qualification/paxeer-x/router-complete-readiness.py (HEAD sites tools/qualification/paxeer-x/authority-evidence-read.py:13, tools/qualification/paxeer-x/authority-evidence-read.py:14, tools/qualification/paxeer-x/router-complete-readiness.py:27, tools/qualification/paxeer-x/router-complete-readiness.py:34, tools/qualification/paxeer-x/router-complete-readiness.py:208, tools/qualification/paxeer-x/router-complete-readiness.py:210) and each Python producer exits 0 under --help. |
| <code>acceptance.16.55.1</code> | req | 16055.1 |
| <code>acceptance.16.55.1</code> | scenario | Given tools/qualification/paxeer-x/storage-recovery-producer.py, tools/qualification/paxeer-x/explorer-receipt-reader.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.55.1</code> | test | tools/qualification/test_retired_callers_8.py::RetiredCallers8Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.55.1</code> | qualification | UNRUN |
| <code>req.16055</code> | ac_1 | The scanner finds no finality-authority, replica, handover-client caller in tools/qualification/paxeer-x/storage-recovery-producer.py, tools/qualification/paxeer-x/explorer-receipt-reader.py (HEAD sites tools/qualification/paxeer-x/storage-recovery-producer.py:18, tools/qualification/paxeer-x/storage-recovery-producer.py:20, tools/qualification/paxeer-x/storage-recovery-producer.py:21, tools/qualification/paxeer-x/explorer-receipt-reader.py:48), none of them names tests/storage/check_daemon_recovery_order.sh and each Python producer exits 0 under --help. |
| <code>acceptance.16.56.1</code> | req | 16056.1 |
| <code>acceptance.16.56.1</code> | scenario | Given tools/qualification/paxeer-x/agent_operation_envelope.py, tools/qualification/paxeer-x/event-delivery-contract.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.56.1</code> | test | tools/qualification/test_retired_callers_9.py::RetiredCallers9Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.56.1</code> | qualification | UNRUN |
| <code>req.16056</code> | ac_1 | The scanner finds no custody-credit, finality-authority, custody-deposit caller in tools/qualification/paxeer-x/agent_operation_envelope.py, tools/qualification/paxeer-x/event-delivery-contract.py (HEAD sites tools/qualification/paxeer-x/agent_operation_envelope.py:123, tools/qualification/paxeer-x/agent_operation_envelope.py:2435, tools/qualification/paxeer-x/event-delivery-contract.py:1772), none of them names tests/storage/lxp_test_maintenance_publication.c and each Python producer exits 0 under --help. |
| <code>acceptance.16.57.1</code> | req | 16057.1 |
| <code>acceptance.16.57.1</code> | scenario | Given tools/qualification/paxeer-x/fixtures/xweb_fixture.py, tools/qualification/paxeer-x/programs_all_form_validation.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.57.1</code> | test | tools/qualification/test_retired_callers_10.py::RetiredCallers10Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.57.1</code> | qualification | UNRUN |
| <code>req.16057</code> | ac_1 | The scanner finds no custody-credit, finality-authority, custody-deposit caller in tools/qualification/paxeer-x/fixtures/xweb_fixture.py, tools/qualification/paxeer-x/programs_all_form_validation.py (HEAD sites tools/qualification/paxeer-x/fixtures/xweb_fixture.py:35, tools/qualification/paxeer-x/programs_all_form_validation.py:153, tools/qualification/paxeer-x/programs_all_form_validation.py:400) and each Python producer exits 0 under --help. |
| <code>acceptance.16.58.1</code> | req | 16058.1 |
| <code>acceptance.16.58.1</code> | scenario | Given tools/qualification/paxeer-x/programs_accounts_winddown.py, tools/qualification/paxeer-x/movement-custody-profile.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.58.1</code> | test | tools/qualification/test_retired_callers_11.py::RetiredCallers11Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.58.1</code> | qualification | UNRUN |
| <code>req.16058</code> | ac_1 | The scanner finds no custody-credit, custody-deposit, anchor, custody-claims caller in tools/qualification/paxeer-x/programs_accounts_winddown.py, tools/qualification/paxeer-x/movement-custody-profile.py (HEAD sites tools/qualification/paxeer-x/programs_accounts_winddown.py:56, tools/qualification/paxeer-x/programs_accounts_winddown.py:258, tools/qualification/paxeer-x/programs_accounts_winddown.py:259, tools/qualification/paxeer-x/movement-custody-profile.py:101, tools/qualification/paxeer-x/movement-custody-profile.py:102) and each Python producer exits 0 under --help. |
| <code>acceptance.16.59.1</code> | req | 16059.1 |
| <code>acceptance.16.59.1</code> | scenario | Given tools/qualification/paxeer-x/custody_governance.py, tools/qualification/paxeer-x/caps-codecs.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.59.1</code> | test | tools/qualification/test_retired_callers_12.py::RetiredCallers12Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.59.1</code> | qualification | UNRUN |
| <code>req.16059</code> | ac_1 | The scanner finds no custody-credit, custody-deposit, anchor, custody-claims caller in tools/qualification/paxeer-x/custody_governance.py, tools/qualification/paxeer-x/caps-codecs.py (HEAD sites tools/qualification/paxeer-x/custody_governance.py:16, tools/qualification/paxeer-x/caps-codecs.py:53, tools/qualification/paxeer-x/caps-codecs.py:54) and each Python producer exits 0 under --help. |
| <code>acceptance.16.60.1</code> | req | 16060.1 |
| <code>acceptance.16.60.1</code> | scenario | Given tools/qualification/paxeer-x/budget-proof-intake.py, tools/qualification/paxeer-x/human-approval-admission.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.60.1</code> | test | tools/qualification/test_retired_callers_13.py::RetiredCallers13Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.60.1</code> | qualification | UNRUN |
| <code>req.16060</code> | ac_1 | The scanner finds no lni caller in tools/qualification/paxeer-x/budget-proof-intake.py, tools/qualification/paxeer-x/human-approval-admission.py (HEAD sites tools/qualification/paxeer-x/authority-lni-readiness.py:243, tools/qualification/paxeer-x/budget-proof-intake.py:28, tools/qualification/paxeer-x/human-approval-admission.py:38) and each Python producer exits 0 under --help. |
| <code>acceptance.16.61.1</code> | req | 16061.1 |
| <code>acceptance.16.61.1</code> | scenario | Given tools/qualification/paxeer-x/native_send_profile.py, tools/qualification/paxeer-x/programs_source_abi.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.61.1</code> | test | tools/qualification/test_retired_callers_14.py::RetiredCallers14Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.61.1</code> | qualification | UNRUN |
| <code>req.16061</code> | ac_1 | The scanner finds no lni caller in tools/qualification/paxeer-x/native_send_profile.py, tools/qualification/paxeer-x/programs_source_abi.py (HEAD sites tools/qualification/paxeer-x/native_send_profile.py:123, tools/qualification/paxeer-x/programs_source_abi.py:299) and each Python producer exits 0 under --help. |
| <code>acceptance.16.62.1</code> | req | 16062.1 |
| <code>acceptance.16.62.1</code> | scenario | Given tools/qualification/paxeer-x/registry-contract.py, tools/qualification/beta_driver.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.62.1</code> | test | tools/qualification/test_retired_callers_15.py::RetiredCallers15Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.62.1</code> | qualification | UNRUN |
| <code>req.16062</code> | ac_1 | The scanner finds no lni caller in tools/qualification/paxeer-x/registry-contract.py, tools/qualification/beta_driver.py (HEAD sites tools/qualification/paxeer-x/registry-contract.py:1160, tools/qualification/beta_driver.py:1108, tools/qualification/beta_driver.py:1114, tools/qualification/beta_driver.py:1126) and each Python producer exits 0 under --help. |
| <code>acceptance.16.63.1</code> | req | 16063.1 |
| <code>acceptance.16.63.1</code> | scenario | Given tools/qualification/paxeer-x/agent_http_bounds.py, tools/qualification/paxeer-x/agent_tenant_readiness.py, tools/qualification/paxeer-x/mcp_web_payer.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.63.1</code> | test | tools/qualification/test_retired_callers_16.py::RetiredCallers16Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.63.1</code> | qualification | UNRUN |
| <code>req.16063</code> | ac_1 | The scanner finds no anchor, checkpoint-evidence caller in tools/qualification/paxeer-x/agent_http_bounds.py, tools/qualification/paxeer-x/agent_tenant_readiness.py, tools/qualification/paxeer-x/mcp_web_payer.py (HEAD sites tools/qualification/paxeer-x/agent_http_bounds.py:634, tools/qualification/paxeer-x/agent_tenant_readiness.py:601, tools/qualification/paxeer-x/mcp_web_payer.py:385) and each Python producer exits 0 under --help. |
| <code>acceptance.16.64.1</code> | req | 16064.1 |
| <code>acceptance.16.64.1</code> | scenario | Given tools/qualification/paxeer-x/program_balance_read.py, tools/qualification/paxeer-x/program_cache_native_equivalence.py, tools/qualification/paxeer-x/program_market_attestation.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.64.1</code> | test | tools/qualification/test_retired_callers_17.py::RetiredCallers17Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.64.1</code> | qualification | UNRUN |
| <code>req.16064</code> | ac_1 | The scanner finds no anchor, checkpoint-evidence caller in tools/qualification/paxeer-x/program_balance_read.py, tools/qualification/paxeer-x/program_cache_native_equivalence.py, tools/qualification/paxeer-x/program_market_attestation.py (HEAD sites tools/qualification/paxeer-x/program_balance_read.py:80, tools/qualification/paxeer-x/program_cache_native_equivalence.py:83, tools/qualification/paxeer-x/program_market_attestation.py:23) and each Python producer exits 0 under --help. |
| <code>acceptance.16.65.1</code> | req | 16065.1 |
| <code>acceptance.16.65.1</code> | scenario | Given tools/qualification/paxeer-x/program_spend_composition.py, tools/qualification/paxeer-x/program_transfer_authority.py, tools/qualification/paxeer-x/wallet-402-caps.py without retired construct references, when the scanner runs on them and each Python producer runs with --help, then all exit 0 |
| <code>acceptance.16.65.1</code> | test | tools/qualification/test_retired_callers_18.py::RetiredCallers18Test.test_producers_name_no_retired_construct (planned) |
| <code>acceptance.16.65.1</code> | qualification | UNRUN |
| <code>req.16065</code> | ac_1 | The scanner finds no anchor, checkpoint-evidence caller in tools/qualification/paxeer-x/program_spend_composition.py, tools/qualification/paxeer-x/program_transfer_authority.py, tools/qualification/paxeer-x/wallet-402-caps.py (HEAD sites tools/qualification/paxeer-x/program_spend_composition.py:87, tools/qualification/paxeer-x/program_transfer_authority.py:35, tools/qualification/paxeer-x/wallet-402-caps.py:163) and each Python producer exits 0 under --help. |
| <code>capability.16.10.1</code> | title | Read the service catalogue from the layerx-merge spec in the release and native custody producers |
| <code>capability.16.10.1</code> | reqs | 16045.1 |
| <code>capability.16.10.2</code> | title | Read the service catalogue from the layerx-merge spec in the deposit, gas station and archive producers |
| <code>capability.16.10.2</code> | reqs | 16046.1 |
| <code>capability.16.10.3</code> | title | Read the service catalogue from the layerx-merge spec in the projection, router and attestation producers |
| <code>capability.16.10.3</code> | reqs | 16047.1 |
| <code>capability.16.11.1</code> | title | Remove retired daemon and codec references from the arbiter producers (1 of 2) |
| <code>capability.16.11.1</code> | reqs | 16048.1 |
| <code>capability.16.11.10</code> | title | Remove retired daemon and codec references from the agent envelope, event delivery and form validation producers (2 of 2) |
| <code>capability.16.11.10</code> | reqs | 16057.1 |
| <code>capability.16.11.11</code> | title | Remove retired daemon and codec references from the custody and credit producers (1 of 2) |
| <code>capability.16.11.11</code> | reqs | 16058.1 |
| <code>capability.16.11.12</code> | title | Remove retired daemon and codec references from the custody and credit producers (2 of 2) |
| <code>capability.16.11.12</code> | reqs | 16059.1 |
| <code>capability.16.11.13</code> | title | Remove retired daemon and codec references from the LNI readiness and admission producers (1 of 3) |
| <code>capability.16.11.13</code> | reqs | 16060.1 |
| <code>capability.16.11.14</code> | title | Remove retired daemon and codec references from the LNI readiness and admission producers (2 of 3) |
| <code>capability.16.11.14</code> | reqs | 16061.1 |
| <code>capability.16.11.15</code> | title | Remove retired daemon and codec references from the LNI readiness and admission producers (3 of 3) |
| <code>capability.16.11.15</code> | reqs | 16062.1 |
| <code>capability.16.11.16</code> | title | Remove retired daemon and codec references from the client evidence producers that cite checkpoint settlement (1 of 3) |
| <code>capability.16.11.16</code> | reqs | 16063.1 |
| <code>capability.16.11.17</code> | title | Remove retired daemon and codec references from the client evidence producers that cite checkpoint settlement (2 of 3) |
| <code>capability.16.11.17</code> | reqs | 16064.1 |
| <code>capability.16.11.18</code> | title | Remove retired daemon and codec references from the client evidence producers that cite checkpoint settlement (3 of 3) |
| <code>capability.16.11.18</code> | reqs | 16065.1 |
| <code>capability.16.11.2</code> | title | Remove retired daemon and codec references from the arbiter producers (2 of 2) |
| <code>capability.16.11.2</code> | reqs | 16049.1 |
| <code>capability.16.11.3</code> | title | Remove retired daemon and codec references from the replay and market sandbox producers (1 of 2) |
| <code>capability.16.11.3</code> | reqs | 16050.1 |
| <code>capability.16.11.4</code> | title | Remove retired daemon and codec references from the replay and market sandbox producers (2 of 2) |
| <code>capability.16.11.4</code> | reqs | 16051.1 |
| <code>capability.16.11.5</code> | title | Remove retired daemon and codec references from the native program link and archive producers (1 of 2) |
| <code>capability.16.11.5</code> | reqs | 16052.1 |
| <code>capability.16.11.6</code> | title | Remove retired daemon and codec references from the native program link and archive producers (2 of 2) |
| <code>capability.16.11.6</code> | reqs | 16053.1 |
| <code>capability.16.11.7</code> | title | Remove retired daemon and codec references from the authority evidence and storage recovery producers (1 of 2) |
| <code>capability.16.11.7</code> | reqs | 16054.1 |
| <code>capability.16.11.8</code> | title | Remove retired daemon and codec references from the authority evidence and storage recovery producers (2 of 2) |
| <code>capability.16.11.8</code> | reqs | 16055.1 |
| <code>capability.16.11.9</code> | title | Remove retired daemon and codec references from the agent envelope, event delivery and form validation producers (1 of 2) |
| <code>capability.16.11.9</code> | reqs | 16056.1 |
| <code>capability.16.5.5</code> | title | Drop the LNI step from the program HTTP producer |
| <code>capability.16.5.5</code> | reqs | 16030.1 |

## replay-gates

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.990.1</code> | req | 16990.1 |
| <code>acceptance.16.990.1</code> | scenario | Given tests/replay/golden/history.lxl, when lxp_test_golden_replay runs golden_replay_two_kernels_match, then both kernels reproduce every recorded root |
| <code>acceptance.16.990.1</code> | test | tests/replay/lxp_test_golden_replay.c::golden_replay_two_kernels_match (existing) |
| <code>acceptance.16.990.1</code> | qualification | UNRUN |
| <code>req.16990</code> | ac_1 | golden_replay_two_kernels_match replays every golden record on two kernels through lxp_module_ctx_init and both produce the recorded roots. |
| <code>capability.16.98.1</code> | title | Golden replay without the kernel replay API |
| <code>capability.16.98.1</code> | reqs | 16990.1 |
| <code>resolution.17.533</code> | title | decision.190 binds: The golden replay release gate is kept and ported |
| <code>resolution.17.533</code> | applies_to | decision.190<br>task.1.4.5<br>task.16.98.1 |
| <code>resolution.17.533</code> | source | decision.190 in fragments/J79/decision.kvx; tasks from affects |

## wave-gates

| Interface | Kind | Status | Source at pinned revision | Signature |
|---|---|---|---|---|
| <code>interface.16.cli-scripts-gates-run-gate-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/run_gate.py</code> (planned) | <code>run_gate.py --wave N --log-dir DIR -- COMMAND [-- COMMAND ...]</code> |
| <code>interface.16.cli-scripts-gates-wave-0-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-0.sh</code> (planned) | <code>scripts/gates/wave-0.sh [--list]</code> |
| <code>interface.16.cli-scripts-gates-wave-1-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-1.sh</code> (planned) | <code>scripts/gates/wave-1.sh [--list]</code> |
| <code>interface.16.cli-scripts-gates-wave-2-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-2.sh</code> (planned) | <code>scripts/gates/wave-2.sh [--list]</code> |
| <code>interface.16.cli-scripts-gates-wave-3-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-3.sh</code> (planned) | <code>scripts/gates/wave-3.sh [--list]</code> |
| <code>interface.16.cli-scripts-gates-wave-4-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-4.sh</code> (planned) | <code>scripts/gates/wave-4.sh [--list]</code> |
| <code>interface.16.cli-scripts-gates-wave-5-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-5.sh</code> (planned) | <code>scripts/gates/wave-5.sh [--list]</code> |
| <code>interface.16.cli-scripts-gates-wave-6-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-6.sh</code> (planned) | <code>scripts/gates/wave-6.sh [--list]</code> |
| <code>interface.16.cli-scripts-gates-wave-programme-j58</code> | <code>cli_command</code> | <code>new</code> | <code>scripts/gates/wave-programme.sh</code> (planned) | <code>scripts/gates/wave-programme.sh [--list]</code> |

| Interface | Field | Value |
|---|---|---|
| <code>interface.16.cli-scripts-gates-run-gate-j58</code> | notes | Runs each command from the repository root and prints one evidence line per command: revision, command, exit code, log path. |
| <code>interface.16.cli-scripts-gates-wave-0-j58</code> | notes | Wave gate: make -n build test; make workspace-build; make workspace-lint. --list prints the commands without running them. |
| <code>interface.16.cli-scripts-gates-wave-1-j58</code> | notes | Wave gate: make workspace-build; make workspace-test; make core-test-all; make test-sanitizer-suite. --list prints the commands without running them. |
| <code>interface.16.cli-scripts-gates-wave-2-j58</code> | notes | Wave gate: make workspace-build; make workspace-test; make core-test-all; make agent-test. --list prints the commands without running them. |
| <code>interface.16.cli-scripts-gates-wave-3-j58</code> | notes | Wave gate: make workspace-build; make workspace-test; make agent-test; make interop-test; make programs-test. --list prints the commands without running them. |
| <code>interface.16.cli-scripts-gates-wave-4-j58</code> | notes | Wave gate: make workspace-build; make workspace-test; make core-qualification-environment; make beta-driver-test. --list prints the commands without running them. |
| <code>interface.16.cli-scripts-gates-wave-5-j58</code> | notes | Wave gate: make workspace-build; make workspace-test; make human-test; tools/containers/check.sh. --list prints the commands without running them. |
| <code>interface.16.cli-scripts-gates-wave-6-j58</code> | notes | Wave gate: make workspace-build; make workspace-test; make workspace-lint; make core-test-all; python3 tools/paxeer-x/tests/retired_callers_test.py --deleted; make programs-test. --list prints the commands without running them. |
| <code>interface.16.cli-scripts-gates-wave-programme-j58</code> | notes | Wave gate: make workspace-ci; make beta-qualify; python3 tools/paxeer-x/tests/retired_callers_test.py --deleted; scripts/ci/beta-ledger-check.sh --revisions. --list prints the commands without running them. |

Contract context <code>contract_context.16.wave-gates</code>:

| Field | Frozen rule |
|---|---|
| interface | scripts/gates/wave-&lt;n&gt;.sh [--list] and scripts/gates/run_gate.py --wave N --log-dir DIR -- COMMAND (planned) |
| storage | Logs under the caller-supplied log directory, one file per command: wave-&lt;n&gt;-&lt;index&gt;.log. |
| errors | Exit code of the first failing command; exit 2 on usage errors. |
| events | One evidence line per command: revision, command, exit code, log path. |
| gas | none (off-chain tooling) |
| activation | Available from wave 0; each wave gate runs once on the merged wave revision. |
| invariants | A wave gate names only commands that exist at HEAD; evidence is never printed for a command that did not run. |
| pause | none |

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.1.1</code> | req | 16001.1 |
| <code>acceptance.16.1.1</code> | scenario | Given the real Makefile at HEAD, when run_gate.py --wave 0 --log-dir &lt;tmp&gt; -- make -n build runs, then it exits 0 and prints one evidence line naming the revision, command, exit 0 and an existing log path |
| <code>acceptance.16.1.1</code> | test | scripts/gates/test_run_gate.py::RunGateTest.test_records_evidence_line_for_passing_command (planned) |
| <code>acceptance.16.1.1</code> | qualification | UNRUN |
| <code>req.16001</code> | ac_1 | scripts/gates/run_gate.py (planned) runs make -n build against Makefile:211 and prints one line revision=&lt;sha&gt; command=make -n build exit=0 log=&lt;path&gt; with the log file present. |
| <code>acceptance.16.1.2</code> | req | 16001.2 |
| <code>acceptance.16.1.2</code> | scenario | Given a command make no-such-target that fails with exit 2, when run_gate.py runs it, then run_gate.py exits 2 and its evidence line reads exit=2 |
| <code>acceptance.16.1.2</code> | test | scripts/gates/test_run_gate.py::RunGateTest.test_nonzero_exit_is_recorded_and_returned (planned) |
| <code>acceptance.16.1.2</code> | qualification | UNRUN |
| <code>req.16001</code> | ac_2 | scripts/gates/run_gate.py (planned) returns the exit code of the first failing command and records it in the evidence line. |
| <code>acceptance.16.2.1</code> | req | 16002.1 |
| <code>acceptance.16.2.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-0.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.2.1</code> | test | scripts/gates/test_wave_0.py::Wave0GateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.2.1</code> | qualification | UNRUN |
| <code>req.16002</code> | ac_1 | scripts/gates/wave-0.sh (planned) --list prints exactly: make -n build test &#124; make workspace-build &#124; make workspace-lint, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>acceptance.16.3.1</code> | req | 16003.1 |
| <code>acceptance.16.3.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-1.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.3.1</code> | test | scripts/gates/test_wave_1.py::Wave1GateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.3.1</code> | qualification | UNRUN |
| <code>req.16003</code> | ac_1 | scripts/gates/wave-1.sh (planned) --list prints exactly: make workspace-build &#124; make workspace-test &#124; make core-test-all &#124; make test-sanitizer-suite, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>acceptance.16.4.1</code> | req | 16004.1 |
| <code>acceptance.16.4.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-2.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.4.1</code> | test | scripts/gates/test_wave_2.py::Wave2GateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.4.1</code> | qualification | UNRUN |
| <code>req.16004</code> | ac_1 | scripts/gates/wave-2.sh (planned) --list prints exactly: make workspace-build &#124; make workspace-test &#124; make core-test-all &#124; make agent-test, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>acceptance.16.5.1</code> | req | 16005.1 |
| <code>acceptance.16.5.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-3.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.5.1</code> | test | scripts/gates/test_wave_3.py::Wave3GateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.5.1</code> | qualification | UNRUN |
| <code>req.16005</code> | ac_1 | scripts/gates/wave-3.sh (planned) --list prints exactly: make workspace-build &#124; make workspace-test &#124; make agent-test &#124; make interop-test &#124; make programs-test, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>acceptance.16.6.1</code> | req | 16006.1 |
| <code>acceptance.16.6.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-4.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.6.1</code> | test | scripts/gates/test_wave_4.py::Wave4GateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.6.1</code> | qualification | UNRUN |
| <code>req.16006</code> | ac_1 | scripts/gates/wave-4.sh (planned) --list prints exactly: make workspace-build &#124; make workspace-test &#124; make core-qualification-environment &#124; make beta-driver-test, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>acceptance.16.7.1</code> | req | 16007.1 |
| <code>acceptance.16.7.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-5.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.7.1</code> | test | scripts/gates/test_wave_5.py::Wave5GateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.7.1</code> | qualification | UNRUN |
| <code>req.16007</code> | ac_1 | scripts/gates/wave-5.sh (planned) --list prints exactly: make workspace-build &#124; make workspace-test &#124; make human-test &#124; tools/containers/check.sh, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>acceptance.16.8.1</code> | req | 16008.1 |
| <code>acceptance.16.8.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-6.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.8.1</code> | test | scripts/gates/test_wave_6.py::Wave6GateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.8.1</code> | qualification | UNRUN |
| <code>req.16008</code> | ac_1 | scripts/gates/wave-6.sh (planned) --list prints exactly: make workspace-build &#124; make workspace-test &#124; make workspace-lint &#124; make core-test-all &#124; python3 tools/paxeer-x/tests/retired_callers_test.py --deleted &#124; make programs-test, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>acceptance.16.9.1</code> | req | 16009.1 |
| <code>acceptance.16.9.1</code> | scenario | Given the Makefile at HEAD, when scripts/gates/wave-programme.sh --list runs and make -n is run for each listed make target, then the list equals the frozen command list and every make -n exits 0 |
| <code>acceptance.16.9.1</code> | test | scripts/gates/test_wave_programme.py::WaveProgrammeGateTest.test_list_names_runnable_commands (planned) |
| <code>acceptance.16.9.1</code> | qualification | UNRUN |
| <code>req.16009</code> | ac_1 | scripts/gates/wave-programme.sh (planned) --list prints exactly: make workspace-ci &#124; make beta-qualify &#124; python3 tools/paxeer-x/tests/retired_callers_test.py --deleted &#124; scripts/ci/beta-ledger-check.sh --revisions, and every make target it names resolves under make -n against the Makefile at HEAD (Makefile:211). |
| <code>capability.16.0.1</code> | title | Add the wave gate runner that records revision, command, exit code and log path |
| <code>capability.16.0.1</code> | reqs | 16001.1<br>16001.2 |
| <code>capability.16.1.1</code> | title | Add the wave 0 gate script over the existing aggregates |
| <code>capability.16.1.1</code> | reqs | 16002.1 |
| <code>capability.16.1.2</code> | title | Add the wave 1 gate script over the existing aggregates |
| <code>capability.16.1.2</code> | reqs | 16003.1 |
| <code>capability.16.1.3</code> | title | Add the wave 2 gate script over the existing aggregates |
| <code>capability.16.1.3</code> | reqs | 16004.1 |
| <code>capability.16.1.4</code> | title | Add the wave 3 gate script over the existing aggregates |
| <code>capability.16.1.4</code> | reqs | 16005.1 |
| <code>capability.16.1.5</code> | title | Add the wave 4 gate script over the existing aggregates |
| <code>capability.16.1.5</code> | reqs | 16006.1 |
| <code>capability.16.1.6</code> | title | Add the wave 5 gate script over the existing aggregates |
| <code>capability.16.1.6</code> | reqs | 16007.1 |
| <code>capability.16.1.7</code> | title | Add the wave 6 gate script over the existing aggregates |
| <code>capability.16.1.7</code> | reqs | 16008.1 |
| <code>capability.16.1.8</code> | title | Add the programme gate script over the existing aggregates |
| <code>capability.16.1.8</code> | reqs | 16009.1 |
| <code>resolution.17.978</code> | title | decision.1607 binds: Wave gates compose existing aggregates |
| <code>resolution.17.978</code> | applies_to | decision.1607<br>task.16.0.1<br>task.16.1.1 |
| <code>resolution.17.978</code> | source | decision.1607 in fragments/J58/decision.kvx; tasks from affects |
| <code>resolution.17.980</code> | title | decision.1609 binds: Generated, relative and non-path names are not defects |
| <code>resolution.17.980</code> | applies_to | decision.1609<br>task.16.0.1 |
| <code>resolution.17.980</code> | source | decision.1609 in fragments/J58/decision.kvx; tasks from no affected task: bound to the domain-16 foundation anchor |
| <code>resolution.17.981</code> | title | decision.1610 binds: Rules whose source names a retired symbol keep their rule |
| <code>resolution.17.981</code> | applies_to | decision.1610<br>task.16.0.1 |
| <code>resolution.17.981</code> | source | decision.1610 in fragments/J58/decision.kvx; tasks from no affected task: bound to the domain-16 foundation anchor |

## wiring-integration

No interface block names this anchor; the citing criteria and rules below define its scope.

| Citing block | Field | Frozen criterion or rule |
|---|---|---|
| <code>acceptance.16.961.1</code> | req | 16961.1 |
| <code>acceptance.16.961.1</code> | scenario | Given the edited suites, when the test greps tests/ for lx_checkpoint_registry_register_deposit_root and the exit and withdrawal claim calls and runs make -n on the four suite targets, then no caller remains and make exits 0. |
| <code>acceptance.16.961.1</code> | test | tests/ci/test_claims_and_replay_removed.py::test_no_claim_or_deposit_root_callers (planned) |
| <code>acceptance.16.961.1</code> | qualification | UNRUN |
| <code>req.16961</code> | ac_1 | No kernel test calls the exit, withdrawal or deposit-root claim functions and the four edited suites build. |
| <code>acceptance.16.961.2</code> | req | 16961.2 |
| <code>acceptance.16.961.2</code> | scenario | Given the edited suites, when the test greps tests/ for LXP_ERR_IDEMPOTENT_REPLAY and runs make test-state-commitment-transition, then the code is absent and the target exits 0. |
| <code>acceptance.16.961.2</code> | test | tests/ci/test_claims_and_replay_removed.py::test_replay_expects_sequence_refusal (planned) |
| <code>acceptance.16.961.2</code> | qualification | UNRUN |
| <code>req.16961</code> | ac_2 | No kernel test expects LXP_ERR_IDEMPOTENT_REPLAY and the state commitment replay case expects a sequence refusal. |
| <code>acceptance.16.962.1</code> | req | 16962.1 |
| <code>acceptance.16.962.1</code> | scenario | Given the edited suite, when the test greps tests/test_receipt_offline.c and runs make -n on its target, then lxp_receipt_verify_checkpointed is absent, lxp_receipt_verify_offline is present and make exits 0. |
| <code>acceptance.16.962.1</code> | test | tests/ci/test_receipt_offline_verify.py::test_offline_suite_uses_offline_verify_only (planned) |
| <code>acceptance.16.962.1</code> | qualification | UNRUN |
| <code>req.16962</code> | ac_1 | The offline receipt suite calls lxp_receipt_verify_offline and not lxp_receipt_verify_checkpointed, and its target builds. |
| <code>acceptance.16.963.1</code> | req | 16963.1 |
| <code>acceptance.16.963.1</code> | scenario | Given the edited suite, when the test greps it for lxp_capacity_ledger and runs make -n on its target, then the name is absent and make exits 0. |
| <code>acceptance.16.963.1</code> | test | tests/ci/test_programs_capacity_feed_removed.py::test_blob_admission_has_no_capacity_ledger (planned) |
| <code>acceptance.16.963.1</code> | qualification | UNRUN |
| <code>req.16963</code> | ac_1 | tests/programs/test_blob_admission.c names no lxp_capacity_ledger and builds. |
| <code>acceptance.16.963.2</code> | req | 16963.2 |
| <code>acceptance.16.963.2</code> | scenario | Given the edited suite, when the test greps it for lxp_kernel_capacity_observe and runs make -n on its target, then the name is absent and make exits 0. |
| <code>acceptance.16.963.2</code> | test | tests/ci/test_programs_capacity_feed_removed.py::test_native_gaps_has_no_capacity_observe (planned) |
| <code>acceptance.16.963.2</code> | qualification | UNRUN |
| <code>req.16963</code> | ac_2 | tests/programs/test_paxai_native_gaps.c names no lxp_kernel_capacity_observe and builds. |
| <code>acceptance.16.963.3</code> | req | 16963.3 |
| <code>acceptance.16.963.3</code> | scenario | Given the six edited suites, when the test greps tests/programs for lx_programs_state_feed_store and the replay declarations, then none occurs. |
| <code>acceptance.16.963.3</code> | test | tests/ci/test_programs_capacity_feed_removed.py::test_programs_suites_have_no_feed_store (planned) |
| <code>acceptance.16.963.3</code> | qualification | UNRUN |
| <code>req.16963</code> | ac_3 | No programs suite names lx_programs_state_feed_store or a replay engine declaration of include/layerx/programs.h. |
| <code>acceptance.16.964.1</code> | req | 16964.1 |
| <code>acceptance.16.964.1</code> | scenario | Given the edited suite, when the test greps it for the queue disposition codec calls and runs make -n on its target, then none occurs and make exits 0. |
| <code>acceptance.16.964.1</code> | test | tests/ci/test_disposition_budget_pull_removed.py::test_no_queue_disposition_codec_case (planned) |
| <code>acceptance.16.964.1</code> | qualification | UNRUN |
| <code>req.16964</code> | ac_1 | The terminal rejection suite has no queue-disposition codec case and builds. |
| <code>acceptance.16.964.2</code> | req | 16964.2 |
| <code>acceptance.16.964.2</code> | scenario | Given the edited suite, when the test greps it for lx_budget_pull_request and runs make -n on its target, then the name is absent and make exits 0. |
| <code>acceptance.16.964.2</code> | test | tests/ci/test_disposition_budget_pull_removed.py::test_no_budget_pull_request_case (planned) |
| <code>acceptance.16.964.2</code> | qualification | UNRUN |
| <code>req.16964</code> | ac_2 | The budget delegate suite names no lx_budget_pull_request and builds. |
| <code>acceptance.16.965.1</code> | req | 16965.1 |
| <code>acceptance.16.965.1</code> | scenario | Given the wave-6 tree, when the test checks the path and greps tests/ for both names, then the file is absent and neither name occurs. |
| <code>acceptance.16.965.1</code> | test | tests/ci/test_kernel_suite_retired_uses.py::test_no_receive_or_grant_store_caller (planned) |
| <code>acceptance.16.965.1</code> | qualification | UNRUN |
| <code>req.16965</code> | ac_1 | tests/test_gateway_receive.c is absent and no kernel test names lxp_receive_environment or lxp_grant_store. |
| <code>acceptance.16.965.2</code> | req | 16965.2 |
| <code>acceptance.16.965.2</code> | scenario | Given the edited table and header, when the test parses both, then the entry sets are equal with 49 members. |
| <code>acceptance.16.965.2</code> | test | tests/ci/test_kernel_suite_retired_uses.py::test_qual_replay_table_matches_header (planned) |
| <code>acceptance.16.965.2</code> | qualification | UNRUN |
| <code>req.16965</code> | ac_2 | The replay table of lxp_qual_replay.c has exactly the 49 entries of lxp_qualification.h. |
| <code>acceptance.16.965.3</code> | req | 16965.3 |
| <code>acceptance.16.965.3</code> | scenario | Given the edited suite, when the test greps it for lxp_handover and runs make -n on its target, then the name is absent and make exits 0. |
| <code>acceptance.16.965.3</code> | test | tests/ci/test_kernel_suite_retired_uses.py::test_no_handover_genesis_case (planned) |
| <code>acceptance.16.965.3</code> | qualification | UNRUN |
| <code>req.16965</code> | ac_3 | tests/protocol/lxp_test_kernel.c names no lxp_handover symbol and builds. |
| <code>acceptance.16.965.4</code> | req | 16965.4 |
| <code>acceptance.16.965.4</code> | scenario | Given the edited suite, when the test greps tests/ for lxp_ledger_apply_bridge_credit, then it does not occur. |
| <code>acceptance.16.965.4</code> | test | tests/ci/test_kernel_suite_retired_uses.py::test_no_bridge_credit_apply_caller (planned) |
| <code>acceptance.16.965.4</code> | qualification | UNRUN |
| <code>req.16965</code> | ac_4 | No kernel test names lxp_ledger_apply_bridge_credit. |
| <code>acceptance.16.966.1</code> | req | 16966.1 |
| <code>acceptance.16.966.1</code> | scenario | Given the wave-6 tree, when the test checks the path and greps tests/, then the file is absent and the name does not occur. |
| <code>acceptance.16.966.1</code> | test | tests/ci/test_requested_suites_absent.py::test_arbiter_native_authority_absent (planned) |
| <code>acceptance.16.966.1</code> | qualification | UNRUN |
| <code>req.16966</code> | ac_1 | tests/daemon/lxp_test_arbiter_native_authority.c is absent and no test names lxp_verified_receipt_authority_facts. |
| <code>acceptance.16.966.2</code> | req | 16966.2 |
| <code>acceptance.16.966.2</code> | scenario | Given the wave-6 tree, when the test checks the path and greps tests/ for lx_web.h, then neither remains. |
| <code>acceptance.16.966.2</code> | test | tests/ci/test_requested_suites_absent.py::test_web_intake_absent (planned) |
| <code>acceptance.16.966.2</code> | qualification | UNRUN |
| <code>req.16966</code> | ac_2 | tests/modules/test_web_intake.c is absent and no test includes layerx/lx_web.h. |
| <code>acceptance.16.966.3</code> | req | 16966.3 |
| <code>acceptance.16.966.3</code> | scenario | Given the wave-6 tree, when the test checks the path and greps tests/ for lxp_daemon.h, then neither remains. |
| <code>acceptance.16.966.3</code> | test | tests/ci/test_requested_suites_absent.py::test_layerxd_suite_absent (planned) |
| <code>acceptance.16.966.3</code> | qualification | UNRUN |
| <code>req.16966</code> | ac_3 | tests/test_layerxd.c is absent and no test includes layerx/lxp_daemon.h. |
| <code>acceptance.16.966.4</code> | req | 16966.4 |
| <code>acceptance.16.966.4</code> | scenario | Given the wave-6 tree, when the test checks the three paths and runs make -n build test, then all are absent and make exits 0. |
| <code>acceptance.16.966.4</code> | test | tests/ci/test_requested_suites_absent.py::test_bridge_credit_suites_absent (planned) |
| <code>acceptance.16.966.4</code> | qualification | UNRUN |
| <code>req.16966</code> | ac_4 | tests/bridge/test_credit.c, tests/bridge/test_light_credit.c and tests/bridge/sign_credit.c are absent and make -n build test exits 0. |
| <code>acceptance.16.967.1</code> | req | 16967.1 |
| <code>acceptance.16.967.1</code> | scenario | Given the wave-6 tree, when the test checks docker/relay-archive, then it does not exist. |
| <code>acceptance.16.967.1</code> | test | tests/ci/test_requested_retirements_absent.py::test_relay_archive_image_absent (planned) |
| <code>acceptance.16.967.1</code> | qualification | UNRUN |
| <code>req.16967</code> | ac_1 | The docker/relay-archive directory is absent. |
| <code>acceptance.16.967.2</code> | req | 16967.2 |
| <code>acceptance.16.967.2</code> | scenario | Given the wave-6 tree, when the test checks tests/relay-archive, then it does not exist. |
| <code>acceptance.16.967.2</code> | test | tests/ci/test_requested_retirements_absent.py::test_relay_archive_tests_absent (planned) |
| <code>acceptance.16.967.2</code> | qualification | UNRUN |
| <code>req.16967</code> | ac_2 | The tests/relay-archive directory is absent. |
| <code>acceptance.16.967.3</code> | req | 16967.3 |
| <code>acceptance.16.967.3</code> | scenario | Given the wave-6 tree, when the test greps tests/solidity for both import paths, then none occurs and ContractIntegration.t.sol, ContractExecutionSafety.t.sol and CheckpointChallengeManager.t.sol are absent. |
| <code>acceptance.16.967.3</code> | test | tests/ci/test_requested_retirements_absent.py::test_security_storage_suites_absent (planned) |
| <code>acceptance.16.967.3</code> | qualification | UNRUN |
| <code>req.16967</code> | ac_3 | No Solidity suite under tests/solidity imports contracts/security or contracts/storage. |
| <code>acceptance.16.967.4</code> | req | 16967.4 |
| <code>acceptance.16.967.4</code> | scenario | Given the wave-6 tree, when the test checks both paths, then neither exists. |
| <code>acceptance.16.967.4</code> | test | tests/ci/test_requested_retirements_absent.py::test_nullifier_registry_suites_absent (planned) |
| <code>acceptance.16.967.4</code> | qualification | UNRUN |
| <code>req.16967</code> | ac_4 | tests/solidity/ContractInvariants.t.sol and tests/solidity/PaxeerBetaDeploy.t.sol are absent. |
| <code>acceptance.16.967.5</code> | req | 16967.5 |
| <code>acceptance.16.967.5</code> | scenario | Given the wave-6 tree, when the test checks the path and greps scripts/ for WithdrawalNullifierRegistry, then neither remains. |
| <code>acceptance.16.967.5</code> | test | tests/ci/test_requested_retirements_absent.py::test_beta_deploy_script_absent (planned) |
| <code>acceptance.16.967.5</code> | qualification | UNRUN |
| <code>req.16967</code> | ac_5 | scripts/PaxeerBetaDeploy.s.sol is absent, so no script imports WithdrawalNullifierRegistry. |
| <code>acceptance.16.967.6</code> | req | 16967.6 |
| <code>acceptance.16.967.6</code> | scenario | Given the wave-6 tree, when the test reads the contracts list of the script, then WithdrawalNullifierRegistry is absent. |
| <code>acceptance.16.967.6</code> | test | tests/ci/test_requested_retirements_absent.py::test_state_surface_names_no_nullifier_registry (planned) |
| <code>acceptance.16.967.6</code> | qualification | UNRUN |
| <code>req.16967</code> | ac_6 | scripts/ci/solidity-state-surface.sh names no WithdrawalNullifierRegistry. |
| <code>acceptance.16.968.1</code> | req | 16968.1 |
| <code>acceptance.16.968.1</code> | scenario | Given scripts/gates/wave-0.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.1</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_0_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.1</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_1 | scripts/gates/wave-0.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.968.2</code> | req | 16968.2 |
| <code>acceptance.16.968.2</code> | scenario | Given scripts/gates/wave-1.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.2</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_1_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.2</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_2 | scripts/gates/wave-1.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.968.3</code> | req | 16968.3 |
| <code>acceptance.16.968.3</code> | scenario | Given scripts/gates/wave-2.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.3</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_2_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.3</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_3 | scripts/gates/wave-2.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.968.4</code> | req | 16968.4 |
| <code>acceptance.16.968.4</code> | scenario | Given scripts/gates/wave-3.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.4</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_3_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.4</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_4 | scripts/gates/wave-3.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.968.5</code> | req | 16968.5 |
| <code>acceptance.16.968.5</code> | scenario | Given scripts/gates/wave-4.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.5</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_4_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.5</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_5 | scripts/gates/wave-4.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.968.6</code> | req | 16968.6 |
| <code>acceptance.16.968.6</code> | scenario | Given scripts/gates/wave-5.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.6</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_5_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.6</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_6 | scripts/gates/wave-5.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.968.7</code> | req | 16968.7 |
| <code>acceptance.16.968.7</code> | scenario | Given scripts/gates/wave-6.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.7</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_6_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.7</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_7 | scripts/gates/wave-6.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.968.8</code> | req | 16968.8 |
| <code>acceptance.16.968.8</code> | scenario | Given scripts/gates/wave-programme.sh, when the test runs it with --list and runs make -n on each listed target, then all exit 0 and the list is nonempty. |
| <code>acceptance.16.968.8</code> | test | tests/ci/test_wave_gate_scripts.py::test_wave_programme_gate_lists_runnable_commands (planned) |
| <code>acceptance.16.968.8</code> | qualification | UNRUN |
| <code>req.16968</code> | ac_8 | scripts/gates/wave-programme.sh exists, prints its commands with --list and exits 0, and every make target it lists resolves with make -n. |
| <code>acceptance.16.969.1</code> | req | 16969.1 |
| <code>acceptance.16.969.1</code> | scenario | Given the edited image files, when the test greps docker/ for both names and runs bash -n docker/kernel/init.sh, then neither name occurs and bash exits 0. |
| <code>acceptance.16.969.1</code> | test | tools/containers/test_requested_image_edits.py::test_images_copy_no_guarantor_script (planned) |
| <code>acceptance.16.969.1</code> | qualification | UNRUN |
| <code>req.16969</code> | ac_1 | No Dockerfile or init script names guarantor.sh or checkpoint-authority.py. |
| <code>acceptance.16.969.2</code> | req | 16969.2 |
| <code>acceptance.16.969.2</code> | scenario | Given the edited Dockerfile, when the test reads its COPY instructions, then none names relay_archive and tools/containers/check.sh kernel exits 0. |
| <code>acceptance.16.969.2</code> | test | tools/containers/test_requested_image_edits.py::test_kernel_image_copies_no_relay_archive (planned) |
| <code>acceptance.16.969.2</code> | qualification | UNRUN |
| <code>req.16969</code> | ac_2 | docker/kernel/Dockerfile copies nothing into /opt/layerx/relay_archive. |
| <code>acceptance.16.969.3</code> | req | 16969.3 |
| <code>acceptance.16.969.3</code> | scenario | Given the edited Dockerfiles, when the test greps both for layerx-explorer-index, then it does not occur. |
| <code>acceptance.16.969.3</code> | test | tools/containers/test_requested_image_edits.py::test_images_build_no_explorer_index (planned) |
| <code>acceptance.16.969.3</code> | qualification | UNRUN |
| <code>req.16969</code> | ac_3 | Neither the human-service nor the kernel Dockerfile builds or copies layerx-explorer-index. |
| <code>acceptance.16.969.4</code> | req | 16969.4 |
| <code>acceptance.16.969.4</code> | scenario | Given the edited rollout file, when the test parses it, then no unit or box names layerx-explorer-index. |
| <code>acceptance.16.969.4</code> | test | tools/containers/test_requested_image_edits.py::test_rollout_has_no_explorer_index_unit (planned) |
| <code>acceptance.16.969.4</code> | qualification | UNRUN |
| <code>req.16969</code> | ac_4 | deploy/rollout.kvx has no layerx-explorer-index.service unit or box entry. |
| <code>acceptance.16.970.1</code> | req | 16970.1 |
| <code>acceptance.16.970.1</code> | scenario | Given the edited files, when the test greps tools/bringup for both names and runs bash -n tools/bringup/kernel-genesis.sh, then neither occurs and bash exits 0. |
| <code>acceptance.16.970.1</code> | test | tools/bringup/tests/test_no_checkpoint_authority.py::test_bringup_names_no_checkpoint_authority (planned) |
| <code>acceptance.16.970.1</code> | qualification | UNRUN |
| <code>req.16970</code> | ac_1 | No bringup script or test names checkpoint-authority.py or publication-policy.py. |
| <code>acceptance.16.971.1</code> | req | 16971.1 |
| <code>acceptance.16.971.1</code> | scenario | Given the edited scripts, when the test greps tools/paxeer-x/gates and tools/qualification for reset_recovery.py and runs bash -n on each edited gate, then the path is absent and bash exits 0. |
| <code>acceptance.16.971.1</code> | test | tools/ci/tests/test_deleted_harness_paths.py::test_no_gate_runs_reset_recovery (planned) |
| <code>acceptance.16.971.1</code> | qualification | UNRUN |
| <code>req.16971</code> | ac_1 | No gate or qualification script names services/platform/hosted/node/tests/reset_recovery.py. |
| <code>acceptance.16.971.2</code> | req | 16971.2 |
| <code>acceptance.16.971.2</code> | scenario | Given the edited xweb-workflow-check.sh, when the test runs it against the tree, then it exits 0 and none of the four paths is listed. |
| <code>acceptance.16.971.2</code> | test | tools/ci/tests/test_deleted_harness_paths.py::test_workflow_list_names_no_gateway_harness (planned) |
| <code>acceptance.16.971.2</code> | qualification | UNRUN |
| <code>req.16971</code> | ac_2 | The workflow path list names none of custody_chain.py, events.rs, funding.rs or lifecycle.rs of the gateway local harness. |
| <code>acceptance.16.971.3</code> | req | 16971.3 |
| <code>acceptance.16.971.3</code> | scenario | Given the edited scripts, when the test greps them for the core kernel test paths, then none occurs. |
| <code>acceptance.16.971.3</code> | test | tools/ci/tests/test_deleted_harness_paths.py::test_no_gate_runs_core_kernel_tests (planned) |
| <code>acceptance.16.971.3</code> | qualification | UNRUN |
| <code>req.16971</code> | ac_3 | No gate, qualification script or workflow list names the core kernel tests deleted by task 8.20.2. |
| <code>acceptance.16.972.1</code> | req | 16972.1 |
| <code>acceptance.16.972.1</code> | scenario | Given the edited scripts, when the test greps them for the deleted test paths and compiles each with py_compile, then none occurs and all compile. |
| <code>acceptance.16.972.1</code> | test | tools/qualification/tests/test_requested_inputs_removed.py::test_no_digest_of_deleted_node_tests (planned) |
| <code>acceptance.16.972.1</code> | qualification | UNRUN |
| <code>req.16972</code> | ac_1 | No qualification script carries a digest or path of the agent boundary kernel node tests. |
| <code>acceptance.16.972.2</code> | req | 16972.2 |
| <code>acceptance.16.972.2</code> | scenario | Given the edited profile, when the test reads its input list, then owner_custody.py is absent. |
| <code>acceptance.16.972.2</code> | test | tools/qualification/tests/test_requested_inputs_removed.py::test_custody_profile_omits_owner_custody (planned) |
| <code>acceptance.16.972.2</code> | qualification | UNRUN |
| <code>req.16972</code> | ac_2 | The movement custody profile inputs omit owner_custody.py. |
| <code>acceptance.16.972.3</code> | req | 16972.3 |
| <code>acceptance.16.972.3</code> | scenario | Given the edited files, when the test greps both for layerx-paxeer-client, then it does not occur. |
| <code>acceptance.16.972.3</code> | test | tools/qualification/tests/test_requested_inputs_removed.py::test_no_paxeer_client_crate_listed (planned) |
| <code>acceptance.16.972.3</code> | qualification | UNRUN |
| <code>req.16972</code> | ac_3 | Neither beta_driver.py nor movement-custody-profile.py names layerx-paxeer-client. |
| <code>acceptance.16.972.4</code> | req | 16972.4 |
| <code>acceptance.16.972.4</code> | scenario | Given the edited script, when the test greps it for layerx-explorer-index, then it does not occur and the script compiles. |
| <code>acceptance.16.972.4</code> | test | tools/qualification/tests/test_requested_inputs_removed.py::test_explorer_availability_has_no_index (planned) |
| <code>acceptance.16.972.4</code> | qualification | UNRUN |
| <code>req.16972</code> | ac_4 | explorer-availability.py neither qualifies nor launches the explorer index. |
| <code>acceptance.16.972.5</code> | req | 16972.5 |
| <code>acceptance.16.972.5</code> | scenario | Given the edited Makefile and runners, when the test runs the three make dry runs and reads both runners, then the two retired targets have no rule, human-test dry-runs cleanly and neither runner names them. |
| <code>acceptance.16.972.5</code> | test | tools/qualification/tests/test_requested_inputs_removed.py::test_retired_human_make_targets_absent (planned) |
| <code>acceptance.16.972.5</code> | qualification | UNRUN |
| <code>req.16972</code> | ac_5 | make -n human-test-explorer and make -n human-test-paxeer fail with no rule, make -n human-test still exits 0, and neither release_runner.py nor beta_driver.py names either target. |
| <code>acceptance.16.973.1</code> | req | 16973.1 |
| <code>acceptance.16.973.1</code> | scenario | Given the edited fragment, when the test reads it, then the path does not occur. |
| <code>acceptance.16.973.1</code> | test | tests/ci/test_programs_suite_references.py::test_build_fragment_names_no_balance_read (planned) |
| <code>acceptance.16.973.1</code> | qualification | UNRUN |
| <code>req.16973</code> | ac_1 | tools/paxeer-x/build/104.31.2.mk names no tests/programs/test_balance_read.c. |
| <code>acceptance.16.973.2</code> | req | 16973.2 |
| <code>acceptance.16.973.2</code> | scenario | Given the edited producer, when the test greps and py_compiles it, then the path is absent and it compiles. |
| <code>acceptance.16.973.2</code> | test | tests/ci/test_programs_suite_references.py::test_interface_producer_names_no_interface_suite (planned) |
| <code>acceptance.16.973.2</code> | qualification | UNRUN |
| <code>req.16973</code> | ac_2 | program-interface-producer.py names no tests/programs/test_interface_protocol.c and compiles. |
| <code>acceptance.16.974.1</code> | req | 16974.1 |
| <code>acceptance.16.974.1</code> | scenario | Given the edited tests, when the test greps tests/agent/sdk/python for the four fixture names and runs test_receipt_fixture.py, then none occurs and pytest exits 0. |
| <code>acceptance.16.974.1</code> | test | tests/ci/test_receipt_fixture_readers.py::test_sdk_tests_read_sequencer_receipt_vectors (planned) |
| <code>acceptance.16.974.1</code> | qualification | UNRUN |
| <code>req.16974</code> | ac_1 | No Python SDK test reads receipt-positive or receipt-refusals v2/v3, and test_receipt_fixture.py reads sequencer-receipt-v1.json. |
| <code>acceptance.16.974.2</code> | req | 16974.2 |
| <code>acceptance.16.974.2</code> | scenario | Given the edited tests, when the test greps tests/agent/sdk/python for receipt-programs-*-v2 and v3 names, then none occurs. |
| <code>acceptance.16.974.2</code> | test | tests/ci/test_receipt_fixture_readers.py::test_sdk_tests_read_no_program_receipt_v2_v3 (planned) |
| <code>acceptance.16.974.2</code> | qualification | UNRUN |
| <code>req.16974</code> | ac_2 | No Python SDK test reads a receipt-programs v2 or v3 fixture. |
| <code>acceptance.16.974.3</code> | req | 16974.3 |
| <code>acceptance.16.974.3</code> | scenario | Given the edited files, when the test greps them, then neither name occurs and bash -n on the script exits 0. |
| <code>acceptance.16.974.3</code> | test | tests/ci/test_receipt_fixture_readers.py::test_nothing_reads_executed_v4_fixture (planned) |
| <code>acceptance.16.974.3</code> | qualification | UNRUN |
| <code>req.16974</code> | ac_3 | Neither generate-abi-vectors.py nor test_native_abi5.py reads receipt-programs-executed-v4.json and verify-sdk-program-python.sh names no terminal-v4.test.py. |
| <code>acceptance.16.975.1</code> | req | 16975.1 |
| <code>acceptance.16.975.1</code> | scenario | Given the edited catalogue, when the test loads the JSON, then it parses and the signature route entry is present. |
| <code>acceptance.16.975.1</code> | test | tools/paxeer-x/tests/test_signature_route_catalogued.py::test_catalogue_lists_signature_route (planned) |
| <code>acceptance.16.975.1</code> | qualification | UNRUN |
| <code>req.16975</code> | ac_1 | The route catalogue lists GET /api/v2/transactions/:transaction_hash_param/signature next to the status route. |
| <code>acceptance.16.975.2</code> | req | 16975.2 |
| <code>acceptance.16.975.2</code> | scenario | Given the edited script, when the test runs bash -n on it and reads its run_leg lines, then bash exits 0 and one leg runs cargo test on the sig-provider manifest. |
| <code>acceptance.16.975.2</code> | test | tools/paxeer-x/tests/test_signature_route_catalogued.py::test_explorer_gate_runs_sig_provider (planned) |
| <code>acceptance.16.975.2</code> | qualification | UNRUN |
| <code>req.16975</code> | ac_2 | scripts/explorer/gate-test.sh runs the sig-provider workspace tests as a run_leg. |
| <code>acceptance.16.976.1</code> | req | 16976.1 |
| <code>acceptance.16.976.1</code> | scenario | Given the edited catalogue, when the test loads it, then the three archive routes are present and the anchors, receipts and receipts/:id routes are absent. |
| <code>acceptance.16.976.1</code> | test | tools/paxeer-x/tests/test_archive_routes_catalogued.py::test_catalogue_lists_archive_routes_only (planned) |
| <code>acceptance.16.976.1</code> | qualification | UNRUN |
| <code>req.16976</code> | ac_1 | The catalogue lists the three /api/v2/paxeer-x/archive routes and no live anchors or receipts route. |
| <code>acceptance.16.977.1</code> | req | 16977.1 |
| <code>acceptance.16.977.1</code> | scenario | Given the edited workflow, when the test parses the YAML, then both paths filters hold apps/bridge/** and the bridge-app job runs the three pnpm commands with working-directory apps/bridge. |
| <code>acceptance.16.977.1</code> | test | tools/ci/tests/test_bridge_workflow_app.py::test_bridge_workflow_runs_app_job (planned) |
| <code>acceptance.16.977.1</code> | qualification | UNRUN |
| <code>req.16977</code> | ac_1 | The bridge workflow triggers on apps/bridge/** in both filters and has a bridge-app job running install, typecheck and test in apps/bridge. |
| <code>acceptance.16.978.1</code> | req | 16978.1 |
| <code>acceptance.16.978.1</code> | scenario | Given the edited check.sh, when the test reads the OUT_OF_SCOPE list and runs tools/containers/check.sh, then the entry is absent and the script exits 0. |
| <code>acceptance.16.978.1</code> | test | tools/containers/test_out_of_scope_list.py::test_out_of_scope_omits_consensus_test_dockerfile (planned) |
| <code>acceptance.16.978.1</code> | qualification | UNRUN |
| <code>req.16978</code> | ac_1 | The OUT_OF_SCOPE list of tools/containers/check.sh omits consensus/test/docker/Dockerfile and the check still passes. |
| <code>acceptance.16.979.1</code> | req | 16979.1 |
| <code>acceptance.16.979.1</code> | scenario | Given the edited check.sh, when the test reads the OUT_OF_SCOPE list and runs tools/containers/check.sh, then the entry is absent and the script exits 0. |
| <code>acceptance.16.979.1</code> | test | tools/containers/test_out_of_scope_localnode.py::test_out_of_scope_omits_localnode_dockerfile (planned) |
| <code>acceptance.16.979.1</code> | qualification | UNRUN |
| <code>req.16979</code> | ac_1 | The OUT_OF_SCOPE list omits consensus/networks/local/localnode/Dockerfile and the check still passes. |
| <code>acceptance.16.980.1</code> | req | 16980.1 |
| <code>acceptance.16.980.1</code> | scenario | Given the edited scripts, when the test greps both and runs bash -n on each, then saturn and venus are absent, cw20_base.wasm is present and bash exits 0. |
| <code>acceptance.16.980.1</code> | test | tests/ci/test_deploy_scripts_cw20.py::test_deploy_scripts_use_cw20_base (planned) |
| <code>acceptance.16.980.1</code> | qualification | UNRUN |
| <code>req.16980</code> | ac_1 | Neither deploy script names saturn.wasm or venus.wasm and both deploy cw20_base.wasm. |
| <code>acceptance.16.981.1</code> | req | 16981.1 |
| <code>acceptance.16.981.1</code> | scenario | Given the repository tree, when the test runs the generator into a temporary directory, then the output names precompiles/perps/abi/abi.json, precompiles/pricefeed/abi/abi.json and the six spot files with addresses and the layerxagent packages. |
| <code>acceptance.16.981.1</code> | test | tools/docs/gen/test_gen_system_docs_abis.py::test_generator_lists_nested_and_embedded_abis (planned) |
| <code>acceptance.16.981.1</code> | qualification | UNRUN |
| <code>req.16981</code> | ac_1 | The generated system docs list the perps, pricefeed and six spot ABIs with their resolved addresses and the layerxagent embedded ABIs. |
| <code>acceptance.16.982.1</code> | req | 16982.1 |
| <code>acceptance.16.982.1</code> | scenario | Given the edited Makefile, when the test runs make build/tests/kernel/codec/lxp_test_codec_primitives and then the built binary, then make exits 0, the binary exists under build/tests/kernel/codec and it exits 0. |
| <code>acceptance.16.982.1</code> | test | tests/ci/test_kernel_test_rule.py::test_kernel_rule_builds_and_runs_codec_primitives (planned) |
| <code>acceptance.16.982.1</code> | qualification | UNRUN |
| <code>req.16982</code> | ac_1 | make build/tests/kernel/codec/lxp_test_codec_primitives builds tests/codec/lxp_test_codec_primitives.c through the pattern rule and the binary exits 0. |
| <code>acceptance.16.983.1</code> | req | 16983.1 |
| <code>acceptance.16.983.1</code> | scenario | Given the edited Makefile and the chain_domain.rs golden, when the test reads the PAXAI_CHAIN_DOMAIN value from make -s -p -n and the golden bytes from chain_domain.rs, then the value is 64 lowercase hex characters and equals the hex of the golden. |
| <code>acceptance.16.983.1</code> | test | tests/ci/test_paxai_chain_domain.py::test_makefile_chain_domain_is_merged_chain_golden (planned) |
| <code>acceptance.16.983.1</code> | qualification | UNRUN |
| <code>req.16983</code> | ac_1 | make -s -p -n reports PAXAI_CHAIN_DOMAIN as 64 lowercase hex characters equal to the merged_chain_domain(229) golden of programs/crates/layerx-programs-ai-market/tests/chain_domain.rs. |
| <code>acceptance.16.984.1</code> | req | 16984.1 |
| <code>acceptance.16.984.1</code> | scenario | Given the edited Makefile, when the test dry-runs both targets and reads the Makefile, then make reports no rule for either and tests/relay-archive does not occur. |
| <code>acceptance.16.984.1</code> | test | tests/ci/test_retired_make_rules.py::test_relay_archive_targets_absent (planned) |
| <code>acceptance.16.984.1</code> | qualification | UNRUN |
| <code>req.16984</code> | ac_1 | make -n relay-archive-build and make -n relay-archive-e2e fail with no rule and no Makefile line names tests/relay-archive. |
| <code>acceptance.16.984.2</code> | req | 16984.2 |
| <code>acceptance.16.984.2</code> | scenario | Given the edited rule, when the test builds and runs build/tests/programs_parallel_differential, then neither the prerequisites nor the compile line name lxp_daemon_batch_wal.c and the binary exits 0. |
| <code>acceptance.16.984.2</code> | test | tests/ci/test_retired_make_rules.py::test_parallel_differential_links_without_batch_wal (planned) |
| <code>acceptance.16.984.2</code> | qualification | UNRUN |
| <code>req.16984</code> | ac_2 | make build/tests/programs_parallel_differential builds without cmd/layerxd/lxp_daemon_batch_wal.c and the binary exits 0. |
| <code>acceptance.16.984.3</code> | req | 16984.3 |
| <code>acceptance.16.984.3</code> | scenario | Given the edited Makefile, when the test dry-runs test-state-diff and test, then the first has no rule and the second exits 0 with no lxp_test_state_diff in its output. |
| <code>acceptance.16.984.3</code> | test | tests/ci/test_retired_make_rules.py::test_state_diff_rules_absent (planned) |
| <code>acceptance.16.984.3</code> | qualification | UNRUN |
| <code>req.16984</code> | ac_3 | make -n test-state-diff fails with no rule and make -n test exits 0 without naming lxp_test_state_diff. |
