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
Output files are named `<service>.json`, contain document, file, digest and
service environment variables, and are created with mode `0600`.

The optional `expected-schema/runtime-paths.json` values file selects the file
form for named services. It is a JSON object whose service members map to
absolute runtime paths. A selected service emits `EXPECTED_SCHEMA_FILE` and
`EXPECTED_SCHEMA_SHA256`; without a selection it emits the inline
`EXPECTED_SCHEMA_DOCUMENT` and digest. The deployment mounts the exact
expected-schema bytes at the selected path, so both forms bind the same
document.

The API values compose `STREAM_RELAY_CONFIG_DOCUMENT` and
`DISCOVERY_RUNTIME_CONFIG_DOCUMENT`. Generate the rendered authority input
from the repository root with:

```bash
(cd services/launchpad/api && go run ./cmd/api-runtime-docs \
  -repo ../../.. -source-schema indexer -endpoints "$PUBLIC_ENDPOINTS_FILE" \
  -pool-runtime "$REVIEWED_POOL_RUNTIME" -out "$AUTHORITY_OUTPUT_DIR")
```

Set `network_authority_rendered_file` and `network_authority_runtime_path` in
the API values. The composer then emits `NETWORK_AUTHORITY_CONFIG_FILE` and
`NETWORK_AUTHORITY_CONFIG_SHA256`; the service accepts that file source or its
`NETWORK_AUTHORITY_CONFIG_DOCUMENT` inline sibling.

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

The inline indexer form takes the complete deployment manifest and ABI catalog
from `sdk/launchpad/shared/config/deployment/current.json` and
`sdk/launchpad/protocol/catalog/reviewed.json`. Role identities retain their
runtime and implementation hashes. ABI revisions are the catalog's ABI digests;
the committed source uses the receipt's epochs and the hash of the complete
manifest. The source manifest digest identifies the reviewed contract bundle.
Factory discovery templates, including reviewed pool and masked ERC20 runtime
hashes, are supplied by the native renderer in the rendered receipt form.
The receipt authority requires an issued, nonzero chain anchor at or after the
manifest's activation height. Example anchors and runtime hashes are illustrative;
deployment uses the chain-issued values with the matching rendered documents.

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

