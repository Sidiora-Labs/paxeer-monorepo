# Hosted Human service

This directory contains Kubernetes manifests and provisioning helpers for the
off-chain Human API, components, identity/security providers, KMS and owner
agent. Service crates live under
[`services/human/crates`](../../../human/crates). Shell entry points are
[`material.sh`](material.sh), [`provision.sh`](provision.sh) and
[`onboarding_provision.sh`](onboarding_provision.sh), sourced by the
[cluster script](../tests/beta-cluster.sh). The website has its own
[`layerx-human-web` Deployment](web-deployment.yaml).

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID `125`
(`0x7d`). The kernel is part of the network and proposes blocks. A receipt is
final when its block is committed. Human provisioning registers identity and
recovery state, opens an asset account and funds it on this same chain.
Use HTTPS JSON-RPC at the node root path and check `eth_chainId` before signing. See
[Public RPC endpoints](../../../../docs/site/docs/reference/public-rpc.md).
Node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `px`, `test` and `txpool`.
The `px` namespace answers from node state; `px_register` and
`px_requestFunds` are operations of the hosted identity and faucet
services declared in [the gateway OpenRPC contract](../gateway/openrpc.json).
Sub-chains are tenant domains in the database, launchable without new chain
infrastructure or a separate public endpoint.

The [node manifest](../node/deployment.yaml) defines the `paxd` StatefulSet,
with JSON-RPC on port `8545`, consensus RPC on `26657` and health on `9460`.
The [`layerx-human` Service](deployment.yaml) selects node pods and forwards
HTTPS port `9443` to the named `human-https` port. The Human runtime listener
must be supplied by a workload matching that binding; the node manifest defines
only the native node container. This directory also supplies an
[owner provisioning Job](provision-owner-job.yaml) and the web Deployment.

Owner provisioning mounts the `layerx-human-state` PVC's `identity` subPath.
Preserve the provider state, seal, keys and recovery policy together. Projected
credentials are copied into private regular files before use.

See [HostedHuman.md](../../../../docs/wiki/HostedHuman.md) for related cluster
material and [RegistryDeploymentJournal.md](../../../../docs/wiki/RegistryDeploymentJournal.md)
for journal `pairs/`. The contracts below describe the merged owner
provisioning path.

Generated owner-agent settings publish `LAYERX_AGENT_NODE_RPC_URL` from the
components' HTTPS `PAXEER_RPC_URL` and `LAYERX_AGENT_NODE_CHAIN_ID` from the
configured EVM chain ID. The current agent reads these settings, checks the
remote chain ID and consumes committed receipts through root JSON-RPC. Its
node dependency no longer uses the retained local daemon socket setting.

## Committed account observations and bank receipts

Use the merged node's root JSON-RPC endpoint for chain observations. Owner
bootstrap checks `eth_chainId`, reads the sender nonce with
`eth_getTransactionCount` at `pending`, submits signed bytes through
`eth_sendRawTransaction` and checks `eth_getTransactionReceipt` against
`eth_getBlockByNumber`. A transaction hash alone does not establish completion.
A successful receipt must belong to its canonical block and contain the exact
expected events. Committed blocks are final on this network.

Account and authority evidence comes from the committed identity/recovery
snapshot, `getLayerXDid(address)`, the asset-specific
`accountId(bytes32,bytes32)` and `balanceOf(bytes32,bytes32)` reads. Bootstrap
requires a funded balance before first publication; later balance observations
can change after spending. Re-reading retained receipts proves the original
effect, not a permanently unchanged balance or zero consumed limits. Missing,
stale or mismatched observations must refuse provisioning or planning.

Current Human intent sends use the bank precompile
`0x0000000000000000000000000000000000001001`:
`send(address,address,string,uint256)` (selector `0x5c05961b`) binds the sender,
resolved recipient, denomination and amount. The service matches the requested
asset to the gateway's `denom:<denomination>` inventory and balance row; it
never accepts a caller-supplied balance as authority. Plans expose chain ID,
calldata and pending EVM nonce, and submission re-plans against current state
and checks the signed digest. These bank sends are distinct from the ledger
`payExact` transaction used by the owner bootstrap below.

