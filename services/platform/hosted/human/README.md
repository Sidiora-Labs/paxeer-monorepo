# Hosted Human service

This directory contains Kubernetes manifests and provisioning helpers for the
off-chain Human API, components, identity/security providers, ledger movement
adapter, KMS and owner agent. Service crates live under
[`services/human/crates`](../../../human/crates). Shell entry points are
[`material.sh`](material.sh), [`provision.sh`](provision.sh) and
[`onboarding_provision.sh`](onboarding_provision.sh), sourced by the
[cluster script](../tests/beta-cluster.sh). The website has its own
[`layerx-human-web` Deployment](web-deployment.yaml).

Paxeer mainnet-beta is one BFT network running `paxd`, with EVM chain ID `125`
(`0x7d`). The kernel is part of the network and proposes blocks. A receipt is
final when its block is committed. Human provisioning registers identity and
recovery state, opens an asset account and funds it on this same chain.
The pre-merge execution/settlement split, standalone `layerxd` daemon, router and guarantor finality are retired.

Use HTTPS JSON-RPC at the root of `https://api-mainnet-beta.paxeer.network` or
`api1.mainnet-beta.paxeer.network` through `api14.mainnet-beta.paxeer.network`;
check `eth_chainId` before signing. See
[Public RPC endpoints](../../../../docs/site/docs/reference/public-rpc.md).
Node namespaces are `debug`, `echo`, `eth`, `net`, `pax`, `test` and `txpool`.
Hosted gateway `px_` methods are defined by
[`gateway/openrpc.json`](../gateway/openrpc.json), not implied by the public
node contract. `node.hyperpaxeer.com` is only the hpx library mirror.
Sub-chains are tenant domains in the database, launchable without new chain
infrastructure or a separate public endpoint.

The checked-in [node manifest](../node/deployment.yaml) co-locates Human
containers with the merged node. The [`layerx-human` Service](deployment.yaml)
forwards HTTPS port `9443` to `human-https` (`9447`). Components and providers
run as UID/GID `4020`; provider sockets are
`/run/layerx/human/{identity,security,movement}.sock`. The owner runs as UID
`4021`, GID `4020`, with its socket under `/run/layerx/human/owner`; KMS uses UID
`4026` and listens on `127.0.0.1:9450`. These sockets connect local services.
The manifest sets `shareProcessNamespace: false`; it does not establish shared
peer-PID visibility or successful owner transport admission.

The retained `layerx-human-state` PVC holds separate process state subPaths.
Runtime containers drop capabilities and use read-only root filesystems.
Projected credentials are staged into private memory volumes as regular `0600`
files. KMS separately pins the components client and movement executor.
Preserve the seal, keys, recovery policy and matching encrypted state together;
regenerating bootstrap secrets is not a state or key migration.

See [HostedHuman.md](../../../../docs/wiki/HostedHuman.md) for related cluster
material and [RegistryDeploymentJournal.md](../../../../docs/wiki/RegistryDeploymentJournal.md)
for journal `pairs/`. The producer contracts below describe the merged owner
path; remaining cluster integration limits are called out explicitly.

## Material generation

[`material.py`](material.py) assembles and verifies the
`layerx.human.owner-bundle.v3` bundle from protected evidence, the
network/chain-bound `paxeer/deployment.json` and a version-2 module registry.
The deployment record is still checked for `network_id` and `chain_id`;
payment addresses come from the live ledger and payment precompiles, rather
than vault, checkpoint, withdrawal-claims or emergency-exit contracts.

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
configuration and applies the Human Secrets. [`bootstrap.py`](bootstrap.py)
removes Human runtime containers and the optional authority configuration group
from the initial node workload. An incomplete evidence set cannot produce a
usable runtime policy.

## Integrated evidence production

[`owner_native.py`](owner_native.py)'s `produce` delegates to
[`merged_owner.py`](merged_owner.py). `human_native_owner_prepare` copies
protected `human-evidence-input/owner-evm.json` to `owner-native.json` after
checking `network_id`; retained configuration must match exactly. The producer
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
`agent:did:layerx:<public key>:asset:<asset id>`, not the old protocol-account
derivation used for bootstrap counterparty exports.

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

## Protected catalog and Job staging

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

The checked-in [`provision-owner-job.yaml`](provision-owner-job.yaml) is an
offline identity helper, not the merged transaction producer. `human_owner_provision`
validates inputs before cluster mutation, refuses enabled Human containers,
requires one scheduled bootstrap pod and pins the Job to that node for the
ReadWriteOnce PVC. The Job uses the runtime's `identity` subPath and state root,
Secret `layerx-human-provision-owner-input`, disabled retries and the image
selected by `image_ref layerx-human`. Results are captured privately; existing
results require reconciliation.

Legacy integration remains: the Job's `validate-account-head` container expects
`account-head-request.json`, checks a signed protocol-3 batch-number-one head
and emits only a zero-consumed result. The evidence assembler still requires
that result. This is a bootstrap-only limitation, not a merged-chain account
observation or a way to provision an existing account. Do not synthesize it
from a balance, substitute a constant, or reset an existing account's consumed
limits. The merged receipt producer does not remove this cluster requirement.

## Evidence provisioning

`human_evidence_provision` orders merged owner preparation and registration,
ledger movement-source generation, naming/registry deployment and journal
export, producer-manifest validation, evidence assembly and the policy graph
input check. It runs before `human_policy_publish`. This ordering does not prove
that every cluster transport has migrated: registry transfer still names the
retired daemon container, and generated owner-agent settings retain pre-merge
LNI transport configuration. These are integration limits, not instructions to
deploy a separate kernel. Applying manifests alone does not qualify the full
Human service path.

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
`PAXEER_PAYMENT_CONTRACT` and emits configuration under
`LAYERX_HUMAN_MOVEMENT_PROVIDER_`. Retired custody references, checkpoint timing,
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
