# Hosted beta operations

`layerx-testnet-control` is the private-network status/parameter boundary and the private funding/reset proxy. The merged network runs one `paxd` chain with an integrated kernel. The control executable probes exactly these eight dependencies; configure each URL for the port its Service exposes.

| Dependency | Service and manifest |
| --- | --- |
| `identity` | `identity`, `services/platform/hosted/identity/deployment.yaml` |
| `faucet` | `faucet`, this package's `deployment.yaml` |
| `core` | `core`, `services/platform/hosted/node/deployment.yaml` |
| `core_admin` | `core-admin`, `services/platform/hosted/node/deployment.yaml` |
| `receipt_authority` | `receipt-authority`, `services/platform/hosted/node/deployment.yaml` |
| `redis` | faucet Redis, this package's `deployment.yaml` |
| `gateway` | `gateway`, `services/platform/hosted/gateway/deployment.yaml` |
| `paxeer` | `paxeer-boundary` (9443), tracked boundary workload `paxeer` in `services/platform/hosted/paxeer/deployment.yaml` |

The tracked Paxeer manifest also declares `paxeer-observer-boundary:9443`, targeting the `observer-https` listener on 9444. Both boundary listeners use the same local native node at `http://127.0.0.1:8545`; the observer endpoint does not represent an independent chain. The deployment command validates and emits this boundary manifest; `node/deployment.yaml` remains the source of native node facts.

`GET /readyz` reports those dependencies and exactly three journeys. Each journey is ready only when every dependency in its row is reachable. Its admission endpoint returns HTTP 200 with `admitted: true` when ready, or HTTP 503 with `admitted: false` and the failing dependency names otherwise.

| Admission endpoint | Status journey | Dependencies |
| --- | --- | --- |
| `/v1/journeys/funding` | `funding` | `identity`, `faucet`, `redis`, `core_admin`, `core` |
| `/v1/journeys/payment` | `payment` | `identity`, `gateway`, `core`, `receipt_authority` |
| `/v1/journeys/receipt-inspection` | `receipt_inspection` | `gateway`, `receipt_authority`, `core` |

Global readiness additionally requires every dependency and journey to be ready and the package semantic version and LXP wire version to match the pending release. Journey admission checks its own dependency set; a release mismatch or a dependency used only by another journey can still leave that journey admitted. `GET /v1/parameters` supplies the release identity, protocol `network_id` 402 and `evm_chain_id` 125. Use `evm_chain_id` for EVM signing. `platform-hosted-topology-check` checks configured Service ports and NetworkPolicy. Provision the merged node genesis, retained hosted services and identity smoke principals before scheduling journeys.

Apply `deployment.yaml` only after provisioning the referenced TLS, outbound CA, identity-client, Redis ACL/client, backend-admin, control-admin and status-publisher Secrets. The default control and faucet Services are ClusterIP-only and install no public DNS or Ingress. An operator-provisioned private edge must preserve the TCP source IP and must not inject forwarding headers into faucet requests. The control admin port is ClusterIP-only with NetworkPolicy ingress limited to the faucet and reset operator. The protocol core and emulator administration routes are not part of either public service.

The Redis StatefulSet uses TLS, ACL authentication, an AOF with `appendfsync always`, and a persistent volume. All faucet replicas reserve identity, address and transport-peer quotas with one Redis transaction keyed by a mandatory idempotency key. Retrying an indeterminate funding result repeats the same private funding identifier; a definite funding refusal atomically releases the quota reservation.

The reset CronJob runs each Tuesday at 09:00 UTC, while `reset-testnet.sh` executes only on days 1 through 7 and uses one idempotency key per calendar month. The status CronJob publishes only the redacted four-component document from `/v1/status`. Use `platform-hosted-smoke` from scheduled CI to fund an account, commit a payment, fetch its receipt and verify it independently; this requires real hosted credentials and is not a deployment health probe.

The smoke entry point is `tests/hosted-smoke.sh`. Supply HTTPS origins in `LAYERX_TESTNET_URL`, `LAYERX_GATEWAY_URL` and `LAYERX_FAUCET_URL`, and the trusted CA PEM in `LAYERX_TEST_CA_FILE`. Supply separate identity sessions in `LAYERX_TEST_AUTH_TOKEN_FILE` and `LAYERX_TEST_DESTINATION_AUTH_TOKEN_FILE`, the corresponding `LAYERX_TEST_SOURCE_DID`, `LAYERX_TEST_SOURCE_PUBLIC_KEY`, `LAYERX_TEST_DESTINATION_DID` and `LAYERX_TEST_DESTINATION_PUBLIC_KEY`, and the source secp256k1 signer in `LAYERX_TEST_SOURCE_KEY_FILE`. The signer must match the source identity's bound EVM address.
Supply `LAYERX_TEST_ACCOUNTS_FILE` with the explicit DID to EVM smoke account bindings in `smoke-accounts.json`, written by `tests/smoke_accounts.py`.
Supply `LAYERX_TEST_SOURCE_EVM_KEY_FILE` with the paying EVM signer's PEM key file, `test-source-evm.key`, written by the same helper.
Secret files must be owned by the caller, mode 0600, regular files with one link. The credential step writes separate source/destination session curl configurations and a signer-bound gateway key configuration.

Set `LAYERX_TEST_ASSET=uhpx` and `LAYERX_TEST_AMOUNT` to a positive canonical integer in native micro-units. `tests/payment-journey.py` accepts `--evm-chain-id 125`, checks `eth_chainId`, funds both bound identities through `px_requestFunds` and reads `px_getBalances`. It signs an EIP-1559 transaction to the native bank at `0x0000000000000000000000000000000000001001`, calling `sendNative(string)` with the destination Pax address and converting each `uhpx` to `10^12` EVM value units. Submission uses `eth_sendRawTransaction`; completion requires `pax_sequencerReceipt` at the `committed` rung, exact balance/fee and nonce checks, and replay refusal with `already known`. Supply `LAYERX_TEST_SEQUENCER_PUBLIC_KEY` independently for the payment verifier. The smoke caller takes `--evm-chain-id` from `.evm_chain_id` in the parameters response.

For node receipt inspection, supply the HTTPS root node RPC origin in `LAYERX_TEST_NODE_URL` and an independently trusted `px_getReceiptKeys` keys object in `LAYERX_TEST_RECEIPT_KEYS_FILE`, with numeric `firstHeight`/`lastHeight` validity bounds. `tests/receipt_inspection.py --node ... --ca ... --receipt-keys ... --payment ... --output ...` consumes the payment directory's `payment.raw`, `receipt.json` (`{"result": <node receipt>}`) and `result.json`. It fetches `pax_sequencerReceipt` by transaction hash and binds the signed receipt to the raw transaction, pinned key and validity height, result code, fee, execution summary and logs. It requires `rung: committed`, a nonzero block hash and a block number at least the signed height; it refuses a corrupted signature and re-fetches the unchanged receipt after replay. `verification.json` reports `verified: true`, `result_code: 0`, `rung: committed` and `verification_level: sequencer-signed`. This proves the sequencer signature and node-reported committed state; independent Comet consensus proof verification requires additional evidence. Retain the smoke evidence directory privately.