Read progress through `GET /v1/journeys/{journey_id}`: `submitted` acknowledges
submission, `receipted` verifies execution and `committed` verifies block
inclusion. The retained `checkpoint-finalised` label means committed evidence
on this same chain; it does not require another finality service. The production
EVM intent path waits for a verified committed receipt for each leg before
advancing. See the [intent contract](../../../human/schema/human-api/intent.kvx)
and [journey contract](../../../human/schema/human-api/journeys.kvx).

## Material generation

[`material.py`](material.py) assembles and verifies the
`layerx.human.owner-bundle.v3` bundle from protected evidence, the
network/chain-bound `paxeer/deployment.json` and a version-2 module registry.
The deployment record is checked for `network_id` and `chain_id`.
Payment addresses are the fixed ledger and payment precompile addresses.

```sh
python3 services/platform/hosted/human/material.py --assemble \
  "$WORK_DIR/human-evidence" "$WORK_DIR/paxeer/deployment.json" \
  "$SECRETS_DIR/module-registry.json" "$WORK_DIR/human-policy/policy.json" \
  "$NODE_NETWORK_ID" "$PAXEER_CHAIN_ID"
```

The output directory must already exist, be owned by the invoking UID and have
mode `0700`; the output filename must be `policy.json`. The bundle contains
`policy.json`, `bundle-manifest.json`, copied `inputs/`, paired `journal/`
records and optional `onboarding/` material. Verification checks hashes and
bindings and re-reads owner receipts from the authenticated chain. An unavailable
RPC or changed retained receipt refuses assembly.

`human_policy_publish` uses
`LAYERX_BETA_HUMAN_POLICY_FILE="$WORK_DIR/human-policy/policy.json"`, verifies the
bundle, projects it into Secret `layerx-human-policy`, generates per-process
configuration and applies the Human Secrets. Bundle publication requires the
complete evidence set and matching network, chain and owner bindings.

## Integrated evidence production

[`owner_native.py`](owner_native.py)'s `produce` delegates to
[`merged_owner.py`](merged_owner.py). `human_native_owner_prepare` copies
protected `human-evidence-input/owner-evm.json` to `owner-native.json` after
validating the exact schema, network/chain bounds, HTTPS CA and protected
signing/funding keys; retained configuration must match exactly. The workload,
private mounts and recovery policy are returned unchanged; owner preparation
does not add a daemon, seed account state or edit genesis. The producer
is exposed through:

```sh
python3 services/platform/hosted/human/provision.py \
  --produce-owner-registration --work-dir "$WORK_DIR"
```

This command submits real transactions. Prepare the original owner result,
recovery policy and signed guardian set first. `owner-native.json` has exactly
these keys:

| Keys | Meaning |
| --- | --- |
| `rpc_url`, `rpc_ca_file` | Authenticated HTTPS node JSON-RPC URL and protected PEM CA file, at most 64 KiB |
| `network_id`, `chain_id`, `confirmations` | Positive network ID, EVM chain ID and receipt observation depth; mainnet uses chain ID `125` |
| `owner_seed_file`, `evm_key_file` | Protected 32-byte Ed25519 owner seed and EVM signing-key file |
| `asset_id`, `funding_source`, `funding_amount`, `funding_key_file` | Registered asset, existing ledger source account, positive U128 funding amount and funder's EVM signing-key file |

Asset and source IDs are nonzero lowercase 64-digit hex strings without a
prefix. Key and CA paths must satisfy protected-file checks. The original
`human-owner-result.json` DID must equal `did:layerx:<owner public key>`.
`recovery-policy.json` must equal the original result's root, threshold and
delay. `recovery-guardians.json` must prove the same root from signed, separately
enrolled guardians. Mismatched identity or recovery material refuses provisioning.

