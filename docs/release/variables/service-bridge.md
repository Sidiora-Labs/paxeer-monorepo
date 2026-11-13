# Bridge service variables

The Paxeer bridge deployment and verification system under `services/bridge` deploys custody contracts and programs, generates governance proposals, checks deployed state, and exports application settings. Paxeer mainnet-beta uses chain ID 125 and serves JSON-RPC at the root path; its receipts are final when their block commits.

Required entries apply to the selected operation and chain: deployment needs its signing input, explorer verification needs its explorer key, and the checklist needs the remote and Paxeer RPC inputs plus the governance authority. `PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR` is required for `--all`, which supplies `PAXEER_BRIDGE_DEPLOYMENT_RECORD` for each child invocation. RPC URLs can contain credentials and must then be treated as secrets. Keypair variables name files containing secret material.

`config.json` denotes `contracts/bridge/evm/chains/<chain>/config.json` or `contracts/bridge/solana/chains/solana/config.json`, subject to the respective chains-root override. JSON sections in brackets are object paths; `root` is the top-level object and `[]` denotes each array item. `overlay.json` is the private file named by `BRIDGE_DEPLOY_OVERLAY`. `attestors.json` is selected by `PAXEER_BRIDGE_ATTESTOR_MANIFEST` or the default path below. `proposal-config.json` denotes the input file accepted by `paxeer-bridge-proposals` when its canonical-mode flags are absent; supplying `-authority` and `-vault` selects the canonical `config.json` schema. `app.env` denotes the file supplied to the application environment check; its listed names must match the chains present in the deployment records. These filenames identify inputs and do not prescribe a new file location.

The root `services/bridge/.env.example` lists runtime inputs and application-file names with empty values where no default exists. Export the runtime inputs before invoking a deployment script; supply the application settings file with `--check` or `PAXEER_BRIDGE_APP_ENV`. The path defaults in the example are relative to the repository root; the scripts resolve their built-in defaults from their own location. Environment names inside chain configurations or overlays can be replaced, and the scripts then read those names indirectly. The concrete names below are those selected by the committed configurations. The EVM deployment script derives their `ETH_RPC_URL` and vault constructor variables from those inputs.

## Required