Livestream supports the provider and platform forms described in the
[deployment README](../README.md#livestream-deployment-forms). The values'
`environment.RUNTIME_PROFILE` selects the matching variable contract. Use
`livestream.platform.example.json` as `livestream.json` to select the platform
form, and compose `--service web` for its two public stream routing build
inputs. Also copy `gateway.platform.example.json` as `gateway.json` to select the
private livestream upstream on port `8443`.

For the provider form of `--service livestream`, the composer also writes `RPC_URL`,
`LIVEPEER_API_URL`, `LIVEPEER_API_KEY`, `BIND_ADDRESS`, `PORT`, `ZONE`,
`MAX_STREAMS_PER_WALLET`, `MAX_GLOBAL_STREAMS`, `SECRETS_REFRESH_SECONDS` and
`LIVEPEER_WEBHOOK_TIMESTAMP_UNIT` from their example values. The example API
key is test data. The public endpoint in the values describes deployment routing;
the service has no `PUBLIC_URL` setting. Connected broadcasting additionally
uses the conditional budget, identity, vault, ingest, playback and audit inputs
listed in the variable contract. The composer emits the connected budget policy
and its SHA-256 binding. A connected budget policy contains the vendor
identity, prices, monthly allowance, broadcast reserve, currency, peak duration
and approval reference read by `streams.BudgetPolicy.ValidateVendor`.

## PnL journey configuration

To enable the portfolio, shares, rewards and referrals, copy
`values/pnl.journeys.example.json` to `pnl.json` in the values directory beside
`common.json` and the generated `expected-schema/pnl.json`. The optional
`journeys` object supplements the eight inventory inputs. Without it, PnL
composition emits only the existing budget, schema and delivery documents.
With it, the composer adds `PNL_JOURNEY_CONFIG_DOCUMENT` and
`PNL_JOURNEY_CONFIG_SHA256`, plus `IDENTITY_SERVICE_URL` and `PNL_FONT_DIR` bound
to the same identity endpoint and font directory. The service reads version 3,
issuer `pnld`, audience `identity`, the embedded font digests and the reward
manifest digest supplied by the composer.

Each member of `journeys` is required, including empty lists and `cdnZone`,
except the client key file pair when using the variable form:

| Input | Operator source and form |
| --- | --- |
| `realm` | Deployment realm, `production` or `private_fixture`; match `DEPLOYMENT_REALM`. |
| `identityEndpoint` | Identity service's HTTPS origin, with no user information, path, query or fragment. |
| `identityPeerCertSha256` | Lowercase SHA-256 of the identity server's DER certificate, obtained from its provisioned certificate. |
| `identityCaFile`, `identityCaSha256` | Absolute clean mounted path and lowercase SHA-256 of the exact PEM trust-root file bytes from the identity PKI. |
| `identityClientCertFile`, `identityClientCertSha256` | Absolute clean mounted path and lowercase SHA-256 of the exact PEM client certificate file bytes issued to PnL. |
| `identityClientKeyFile`, `identityClientKeySha256` | Absolute clean mounted path and lowercase SHA-256 of the matching PEM private key file bytes, provisioned separately. |
| `fontDirectory` | Absolute clean private directory selected for materialized embedded fonts; owned by the runtime with mode `0700`. |
| `liveBatch` | Operator's live-transfer worker batch bound, an integer from 1 to 32. |
| `releaseDigest` | Lowercase SHA-256 from the approved release tuple. |
| `interfaceDigest` | Lowercase SHA-256 from the interface contract accompanying that release. |
| `nodeIds` | Two distinct nonempty identifiers for the primary and archive RPC nodes, each at most 256 UTF-8 bytes. |
| `exportsDirectory` | Absolute clean private directory for portfolio exports, owned by the runtime with mode `0700`. |
| `brokerStream` | Broker stream identifier provisioned for this journey delivery policy. |
| `cdnOrigin` | Public HTTPS origin serving cards, without user information, path, query or fragment. |
| `cdnZone` | Cloudflare zone ID from the CDN configuration, 32 lowercase hexadecimal characters; empty for direct origin serving. |
| `rewardRuntimeHashes` | Map of lowercase contract addresses to `0x`-prefixed lowercase keccak digests of runtime code, read as described below. |
| `programs` | Reviewed reward programmes as `l18.Program` objects, using the exact member names below; empty when no programme is admitted. |
| `approvedCommunityWallets` | At most 64 distinct nonzero Ethereum addresses from the approved community wallet policy; comparison ignores case. |

Select exactly one client key form. For the file form, provide both
`identityClientKeyFile` and `identityClientKeySha256` and leave
`IDENTITY_MTLS_CLIENT_KEY_PEM` empty. For the variable form, omit both members
and set `IDENTITY_MTLS_CLIENT_KEY_PEM` to PEM text, at most 1048576 bytes,
through the service's deployment variables. The composer emits both members
as empty strings for this form. In both forms, the key must belong to the
client certificate named by `identityClientCertFile`.

A programme uses `ID`, `Version` and `DeploymentID` from the reviewed policy,
each an identifier of 1–128 letters, digits, underscores, dots, colons or
hyphens. `Asset` is the `l18.USDL` address. `ReferralRaw`, `AwardCapRaw`,
`DailyCapRaw` and `BeneficiaryCapRaw` are canonical unsigned decimal uint256
strings in the asset's raw units. `CashbackBPS` is an integer from 0 to 10000.
`CashbackStartHeight` and `CashbackEndHeight` are decimal uint64 strings from
the policy's chain interval: start is positive, end is at least start; both
may be empty, and an end requires a start. `Treasury` and `Custody` are
lowercase nonzero addresses from the funding authority, `Adapter` is its
adapter identifier and `AdapterCodeHash` its `0x`-prefixed lowercase keccak
runtime digest. These four strings may be empty for programmes without
settlement authority. Every programme member is present, and `(ID, Version)`
pairs are distinct.

For `rewardRuntimeHashes`, read the code of each of the constants `NFT`,
`Accumulator`, `FeesRouter` and `USDL` in
`services/launchpad/pnl-tracker/internal/production/l18` at one committed block.
At that same block read the EIP-1967 implementation storage position
`0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc`.
For every nonzero implementation address it names, also read that address's
code. Map every target and implementation address to the keccak-256 digest of
its code, using `0x` plus 64 lowercase hexadecimal characters. Use the
committed block hash with `requireCanonical: true` for both `eth_getCode` and
`eth_getStorageAt`; the service checks these bindings against its committed
indexer checkpoint. Example pins and digests are illustrative; provision real
files and chain bindings for deployment.

Supply these variables separately through the configured secret provider:

| Variable | Form |
| --- | --- |
| `IDENTITY_SIGNING_KEY` | Unpadded standard base64 of a 64-byte Ed25519 private key whose public half matches its seed. |
| `PNL_CURSOR_KEY` | Unpadded base64url key of at least 32 bytes. |
| `PNL_CDN_TOKEN` | Nonempty Cloudflare API token authorized to purge the configured zone; required with a nonempty `cdnZone`, absent or empty for direct origin serving. |

`RPC_URL_FALLBACK` differs from `RPC_URL` and selects the archive node paired
with the second node ID. The deployment's broker values must provision the
stream named by `brokerStream` with every subject in the constructor's delivery
policy: `inventory.rebuild.v2`, `portfolio.snapshot.v2`, `fee.claimed`,
`airdrop.epoch`, `airdrop.claimed`, `reward.funding`, `reward.liability`,
`reward.review`, `reward.claim_prepared`, `reward.community_authority`,
`reward.payout_intent`, `reward.paid`, `reward.payout_state`,
`reward.correction_review`, `referral.analytics`, `referral.bound`,
`referral.code`, `referral.consent`, `referral.click` and `referral.bind`.
Add this stream to the deployment's broker values before starting the service.
The deployment's gateway values must include an `identityPeers` entry with
issuer `pnld`, the SHA-256 pin of PnL's client certificate and the public key of
`IDENTITY_SIGNING_KEY` in unpadded base64url form. Keep that entry synchronized
with the separately provisioned signing key and client certificate.

The contract's `optional` names are parsed by the image alongside its fixed and
required names. `forms` describes their environment representation and `defaults`
records the configuration reader's defaults. `documentCredentials` names the
secret-provider inputs referenced inside composed documents; the deployment
supplies their resolved values separately. `buildInputs` describes Docker build
arguments. The web entry also records its image environment and the Next.js build
and runtime context in `imageEnvironment` and `contextEnvironment`. Context
settings are read by Next.js configuration; Docker build arguments are the names
declared in `Dockerfile.web`.

The Compose stack mounts each service's document directory at
`/run/launchpad/<service>`. Mount certificate files and rendered documents at the
absolute paths used by their documents. Private listener values match the
variable contract's IPv6 wildcard addresses.

Startup document readers and validation are in these service modules:

| Service | Documents and startup readers |
| --- | --- |
| All runtime services | `internal/config`: bound JSON sources, digest binding, `OpenRuntimePool`, `ExpectedSchema`; shared `db/production.ConnectionBudget.Reconcile` and readiness schema contracts |
| api | `internal/app`: accounting read configuration, `streamRelayDocument`, `discoveryRuntimeDocument`, network authority configuration |
| api-sockets | `internal/app`: `[]natsdriver.ConsumerConfig`, each checked by `BrokerConsumerConfig` |
| indexer | `internal/runtimedocs`, `internal/receipt.NewRPCQualificationWithAttestation`, `internal/decode.NewRegistry`, `internal/app`: receipt, delivery and lifecycle documents |
| charts | `internal/config.RuntimeFeatures`, `internal/app.chartOutboxConfig`, `internal/consumer`: chart features, delivery policy and consumer configuration |
| stats | `internal/app.statsRuntimeDocument.validate`, `statsPhase2Document.validate`: delivery and phase-two workers |
| pnl | `internal/app`: `DELIVERY_CONFIG_DOCUMENT` delivery consumer; optional `PNL_JOURNEY_CONFIG_DOCUMENT` (`pnlJourneyDocument`, `newPnLJourneys`) |
| ranking | `internal/config.RankingRuntimeBinding`, `internal/app`: version-two runtime binding |
| gateway | `internal/app/app.go`, `internal/config`: runtime, object and identity bindings |
| user | `internal/app/account_runtime.go`: account event subject map |
| social | `internal/config.SocialRuntimeBinding`, `internal/app`: private runtime, certificates, resolved credentials and recovery binding |
| metadata | `internal/config.ReadPublicationRuntime`: publication version, environment, header names and audit key |
| livestream | `internal/config.ValidateConnected`, `internal/streams.BudgetPolicy`: shared documents and conditional connected policy |

Consumer durations use nanoseconds; the sockets and PnL values express
milliseconds and their composers convert them. Document variables contain
standard base64, file variables contain mounted paths, and SHA-256 variables
identify the exact decoded or mounted bytes. Native service startup additionally
resolves credentials, reads certificates, and admits the configured external
dependencies.