The producer uses these EVM precompiles with zero native transaction value:

| Address | Bootstrap operation |
| --- | --- |
| `0x0000000000000000000000000000000000001004` | `bindLayerX(bytes32,bytes)` binds the EVM sender to the owner's Ed25519 key |
| `0x000000000000000000000000000000000000101c` | `recordLifecycle(bytes,bytes32)` records identity and recovery policy |
| `0x000000000000000000000000000000000000101a` | `openAccount(bytes32)` opens the asset account; `payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)` funds it from the configured ledger source |

Signers need transaction gas; the funder also needs authority over its ledger
source. The producer checks successful receipts, canonical block inclusion,
exact events, owner binding and the asset account. Before first registration
publication it also checks the funded balance. `confirmations` adds an
observation-depth check; finality still comes from BFT block commitment.

Signed transactions are saved under `human-evidence-input/owner-native-run`
before broadcast. A restart reuses each retained transaction and checks its
intent, hash and receipt. Preserve this directory after a timeout or refusal;
do not reset it or create a new owner to retry an uncertain submission.

## Offline owner provisioning

With the identity provider stopped, `layerx-human-identity-provider provision-owner`
uses `LAYERX_HUMAN_IDENTITY_PROVIDER_STATE_ROOT` and
`LAYERX_HUMAN_IDENTITY_PROVIDER_RECOVERY_POLICY_FILE`, as does `bind-device`.
It acquires the provider's exclusive state lock and calls LXIP operation 1.
Stdin JSON has exactly `email`, `display_name`, `idempotency_key` and `now`
(unsigned seconds), at most `16384` bytes. Repeating the same idempotency key
and identity returns the durable owner; conflicting inputs refuse.

Single-line JSON stdout has exactly `principal`, `did`, `recovery_root`
(a 32-byte integer array), `recovery_threshold` and `recovery_delay_seconds`.
The established recovery policy has exactly `root`, `threshold` and
`delay_seconds`; threshold and delay are positive. Preserve these values and
the provider state for runtime use. This local operation does not create a
ledger account, registration receipt or authority policy. The merged producer
independently checks that the original DID matches its binding key.

[`provision.py`](provision.py)'s `--validate-job-input` validates the protected
request and recovery policy. `--validate-owner-result --request RESULT`
checks the exact single-line five-field result and policy equality. Both take
`--work-dir "$WORK_DIR"`. Its `--preserve-binding` mode takes `--request`,
`--response` and `--output` paths and preserves the response tenant and matching
subject only after checking them against the creation request. Output creation
is exclusive.

## Owner registration input contract

The merged producer publishes `human-evidence-input/owner-registration.json`
with exactly `schema`, `owner_account`, `authority` and `identity`.
`schema` is `layerx.human.merged-owner.v1`. The account and authority are nonzero
lowercase 64-digit hex strings; authority is the owner's Ed25519 public key.
The account is the ledger's asset-specific account derived from
`agent:did:layerx:<public key>:asset:<asset id>` using
`SHA256("LX:ACCOUNT:v1" || u32be(len) || name)`, where `name` is the UTF-8
account reference and `len` is its byte length.

The identity object has exactly `did`, `authorities`, `revocation_sequence`,
`frozen`, `evidence`, `capabilities` and `recovery`. It contains one
`primary_key` authority matching the public key, `frozen: false`, and an empty
capability list. The bootstrap proves recovery through the committed
`IdentityStateUpdated` event. It does not advertise a rotation policy that this
producer cannot establish. Recovery has `policy_revision`,
`required_delay_seconds`, `maximum_delay_seconds`, `effective_sequence` and
`evidence`; evidence references have `activity_id` and `receipt_digest`.
See the [receipt authority](../authority/README.md) for the consuming service;
the merged registration schema is enforced in `merged_owner.validate_registration`.

