# Interop service variables

The interoperability services in `services/interop` provide the adapter gateway, bridge relayer, gas station, mirror publisher and signer, migration client, and web search sidecar. They operate with the merged Paxeer network; EVM chain id 125 identifies Paxeer and a committed block is final.

Required entries have no source default and are required by the named executable, selected mode or configuration section. Optional entries have a default or enable an optional feature. Nested fields remain required when their optional parent is configured. A default shown in a deployment template is not a default applied by the service loader. File paths in the name column identify configuration inputs; `::` separates the file from its section and key, `[]` denotes an array element, and angle brackets denote a configured name. Names selected by `relayer_key_env` and `rate_owner_key_env` can vary; the examples use `GAS_STATION_RELAYER_KEY` and `GAS_STATION_RATE_OWNER_KEY`.

The inventory includes the container entrypoints referenced by the deployment files. Secret-file rows identify paths, never file contents. Public key and account fields below are names only. Endpoint and identity values are supplied by the deployment. JSON file names selected by command-line options are descriptive; their actual paths are supplied to those options. Mirror publisher configuration is a positional file argument; verifier configuration is also a positional file argument. TLS signer fields apply to the selected `uds` or `mutual_tls` variant, and verifier source fields apply to the selected `ethereum` or `solana` variant.

The environment examples list required inputs with empty values when the source has no default. Optional inputs without literal defaults are commented out, since an empty value can differ from an unset variable. Select one service or role from the root example before loading it. The gas station endpoint default is left unset in examples so the source selects its published node root without copying a host value here.

## Required

