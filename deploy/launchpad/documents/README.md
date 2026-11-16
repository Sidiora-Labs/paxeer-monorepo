# Startup document composer

The composer turns reviewed launchpad values into one protected environment
document per runtime service. It binds every JSON document to its lowercase
SHA-256 digest and emits the document as standard base64 for the service's
`*_DOCUMENT` variable.

```bash
python3 deploy/launchpad/documents/compose.py \
  --values deploy/launchpad/documents/values \
  --out /run/kindle/documents \
  --service api --service api-sockets
```

Use `--examples` instead of `--values` for the checked-in example directory.
The values directory contains `common.json` (or `common.example.json`), an
`expected-schema/` directory with one `<service>.json` object per service, and
optional service modules beside `compose.py` for service-specific documents.
Each expected-schema document is the output of the `launchpad-schema expected`
command. The composer compares its `service` member with the service registry
name while retaining the platform service name in the values filename.
Output files are named `<service>.json`, contain only document, file and digest
variables, and are created with mode `0600`.

The charts, stats, API, indexer, PnL and ranking examples form one deployment
set. Their source streams equal the indexer's `canonical_stream_id`; charts,
stats, ranking and the API use its `projection_version`, while PnL uses its
`inventory_projection_version` and `inventory_generation`. Charts, stats and
the API share one projection generation. The API's charts and stats delivery
schemas and consumer IDs equal those services' `schema` and `consumerId`:
partition delivery tables live in `candles` and `stats`, respectively. Its
`ChartsSchema` and `StatsSchema` name those same schemas, and its
`SourcePartitionSchema` is `indexer`, which charts and stats use for bootstrap
evidence. Each input consumer's broker stream equals the indexer's `stream`,
separate from the canonical source stream ID.

For the indexer, receipt and lifecycle documents each have two forms. The default
form composes the document from the indexer values and writes its standard base64
to `INDEXER_RECEIPT_CONFIG_DOCUMENT` or `INDEXER_LIFECYCLE_CONFIG_DOCUMENT`, with
the corresponding `*_SHA256`.

To use files rendered by `services/launchpad/indexer/cmd/indexer-runtime-docs`,
set these pairs in `indexer.json` (or `indexer.example.json`):

```json
{
  "receipt_rendered_file": "rendered/receipt.json",
  "receipt_runtime_path": "/run/launchpad/indexer/receipt.json",
  "lifecycle_rendered_file": "rendered/lifecycle.json",
  "lifecycle_runtime_path": "/run/launchpad/indexer/lifecycle.json"
}
```

These members supplement the other indexer values. A rendered input path is
absolute or relative to the values directory; the runtime path is where the
service reads the mounted file. Supply both members of each pair. Each input
must contain one JSON object. The composer preserves its bytes and emits the
runtime path in `INDEXER_RECEIPT_CONFIG_FILE` or `INDEXER_LIFECYCLE_CONFIG_FILE`
and the SHA-256 of those exact bytes in the corresponding `*_SHA256`, omitting
the `*_DOCUMENT` variable. Mount the same bytes at the runtime path. Receipt and
lifecycle may select their forms independently.

The variable contract's `documentSources` maps each document variable to its
file alternative. The composer and variable apply tool require exactly one
source per document, alongside the required digest. When changing an existing
deployment's form, clear its previous source before applying the new variables.

The broker stream and durable example is `values/broker.example.json`. Apply it
with the shared module's `brokerprovision` command before starting consumers.
Its `check` action compares the broker with the reviewed document without writes.
Consumer durations are nanoseconds, matching the composed delivery documents.

For `--service gateway`, `gateway.json` (or `gateway.example.json`) supplies
`runtime`, `identity` and credential references. Routes use `method`, `template`
and the boolean flags `authenticate`, `optional` and `upload`; omitted flags
are false. The runtime reads these as `proxy.ConnectedRoute` fields. A bound
route must have the flags admitted for its upstream by `allowedConnectedRoute`
in `services/launchpad/gateway/internal/proxy/connected.go`. That file holds
`UserConnectedRoutes`, `MetadataConnectedRoutes`, `SocialConnectedRoutes`,
`LivestreamConnectedRoutes` and `FrozenConnectedSurface`. The app joins each
bound set with its upstream's route list before admission; the bound entry
wins for an existing method and template. The composer checks route structure,
flag consistency and duplicates; route membership belongs to the gateway.

`runtime.approvedBucket` is a logical key returned by `config.Config.Buckets`
in `services/launchpad/gateway/internal/config/config.go`: `token`, `user` or
`og`, with its corresponding bucket configured. The example uses `token`,
whose `METADATA_BUCKET` value is the physical bucket `media-approved`.
`runtime.objectWorkers[].bucket` is a physical bucket name. Each worker's
`kind` selects its exact prefix: `variant` uses `approved/`, `original` uses
`quarantine/`, `temporary` uses `temporary/`, and `recording` uses `private/`.
The first three kinds are required. `configureObjects` in
`services/launchpad/gateway/internal/app/app.go` holds these bindings and
requires distinct delete and inventory credential references across all
workers, separate from the serving credentials. Their provisioned access
keys must also differ from each other and from the serving access key.

`runtime.recoverySchema` names the schema containing `assert_epoch`, installed
from `deploy/launchpad/production/recovery/epoch.sql`. The gateway, ranking and
social examples use `recovery`; the identity delivery schema remains `users`.
The gateway constructs `queue.NewEpochGuard` with that schema, a positive
`recoveryEpoch` and a `recoveryRegion` containing lowercase letters, digits
and hyphens. The shared example budget gives `mediagatewayd_pool` a direct
`identity_runtime` allocation of 12 connections, covering the identity and
control limits of 8 and 4. The app checks the split against this bound budget
before opening either pool.

Example certificate digests and identity peer pins are illustrative. Supply
real certificate files, their digests, matching private keys and independently
provisioned credentials for deployment. The composer validates fields it can
check locally, including exact authority URLs, control schemas, object role
references, numeric bounds, epoch syntax and identity key dates and overlap.
The gateway validates certificate contents, resolved keys, configured buckets
and route admission at startup.

For `--service metadata`, the composer writes
`METADATA_PUBLICATION_CONFIG_DOCUMENT` and its `*_SHA256` binding from the
example's `environment`. Listener PEM materials and assertion keys are supplied
separately under the metadata variable contract. The contract selects `mtls`
with `METADATA_TLS_CERT_PEM`, `METADATA_TLS_KEY_PEM` and
`METADATA_TLS_CLIENT_CA_PEM`; these materials are not publication documents.
