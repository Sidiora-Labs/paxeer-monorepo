# Human service

`layerx-human-service` implements the person-facing HTTP API and privileged
components for passkey authentication, native onboarding, bank-transfer journeys,
managed agents, approvals, activity and security settings. Custody signs the
authorized native calls. The [Human API schema](../../schema/human-api/README.md) defines the versioned HTTP
contract. Principal and tenant authorization precede operation dispatch;
caller aliases and currency labels are selectors, not signing authority.

Paxeer mainnet-beta is one BFT network with EVM chain id **125** (`0x7d`). The
kernel is part of the network and proposes blocks. A receipt becomes final when
its block is committed. Tenant domains are database namespaces. Human services
consume that network's transaction and receipt paths.

## Processes and dependencies

| Binary | Responsibility |
| --- | --- |
| `layerx-human-service` | HTTP listener; forwards authenticated work to the component socket |
| `layerx-human-components` | Privileged production composition, principal stores and maintained component listener |
| `layerx-human-kms-gateway` | LXKP gateway for the configured custody provider |
| `layerx-human-onboarding` | Onboarding administration and sponsorship commands |

The default binary is in [src/main.rs](src/main.rs); the other three are in
[src/bin](src/bin). The components use the
[identity provider](../layerx-human-identity-provider/README.md),
[security provider](../layerx-human-security-provider/README.md), agent boundary
and either the [remote KMS](../layerx-human-kms/README.md) or configured attestor
custody backend. There is no movement-provider process to deploy.

## Money movement and signing

The active bank-transfer executor serves `POST /v1/moves/quote` and
`POST /v1/moves`. Wallet binding/rebinding, deposits, withdrawals and emergency
exits remain retired compatibility operations: their declared HTTP routes return
**410 `operation-retired`**. They are not setup steps or readiness dependencies.
Do not configure a custody bridge, withdrawal precompile or independent RPC-vote
threshold for those retired journeys.

The production bank-movement quote resolves the signed-in owner's custody
binding and wallet, recipient, asset denomination and observed balance. It stores
the exact quote in that principal's `Journeys` table, including tenant, binding,
identity digest, wallet, validity window and EIP-1559 fields. Commit reloads it
from that scope, checks its digest and bindings, rejects an expired new commit,
and binds the retained journey to its commit idempotency key. Signed bytes are
retained for retry rather than preparing a different transaction after submission.
See [production_components.rs](src/server/production_components.rs).

Bank sends are zero-value EIP-1559 calls on chain **125** to
`0x0000000000000000000000000000000000001001`, using
`send(address,address,string,uint256)`. The disclosure binds sender, recipient,
denomination and amount. The executor reads the sender's **pending nonce** using
`eth_getTransactionCount` with the `pending` block tag; the kernel account sequence
is not an EVM nonce. Multi-leg plans bind consecutive pending nonces and reject
nonce mismatches. Retries reconcile the retained transaction hash and signed bytes
with receipt lookup rather than assigning a fresh nonce to an accepted send.
The KMS action path checks the exact plan, reserves the wallet nonce and journals the signed type-2 transaction before
returning success. Its restricted executor certificate cannot register a new
plan authorization. The version-3 send operation returns signed transaction
bytes; the version-7 Sign operation returns a signature over the checked unsigned
transaction. Their encodings and recovery rules are in the
[KMS contract](../layerx-human-kms/README.md).

Retained typed intents are compiled by `layerx-intents`. The agent runtime adapter
checks the prepared precompile target, calldata, nonce and fee ceiling against
the authorized intent before signing. EIP-1559 transaction encoding is handled
by the shared crypto and custody paths, rather than by a second intent compiler.
The adapter is in [agent_runtime.rs](src/server/agent_runtime.rs); intent binding
checks are in [intent_submit.rs](src/journeys/intent_submit.rs).

The onboarding sponsor reads a chain-125 registry, requests gas-station
sponsorship and requires a successful matching `LayerXBound` log before its
submitted operation can report completion. A submission-only reply remains
incomplete. See [onboarding_sponsor.rs](src/server/onboarding_sponsor.rs).

## Configuration

This table locates the main configuration boundaries; it is not a complete
deployment environment. Required fields and bounds are read by
`ProductionComponentsConfig::from_environment` and its helper parsers in
[production_components.rs](src/server/production_components.rs).