| Name / file :: section.key | What it sets | Source default | Secret | Source reader |
| --- | --- | --- | --- | --- |
| `BRIDGE_APPROVED_MEMBERSHIP` | Base64 file content decoded to /run/secrets/bridge/cosign/membership.json | None | No | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_COSIGN_CERTIFICATE` | Base64 file content decoded to /run/secrets/bridge/cosign/certificate.pem | None | No | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_COSIGN_PRIVATE_KEY` | Base64 file content decoded to /run/secrets/bridge/cosign/private-key.pem | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_COSIGN_TRANSPORT_CONFIG` | Base64 file content decoded to /run/secrets/bridge/cosign/transport.json | None | No | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_COSIGN_TRUST_ANCHOR` | Base64 file content decoded to /run/secrets/bridge/cosign/ca.pem | None | No | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_OPERATOR_INVENTORY` | Base64 file content decoded to /run/secrets/bridge/cosign/operators.json | None | No | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_RELAYER_CONFIG` | Base64 file content decoded to /run/secrets/bridge/relayer.json | None | No | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_RELAYER_RPC_TOKENS` | Base64 file content decoded to /run/secrets/bridge/rpc-tokens | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_ARBITRUM_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/arbitrum-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_ATTESTOR_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/attestor-1.key; selected by the container role | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_AVALANCHE_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/avalanche-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_BASE_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/base-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_BNB_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/bnb-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_ETHEREUM_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/ethereum-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_HYPEREVM_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/hyperevm-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_KEYS` | Base64 file content decoded to /etc/layerx/bridge-signer/keys.json | None | No | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_OPTIMISM_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/optimism-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_PAXEER_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/paxeer-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_POLYGON_SUBMITTER_KEY` | Base64 file content decoded to /run/secrets/bridge/signer/polygon-submitter.key | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `BRIDGE_SIGNER_SOLANA_FEE_PAYER_KEYPAIR` | Base64 file content decoded to /run/secrets/bridge/signer/solana-fee-payer.json | None | Yes | `docker/common/env-files.sh`; `docker/bridge-relayer/files.tsv` |
| `GAS_STATION_BALANCE_FLOOR` | Sponsor balance floor in PAX base units | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_CHAIN_ID` | Chain identifier | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_DECIMALS` | Fee token decimal exponent | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_DID_GAS_LIMIT` | Positive gas budget per DID per UTC day | None | No | `services/interop/crates/layerx-gas-station/src/station.rs` |
| `GAS_STATION_GAS_LIMIT` | Sponsored transaction gas limit | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_INTERVAL_SECONDS` | Policy interval in seconds | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_LISTEN` | Listener socket address and port | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_MARGIN_BPS` | Quote margin in basis points | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_MAX_PRIORITY_FEE_PER_GAS` | Transaction priority fee per gas in wei | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_MAX_RATE_AGE` | Maximum quotable rate age in seconds, at most 300 | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_PAYMASTER` | Paymaster contract identity; must equal the JSON configuration | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `GAS_STATION_PER_ACCOUNT_LIMIT` | Per-account sponsorship limit per interval | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_PER_INTERVAL_LIMIT` | Total sponsorship limit per interval | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_PER_QUOTE_LIMIT` | Maximum amount of one quote | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RATE_BALANCE_FLOOR` | Rate publisher balance floor in PAX base units | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RATE_CADENCE_SECONDS` | Rate publication cadence in seconds | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RATE_CONFIRMATION_RETRY_SECONDS` | Rate confirmation retry interval in seconds | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RATE_GAS_BUDGET_PER_DAY` | Rate publication gas budget per chain day | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RATE_MAX_FEE_PER_GAS` | Rate publication fee ceiling per gas in wei | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RATE_OWNER_KEY` | Paymaster owner secp256k1 secret; name is selected by rate_owner_key_env | None | Yes | `services/interop/crates/layerx-gas-station/src/signer.rs` |
| `GAS_STATION_RATE_OWNER_KEY_ENV` | Environment variable name containing the distinct owner secret | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RELAYER_KEY` | Sponsor secp256k1 secret; name is selected by relayer_key_env | None | Yes | `services/interop/crates/layerx-gas-station/src/signer.rs` |
| `GAS_STATION_RELAYER_KEY_ENV` | Environment variable name containing the sponsor secret | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_SPREAD_BPS` | Quote spread in basis points | None | No | `docker/gas-station/init.sh` |
| `GAS_STATION_TOKEN` | Fee token contract identity | None | No | `docker/gas-station/init.sh` |
| `INTEROP_AUTHORITY_TOKEN` | Base64 file content decoded to /run/layerx/authority/token | None | Yes | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_CLIENT_CA_DER` | Base64 file content decoded to /run/layerx/trust/ca.crt.der | None | No | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_CLIENT_P12` | Base64 file content decoded to /run/layerx/client/client.p12 | None | Yes | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_CLIENT_PASSWORD` | Base64 file content decoded to /run/layerx/client/password | None | Yes | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_CONFIG` | Base64 file content decoded to /run/layerx/interop/config.json | None | No | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_MODULE_REGISTRY` | Base64 file content decoded to /run/layerx/interop/registry.json | None | No | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_REDIS_PASSWORD` | Base64 file content decoded to /run/layerx/redis/password | None | Yes | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_REDIS_USERNAME` | Base64 file content decoded to /run/layerx/redis/username | None | Yes | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_SEQUENCER_FIRST_BATCH` | Base64 file content decoded to /run/layerx/authority/sequencer-first-batch | None | No | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_SEQUENCER_ID` | Base64 file content decoded to /run/layerx/authority/sequencer-id | None | No | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_SEQUENCER_LAST_BATCH` | Base64 file content decoded to /run/layerx/authority/sequencer-last-batch | None | No | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `INTEROP_SEQUENCER_PUBLIC_KEY` | Base64 file content decoded to /run/layerx/authority/sequencer-public-key | None | No | `docker/common/env-files.sh`; `docker/interop-gateway/files.tsv` |
| `LAYERX_BRIDGE_APPROVED_MEMBERSHIP` | Approved attestor membership JSON file | None | No | `docker/bridge-relayer/entrypoint.sh` |
| `LAYERX_BRIDGE_ATTESTOR_INDEX` | Positive local attestor index | None | No | `docker/bridge-relayer/entrypoint.sh` |
| `LAYERX_BRIDGE_ATTESTOR_KEY_FILE` | Local attestor secret file path matched to the index | None | Path to secret | `docker/bridge-relayer/entrypoint.sh` |
| `LAYERX_BRIDGE_ATTESTOR_SET_SIZE` | Number of distinct attestors | None | No | `docker/bridge-relayer/entrypoint.sh` |
| `LAYERX_BRIDGE_ATTESTOR_THRESHOLD` | Majority signature threshold | None | No | `docker/bridge-relayer/entrypoint.sh` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG` | Cosign TLS transport JSON file; required by the cosign executable | None | No | `services/interop/crates/layerx-bridge-relayer/src/bin/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: attestor` | Local attestor identity or signer policy | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: certificate` | Cosign TLS certificate file | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: cosign_directory` | Persistent cosign share directory | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: delivery_directory` | Persistent peer delivery journal directory | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: listen` | Listener socket address and port | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: max_connections` | Maximum concurrent cosign connections | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: peers` | Peer endpoint list | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: peers[].address` | Registered sender or cosign peer address | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: peers[].attestor` | Local attestor identity or signer policy | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: peers[].server_name` | TLS peer name | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: peers[].spki_sha256` | Pinned TLS SubjectPublicKeyInfo digest | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: private_key` | TLS private key file | None | Path to secret | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: server_name` | TLS peer name | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: sweep_interval_ms` | Cosign delivery sweep interval in milliseconds | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: timeout_ms` | Signer or cosign operation timeout in milliseconds | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_COSIGN_TRANSPORT_CONFIG JSON :: trust_anchor` | Pinned TLS CA certificate file | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: approval` | Operator inventory approval identifier | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: authority` | Target or inventory authority identifier | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: destinations` | Destinations | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: destinations[].attestors` | Authorized attestor identities | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: destinations[].chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: destinations[].observed_block_hash` | Observed block hash | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: destinations[].threshold` | Required attestor signature count | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: destinations[].vault` | Vault | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: membership` | Approved membership JSON file | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators` | Operators | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].attestor` | Local attestor identity or signer policy | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].cosign_directory` | Persistent cosign share directory | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].delivery_directory` | Persistent peer delivery journal directory | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].fee_payers` | Fee payers | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].fee_payers[].chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].fee_payers[].handle` | Remote signer policy handle | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].fee_payers[].public_key` | Public verification key pin | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].instance` | Instance | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].journal_path` | Persistent relayer journal file | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].signer_handle` | Signer handle | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].signer_public_key` | Signer public key | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].spki_sha256` | Pinned TLS SubjectPublicKeyInfo digest | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: operators[].storage` | Independent operator storage identity | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY JSON :: version` | Protocol or schema version | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: checkout` | Checkout | None | No | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: checkout.issuer` | AP2 receipt issuer identifier | None | No | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: checkout.key_id` | Verification key identifier | None | No | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: checkout.private_key_hex` | AP2 receipt signing private key in hexadecimal | None | Yes | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: payment` | Metered payment policy; every nested field is optional | None | No | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: payment.issuer` | AP2 receipt issuer identifier | None | No | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: payment.key_id` | Verification key identifier | None | No | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE JSON :: payment.private_key_hex` | AP2 receipt signing private key in hexadecimal | None | Yes | `services/interop/crates/layerx-interop-service/src/ap2_export.rs` |
| `LAYERX_INTEROP_CHAIN_ID` | EVM chain identifier; must be 125 | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CLIENT_IDENTITY_PASSWORD_FILE` | File containing the outbound PKCS#12 identity password | None | Path to secret | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CLIENT_IDENTITY_PKCS12` | Outbound client identity PKCS#12 file | None | Path to secret | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG` | JSON adapter and transport manifest file | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters` | Adapters | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].conformance_sha256` | Conformance vector digest | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].conformance_suite` | Conformance suite identifier or payment exchange directory | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].conformance_vectors` | Number of conformance vectors | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].evidence_policy` | Evidence required by the adapter | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].id` | Id | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].specification` | Specification identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].specification_sha256` | Pinned specification digest | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: adapters[].version` | Protocol or schema version | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets` | Ap2 assets | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].asset` | Asset binding | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].atomic_units_per_minor_unit` | Asset atomic units represented by one currency minor unit | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].audience` | AP2 mandate audience origin | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].currency` | AP2 currency code | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].minor_unit_exponent` | Currency minor-unit decimal exponent | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].payee_account` | Payee account binding | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].payee_merchant_id` | Payee merchant identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].payee_merchant_name` | Payee merchant display name | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].payer_account` | Payer account binding | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].principal_digest` | Principal identity digest | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_keys` | Ap2 keys | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_keys[].key_id` | Verification key identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_keys[].public_key_sec1` | Public key sec1 | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_keys[].use_case` | Use case | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: fiat_providers` | Fiat providers | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: fiat_providers[].provider` | Provider | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: fiat_providers[].public_key_ed25519` | Public key ed25519 | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: transports` | Transports | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: transports[].conformance_sha256` | Conformance vector digest | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: transports[].id` | Id | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: transports[].specification_sha256` | Pinned specification digest | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: transports[].version` | Protocol or schema version | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ucp_payment_handler` | Ucp payment handler | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ucp_payment_handler.id` | Id | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ucp_payment_handler.schema` | Schema | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ucp_payment_handler.spec` | Spec | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ucp_payment_handler.version` | Protocol or schema version | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents` | Visa agents | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].agent_domain` | Agent domain | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].agent_id` | Agent id | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].algorithm` | Signing algorithm identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].expires_at` | Trusted agent expiry timestamp | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].key_id` | Verification key identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].layerx_agent` | Layerx agent | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].public_key` | Public verification key pin | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_agents[].status` | Trusted agent status | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_targets` | Visa targets | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_targets[].authority` | Target or inventory authority identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_targets[].path` | Path | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: visa_targets[].principal_digest` | Principal identity digest | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported` | X402 supported | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.extensions` | Supported x402 extension identifiers | None | No | `services/interop/crates/layerx-x402/src/facilitator.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.kinds` | Supported scheme, network and version declarations | None | No | `services/interop/crates/layerx-x402/src/facilitator.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.kinds[].network` | CAIP-2 network binding | None | No | `services/interop/crates/layerx-x402/src/facilitator.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.kinds[].scheme` | Payment scheme; exact | None | No | `services/interop/crates/layerx-x402/src/facilitator.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.kinds[].x402Version` | x402 wire version; must be 2 | None | No | `services/interop/crates/layerx-x402/src/facilitator.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.signers` | Network-to-receipt-authority DID mapping | None | No | `services/interop/crates/layerx-x402/src/facilitator.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.signers.<network>` | Receipt authority DIDs admitted for the configured CAIP-2 network | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_LISTEN` | Listener socket address and port | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.chain_id` | Expected chain identifier; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody` | Custody; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.custody_reference` | Custody reference; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.custody_reference.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.custody_reference.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.destination` | Destination; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.destination.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.destination.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.event_topic` | Custody event topic; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.function_selector` | Custody function selector; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.layerx_amount` | Layerx amount; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.layerx_amount.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.layerx_amount.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.layerx_asset` | Layerx asset; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.layerx_asset.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.layerx_asset.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source` | Source; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source_amount` | Source amount; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source_amount.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source_amount.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source_asset` | Source asset; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source_asset.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody.source_asset.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody_code_hash` | Custody code hash; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody_contract` | Custody contract; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody_identity` | Custody bytecode identity policy: Immutable or Proxy with pinned implementation fields; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody_identity.Proxy.implementation_address` | Pinned proxy implementation identity; Required when ethereum is configured; required for Proxy custody identity | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody_identity.Proxy.implementation_code_hash` | Pinned implementation bytecode hash; Required when ethereum is configured; required for Proxy custody identity | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.custody_identity.Proxy.implementation_slot` | Proxy implementation storage slot; Required when ethereum is configured; required for Proxy custody identity | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.finality_tag` | Finality tag; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.genesis_hash` | Expected genesis block hash; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal` | Journal; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.authentication_key_file` | Journal authentication secret file; Required when ethereum is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.directory` | Durable journal directory; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.namespace` | Durable journal namespace; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_id` | Rollback protection anchor identifier; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc` | Rollback protection RPC quorum; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.connect_timeout_ms` | Connection timeout in milliseconds; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.endpoints` | RPC endpoint declarations; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.endpoints[].bearer_token_file` | RPC bearer token file; Required when ethereum is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.endpoints[].independent_backend` | Independent RPC backend identity; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.endpoints[].url` | RPC endpoint URL; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.maximum_response_bytes` | Maximum accepted RPC response bytes; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.quorum` | Required matching independent RPC responses; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.journal.rollback_anchor_rpc.request_timeout_ms` | Request deadline in milliseconds; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.maximum_ancestry` | Maximum ancestry; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.maximum_history_blocks` | Maximum history blocks; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.maximum_ownership_ttl_seconds` | Maximum ownership ttl seconds; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.minimum_confirmations` | Minimum confirmations; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.native_asset` | Native asset; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc` | Rpc; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.connect_timeout_ms` | Connection timeout in milliseconds; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.endpoints` | RPC endpoint declarations; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.endpoints[].bearer_token_file` | RPC bearer token file; Required when ethereum is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.endpoints[].independent_backend` | Independent RPC backend identity; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.endpoints[].url` | RPC endpoint URL; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.maximum_response_bytes` | Maximum accepted RPC response bytes; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.quorum` | Required matching independent RPC responses; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum.rpc.request_timeout_ms` | Request deadline in milliseconds; Required when ethereum is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.authentication_key_file` | Journal authentication secret file; Required when history_journal is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.directory` | Durable journal directory; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.namespace` | Durable journal namespace; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_id` | Rollback protection anchor identifier; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc` | Rollback protection RPC quorum; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.connect_timeout_ms` | Connection timeout in milliseconds; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.endpoints` | RPC endpoint declarations; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.endpoints[].bearer_token_file` | RPC bearer token file; Required when history_journal is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.endpoints[].independent_backend` | Independent RPC backend identity; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.endpoints[].url` | RPC endpoint URL; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.maximum_response_bytes` | Maximum accepted RPC response bytes; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.quorum` | Required matching independent RPC responses; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal.rollback_anchor_rpc.request_timeout_ms` | Request deadline in milliseconds; Required when history_journal is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal` | Durable account mapping journal policy | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.authentication_key_file` | Journal authentication secret file | None | Path to secret | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.directory` | Durable journal directory | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.namespace` | Durable journal namespace | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_id` | Rollback protection anchor identifier | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc` | Rollback protection RPC quorum | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: mapping_journal.rollback_anchor_rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding` | Paxeer chain binding verification policy | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-migrate/src/mapping_v2.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.genesis_hash` | Expected genesis block hash | None | No | `services/interop/crates/layerx-migrate/src/mapping_v2.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc` | Rpc | None | No | `services/interop/crates/layerx-migrate/src/mapping_v2.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: paxeer_binding.rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ramp_intake.endpoint` | Endpoint or tagged signer transport declaration; Required when ramp_intake is configured | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ramp_intake.token_file` | Service credential file; Required when ramp_intake is configured | None | Path to secret | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody` | Custody; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.amount_endian` | Custody instruction amount byte order; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.custody_account` | Custody account; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.custody_reference_offset` | Custody reference offset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.destination_offset` | Destination offset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.discriminator` | Custody instruction discriminator; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.layerx_amount_offset` | Layerx amount offset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.layerx_asset_offset` | Layerx asset offset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.source_account` | Source account; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.source_amount_offset` | Source amount offset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.source_asset_account` | Source asset account; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody.source_asset_offset` | Source asset offset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_account` | Custody account; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_account_owner` | Custody account owner; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_program` | Custody program; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_program_data_account` | Custody program data account; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_program_data_hash` | Custody program data hash; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_program_data_reference_offset` | Custody program data reference offset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_program_owner` | Custody program owner; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.custody_token_authority` | Custody token authority; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.genesis_hash` | Expected genesis block hash; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal` | Journal; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.authentication_key_file` | Journal authentication secret file; Required when solana is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.directory` | Durable journal directory; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.namespace` | Durable journal namespace; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_id` | Rollback protection anchor identifier; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc` | Rollback protection RPC quorum; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.connect_timeout_ms` | Connection timeout in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.endpoints` | RPC endpoint declarations; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.endpoints[].bearer_token_file` | RPC bearer token file; Required when solana is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.endpoints[].independent_backend` | Independent RPC backend identity; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.endpoints[].url` | RPC endpoint URL; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.maximum_response_bytes` | Maximum accepted RPC response bytes; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.quorum` | Required matching independent RPC responses; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.journal.rollback_anchor_rpc.request_timeout_ms` | Request deadline in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.maximum_ancestry` | Maximum ancestry; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.maximum_history_slots` | Maximum history slots; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.maximum_ownership_ttl_seconds` | Maximum ownership ttl seconds; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.minimum_rooted_slots` | Minimum rooted slots; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.native_asset` | Native asset; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc` | Rpc; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.connect_timeout_ms` | Connection timeout in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.endpoints` | RPC endpoint declarations; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.endpoints[].bearer_token_file` | RPC bearer token file; Required when solana is configured | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.endpoints[].independent_backend` | Independent RPC backend identity; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.endpoints[].url` | RPC endpoint URL; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.maximum_response_bytes` | Maximum accepted RPC response bytes; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.quorum` | Required matching independent RPC responses; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.rpc.request_timeout_ms` | Request deadline in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana.source_asset_account_owner` | Source asset account owner; Required when solana is configured | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `LAYERX_INTEROP_MODULE_REGISTRY_FILE` | JSON module and activity ordinal registry file | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MODULE_REGISTRY_FILE JSON :: modules` | Module declarations | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MODULE_REGISTRY_FILE JSON :: modules[].module` | Registered module identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MODULE_REGISTRY_FILE JSON :: modules[].ordinals` | Registered activity ordinals | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_NETWORK_ID` | Network label matched to the x402 capability binding | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_OUTBOUND_CA_DER` | DER certificate file trusted by outbound TLS clients | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_PROTOCOL_NETWORK_ID` | Nonzero LXP protocol network identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_READINESS_MAX_AGE_MS` | Maximum readiness observation age in milliseconds, from 1 through 30000 | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_RECEIPT_KEYS_FILE` | JSON receipt authority pins file | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_RECEIPT_KEYS_FILE JSON :: [].firstHeight` | First authorized block height | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_RECEIPT_KEYS_FILE JSON :: [].keyId` | Verification key identifier | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_RECEIPT_KEYS_FILE JSON :: [].lastHeight` | Last authorized block height | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_RECEIPT_KEYS_FILE JSON :: [].publicKey` | Public verification key pin | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_REDIS_PASSWORD_FILE` | File containing the Redis password | None | Path to secret | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_REDIS_URL` | TLS Redis connection URL | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_REDIS_USERNAME_FILE` | File containing the Redis username | None | Path to secret | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_SEQUENCER_URL` | Receipt and transaction client endpoint URL | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_TAP_CLOCK_SKEW_SECONDS` | Maximum TAP credential clock skew in seconds, bounded by the adapter | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_TLS_CERT_DER` | DER listener certificate file; required in TLS mode and must be unset in plain mode | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_TLS_KEY_DER` | PKCS#8 DER listener private key file; required in TLS mode and must be unset in plain mode | None | Path to secret | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_WIRE_VERSION` | Numeric LXP wire protocol version supporting occupancy | None | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_MIRROR_CHAIN_ID` | Expected target EVM chain identifier | None | No | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_MIRROR_DEPLOYER_KEY_FILE` | Deployer private key file | None | Path to secret | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_MIRROR_DEPLOYMENT_RECORD` | Deployment record output file | None | No | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_MIRROR_FOUNDRY_BIN` | Directory containing forge and cast | None | No | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_MIRROR_PUBLISHER_ADDRESS` | Publisher contract constructor identity | None | No | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_MIRROR_RECEIPT_KEYS_FILE` | Pinned receipt authority JSON file | None | No | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_MIRROR_RPC_CA_PEM` | RPC TLS CA PEM file | None | No | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_MIRROR_RPC_URL` | Deployment JSON-RPC endpoint URL | None | No | `services/interop/deploy/mirror/deploy-ethereum-mirror.sh` |
| `LAYERX_SOLANA_MIRROR_DEPLOYMENT_RECORD` | Deployment record output file | None | No | `services/interop/deploy/mirror/deploy-solana-mirror.sh` |
| `LAYERX_SOLANA_MIRROR_FINALIZE` | Explicit deployment finalization selection; must be YES | None | No | `services/interop/deploy/mirror/deploy-solana-mirror.sh` |
| `LAYERX_SOLANA_MIRROR_KEYPAIR_FILE` | Publisher and deployer Solana keypair file | None | Path to secret | `services/interop/deploy/mirror/deploy-solana-mirror.sh` |
| `LAYERX_SOLANA_MIRROR_RPC_URL` | HTTPS deployment RPC URL; may contain an endpoint credential | None | May contain a credential | `services/interop/deploy/mirror/deploy-solana-mirror.sh` |
| `LAYERX_SOLANA_MIRROR_TOOLCHAIN_BIN` | Directory containing the Solana deployment tools | None | No | `services/interop/deploy/mirror/deploy-solana-mirror.sh` |
| `LAYERX_TRUSTED_PROXIES` | Comma- or space-separated trusted proxy addresses or CIDRs | None | No | `docker/search-front/real-ip.sh` |
| `SEARCH_FRONT_UPSTREAMS` | Base64 file content decoded to /etc/nginx/search/upstreams.conf | None | No | `docker/common/env-files.sh`; `docker/search-front/files.tsv` |
| `XWEB_ATTESTOR_1_HOST` | Peer DNS name substituted into the other attestor deployment templates | None | No | `services/interop/deploy/x-websearch/render_deploy_template.py` |
| `XWEB_ATTESTOR_2_HOST` | Peer DNS name substituted into the other attestor deployment templates | None | No | `services/interop/deploy/x-websearch/render_deploy_template.py` |
| `XWEB_ATTESTOR_3_HOST` | Peer DNS name substituted into the other attestor deployment templates | None | No | `services/interop/deploy/x-websearch/render_deploy_template.py` |
| `XWEB_ATTESTOR_4_HOST` | Peer DNS name substituted into the other attestor deployment templates | None | No | `services/interop/deploy/x-websearch/render_deploy_template.py` |
| `XWEB_HOPS` | Space-separated local-port-to-peer mappings for the Fly attestor image | None | No | `docker/x-websearch/entrypoint.sh` |
| `X_WEBSEARCH_ATTESTOR_KEY_FILE` | Attestation private key file; required for an attesting sidecar | None | Path to secret | `services/interop/crates/x-websearch/src/keys.rs` |
| `X_WEBSEARCH_RECEIVER_KEY_FILE` | Payment receiver private key file; required for every role | None | Path to secret | `services/interop/crates/x-websearch/src/keys.rs` |
| `X_WEBSEARCH_SUBMITTER_KEY_FILE` | Program transaction submitter private key file; required for an attesting sidecar | None | Path to secret | `services/interop/crates/x-websearch/src/keys.rs` |
| `beta-roots.json (--beta-roots-file) :: ap2_keys.checkout-mandate` | Public P-256 verification key for this AP2 use case | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: ap2_keys.merchant-checkout` | Public P-256 verification key for this AP2 use case | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: ap2_keys.payment-mandate` | Public P-256 verification key for this AP2 use case | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: asset` | Asset binding | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: asset_decimals` | Asset decimals | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: audience` | AP2 mandate audience origin | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: currency` | AP2 currency code | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: fiat_provider_public_key` | Fiat provider public key | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: layerx_agent` | Layerx agent | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: payee_account` | Payee account binding | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: payer_account` | Payer account binding | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: principal_digest` | Principal identity digest | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: visa_agent_expires_at` | Visa agent expires at | None | No | `services/interop/deploy/gateway/render.py` |
| `beta-roots.json (--beta-roots-file) :: visa_agent_public_key` | Visa agent public key | None | No | `services/interop/deploy/gateway/render.py` |
| `deploy/bridge-relayer/relayer-<instance>.toml :: app` | Deployment identity, attestor policy or secret mount used to bind the signer inventory | None | No | `services/interop/deploy/bridge-relayer/render-signer-keys.py` |
| `deploy/bridge-relayer/relayer-<instance>.toml :: env.LAYERX_BRIDGE_ATTESTOR_INDEX` | Deployment identity, attestor policy or secret mount used to bind the signer inventory | None | No | `services/interop/deploy/bridge-relayer/render-signer-keys.py` |
| `deploy/bridge-relayer/relayer-<instance>.toml :: env.LAYERX_BRIDGE_ATTESTOR_KEY_FILE` | Deployment identity, attestor policy or secret mount used to bind the signer inventory | None | No | `services/interop/deploy/bridge-relayer/render-signer-keys.py` |
| `deploy/bridge-relayer/relayer-<instance>.toml :: env.LAYERX_BRIDGE_ATTESTOR_SET_SIZE` | Deployment identity, attestor policy or secret mount used to bind the signer inventory | None | No | `services/interop/deploy/bridge-relayer/render-signer-keys.py` |
| `deploy/bridge-relayer/relayer-<instance>.toml :: env.LAYERX_BRIDGE_ATTESTOR_THRESHOLD` | Deployment identity, attestor policy or secret mount used to bind the signer inventory | None | No | `services/interop/deploy/bridge-relayer/render-signer-keys.py` |
| `deploy/bridge-relayer/relayer-<instance>.toml :: files[].guest_path` | Deployment identity, attestor policy or secret mount used to bind the signer inventory | None | No | `services/interop/deploy/bridge-relayer/render-signer-keys.py` |
| `deploy/bridge-relayer/relayer-<instance>.toml :: files[].secret_name` | Deployment identity, attestor policy or secret mount used to bind the signer inventory | None | No | `services/interop/deploy/bridge-relayer/render-signer-keys.py` |
| `gateway/receipt-keys.json (--receipt-keys-file) :: [].firstHeight` | First authorized block height | None | No | `services/interop/deploy/gateway/render.py` |
| `gateway/receipt-keys.json (--receipt-keys-file) :: [].keyId` | Verification key identifier | None | No | `services/interop/deploy/gateway/render.py` |
| `gateway/receipt-keys.json (--receipt-keys-file) :: [].lastHeight` | Last authorized block height | None | No | `services/interop/deploy/gateway/render.py` |
| `gateway/receipt-keys.json (--receipt-keys-file) :: [].publicKey` | Public verification key pin | None | No | `services/interop/deploy/gateway/render.py` |
| `membership.json :: attestors` | Authorized attestor identities | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `membership.json :: threshold` | Required attestor signature count | None | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `migration/binding.json (--binding-config) :: chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-migrate/src/mapping_v2.rs` |
| `migration/binding.json (--binding-config) :: genesis_hash` | Expected genesis block hash | None | No | `services/interop/crates/layerx-migrate/src/mapping_v2.rs` |
| `migration/binding.json (--binding-config) :: rpc` | Rpc | None | No | `services/interop/crates/layerx-migrate/src/mapping_v2.rs` |
| `migration/binding.json (--binding-config) :: rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/binding.json (--binding-config) :: rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody` | Custody | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.custody_reference` | Custody reference | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.custody_reference.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.custody_reference.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.destination` | Destination | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.destination.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.destination.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.event_topic` | Custody event topic | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.function_selector` | Custody function selector | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.layerx_amount` | Layerx amount | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.layerx_amount.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.layerx_amount.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.layerx_asset` | Layerx asset | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.layerx_asset.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.layerx_asset.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source` | Source | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source_amount` | Source amount | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source_amount.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source_amount.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source_asset` | Source asset | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source_asset.Data` | ABI data word offset; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody.source_asset.Topic` | Event topic index; use exactly one selector variant for this field | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody_code_hash` | Custody code hash | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody_contract` | Custody contract | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody_identity` | Custody bytecode identity policy: Immutable or Proxy with pinned implementation fields | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody_identity.Proxy.implementation_address` | Pinned proxy implementation identity; required for Proxy custody identity | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody_identity.Proxy.implementation_code_hash` | Pinned implementation bytecode hash; required for Proxy custody identity | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: custody_identity.Proxy.implementation_slot` | Proxy implementation storage slot; required for Proxy custody identity | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: finality_tag` | Finality tag | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: genesis_hash` | Expected genesis block hash | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: journal` | Journal | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: journal.authentication_key_file` | Journal authentication secret file | None | Path to secret | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/ethereum.json (--config) :: journal.directory` | Durable journal directory | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/ethereum.json (--config) :: journal.namespace` | Durable journal namespace | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_id` | Rollback protection anchor identifier | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc` | Rollback protection RPC quorum | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: journal.rollback_anchor_rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: maximum_ancestry` | Maximum ancestry | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: maximum_history_blocks` | Maximum history blocks | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: maximum_ownership_ttl_seconds` | Maximum ownership ttl seconds | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: minimum_confirmations` | Minimum confirmations | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: native_asset` | Native asset | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: rpc` | Rpc | None | No | `services/interop/crates/layerx-migrate/src/ethereum.rs` |
| `migration/ethereum.json (--config) :: rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/ethereum.json (--config) :: rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/gateway.json (--gateway-config) :: ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/gateway_client.rs` |
| `migration/gateway.json (--gateway-config) :: connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/gateway_client.rs` |
| `migration/gateway.json (--gateway-config) :: customer_authorization_file` | Customer authorization secret file | None | Path to secret | `services/interop/crates/layerx-migrate/src/gateway_client.rs` |
| `migration/gateway.json (--gateway-config) :: endpoint` | Endpoint or tagged signer transport declaration | None | No | `services/interop/crates/layerx-migrate/src/gateway_client.rs` |
| `migration/gateway.json (--gateway-config) :: maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/gateway_client.rs` |
| `migration/gateway.json (--gateway-config) :: request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/gateway_client.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: authentication_key_file` | Journal authentication secret file | None | Path to secret | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: directory` | Durable journal directory | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: namespace` | Durable journal namespace | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_id` | Rollback protection anchor identifier | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc` | Rollback protection RPC quorum | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/journal.json (--mapping-config or --store-config) :: rollback_anchor_rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: custody` | Custody | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.amount_endian` | Custody instruction amount byte order | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.custody_account` | Custody account | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.custody_reference_offset` | Custody reference offset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.destination_offset` | Destination offset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.discriminator` | Custody instruction discriminator | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.layerx_amount_offset` | Layerx amount offset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.layerx_asset_offset` | Layerx asset offset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.source_account` | Source account | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.source_amount_offset` | Source amount offset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.source_asset_account` | Source asset account | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody.source_asset_offset` | Source asset offset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_account` | Custody account | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_account_owner` | Custody account owner | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_program` | Custody program | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_program_data_account` | Custody program data account | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_program_data_hash` | Custody program data hash | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_program_data_reference_offset` | Custody program data reference offset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_program_owner` | Custody program owner | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: custody_token_authority` | Custody token authority | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: genesis_hash` | Expected genesis block hash | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: journal` | Journal | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: journal.authentication_key_file` | Journal authentication secret file | None | Path to secret | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/solana.json (--config) :: journal.directory` | Durable journal directory | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/solana.json (--config) :: journal.namespace` | Durable journal namespace | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_id` | Rollback protection anchor identifier | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc` | Rollback protection RPC quorum | None | No | `services/interop/crates/layerx-migrate/src/journal.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: journal.rollback_anchor_rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: maximum_ancestry` | Maximum ancestry | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: maximum_history_slots` | Maximum history slots | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: maximum_ownership_ttl_seconds` | Maximum ownership ttl seconds | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: minimum_rooted_slots` | Minimum rooted slots | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: native_asset` | Native asset | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: rpc` | Rpc | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `migration/solana.json (--config) :: rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-migrate/src/rpc.rs` |
| `migration/solana.json (--config) :: source_asset_account_owner` | Source asset account owner | None | No | `services/interop/crates/layerx-migrate/src/solana.rs` |
| `mirror/config.json (publisher file argument) :: ethereum` | Ethereum source policy section | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.archive_code_hash_hex` | Archive code hash hex | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.archive_contract_hex` | Archive contract hex | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.chunk_bytes` | Chunk bytes | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.genesis_hash_hex` | Expected genesis block hash in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.maximum_fee_per_gas` | Maximum fee per gas | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.maximum_priority_fee_per_gas` | Maximum priority fee per gas | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.maximum_reorg_depth` | Maximum reorg depth | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.required_confirmations` | Required confirmations | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc` | Rpc | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer` | Signer | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.key_handle` | Remote signer policy handle | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.public_key` | Public verification key pin | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.timeout_ms` | Signer or cosign operation timeout in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport` | Signer transport variant | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport.client_certificate` | Signer transport client certificate; required for its selected transport variant | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport.client_private_key` | Signer transport client private key; required for its selected transport variant | None | Path to secret | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport.endpoint` | Signer transport endpoint; required for its selected transport variant | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport.kind` | Selected source or signer transport variant | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport.server_name` | Signer transport server name; required for its selected transport variant | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport.socket` | Signer transport socket; required for its selected transport variant | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.signer.transport.trust_anchor` | Signer transport trust anchor; required for its selected transport variant | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: ethereum.transaction_gas_limit` | Transaction gas limit | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: first_height` | First block height or authority window start | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: freshness_budget_blocks` | Maximum committed-block lag accepted as fresh | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: paxeer` | Paxeer source policy section | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.genesis_hash_hex` | Expected genesis block hash in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.receipt_keys` | Receipt keys | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.receipt_keys[].first_height` | First block height or authority window start | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.receipt_keys[].last_height` | Last authorized block height | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.receipt_keys[].public_key_hex` | Public verification key pin in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.receipt_keys[].sequencer_id_hex` | Receipt authority identifier in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc` | Rpc | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: paxeer.rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: poll_interval_ms` | Polling interval in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.archive_program_base58` | Archive program base58; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.chunk_bytes` | Chunk bytes; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.genesis_hash_base58` | Expected genesis hash in base58; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.maximum_ancestry` | Maximum ancestry; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.program_code_hash_hex` | Program code hash hex; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.program_data_account_base58` | Program data account base58; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.required_rooted_slots` | Required rooted slots; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc` | Rpc; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.connect_timeout_ms` | Connection timeout in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.endpoints` | RPC endpoint declarations; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.endpoints[].bearer_token_file` | RPC bearer token file; Required when solana is configured | None | Path to secret | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.endpoints[].independent_backend` | Independent RPC backend identity; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.endpoints[].url` | RPC endpoint URL; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.maximum_response_bytes` | Maximum accepted RPC response bytes; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.quorum` | Required matching independent RPC responses; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.rpc.request_timeout_ms` | Request deadline in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer` | Signer; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.key_handle` | Remote signer policy handle; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.public_key` | Public verification key pin; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.timeout_ms` | Signer or cosign operation timeout in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport` | Signer transport variant; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport.client_certificate` | Signer transport client certificate; required for its selected transport variant; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport.client_private_key` | Signer transport client private key; required for its selected transport variant; Required when solana is configured | None | Path to secret | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport.endpoint` | Signer transport endpoint; required for its selected transport variant; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport.kind` | Selected source or signer transport variant; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport.server_name` | Signer transport server name; required for its selected transport variant; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport.socket` | Signer transport socket; required for its selected transport variant; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.signer.transport.trust_anchor` | Signer transport trust anchor; required for its selected transport variant; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: solana.upgradeable_loader_base58` | Upgradeable loader base58; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: state_directory` | Persistent mirror journal and archive state directory | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/config.json (publisher file argument) :: status_listen` | Mirror status listener socket address and port | None | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `mirror/verify-config.json :: layerx_network_id` | Archive network identifier | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: receipt_keys` | Receipt keys | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: receipt_keys[].first_height` | First block height or authority window start | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/verify-config.json :: receipt_keys[].last_height` | Last authorized block height | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/verify-config.json :: receipt_keys[].public_key_hex` | Public verification key pin in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/verify-config.json :: receipt_keys[].sequencer_id_hex` | Receipt authority identifier in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/committed.rs` |
| `mirror/verify-config.json :: sources` | Sources | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].archive_code_hash_hex` | Ethereum mirror archive code hash hex; required for an ethereum source | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].archive_contract_hex` | Ethereum mirror archive contract hex; required for an ethereum source | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].archive_program_hex` | Solana mirror archive program hex; required for a solana source | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].chain_id` | Ethereum mirror chain id; required for an ethereum source | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].genesis_hash_hex` | Expected genesis block hash in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].id` | Id | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].kind` | Selected source or signer transport variant | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].program_code_hash_hex` | Solana mirror program code hash hex; required for a solana source | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].program_data_account_hex` | Solana mirror program data account hex; required for a solana source | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].publisher_hex` | Pinned archive publisher identity in hexadecimal | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].rpc` | Rpc | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `mirror/verify-config.json :: sources[].rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `mirror/verify-config.json :: sources[].upgradeable_loader_hex` | Solana mirror upgradeable loader hex; required for a solana source | None | No | `services/interop/crates/layerx-mirror/src/bin/layerx-mirror-verify.rs` |
| `rate.json (--config) :: balance_floor` | Minimum remaining sponsor balance in PAX base units | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: decimals` | Fee token decimal exponent | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: interval_seconds` | Policy interval in seconds | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: margin_bps` | Quote margin in basis points | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: max_priority_fee_per_gas` | Priority fee per gas in wei | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: max_rate_age` | Maximum quotable rate age in seconds | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: paymaster` | Paymaster contract identity | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: per_account_limit` | Per-account budget per policy interval | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: per_interval_limit` | Total budget per policy interval | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: per_quote_limit` | Maximum budget consumed by one quote | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: rate_balance_floor` | Rate publisher balance floor in PAX base units | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: rate_cadence_seconds` | Rate publication cadence in seconds | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: rate_confirmation_retry_seconds` | Rate confirmation retry interval in seconds | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: rate_gas_budget_per_day` | Daily rate publication gas budget | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: rate_max_fee_per_gas` | Rate publication fee ceiling per gas in wei | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: rate_owner_key_env` | Environment variable name containing the paymaster owner secret | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: relayer_key_env` | Environment variable name containing the sponsor secret | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: sequencer_receipt_keys` | Sequencer receipt keys | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: sequencer_receipt_keys[].firstHeight` | First authorized block height | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: sequencer_receipt_keys[].keyId` | Verification key identifier | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: sequencer_receipt_keys[].lastHeight` | Last authorized block height | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: sequencer_receipt_keys[].publicKey` | Public verification key pin | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: spread_bps` | Quote spread in basis points | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.json (--config) :: token` | Fee token contract identity | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.toml (--rate-file) :: rate` | Owner-authorized conversion rate | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `rate.toml (--rate-file) :: set_at` | Chain timestamp at which the owner set the rate | None | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `receipt-keys.json (--receipt-keys-file) :: [].firstHeight` | First authorized block height | None | No | `services/interop/deploy/mirror/render-config.py` |
| `receipt-keys.json (--receipt-keys-file) :: [].keyId` | Verification key identifier | None | No | `services/interop/deploy/mirror/render-config.py` |
| `receipt-keys.json (--receipt-keys-file) :: [].lastHeight` | Last authorized block height | None | No | `services/interop/deploy/mirror/render-config.py` |
| `receipt-keys.json (--receipt-keys-file) :: [].publicKey` | Public verification key pin | None | No | `services/interop/deploy/mirror/render-config.py` |
| `relayer.json (--config) :: attestor` | Local attestor identity or signer policy | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: attestor.handle` | Remote signer policy handle | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: attestor.public_key` | Public verification key pin | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: attestor_set.members` | Members; Required when attestor_set is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: attestor_set.threshold` | Required attestor signature count; Required when attestor_set is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains` | Chains | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].finality_depth` | Source finality depth | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].gas` | Gas | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].gas.gas_limit` | Transaction gas limit | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].gas.max_fee_per_gas` | Maximum fee per gas in wei | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].gas.max_priority_fee_per_gas` | Priority fee per gas in wei | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].max_block_range` | Maximum blocks scanned per request | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].rpc` | Rpc | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].rpc.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.endpoints[].bearer_token_file` | RPC bearer token file | None | Path to secret | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.endpoints[].independent_backend` | Independent RPC backend identity | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.maximum_response_bytes` | Maximum accepted RPC response bytes | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.quorum` | Required matching independent RPC responses | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].rpc.request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: chains[].start_block` | First block scanned | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].submitter` | Submitter | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].submitter.handle` | Remote signer policy handle | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].submitter.public_key` | Public verification key pin | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: chains[].vault` | Vault | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: journal_path` | Persistent relayer journal file | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: max_submissions` | Max submissions | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer` | Paxeer source policy section | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.endpoints[].request_timeout_ms` | Request deadline in milliseconds | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.endpoints[].url` | RPC endpoint URL | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.finality_depth` | Source finality depth | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.gas` | Gas | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.gas.gas_limit` | Transaction gas limit | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.gas.max_fee_per_gas` | Maximum fee per gas in wei | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.gas.max_priority_fee_per_gas` | Priority fee per gas in wei | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.max_block_range` | Maximum blocks scanned per request | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.receipt_keys` | Receipt keys | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.receipt_keys[].first_height` | First block height or authority window start | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.receipt_keys[].last_height` | Last authorized block height | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.receipt_keys[].public_key` | Public verification key pin | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.receipt_keys[].sequencer_id` | Receipt authority identifier | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.start_block` | First block scanned | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.submitter` | Submitter | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.submitter.handle` | Remote signer policy handle | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.submitter.public_key` | Public verification key pin | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: poll_interval_ms` | Polling interval in milliseconds | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer` | Signer | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint` | Endpoint or tagged signer transport declaration | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint.client_certificate` | Signer transport client certificate; required for its selected transport variant | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint.client_private_key` | Signer transport client private key; required for its selected transport variant | None | Path to secret | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint.endpoint` | Signer transport endpoint; required for its selected transport variant | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint.server_name` | Signer transport server name; required for its selected transport variant | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint.socket` | Signer transport socket; required for its selected transport variant | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint.transport` | Signer transport variant | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.endpoint.trust_anchor` | Signer transport trust anchor; required for its selected transport variant | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: signer.timeout_ms` | Signer or cosign operation timeout in milliseconds | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.chain_id` | Expected chain identifier; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.commitment` | Solana observation commitment level; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.fee_payer` | Fee payer; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.fee_payer.handle` | Remote signer policy handle; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.fee_payer.public_key` | Public verification key pin; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.finality_depth` | Source finality depth; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.max_slot_range` | Maximum slots scanned per request; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.program_id` | Program id; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.rpc` | Rpc; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.rpc.connect_timeout_ms` | Connection timeout in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.endpoints` | RPC endpoint declarations; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.endpoints[].bearer_token_file` | RPC bearer token file; Required when solana is configured | None | Path to secret | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.endpoints[].ca_certificate_der` | RPC trust anchor DER file; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.endpoints[].independent_backend` | Independent RPC backend identity; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.endpoints[].url` | RPC endpoint URL; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.maximum_response_bytes` | Maximum accepted RPC response bytes; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.quorum` | Required matching independent RPC responses; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.rpc.request_timeout_ms` | Request deadline in milliseconds; Required when solana is configured | None | No | `services/interop/crates/layerx-mirror/src/rpc.rs` |
| `relayer.json (--config) :: solana.start_slot` | First slot scanned; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.vault` | Vault; Required when solana is configured | None | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `signer-keys.json :: keys` | Keys | None | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `signer-keys.json :: keys[].algorithm` | Signing algorithm identifier | None | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `signer-keys.json :: keys[].domains` | Allowed signing policy domains | None | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `signer-keys.json :: keys[].handle` | Remote signer policy handle | None | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `signer-keys.json :: keys[].key_file` | Private signing key file | None | Path to secret | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `station.json (--config) :: balance_floor` | Minimum remaining sponsor balance in PAX base units | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: chain_id` | Expected chain identifier | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: decimals` | Fee token decimal exponent | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: endpoints` | RPC endpoint declarations | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: gas_limit` | Transaction gas limit | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: interval_seconds` | Policy interval in seconds | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: listen` | Listener socket address and port | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: margin_bps` | Quote margin in basis points | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: max_priority_fee_per_gas` | Priority fee per gas in wei | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: max_rate_age` | Maximum quotable rate age in seconds | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: paymaster` | Paymaster contract identity | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: per_account_limit` | Per-account budget per policy interval | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: per_interval_limit` | Total budget per policy interval | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: per_quote_limit` | Maximum budget consumed by one quote | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: registered_senders[].address` | Registered sender or cosign peer address; Required when registered_senders is configured | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: registered_senders[].budget` | Registered sender budget per interval; Required when registered_senders is configured | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: relayer_key_env` | Environment variable name containing the sponsor secret | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: spread_bps` | Quote spread in basis points | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `station.json (--config) :: token` | Fee token contract identity | None | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `x-websearch/config.json (--config) :: assets` | Assets | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.PAX` | Pax | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.PAX.asset_id` | Asset identifier | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.PAX.price` | Price in the asset base units as a decimal string | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.SID` | Sid | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.SID.asset_id` | Asset identifier | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.SID.price` | Price in the asset base units as a decimal string | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.USDC` | Usdc | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.USDC.asset_id` | Asset identifier | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.USDC.price` | Price in the asset base units as a decimal string | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.USDL` | Usdl | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.USDL.asset_id` | Asset identifier | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: assets.USDL.price` | Price in the asset base units as a decimal string | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: crawl` | Crawl | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: crawl.max_depth` | Maximum crawl depth | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: crawl.pages_per_cycle` | Maximum pages crawled per cycle | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: crawl.pages_per_host` | Maximum pages crawled per host per cycle | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: crawl.politeness_delay_ms` | Delay between host requests in milliseconds | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: data_dir` | Persistent crawler, index and payment state directory | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: evm` | Evm | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: evm.chain_id` | Expected chain identifier | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: evm.confirmations` | Confirmations | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: evm.endpoint` | Endpoint or tagged signer transport declaration | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: fetch` | Fetch | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: fetch.allow_loopback` | Permit loopback seed targets | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: fetch.connect_timeout_ms` | Connection timeout in milliseconds | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: fetch.max_body_bytes` | Maximum fetched response body bytes | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: fetch.max_redirects` | Maximum followed redirects | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: fetch.total_timeout_ms` | Total fetch timeout in milliseconds | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: gateway` | Gateway | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: gateway.endpoint` | Endpoint or tagged signer transport declaration | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: gateway.receipt_keys` | Receipt keys | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: gateway.receipt_keys[].first_height` | First block height or authority window start | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: gateway.receipt_keys[].key_id` | Verification key identifier | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: gateway.receipt_keys[].last_height` | Last authorized block height | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: gateway.receipt_keys[].public_key` | Public verification key pin | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: listen` | Listener socket address and port | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: peers` | Peer endpoint list | None | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: seeds` | Seed URL list | None | No | `services/interop/crates/x-websearch/src/config.rs` |

## Optional

| Name / file :: section.key | What it sets | Source default | Secret | Source reader |
| --- | --- | --- | --- | --- |
| `<relayer_key_env>_KEY_FILE` | Configured sponsor prefix plus this suffix; checked to keep owner and sponsor key sources distinct | Unset | Path to secret | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `<relayer_key_env>_KMS_KEY_ID` | Configured sponsor prefix plus this suffix; checked to keep owner and sponsor key sources distinct | Unset | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `CARGO_PKG_VERSION` | Cargo-provided package version compiled into service version responses | Version in the corresponding Cargo.toml | No | `services/interop/crates/layerx-mirror/src/runtime.rs`; `services/interop/crates/layerx-interop-service/src/server.rs`; `services/interop/crates/x-websearch/src/api.rs`; `services/interop/crates/x-websearch/src/fetch.rs` |
| `GAS_STATION_BROWSER_ORIGINS` | Comma-separated HTTPS origins admitted for browser quote and submission requests | Unset; browser origins not admitted | No | `services/interop/crates/layerx-gas-station/src/service.rs` |
| `GAS_STATION_ENDPOINTS` | Space-separated HTTPS node root RPC endpoints for the image configuration | Public node root endpoint selected in init.sh; host value omitted | No | `docker/gas-station/init.sh` |
| `GAS_STATION_RELAYER_KEY_KEY_FILE` | Owner/sponsor source-isolation input when relayer_key_env selects GAS_STATION_RELAYER_KEY | Unset | Path to secret | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `GAS_STATION_RELAYER_KEY_KMS_KEY_ID` | Owner/sponsor source-isolation input when relayer_key_env selects GAS_STATION_RELAYER_KEY | Unset | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `LAYERX_BETA_INTEROP_AP2_ASSETS` | JSON array of AP2 asset, currency, principal and merchant bindings | Generated beta trust roots; otherwise required from a root or manifest override | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_AP2_KEYS` | JSON array of AP2 use-case public key pins | Generated beta trust roots; otherwise required from a root or manifest override | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_A2A` | Override transport conformance SHA-256 | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_AP2` | Override adapter conformance suite identifier, vector count and SHA-256 as one comma-separated value | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_FIAT` | Override adapter conformance suite identifier, vector count and SHA-256 as one comma-separated value | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_HTTP` | Override transport conformance SHA-256 | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_MCP` | Override transport conformance SHA-256 | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_UCP` | Override adapter conformance suite identifier, vector count and SHA-256 as one comma-separated value | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_VISA_TAP` | Override adapter conformance suite identifier, vector count and SHA-256 as one comma-separated value | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_CONFORMANCE_X402` | Override adapter conformance suite identifier, vector count and SHA-256 as one comma-separated value | Derived from repository conformance vectors | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_FIAT_PROVIDERS` | JSON array of fiat provider public key pins | Generated beta trust roots; otherwise required from a root or manifest override | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE` | JSON file overriding manifest fields by adapter or transport identifier | Unset; no file override | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: adapters.<id>.conformance_sha256` | Conformance vector digest | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: adapters.<id>.conformance_suite` | Conformance suite identifier or payment exchange directory | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: adapters.<id>.conformance_vectors` | Number of conformance vectors | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: adapters.<id>.evidence_policy` | Evidence required by the adapter | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: adapters.<id>.specification` | Specification identifier | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: adapters.<id>.specification_sha256` | Pinned specification digest | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: adapters.<id>.version` | Protocol or schema version | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: ap2_assets` | Override the corresponding runtime manifest section; nested fields use the LAYERX_INTEROP_CONFIG schema | Derived or supplied through root inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: ap2_keys` | Override the corresponding runtime manifest section; nested fields use the LAYERX_INTEROP_CONFIG schema | Derived or supplied through root inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: fiat_providers` | Override the corresponding runtime manifest section; nested fields use the LAYERX_INTEROP_CONFIG schema | Derived or supplied through root inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: transports.<id>.conformance_sha256` | Conformance vector digest | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: transports.<id>.specification_sha256` | Pinned specification digest | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: transports.<id>.version` | Protocol or schema version | Derived from repository specification and conformance inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: ucp_payment_handler` | Override the corresponding runtime manifest section; nested fields use the LAYERX_INTEROP_CONFIG schema | Derived or supplied through root inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: visa_agents` | Override the corresponding runtime manifest section; nested fields use the LAYERX_INTEROP_CONFIG schema | Derived or supplied through root inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: visa_targets` | Override the corresponding runtime manifest section; nested fields use the LAYERX_INTEROP_CONFIG schema | Derived or supplied through root inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_MANIFEST_FILE JSON :: x402_supported` | Override the corresponding runtime manifest section; nested fields use the LAYERX_INTEROP_CONFIG schema | Derived or supplied through root inputs | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_UCP_PAYMENT_HANDLER` | JSON UCP payment handler declaration | Derived from the pinned UCP revision | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_VISA_AGENTS` | JSON array of trusted TAP agent declarations | Generated beta trust roots; otherwise required from a root or manifest override | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_VISA_TARGETS` | JSON array of principal, authority and path bindings | Generated beta trust roots; otherwise required from a root or manifest override | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BETA_INTEROP_X402_SUPPORTED` | JSON x402 capability declaration | Derived from network and receipt authorities | No | `services/interop/deploy/gateway/render.py` |
| `LAYERX_BRIDGE_OPERATOR_INVENTORY` | Approved operator inventory JSON file; required by the container when cosigning is enabled | Unset; no inventory binding | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_RELAYER_CONFIG` | Relayer JSON file paired with the operator inventory; required when that inventory is supplied | Unset in library; container sets its mounted relayer file | No | `services/interop/crates/layerx-bridge-relayer/src/cosign.rs` |
| `LAYERX_BRIDGE_SIGNER_KEYS_FILE` | Bridge signer policy manifest file | /etc/layerx/bridge-signer/keys.json | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `LAYERX_BRIDGE_SIGNER_SOCKET` | Bridge signer Unix socket | /run/bridge-signer/signer.sock | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `LAYERX_FILES_OWNER` | Owner applied to materialized files when the entrypoint runs as root | Unset; current owner | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_<name>` | Per-file owner override, where name is a variable in the file table | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_APPROVED_MEMBERSHIP` | Owner override for the decoded BRIDGE_APPROVED_MEMBERSHIP file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_COSIGN_CERTIFICATE` | Owner override for the decoded BRIDGE_COSIGN_CERTIFICATE file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_COSIGN_PRIVATE_KEY` | Owner override for the decoded BRIDGE_COSIGN_PRIVATE_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_COSIGN_TRANSPORT_CONFIG` | Owner override for the decoded BRIDGE_COSIGN_TRANSPORT_CONFIG file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_COSIGN_TRUST_ANCHOR` | Owner override for the decoded BRIDGE_COSIGN_TRUST_ANCHOR file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_OPERATOR_INVENTORY` | Owner override for the decoded BRIDGE_OPERATOR_INVENTORY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_RELAYER_CONFIG` | Owner override for the decoded BRIDGE_RELAYER_CONFIG file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_RELAYER_RPC_TOKENS` | Owner override for the decoded BRIDGE_RELAYER_RPC_TOKENS file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_ARBITRUM_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_ARBITRUM_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_ATTESTOR_KEY` | Owner override for the decoded BRIDGE_SIGNER_ATTESTOR_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_AVALANCHE_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_AVALANCHE_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_BASE_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_BASE_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_BNB_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_BNB_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_ETHEREUM_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_ETHEREUM_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_HYPEREVM_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_HYPEREVM_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_KEYS` | Owner override for the decoded BRIDGE_SIGNER_KEYS file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_OPTIMISM_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_OPTIMISM_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_PAXEER_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_PAXEER_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_POLYGON_SUBMITTER_KEY` | Owner override for the decoded BRIDGE_SIGNER_POLYGON_SUBMITTER_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_BRIDGE_SIGNER_SOLANA_FEE_PAYER_KEYPAIR` | Owner override for the decoded BRIDGE_SIGNER_SOLANA_FEE_PAYER_KEYPAIR file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_AUTHORITY_TOKEN` | Owner override for the decoded INTEROP_AUTHORITY_TOKEN file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_CLIENT_CA_DER` | Owner override for the decoded INTEROP_CLIENT_CA_DER file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_CLIENT_P12` | Owner override for the decoded INTEROP_CLIENT_P12 file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_CLIENT_PASSWORD` | Owner override for the decoded INTEROP_CLIENT_PASSWORD file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_CONFIG` | Owner override for the decoded INTEROP_CONFIG file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_MODULE_REGISTRY` | Owner override for the decoded INTEROP_MODULE_REGISTRY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_REDIS_PASSWORD` | Owner override for the decoded INTEROP_REDIS_PASSWORD file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_REDIS_USERNAME` | Owner override for the decoded INTEROP_REDIS_USERNAME file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_SEQUENCER_FIRST_BATCH` | Owner override for the decoded INTEROP_SEQUENCER_FIRST_BATCH file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_SEQUENCER_ID` | Owner override for the decoded INTEROP_SEQUENCER_ID file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_SEQUENCER_LAST_BATCH` | Owner override for the decoded INTEROP_SEQUENCER_LAST_BATCH file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_INTEROP_SEQUENCER_PUBLIC_KEY` | Owner override for the decoded INTEROP_SEQUENCER_PUBLIC_KEY file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_OWNER_SEARCH_FRONT_UPSTREAMS` | Owner override for the decoded SEARCH_FRONT_UPSTREAMS file; applied when the entrypoint runs as root | LAYERX_FILES_OWNER | No | `docker/common/env-files.sh` |
| `LAYERX_FILES_TABLE` | Container file-materialization table | /etc/layerx/files.tsv | No | `docker/common/env-files.sh` |
| `LAYERX_INTEROP_ALLOW_RAILWAY_PRIVATE_NETWORK` | Enable the Railway private HTTP endpoint form with 1; otherwise 0 | 0 | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_AP2_RECEIPT_SIGNERS_FILE` | Private JSON file for AP2 payment and checkout receipt signers | Unset; receipt signing disabled | Path to secret | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: ap2_assets[].payee_merchant_website` | Optional payee merchant website | Unset | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_CONFIG JSON :: x402_supported.kinds[].extra` | Additional x402 capability metadata | Unset | No | `services/interop/crates/layerx-x402/src/facilitator.rs` |
| `LAYERX_INTEROP_IDEMPOTENCY_SECONDS` | Redis relay and idempotency retention in seconds, from 1 through 2592000 | 604800 | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_LISTENER` | Listener mode: tls or plain | tls | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG` | Private JSON migration profile file | Unset; migration V2 disabled | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ethereum` | Ethereum source policy section | Unset | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: history_journal` | Durable external history journal policy | Unset | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: ramp_intake` | Authenticated fiat ramp intake client binding | Unset | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_INTEROP_MIGRATION_V2_CONFIG JSON :: solana` | Solana source policy section | Unset | No | `services/interop/crates/layerx-interop-service/src/config.rs` |
| `LAYERX_MIGRATION_SECRET_DIR` | Canonical absolute directory substituted only when the migration JSON contains its matching placeholder | Unset; no substitution | No | `services/interop/crates/layerx-migrate/src/main.rs` |
| `LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_FILE` | Ethereum publisher secret file | /etc/layerx/mirror-signer/ethereum.key | Path to secret | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `LAYERX_MIRROR_SIGNER_ETHEREUM_KEY_HANDLE` | Ethereum publisher policy handle | mirror/ethereum/beta | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `LAYERX_MIRROR_SIGNER_SOCKET` | Mirror signer Unix socket | /run/mirror-signer/signer.sock | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `LAYERX_MIRROR_SIGNER_SOLANA_KEY_FILE` | Solana publisher keypair file | /etc/layerx/mirror-signer/solana.json | Path to secret | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `LAYERX_MIRROR_SIGNER_SOLANA_KEY_HANDLE` | Solana publisher policy handle | mirror/solana/beta | No | `services/interop/crates/layerx-mirror-signer/src/lib.rs` |
| `LAYERX_ROLE` | Role selecting rows of that table | Empty | No | `docker/common/env-files.sh` |
| `LAYERX_SOLANA_MIRROR_PROGRAM_KEYPAIR_FILE` | Optional program keypair file preserving the program identity | Unset; deployment creates the program identity | Path to secret | `services/interop/deploy/mirror/deploy-solana-mirror.sh` |
| `NGINX_ENTRYPOINT_LOCAL_RESOLVERS` | Enable the nginx image resolver discovery | 1 | No | `docker/search-front/Dockerfile` |
| `NGINX_ENVSUBST_OUTPUT_DIR` | Directory for rendered nginx configuration | /etc/nginx/search/conf.d | No | `docker/search-front/Dockerfile` |
| `PAXEER_X_SOURCE_REVISION` | Build-time source revision included in the web search version response | Unset; JSON null | No | `services/interop/crates/x-websearch/src/main.rs` |
| `mirror/config.json (publisher file argument) :: solana` | Solana source policy section | Unset | No | `services/interop/crates/layerx-mirror/src/runtime.rs` |
| `rate.toml (--rate-file) :: not_after` | Optional chain timestamp withdrawing the rate | Unset | No | `services/interop/crates/layerx-gas-station/src/rate.rs` |
| `relayer.json (--config) :: attestation_allowed_origins` | Browser origins admitted by the attestation API | [] | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: attestation_listen` | Optional attestation API listener socket address | Unset | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: attestor_set` | Approved attestor membership and signature threshold | Unset | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: cosign_directory` | Persistent cosign share directory | Unset | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.endpoints[].independent_backend` | Independent RPC backend identity | Unset | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.endpoints[].local_emulator` | Select the local emulator transport mode | false | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: paxeer.endpoints[].trust_anchor_der` | TLS trust anchor DER file; required for a TLS endpoint | Unset | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana` | Solana source policy section | Unset | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `relayer.json (--config) :: solana.release_mints` | Solana mints eligible for outbound release; Applies when solana is configured | [] | No | `services/interop/crates/layerx-bridge-relayer/src/config.rs` |
| `station.json (--config) :: registered_senders` | Registered sender identities and interval budgets | [] | No | `services/interop/crates/layerx-gas-station/src/config.rs` |
| `x-websearch/config.json (--config) :: crawl_interval_seconds` | Seconds between crawl cycle starts | 900 | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: kernel_network_id` | Kernel domain network identifier for program requests | Unset | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: payment` | Metered payment policy; every nested field is optional | PaymentConfig defaults | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: payment.conformance_suite` | Conformance suite identifier or payment exchange directory; Applies when payment is configured | Unset; pinned suite registered without reading a directory | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: payment.draw_fee_limit` | Maximum draw fee in fee-asset base units; Applies when payment is configured | 1000000000000 | No | `services/interop/crates/x-websearch/src/config.rs` |
| `x-websearch/config.json (--config) :: payment.payer_address` | Fixed metered payment payer; otherwise read per request; Applies when payment is configured | Unset; request supplies the payer | No | `services/interop/crates/x-websearch/src/config.rs` |