Alongside registration, the producer writes `merged-owner-evidence.json`
(raw signed transactions and receipts for `binding`, `identity`, `recovery`,
`account`, `funding`), `native-context.json` (network, chain, URL, CA SHA-256
and observation depth) and `rpc-ca.pem`. Consumers re-query the authenticated
RPC and compare retained receipts and event payloads. Structural validation:

```sh
python3 services/platform/hosted/human/provision.py \
  --validate-owner-registration --work-dir "$WORK_DIR"
```

does not perform those chain checks. Bundle assembly and verification do.
Protected JSON inputs must use absolute canonical paths, be regular files owned
by the invoking UID, have mode `0600` and one link, and be nonempty and at most
1 MiB. Duplicate and unknown schema fields refuse validation.

## Purpose catalog

Generate the catalog from [`beta-purpose-catalog.json`](beta-purpose-catalog.json),
the v2 module registry, selected asset and protected treasury/sequencer
counterparty account exports:

```sh
python3 services/platform/hosted/human/provision.py --catalog \
  --work-dir "$WORK_DIR" --registry "$SECRETS_DIR/module-registry.json" \
  --treasury "$WORK_DIR/human-evidence-input/treasury.json" \
  --sequencer "$WORK_DIR/human-evidence-input/sequencer.json" \
  --asset "$NODE_ASSET_ID" --output "$WORK_DIR/purpose-catalog.json"
```

The identifier-free template is not a loadable runtime catalog. Generation
adds counterparties and asset IDs as 32-byte integer arrays and preserves
`call_selectors` and bounded amount, rate, session and budget settings.
`material.authorize_call` checks selector, recipient, asset and amount.
Counterparty export filenames do not imply another execution service.
Payments use the merged ledger at `0x101a`; budgets use the payment precompile
at `0x000000000000000000000000000000000000101b`.

## Evidence provisioning

`human_evidence_provision` orders merged owner preparation and registration,
authenticated ledger funding validation, naming/registry deployment and journal
export, producer-manifest validation, evidence assembly and the policy graph
input check. It runs before `human_policy_publish`. Account observations bind
the owner and asset to committed node state, as described below.

The assembled `$WORK_DIR/human-evidence` set contains:

| Published file | Source and meaning |
| --- | --- |
| `components.json` | `AGENT_ACTOR`, `AGENT_AUTHORITY`, `AGENT_OWNER_ACCOUNT`, unpadded-base64url `AGENT_RECOVERY_ROOT` and `AGENT_RECOVERY_THRESHOLD` |
| `authority.json` | Identity response tenant/principal binding and positive `core-clock-horizon` |
| `agent.json` | Same binding in `HUMAN_PEERS`, agent-scoped limit and bootstrap consumed result described above |
| `principal-policy.json` | Registration identity/recovery evidence, configured budgets, account and registered asset |
| `recovery-policy.json` | Unchanged original owner root, threshold and delay |
| `purpose-catalog.json` | Template plus actual registry asset and counterparty exports |
| `movement-policy.json` | Only `LEDGER_ASSET_ID` and `LEDGER_OWNER_ACCOUNT`, both 0x-prefixed H32s |
| `producer-records/` | Original binding/result/registration, naming deployment, counterparty exports, RPC context, CA and merged receipts |
| `journal/` | Unmodified paired admission/deployment records from `LAYERX_REGISTRY_JOURNAL` |

The `movement-policy.json` filename is retained, but its contents bind the
merged ledger account. The material builder adds `PAXEER_LEDGER_CONTRACT` and
`PAXEER_PAYMENT_CONTRACT`. Retired custody references, checkpoint timing,
guarantor keys and settlement contract addresses are not assembly inputs.

`HUMAN_PEERS` must be exactly
`uid=4020;tenant=<tenant>;principal=<principal>`, matching the authority binding.
The producer accepts tenants of 1–128 lowercase ASCII letters, digits, hyphens
or underscores. Principals use `did:<method>:<id>`, with a nonempty method of
lowercase ASCII letters or digits and a nonempty identifier, at most 255 UTF-8
bytes overall; whitespace, control characters, commas and semicolons refuse.
Tenancy must remain explicit through the identity, policy and peer binding.

