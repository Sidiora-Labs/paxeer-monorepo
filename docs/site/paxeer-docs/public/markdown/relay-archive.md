<!-- Source: https://docs.paxeer.app/relay-archive/ -->

# Relay & Archive Nodes

Status of the former relay/archive node product and its RPC surface.

## Relay/archive is retired

The relay/archive node product is retired. There is no installable relay/archive service in the monorepo: the install assets (`install.sh`, `config.example.json`, `deployment.yaml`, and the `layerx-relay-archive.service` unit) must be absent, and retirement tests assert their absence. There is no `services/platform/relay_archive/` service, no `docker/relay-archive/` image, and the beta cluster deploys no relay-archive component. The former startup command `layerxd --relay-archive` no longer exists; the `layerxd` daemon itself is retired.

Do not follow older install, configuration, or tuning instructions for a relay/archive node, including release-bundle contents such as `layerx-archive-codec`. No successor install path was found in the monorepo.

## lx_* RPC methods are switched off

All 26 `lx_*` JSON-RPC methods are switched off (Decision 701) at both the node and the gateway. This includes the archive and subscription methods previously documented here — `lx_getArchiveNetwork`, `lx_getArchiveHead`, `lx_listActivities`, `lx_listBatches`, `lx_getNodeInfo`, `lx_getActivityStatus`, `lx_getReceipt`, `lx_getBatchHeader`, `lx_sendActivity`, and `lx_subscribe` with its resume-window error codes. Clients must not depend on any `lx_*` method.

## HTTP sync and history routes

The previously documented `/v1/sync/*` and `/v1/history/*` HTTP surface has no live implementation in the monorepo. Only the specification contract (`contracts/platform-relay-archive.md` in the spec tree) remains. Whether any non-retired successor serves those routes could not be confirmed from source, so they are not documented as available endpoints.

## What to use instead

For chain history and state, run a `paxd` full or archive node (see [Run a Node](https://docs.paxeer.app/run-node) and [Configuration](https://docs.paxeer.app/configuration)). For receipts and proofs, see [Receipts & Proofs](https://docs.paxeer.app/proofs) and [Data Availability](https://docs.paxeer.app/data-availability). For external mirrors, see [External Mirrors](https://docs.paxeer.app/mirrors).

**Source:** `services/platform/tests/test_relay_archive_install_retired.py`, `services/platform/hosted/tests/test_beta_relay_archive_retired.py`, `rpc/lx_switch_off_test.go`.
