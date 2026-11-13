# Paxeer 7.0.0 release files

This index covers the files used to prepare and operate release `paxeer-network/v7.0.0` and the governance upgrade `v7.0`. Paxeer uses one BFT network with EVM chain ID 125; the kernel executes inside `paxd` and receipts become final when their blocks commit. Commands run from the repository root unless a page says otherwise. Deployment values belong in the operator’s private configuration.

## Files

### Pages and inventory

| File | Who uses it | Purpose |
| --- | --- | --- |
| [7.0.0-systems.md](7.0.0-systems.md) | All operators | Find systems, directories, upgrade changes and health checks. |
| [7.0.0-systems.kvx](7.0.0-systems.kvx) | Operators and inventory consumers | Read structured build, output, configuration and dependency information. |
| [7.0.0-variables.md](7.0.0-variables.md) | All operators | Select each system’s required and optional configuration inputs. |
| [7.0.0-upgrade-plan.md](7.0.0-upgrade-plan.md) | Release coordinators and validators | Schedule the height, roles, go/no-go decisions and service return. |
| [7.0.0-release-notes.md](7.0.0-release-notes.md) | Operators, developers and users | Read activation behavior, supported interfaces and client changes. |
| [7.0.0-readiness-matrix.md](7.0.0-readiness-matrix.md) | Release maintainers | Find build and test entry points, release identities and operational checks. |
| [7.0.0-candidate-report.md](7.0.0-candidate-report.md) | Release maintainers | Find activation, receipt, retirement, parity and artifact checks. |
| [7.0.0-readiness-checklist.md](7.0.0-readiness-checklist.md) | Release coordinators and operators | Check release, governance, host and service readiness and acceptance. |
| [7.0.0-proposal.md](7.0.0-proposal.md) | Proposers and validators | Choose the height, generate plan info, submit, vote and query governance. |
| [7.0.0-proposal.example.json](7.0.0-proposal.example.json) | Proposers | Inspect an unsigned upgrade transaction; replace placeholders before use. |
| [7.0.0-validator-runbook.md](7.0.0-validator-runbook.md) | Validator and RPC node operators | Prepare the host, verify the release, halt, back up, swap and check the node. |
| [7.0.0-service-operator-runbook.md](7.0.0-service-operator-runbook.md) | Service and application operators | Stage configuration, pause writes and restore services in dependency order. |
| [7.0.0-rollback.md](7.0.0-rollback.md) | Validator and recovery operators | Cancel before the height, preserve stopped state and recover after activation. |
| [consensus-commits.md](consensus-commits.md) | Release maintainers | Review the consensus-changing commit inventory. |
| [../../RELEASE.md](../../RELEASE.md) | Release maintainers and node operators | Build and identify the release assets. |
| [../../tools/release/README.md](../../tools/release/README.md) | Release and node operators | Read release-tool inputs, layout and commands. |
| [../runbooks/paxeer-x-fork.md](../runbooks/paxeer-x-fork.md) | Validator and RPC node operators | Follow detailed host backup, signing-state and recovery procedures. |

### System variables

Use the page for each deployed component before preparing its configuration. Required settings can depend on the selected command, mode or feature; an empty value and an unset value can have different meanings.

| File | Who uses it | Purpose |
| --- | --- | --- |
| [variables/app-bridge.md](variables/app-bridge.md) | Bridge application operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/app-explorer.md](variables/app-explorer.md) | Explorer application operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/app-inference-peer.md](variables/app-inference-peer.md) | Inference peer operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/app-launchpad.md](variables/app-launchpad.md) | Launchpad application operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/app-names.md](variables/app-names.md) | Paxeer Names shell operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/app-perps.md](variables/app-perps.md) | Perps application operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/app-wallet.md](variables/app-wallet.md) | Wallet application operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/app-web.md](variables/app-web.md) | Web application operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/chain-node.md](variables/chain-node.md) | Chain node operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/operations.md](variables/operations.md) | Backup and monitoring operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-agent.md](variables/service-agent.md) | Agent service operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-bridge.md](variables/service-bridge.md) | Bridge service operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-explorer.md](variables/service-explorer.md) | Explorer service operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-human.md](variables/service-human.md) | Human service operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-interop.md](variables/service-interop.md) | Interop service operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-launchpad.md](variables/service-launchpad.md) | Launchpad service operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-platform.md](variables/service-platform.md) | Platform service operators | Configure required and optional inputs, defaults and feature dependencies. |
| [variables/service-wallet.md](variables/service-wallet.md) | Wallet service operators | Configure required and optional inputs, defaults and feature dependencies. |