[`beta-owner-policy.json`](beta-owner-policy.json) defines an agent-scoped
configuration limit whose scope ID is the registration account. The limit ID
is 16 bytes encoded as 32 lowercase hex digits. Activity membership comes from
registration evidence; the default empty budget allowlist grants no budgets.
A configuration limit is not a deployed on-chain budget.

Journal packaging accepts 2–128 records with paired
`<receipt-digest>.admission` and `.deployment` names and at most 512 KiB total.
Evidence publication uses a private sibling directory, fsync and rename under
an exclusive lock. Identical retained sets can be reused; changed or incomplete
sets refuse rather than overwriting protected bindings.

Verify an already assembled bundle with:

```sh
python3 services/platform/hosted/human/material.py --verify-bundle \
  "$WORK_DIR/human-policy" "$NODE_NETWORK_ID" "$PAXEER_CHAIN_ID"
```

The broader `provision.py --qualify-generated-set` helper also runs the material
installer and `test_material.py`; it is separate from structural registration
validation and requires all live producer inputs. Its arguments are
`--work-dir`, `--registry`, `--secrets-dir`, `--network` and `--chain`.

## Pinned chain receipt material and recovery

`history_material.py WORK_DIR MATERIAL_DIR NETWORK_ID CHAIN_ID` validates the
merged funding receipts against `human-evidence-input/owner-native.json`,
including the independently supplied HTTPS URL, CA fingerprint, network, chain
and observation depth. It publishes only `native-context.json`,
`merged-owner-evidence.json`, `rpc-ca.pem` and `owner-registration.json` into the
private `agent`, `security` and `authority` directories. The material installer
compares these files with the verified bundle, validates the actual receipt
path again and refuses partial or mismatched retained projections.

`provision.py --funding-evidence --work-dir WORK_DIR` validates these same
receipts, the configured funder, asset and exact amount. The producer manifest
calls this input `ledger-funding`; it points to `merged-owner-evidence.json`.
There is no separate movement-source input. The retained `movement-policy.json`
output is an adapter projection derived from the verified owner account and
asset; its exact schema is enforced before bundling. Changing the funder or
amount requires reconciliation with the original retained transactions.

Material publication validates the retained inventory and authenticated chain
evidence. Preserve the seal, private keys, encrypted provider state, guardian
bindings and recovery policy together. A resumed secrets-generation call
validates the full retained inventory and preserves the KMS seal.

`owner_kms.prepare` gives its real `prepare` child ten seconds and at most
65536 bytes of combined stdout/stderr. It kills and reaps the process group on
expiry, excess output or failure, and publishes no owner result from that run.
The error instructs the operator to retain custody and retry the same request.
The KMS's private state and original idempotency key remain the recovery source;
a timeout does not authorize a new owner or regenerated keys.

## Committed account observations

`human_owner_provision` runs the offline identity and counterparty jobs with
exclusive access to identity state. The merged owner producer then registers
and funds the asset account. `human_account_head_observe` runs
`validate-account-head` separately with the authenticated HTTPS RPC, CA,
chain ID, owner DID, account, asset and maximum observation age.

The `layerx.human.account-head.v1` result contains `schema`, `chain_id`,
`owner_did`, `account`, `asset_id`, `committed_height`, `block_hash`, `state_root`,
`observed_at`, `balance` and `consumed`. Provisioning requires a positive
committed height and `consumed = 0` for the initial limit. It retains the full
observation in `account-observation.json` and its consumed result in
`account-head-result.json`. A retained account's later observations must reflect
its actual balance and usage.

`registry_kernel_material` reads an authenticated signed registry generation
and verifies its network, signer and replica bindings before publishing the
registry Secret. The registry input is separate from owner account observation.
