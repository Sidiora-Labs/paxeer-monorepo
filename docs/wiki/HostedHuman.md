# Hosted Human material

The Human application contract is documented in
[Human journeys](HumanJourneys.md), with signing and verification described in
[Human custody](HumanCustody.md). Human policy and evidence material is assembled
by [`material.py`](../../services/platform/hosted/human/material.py) and
[`provision.py`](../../services/platform/hosted/human/provision.py).

## Policy inputs

The assembler consumes a schema-version-2 asset and module registry. Asset
identifiers are nonzero; optional metadata includes `symbol`, `currency` and
`decimals`. The selected asset must be present in the registry. Registry metadata
is configuration rather than an on-chain asset proof.

Owner evidence includes component and agent configuration, a purpose catalogue,
authority policy, principal policy, recovery policy and movement policy. The
authority's tenant and principal must identify exactly one entry in the principal
policy. Protected material uses owner-controlled directories and files with
modes `0700` and `0600` respectively.

The owner agent reads `LAYERX_AGENT_HUMAN_PEERS` as comma-separated
`uid=<u32>;tenant=<tenant>;principal=<principal>` entries. The material assembler
checks the named fields and their binding to the authority principal policy.
The parser is
[`human_peer_config.rs`](../../services/agent/crates/layerx-agentd/src/human_peer_config.rs).

## Evidence and recovery

The [hosted Human README](../../services/platform/hosted/human/README.md#evidence-provisioning)
describes evidence inputs for owner identity, recovery, ledger funding, naming
and program deployment. The assembler validates the producer manifest and
paired admission/deployment journal records before publishing a protected
owner bundle. Keep the complete material set with its matching KMS seal;
[Human owner registration](HumanOwnerRegistration.md) describes the account and
protocol bindings.

The node deployment runs the native chain described in
[Hosted node](HostedNode.md). The beta helper's `up` and `render` entry points
use [`beta-native.sh`](../../services/platform/hosted/tests/beta-native.sh) to
prepare native material and the node manifest. Human policy assembly is a
separate configuration step. Availability of a Human API operation depends on
the service configuration and its required evidence.

[Human journeys](HumanJourneys.md) · [Human custody](HumanCustody.md) · [Home](Home.md)