### Configuration examples

Fill a private copy of the example for the selected component and supply it through the launcher described on its variable page. Component-specific examples and configuration-file schemas are described on those pages.

| File | Who uses it | Purpose |
| --- | --- | --- |
| [../../node/.env.example](../../node/.env.example) | Node operators | Prepare node environment inputs; TOML settings are described in the node variable page. |
| [../../services/agent/.env.example](../../services/agent/.env.example) | Agent service operators | Select inputs for the service or role being started. |
| [../../services/bridge/.env.example](../../services/bridge/.env.example) | Bridge service operators | Select inputs for the service or role being started. |
| [../../services/explorer/.env.example](../../services/explorer/.env.example) | Explorer service operators | Select inputs for the service or role being started. |
| [../../services/human/.env.example](../../services/human/.env.example) | Human service operators | Select inputs for the service or role being started. |
| [../../services/interop/.env.example](../../services/interop/.env.example) | Interop service operators | Select inputs for the service or role being started. |
| [../../services/launchpad/.env.example](../../services/launchpad/.env.example) | Launchpad service operators | Select inputs for the service or role being started. |
| [../../services/platform/.env.example](../../services/platform/.env.example) | Platform service operators | Select inputs for the service or role being started. |
| [../../services/wallet/.env.example](../../services/wallet/.env.example) | Wallet service operators | Select inputs for the service or role being started. |
| [../../apps/bridge/.env.example](../../apps/bridge/.env.example) | Bridge application operators | Supply application and build inputs for the selected deployment. |
| [../../apps/explorer/.env.example](../../apps/explorer/.env.example) | Explorer application operators | Supply application and build inputs for the selected deployment. |
| [../../apps/inference-peer/.env.example](../../apps/inference-peer/.env.example) | Inference-peer application operators | Supply application and build inputs for the selected deployment. |
| [../../apps/launchpad/.env.example](../../apps/launchpad/.env.example) | Launchpad application operators | Supply application and build inputs for the selected deployment. |
| [../../apps/names/.env.example](../../apps/names/.env.example) | Names application operators | Supply application and build inputs for the selected deployment. |
| [../../apps/perps/.env.example](../../apps/perps/.env.example) | Perps application operators | Supply application and build inputs for the selected deployment. |
| [../../apps/wallet/.env.example](../../apps/wallet/.env.example) | Wallet application operators | Supply application and build inputs for the selected deployment. |
| [../../apps/web/.env.example](../../apps/web/.env.example) | Web application operators | Supply application and build inputs for the selected deployment. |
| [../../tools/ops/.env.example](../../tools/ops/.env.example) | Operations teams | Supply keyring migration inputs. |
| [../../tools/ops/backup/backup.env.example](../../tools/ops/backup/backup.env.example) | Backup operators | Supply backup and restore inputs. |
| [../../tools/ops/monitoring/hosts.env.example](../../tools/ops/monitoring/hosts.env.example) | Monitoring operators | Supply monitoring target inputs. |
| [../../tools/release/env/rollout.env.example](../../tools/release/env/rollout.env.example) | Release operators | Supply release identity, homes, units and fleet inputs; use the height convention in the rollback page. |
| [../../tools/release/env/rehearsal.env.example](../../tools/release/env/rehearsal.env.example) | Rehearsal operators | Supply source state, binaries, local homes and sidecar inputs. |
| [../../tools/release/upgradeinfo/merge-plan-info.example.json](../../tools/release/upgradeinfo/merge-plan-info.example.json) | Release metadata maintainers | Inspect sample sequencer plan information. |