| Variables | Meaning |
| --- | --- |
| `LAYERX_HUMAN_BIND`, `LAYERX_HUMAN_LISTENER` | HTTP bind address and `tls` or `plain` listener mode; TLS is the default |
| `LAYERX_HUMAN_TLS_CERT_DER`, `LAYERX_HUMAN_TLS_KEY_DER` | Required server identity for TLS mode |
| `LAYERX_HUMAN_COMPONENT_SOCKET` | Private component socket used by the HTTP service and component process |
| `LAYERX_HUMAN_STORE_ROOT`, `LAYERX_HUMAN_CUSTODY_ROOT`, `LAYERX_HUMAN_AUTH_INDEX_ROOT` | Absolute durable-state roots |
| `LAYERX_HUMAN_TENANCY_DIGEST` | Installed principal-to-tenant mapping digest |
| `LAYERX_HUMAN_AUTH_INDEX_KEY`, `LAYERX_HUMAN_STREAM_CURSOR_KEY` | Authentication index and stream cursor keys |
| `LAYERX_HUMAN_RP_ID`, `LAYERX_HUMAN_WEB_ORIGIN` | Passkey relying party and allowed browser origins; defaults are `paxportwallet.com` and `https://paxportwallet.com,https://api-mainnet-beta.paxeer.network` |
| `LAYERX_HUMAN_RP_NAME`, `LAYERX_HUMAN_ORIGIN` | Required passkey display name and ceremony origin |
| `LAYERX_HUMAN_AGENT_SOCKET` | Agent boundary socket; its frame, connection, stream, queue and deadline limits use the same prefix |
| `LAYERX_HUMAN_IDENTITY_SOCKET`, `LAYERX_HUMAN_SECURITY_SOCKET` | Local provider sockets; each has its own deadline and frame bounds, and identity also requires peer UID/GID |
| `LAYERX_HUMAN_NETWORK_ID` | Network binding checked against the EVM node chain id; 125 for mainnet-beta |
| `LAYERX_HUMAN_PROTOCOL_VERSION` | Native custody protocol version 3; unset selects 3 and legacy version 2 is refused |
| `LAYERX_HUMAN_EVM_GAS_LIMIT` | Authorized EVM call gas limit |
| `LAYERX_HUMAN_EVM_MAX_FEE_PER_GAS` | Exact maximum gas price in wei |
| `LAYERX_HUMAN_EVM_MAX_PRIORITY_FEE_PER_GAS` | Exact priority fee in wei, no greater than maximum fee |
| `LAYERX_AGENT_NODE_RPC_URL`, `LAYERX_AGENT_NODE_CHAIN_ID` | Node JSON-RPC URL and expected EVM chain id for direct transaction/receipt reads |
| `LAYERX_HUMAN_ONBOARDING_REGISTRY_FILE` | Sponsor JSON with `chain_id: 125` and optional `gas_station` HTTPS origin |
| `LAYERX_HUMAN_GAS_STATION_ENDPOINT` | Required sponsorship endpoint when the registry does not supply it |
| `LAYERX_RUNTIME_CLOCK_SOCKET`, `LAYERX_RUNTIME_CLOCK_PID`, `LAYERX_RUNTIME_CLOCK_UID` | Authenticated runtime-clock supervisor bindings |

The public node URL uses the root path, for example
`https://api1.mainnet-beta.paxeer.network`. The
[workspace guide](../../README.md) lists the public node names and distinguishes
node namespaces from hosted gateway `px_*` methods. Gateway projections carry
`gateway-reported` evidence; they are not proof of a committed balance.

Remote KMS settings are parsed by `RemoteKmsConfig::from_environment` in the
production composition module: `LAYERX_HUMAN_KMS_PROVIDER_REFERENCE`,
`LAYERX_HUMAN_KMS_ENDPOINT`, `LAYERX_HUMAN_KMS_SERVER_NAME`,
`LAYERX_HUMAN_KMS_ROOT_CERTIFICATE_DER`,
`LAYERX_HUMAN_KMS_CLIENT_CERTIFICATE_DER` and
`LAYERX_HUMAN_KMS_CLIENT_PRIVATE_KEY_DER`, plus the
`LAYERX_HUMAN_KMS_MAX_FRAME_BYTES`, `LAYERX_HUMAN_KMS_MAX_CONNECTIONS`,
`LAYERX_HUMAN_KMS_MAX_STREAMS`, `LAYERX_HUMAN_KMS_MAX_QUEUED_BYTES` and
`LAYERX_HUMAN_KMS_DEADLINE_SECONDS` bounds. Setting
`LAYERX_HUMAN_ATTESTOR_NODES` selects its attestor custody backend instead. That backend requires the signer list, trust
certificate, client certificate/private key and deadline under
`LAYERX_HUMAN_ATTESTOR_*`; it resolves the owner's wallet using the owner assertion.
The remote KMS provider has its own principal-bound attestor file configuration.
Do not interchange those two configuration formats. Authentication policy,
retention, signing limits, identity-binding reader settings and component worker
limits are also mandatory where their respective configuration parsers require
them.

## Receipt evidence and development

A transaction hash proves submission only. Economic finality requires
**Committed** receipt evidence; **Submitted** and **Receipted** observations alone
do not establish it. Receipt paths distinguish
`Submitted`, `Receipted` and `Committed` observations, validate receipt signatures
and bind evidence to the retained transaction. For bank journeys,
`BankCommitEvidence::authenticate` verifies the governed receipt key, signed
sequence batch, trusted validator quorum, committed block and transaction inclusion
before returning a `BankCommittedReceipt`. A receipt signature alone is
insufficient. The network's finality comes from its BFT commit, not a count of independent JSON-RPC responses.

Service integration suites use
[layerx-human-test-support](../layerx-human-test-support/README.md). From the
repository root, these focused targets run with in-process clocks and loopback or
recorded peers; they need no credentials, live node, native artifact or external
clock process:

```sh
cargo test --locked --manifest-path services/human/crates/layerx-human-service/Cargo.toml --test repair_22_33_45
cargo test --locked --manifest-path services/human/crates/layerx-human-service/Cargo.toml --test agent_runtime_eip1559
cargo test --locked --manifest-path services/human/crates/layerx-human-service/Cargo.toml --test agent_runtime_nonces
```

`repair_22_33_45` checks this README against the production bank-call producer,
transaction/disclosure consumer, pending-nonce transport, committed receipt verifier,
configuration parsers and retired operation inventory. Its receipt peer replays the
committed node transcript retained under `src/journeys`.

Production still requires its authenticated runtime-clock supervisor. Broader suites
that exercise native custody or hosted dependencies need the prerequisites in the
root Makefile's `human-test` targets; the clock wrapper alone does not supply them.
See the [workspace guide](../../README.md) for broader developer targets.
