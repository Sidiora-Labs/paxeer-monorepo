# Hosted Human material

The Human service uses protected identity, recovery, signing and policy
material to authorize operations on the merged Paxeer network. Its service
crates live under `services/human/crates/`; manifests and material assemblers
live under `services/platform/hosted/human/`.

## Owner policy bundle

`material.py` assembles and verifies schema `layerx.human.owner-bundle.v3`.
It consumes protected evidence, a network/chain-bound deployment document
and a schema-version-2 module registry. The bundle contains `policy.json`,
`bundle-manifest.json`, copied `inputs/`, paired `journal/` records and
optional onboarding material. Its output directory must belong to the
invoking user and have mode `0700`; the policy filename is `policy.json`.

The assembler checks hashes, identity and account bindings, recovery policy,
registry facts and the owner's retained chain receipts. It reads receipts
again from authenticated chain access when verifying the bundle. An
unavailable dependency or changed binding refuses assembly. Asset metadata
alone does not prove on-chain asset registration or funding.

Owner registration uses `merged_owner.py`: wallet binding, identity and
recovery registration, per-asset account opening and exact ledger funding
all execute on chain `125`. See [Owner registration](owner-registration.md)
and [Provisioning details](owner-registration-details.md).

## Signing and process configuration

`material.sh` provides Human material generation and Secret projection.
KMS material includes its seal and pinned client/executor identities. Preserve
the seal, signing keys, recovery policy and matching encrypted state together.
Generated files use private directories and protected regular files; secret
contents stay in mounted storage.

The owner agent reads `LAYERX_AGENT_HUMAN_PEERS` from its generated agent
configuration. The value consists of comma-separated
`uid=<u32>;tenant=<tenant>;principal=<principal>` entries. The component
binding uses UID `4020` with the same tenant and principal as the authority
policy. Assembly validates the named fields and refuses duplicate or
conflicting bindings.

Owner-agent chain access uses `LAYERX_AGENT_NODE_RPC_URL` and
`LAYERX_AGENT_NODE_CHAIN_ID`. Service configuration separately supplies its
providers, sessions, signing authority and upstream trust. A generated
bundle is not evidence that these dependencies are serving requests.

## Deployment boundary

The Human manifest declares Service `layerx-human` on port `9443`, selecting
node pods and targeting the named `human-https` port. The current native node
manifest declares its `paxd` container and native ports; Human application
containers and their port must be supplied by the hosting deployment.
The web workload has its own `web-deployment.yaml`.

The disposable cluster's `up` and `render` commands use the native material
path in `services/platform/hosted/tests/beta-native.sh`. They prepare the
native chain rather than assemble or install the Human policy bundle.
`LAYERX_BETA_RETAIN_MATERIAL=1` reopens native chain material; it does not
provide a Human backup or restore operation. See
[Disposable beta cluster](../operators/beta-cluster.md) for those commands.

[Human journeys](journeys.md) · [Web application](web-app.md) · [Human](index.md)