| Name | Sets | Default applied by source | Secret | Source file that reads it |
| --- | --- | --- | --- | --- |
| `PAXEER_BRIDGE_ARBITRUM_DEPLOY_KEY` | Arbitrum deployment signing key | none | yes | `contracts/bridge/evm/chains/arbitrum/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_ARBITRUM_EXPLORER_KEY` | Arbitrum explorer verification key | none | yes | `contracts/bridge/evm/chains/arbitrum/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_ARBITRUM_RPC_URL` | Arbitrum JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/arbitrum/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_AVALANCHE_DEPLOY_KEY` | Avalanche deployment signing key | none | yes | `contracts/bridge/evm/chains/avalanche/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_AVALANCHE_EXPLORER_KEY` | Avalanche explorer verification key | none | yes | `contracts/bridge/evm/chains/avalanche/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_AVALANCHE_RPC_URL` | Avalanche JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/avalanche/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_BASE_DEPLOY_KEY` | Base deployment signing key | none | yes | `contracts/bridge/evm/chains/base/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_BASE_EXPLORER_KEY` | Base explorer verification key | none | yes | `contracts/bridge/evm/chains/base/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_BASE_RPC_URL` | Base JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/base/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_BNB_DEPLOY_KEY` | BNB deployment signing key | none | yes | `contracts/bridge/evm/chains/bnb/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_BNB_EXPLORER_KEY` | BNB explorer verification key | none | yes | `contracts/bridge/evm/chains/bnb/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_BNB_RPC_URL` | BNB JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/bnb/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_DEPLOYMENT_RECORD` | Deployment record path | none | no | `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh`, `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_DEPLOYMENT_RECORD_DIR` | Directory for the per-chain records used by `--all` | none | no | `services/bridge/deploy/overlay.sh` |
| `PAXEER_BRIDGE_ETHEREUM_DEPLOY_KEY` | Ethereum deployment signing key | none | yes | `contracts/bridge/evm/chains/ethereum/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_ETHEREUM_EXPLORER_KEY` | Ethereum explorer verification key | none | yes | `contracts/bridge/evm/chains/ethereum/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_ETHEREUM_RPC_URL` | Ethereum JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/ethereum/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_GOVERNANCE_AUTHORITY` | Governance authority used to generate and check bridge bodies | none | no | `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_HYPEREVM_DEPLOY_KEY` | HyperEVM deployment signing key | none | yes | `contracts/bridge/evm/chains/hyperevm/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_HYPEREVM_EXPLORER_KEY` | HyperEVM explorer verification key | none | yes | `contracts/bridge/evm/chains/hyperevm/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_HYPEREVM_RPC_URL` | HyperEVM JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/hyperevm/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_OPTIMISM_DEPLOY_KEY` | Optimism deployment signing key | none | yes | `contracts/bridge/evm/chains/optimism/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_OPTIMISM_EXPLORER_KEY` | Optimism explorer verification key | none | yes | `contracts/bridge/evm/chains/optimism/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_OPTIMISM_RPC_URL` | Optimism JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/optimism/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_PAXEER_RPC_URL` | Paxeer JSON-RPC endpoint used by the checklist | none | only if the URL carries credentials | `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_POLYGON_DEPLOY_KEY` | Polygon deployment signing key | none | yes | `contracts/bridge/evm/chains/polygon/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-evm-chain.sh` |
| `PAXEER_BRIDGE_POLYGON_EXPLORER_KEY` | Polygon explorer verification key | none | yes | `contracts/bridge/evm/chains/polygon/config.json`, `[environment] explorer_key`; read by `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_POLYGON_RPC_URL` | Polygon JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/evm/chains/polygon/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-evm-chain.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_SOLANA_ADMIN_CLI` | Solana bridge administration executable | none | no | `services/bridge/deploy/deploy-solana-program.sh` |
| `PAXEER_BRIDGE_SOLANA_KEYPAIR_FILE` | Solana publisher keypair file | none | yes | `contracts/bridge/solana/chains/solana/config.json`, `[environment] deploy_key`; read by `services/bridge/deploy/deploy-solana-program.sh` |
| `PAXEER_BRIDGE_SOLANA_RPC_URL` | Solana JSON-RPC endpoint | none | only if the URL carries credentials | `contracts/bridge/solana/chains/solana/config.json`, `[environment] rpc_url`; read by `services/bridge/deploy/deploy-solana-program.sh` and `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_SOLANA_TOOLCHAIN_BIN` | Pinned Solana toolchain directory | none | no | `contracts/bridge/solana/chains/solana/config.json`, `[environment] toolchain_bin`; read by `services/bridge/deploy/deploy-solana-program.sh` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_SOLANA_PROGRAM_ID` | Deployed Solana program identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_ARBITRUM` | Deployed Arbitrum vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_AVALANCHE` | Deployed Avalanche vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_BASE` | Deployed Base vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_BNB` | Deployed BNB vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_ETHEREUM` | Deployed Ethereum vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_HYPEREVM` | Deployed HyperEVM vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_OPTIMISM` | Deployed Optimism vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `app.env [root] NEXT_PUBLIC_BRIDGE_VAULT_POLYGON` | Deployed Polygon vault identity | none | no | `services/bridge/deploy/appenv/appenv.go` |
| `attestors.json [root] attestors` | Approved public signer set; 1–64 entries in ascending order | none | no | `services/bridge/deploy/check-attestor-manifest.py`, `services/bridge/deploy/proposals/manifest.go` |
| `attestors.json [root] threshold` | Required signatures; between 1 and the signer count | none | no | `services/bridge/deploy/check-attestor-manifest.py`, `services/bridge/deploy/proposals/manifest.go` |
| `config.json [assets[]] address` | Asset address or mint | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [assets[]] asset_id` | Bridge asset identifier | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [assets[]] decimals` | Asset precision | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [assets[]] per_tx_cap` | Per-transaction asset cap | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [assets[]] symbol` | Asset symbol | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [assets[]] total_cap` | Total custody cap | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [big_blocks] acknowledged` | Big-block deployment acknowledgement (HyperEVM only) | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh` |
| `config.json [big_blocks] required` | Whether big blocks are required (HyperEVM only) | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh` |
| `config.json [big_blocks] requirement` | Big-block requirement text (HyperEVM only) | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh` |
| `config.json [environment] deploy_key` | Name of the deployment key variable | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [environment] explorer_key` | Name of the explorer key variable (EVM only) | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/verify-evm-chain.sh` |
| `config.json [environment] rpc_url` | Name of the chain RPC variable | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [environment] toolchain_bin` | Name of the Solana toolchain variable (Solana only) | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [native] decimals` | Native asset precision | none | no | `services/bridge/deploy/chainconfig/config.go` |
| `config.json [native] symbol` | Native asset symbol | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh` |
| `config.json [root] assets` | Ordered asset list; native coin first | none | no | `services/bridge/deploy/chainconfig/config.go` |
| `config.json [root] attestors` | Ordered attestor set | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [root] big_blocks` | Big-block deployment settings (HyperEVM only) | none | no | `services/bridge/deploy/chainconfig/config.go` |
| `config.json [root] chain` | Chain name | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh` |
| `config.json [root] chain_id` | Destination chain identifier | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [root] environment` | Names of the environment variables carrying runtime inputs | none | no | `services/bridge/deploy/chainconfig/config.go` |
| `config.json [root] finality_depth` | Confirmation or slot depth | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [root] kind` | EVM or Solana chain kind | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [root] native` | Native coin metadata | none | no | `services/bridge/deploy/chainconfig/config.go` |
| `config.json [root] owner` | Custody owner | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [root] solana` | Custody program settings (Solana only) | none | no | `services/bridge/deploy/chainconfig/config.go` |
| `config.json [root] threshold` | Required attestor signatures | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [solana] commitment` | Solana read commitment (Solana only) | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `config.json [solana] program_id` | Solana custody program identity (Solana only) | none | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `proposal-config.json [assets[]] address` | Asset address or mint | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [assets[]] asset_id` | Bridge asset identifier | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [assets[]] decimals` | Asset precision | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [assets[]] max_per_tx` | Per-transaction cap in base units | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [assets[]] max_total` | Total custody cap in base units | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] assets` | Ordered remote asset list; native asset first | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] attestors` | Approved public signer set | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] chain_id` | Remote chain identifier | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] finality_depth` | Remote chain confirmation depth | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] governance_authority` | Governance authority for generated messages | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] owner` | Remote custody owner | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] threshold` | Required signature count | none | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] vault` | Deployed custody vault or Solana authority handle | none | no | `services/bridge/deploy/proposals/proposals.go` |

## Optional

| Name | Sets | Default applied by source | Secret | Source file that reads it |
| --- | --- | --- | --- | --- |
| `BRIDGE_DEPLOY_OVERLAY` | Private deployment overlay path | empty; committed configuration is used | no | `services/bridge/deploy/overlay.sh`, `services/bridge/deploy/chainconfig/config.go` |
| `PAXEER_BRIDGE_APP_ENV` | Application environment file checked against deployment records | empty | no | `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_ATTESTOR_MANIFEST` | Approved attestor manifest path | `services/bridge/deploy/attestors.json` | no | `services/bridge/deploy/check-attestor-manifest.py`, `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_EVM_CHAINS_ROOT` | EVM chain configuration root | `contracts/bridge/evm/chains` | no | `services/bridge/deploy/deploy-evm-chain.sh`, `services/bridge/deploy/checklist.sh`, `services/bridge/deploy/verify-evm-chain.sh` |
| `PAXEER_BRIDGE_SOLANA_CHAINS_ROOT` | Solana chain configuration root | `contracts/bridge/solana/chains` | no | `services/bridge/deploy/deploy-solana-program.sh`, `services/bridge/deploy/checklist.sh` |
| `PAXEER_BRIDGE_SOLANA_EXECUTABLE_WAIT_SECONDS` | Maximum wait for the deployed Solana program to become executable | `120` | no | `services/bridge/deploy/deploy-solana-program.sh` |
| `PAXEER_BRIDGE_SOLANA_PROGRAM_KEYPAIR_FILE` | Solana program keypair; required when the configured program identity is filled | generated beside the record on first deployment, with `-program-keypair.json` replacing `.json` | yes | `services/bridge/deploy/deploy-solana-program.sh` |
| `attestors.json [root] pending_governance` | Approval marker; its presence prevents deployment | absent | no | `services/bridge/deploy/check-attestor-manifest.py` |
| `config.json [root] deployer` | Optional configured deployer identity | empty | no | `services/bridge/deploy/chainconfig/config.go`, `services/bridge/deploy/deploy-evm-chain.sh` |
| `overlay.json [chains.<chain>.caps.<symbol>] per_tx_cap` | Per-transaction cap override in base units | both caps required for a supplied symbol | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>.caps.<symbol>] total_cap` | Total custody cap override in base units | both caps required for a supplied symbol | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>.environment] deploy_key` | Deployment key variable name | no field fallback when environment is replaced | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>.environment] explorer_key` | EVM explorer key variable name | no field fallback when environment is replaced | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>.environment] rpc_url` | RPC variable name | no field fallback when environment is replaced | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>.environment] toolchain_bin` | Solana toolchain variable name | no field fallback when environment is replaced | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] attestors` | Ordered public signer set override | committed set | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] big_blocks_acknowledged` | Acknowledge the HyperEVM big-block requirement | false; committed acknowledgement retained | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] caps` | Asset cap overrides, indexed by symbol | empty map | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] deployer` | Deployment signer identity override | committed deployer | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] environment` | Replace the environment variable-name mapping | committed mapping | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] owner` | Custody owner override | committed owner | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] program_id` | Solana custody program identity override | committed program identity | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [chains.<chain>] threshold` | Attestor signature threshold override | committed threshold | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `overlay.json [root] chains` | Per-chain deployment overrides | empty map | no | `services/bridge/deploy/chainconfig/overlay.go`, `services/bridge/deploy/overlay.jq` |
| `proposal-config.json [root] big_blocks_required` | Big-block requirement metadata | false | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] commitment` | Solana commitment metadata | empty | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] explorer_key_env` | Explorer key variable-name metadata | empty | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] name` | Chain display name in generated output | empty | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] native_decimals` | Native precision metadata | 0 | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] native_symbol` | Native symbol metadata | empty | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] program_id` | Solana program identity metadata | empty | no | `services/bridge/deploy/proposals/proposals.go` |
| `proposal-config.json [root] rpc_endpoint_env` | RPC variable-name metadata | empty | no | `services/bridge/deploy/proposals/proposals.go` |
