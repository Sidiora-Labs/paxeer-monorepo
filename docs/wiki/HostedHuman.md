# Hosted Human material

End-user `/v1` journeys (onboarding, deposit, move, withdraw, exit,
approvals, managed agents, security) are documented on
[Human journeys](HumanJourneys.md). The custodial keystore model and
verification levels are on [Human custody](HumanCustody.md). This page
is the hosted-cluster material assembler, not the browser product
contract.

Human policy assembly consumes the same generated schema-version-2 asset and
module registry published to `layerx-core-module-registry`. The native producer
uses the module interfaces registered by the integrated node and kernel host.
The schema-version-2 registry supplies nonzero asset identifiers and optional
`symbol`, `currency` and `decimals` metadata. The material assembler validates
the selected asset against that registry. `services/platform/hosted/node/bootstrap.sh`
delegates to the native runtime; it does not define fixed token metadata.
Registry metadata alone does not constitute an on-chain asset proof.

A fresh cluster generates its Human KMS seal and policy material. Repeating
`up` with `LAYERX_BETA_RETAIN_MATERIAL=1` reuses the complete saved material set.
The driver refuses a missing inventory, missing or extra files, symbolic links,
wrong ownership, files outside mode 0600, or directories outside mode 0700.
Before applying Secrets it compares the live `layerx-human-kms-material`
Secret's `kms-seal` digest with the retained disk file, without printing either
seal or digest. A mismatch refuses bring-up.

The complete inventory is saved after policy assembly, principal provisioning,
and the node registry comparison. A partial earlier bring-up has no complete
inventory and is refused. Retained mode also preserves settlement bindings,
builder digests and principal credentials; it does not regenerate Human policy
or redeploy settlement contracts. `render` validates retained material and the
live seal but only renders manifests, as in fresh mode.

`down` still deletes the cluster (or owner namespaces), including the Human
state PVC, and removes local material. Retention supports repeated `up` against
a live cluster; it is not recovery after `down` or a backup protocol.

The owner agent consumes `LAYERX_AGENT_HUMAN_PEERS` from
`layerx-human-agent-config`. Its encoding is comma-separated
`uid=<u32>;tenant=<tenant>;principal=<principal>` entries. The component binding
uses UID 4020 and the same tenant and DID as the authority principal policy.
The material assembler validates that named-field encoding and refuses malformed,
duplicate, additional or differently bound peer entries.

## Evidence provisioning

The beta provisioning contract and output sources are documented in
[the hosted Human README](../../services/platform/hosted/human/README.md#evidence-provisioning).
Owner preparation and registration bind the identity, recovery policy and ledger
account. Evidence assembly consumes authenticated ledger funding, naming and
registry deployment evidence, journal pairs and the producer manifest. Retired
custody references, checkpoint timing and guarantor keys are not assembly inputs.
The assembler publishes one complete protected set and refuses unavailable inputs
by path. The owner-provisioning Job mounts the runtime identity PVC.