### Release scripts and inputs

| File | Who uses it | Purpose |
| --- | --- | --- |
| [../../tools/release/build-paxd.sh](../../tools/release/build-paxd.sh) | Release builders | Build the node, price sidecar, runtime libraries, metadata and checksums. |
| [../../tools/release/extld.sh](../../tools/release/extld.sh) | Release builders | Provide the external linker and relocatable Wasm runtime library paths. |
| [../../tools/release/reproducible-check.sh](../../tools/release/reproducible-check.sh) | Release maintainers | Compare independent builds and their checksums. |
| [../../tools/release/verify-release.sh](../../tools/release/verify-release.sh) | Release maintainers and node operators | Compare published assets with a local release directory. |
| [../../tools/release/upgrade_info.py](../../tools/release/upgrade_info.py) | Proposers and node operators | Generate and validate release, sequencer plan and node upgrade metadata. |
| [../../tools/release/preflight.sh](../../tools/release/preflight.sh) | Validator and RPC node operators | Check the binary, staged layout, configuration, key permissions, disk and clock before the height. |
| [../../tools/release/postflight.sh](../../tools/release/postflight.sh) | Validator, RPC and service operators | Read node version, advancing heights, chain ID, module state, precompiles and RPC namespaces after the height. |
| [../../tools/release/stage.sh](../../tools/release/stage.sh) | Release operators | Stage the new binary beside the installed binary and retain the earlier executable. |
| [../../tools/release/snapshot.sh](../../tools/release/snapshot.sh) | Node operators | Archive and verify stopped node data using the script halt-height convention. |
| [../../tools/release/swap.sh](../../tools/release/swap.sh) | Node operators | Validate and install the staged binary, write upgrade metadata and start selected homes. |
| [../../tools/release/rollback.sh](../../tools/release/rollback.sh) | Recovery operators | Check retained material, recover forward or restore an isolated fork under the documented conditions. |
| [../../tools/release/fleet_check.py](../../tools/release/fleet_check.py) | Fleet operators | Validate release inputs and check node, market and service readiness. |
| [../../tools/release/feed_key.py](../../tools/release/feed_key.py) | Price-feed operators | Generate price-feed key material and registration calldata. |
| [../../tools/release/rehearse-v6120.sh](../../tools/release/rehearse-v6120.sh) | Rehearsal operators | Run the local four-validator rehearsal with the plan selected by release metadata and supplied plan info. |
| [../../tools/release/rehearsal/rewrite_genesis.py](../../tools/release/rehearsal/rewrite_genesis.py) | Rehearsal operators | Prepare genesis for the local rehearsal network. |
| [../../tools/release/rehearsal/programme.py](../../tools/release/rehearsal/programme.py) | Rehearsal operators | Exercise native operations, receipts and refusal comparisons. |
| [../../tools/release/rehearsal/pool_parity.py](../../tools/release/rehearsal/pool_parity.py) | Launchpad and rehearsal operators | Record pool state and compare it after activation. |
| [../../tools/release/rehearsal/receipt.sh](../../tools/release/rehearsal/receipt.sh) | Rehearsal operators | Write rehearsal receipts and handoff inputs. |
| [../../tools/release/version.json](../../tools/release/version.json) | Release builders and operators | Read the release version and upgrade plan name. |
| [../../tools/release/toolchain.env](../../tools/release/toolchain.env) | Release builders | Select the release C/C++ compiler version. |
| [../../tools/release/fleet-views.json](../../tools/release/fleet-views.json) | Fleet operators | Select precompile views and pointer checks for fleet verification. |
| [../../tools/release/rehearsal/share.golden.json](../../tools/release/rehearsal/share.golden.json) | Rehearsal operators | Read the expected share calculation for programme comparisons. |

## Order of use

1. **Prepare the host and inventory.** Select deployed systems in [7.0.0-systems.md](7.0.0-systems.md) and [7.0.0-systems.kvx](7.0.0-systems.kvx). Use [7.0.0-variables.md](7.0.0-variables.md), [chain-node.md](variables/chain-node.md) and [operations.md](variables/operations.md) to prepare configuration, storage, backup capacity and monitoring. Follow the host preparation in [7.0.0-validator-runbook.md](7.0.0-validator-runbook.md).
2. **Review and identify the release.** Read [7.0.0-release-notes.md](7.0.0-release-notes.md), [7.0.0-readiness-matrix.md](7.0.0-readiness-matrix.md), [7.0.0-candidate-report.md](7.0.0-candidate-report.md) and [7.0.0-readiness-checklist.md](7.0.0-readiness-checklist.md). Build with [build-paxd.sh](../../tools/release/build-paxd.sh), compare independent builds with [reproducible-check.sh](../../tools/release/reproducible-check.sh), and check published assets with [verify-release.sh](../../tools/release/verify-release.sh).
3. **Schedule and review the proposal.** Use [7.0.0-upgrade-plan.md](7.0.0-upgrade-plan.md) and [7.0.0-proposal.md](7.0.0-proposal.md). Replace the placeholders in [7.0.0-proposal.example.json](7.0.0-proposal.example.json), generate and validate plan info with [upgrade_info.py](../../tools/release/upgrade_info.py), and compare the passed plan’s name, height `H` and receipt-key coverage with the reviewed values.
4. **Stage node and service configuration.** Follow [7.0.0-validator-runbook.md](7.0.0-validator-runbook.md) for the verified binary, libraries and sequencing configuration. Follow [7.0.0-service-operator-runbook.md](7.0.0-service-operator-runbook.md) and its system variable pages for service configuration. Keep the retained binary and stopped-backup plan described in [7.0.0-rollback.md](7.0.0-rollback.md). Script-managed installations use [stage.sh](../../tools/release/stage.sh) and [rollout.env.example](../../tools/release/env/rollout.env.example).
5. **Check before the height.** After staging, run [preflight.sh](../../tools/release/preflight.sh) with the inputs shown in [7.0.0-validator-runbook.md](7.0.0-validator-runbook.md#check-before-the-height). Complete the preparation items in [7.0.0-readiness-checklist.md](7.0.0-readiness-checklist.md) before the go/no-go decisions in [7.0.0-upgrade-plan.md](7.0.0-upgrade-plan.md).
6. **Pause, halt, back up and replace.** Pause service writes using [7.0.0-service-operator-runbook.md](7.0.0-service-operator-runbook.md). At `H`, follow the stopped-node backup and manual swap in [7.0.0-validator-runbook.md](7.0.0-validator-runbook.md). For script-managed snapshots and swaps, consult [snapshot.sh](../../tools/release/snapshot.sh), [swap.sh](../../tools/release/swap.sh) and the `HALT_HEIGHT = H-1` convention in [7.0.0-rollback.md](7.0.0-rollback.md#at-the-upgrade-height).
7. **Check after the height.** Once blocks advance above `H`, run [postflight.sh](../../tools/release/postflight.sh) as shown in [7.0.0-validator-runbook.md](7.0.0-validator-runbook.md#7-post-upgrade-verification). Confirm the applied plan, governed keys, committed state and service acceptance using [7.0.0-readiness-checklist.md](7.0.0-readiness-checklist.md#after-s-acceptance).
8. **Return services in order.** After `S = H+1` commits and node checks pass, follow [7.0.0-service-operator-runbook.md](7.0.0-service-operator-runbook.md): gateway, explorer, agent, bridge, launchpad and wallet, with each service’s checks before reopening its traffic.
9. **Recover when required.** At the relevant stage, use [7.0.0-rollback.md](7.0.0-rollback.md) for cancellation before `H`, state preservation at `H`, or forward recovery after commitment. [rollback.sh](../../tools/release/rollback.sh) refuses same-chain snapshot restoration. Repeat node and service checks before returning traffic.
