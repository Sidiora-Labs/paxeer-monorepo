# Attestor node loss and replacement

Wallet custody uses three-of-five shares. Any three current holders can sign,
but replacement admits exactly four surviving holders and one new participant.
With only three current holders available, signing quorum alone is insufficient
for this replacement procedure.

Runtime examples are
[`attestor-1.env.example`](../../../../services/wallet/deploy/attestor-1.env.example)
through `attestor-5.env.example`. The daemon reads file-backed keys, TLS
material, policy and signed inventory through its
[configuration loader](../../../../services/wallet/attestor/internal/config/config.go).
Container input variables such as `ATTESTOR_NODE_KEY` carry base64 material
that the image writes to files; the daemon reads `ATTESTOR_NODE_KEY_FILE`.

## Diagnose and preserve quorum

Read each node's authenticated `/health` response and the gateway's
`GET /v1/admin/readiness`, using bearer `GATEWAY_ADMIN_TOKEN` for the latter.
The admin report includes `components.attestors.nodes` and `healthy`.
Public `/readyz` contains only component states and reasons.

If the original store and key remain intact and trusted, restart that same
participant with its original data and configuration. Compare key descriptions
with surviving holders before using it for signing. For a compatible encrypted
snapshot, see [backup restore](backup-restore.md).

When the store or node key is lost or untrusted, isolate the old instance.
Preserve the remaining four holders and do not run two instances of one
participant. Use a maintenance window for affected keys while changing membership
and inventory. Signing admission requires the held participant set and epoch to
match the approved inventory; resume after those records agree.

## Operator client

Import, refresh, add-share and describe use a client certificate chaining to
`ATTESTOR_OPERATOR_CA_FILE`. The following shell variables hold operator
certificate paths and a node API base, not daemon configuration:

```sh
attestor_post() {
  curl --silent --show-error --fail-with-body --max-time 180 \
    --cert "$CHECK_LIVE_CLIENT_CERT" --key "$CHECK_LIVE_CLIENT_KEY" \
    --cacert "$CHECK_LIVE_CA" \
    -H 'Content-Type: application/json' --data @"$3" "$1$2"
}
```

Set `CHECK_LIVE_CLIENT_CERT`, `CHECK_LIVE_CLIENT_KEY` and `CHECK_LIVE_CA` to
the operator client certificate, private key and trusted server CA bundle.
`keys.describe` is local; add-share and refresh require the same body and
session ID submitted concurrently to every participant in that operation.
Request and response types are declared in
[`keys.go`](../../../../services/wallet/attestor/internal/server/keys.go).

## Prepare the replacement

1. Choose a new participant ID outside the key's current holder set. Provision
   an empty data directory, a fresh node key and backup key, and TLS material.
   Configure `ATTESTOR_NODE_KEY_FILE`, `ATTESTOR_BACKUP_KEY_FILE`,
   `ATTESTOR_TLS_CERT_FILE`, `ATTESTOR_TLS_KEY_FILE`, `ATTESTOR_TLS_CA_FILE`
   and `ATTESTOR_OPERATOR_CA_FILE` with the mounted files.
2. Configure `ATTESTOR_NODE_ID`, `ATTESTOR_PEERS` and `ATTESTOR_PEER_PINS` on
   the replacement and survivors. Pins are SHA-256 hashes of certificate SPKI.
   The replacement has the four survivors as peers; each survivor replaces
   the lost peer's ID and pin with the replacement's values.
3. Supply a current signed wallet inventory through `ATTESTOR_INVENTORY_FILE`,
   verified with `ATTESTOR_INVENTORY_PUBLIC_KEY_FILE`. It must approve exactly
   the four survivors and replacement, their configured pins, threshold three,
   and each key's owner, curve, public key, current epoch and `refresh`
   operation. The inventory issuer and tenant match the configured custody
   authority. See [inventory admission](../../../../services/wallet/attestor/internal/server/authority.go).
4. Start the replacement, then load the peer and inventory settings on survivors
   one at a time. Check authenticated health and peer connectivity after each.

## Replace each key

List wallet key IDs from the gateway database and include keys retained by
in-progress provisioning operations. The key fields on wallet rows are
`attestor_key_id` and `layerx_key_id`:

```sh
psql "$DATABASE_URL" -Atc 'select attestor_key_id from wallets where attestor_key_id is not null union select layerx_key_id from wallets where layerx_key_id is not null'
```

Describe each key on surviving holders:

```json
{"session_id":"<describe-session>","key_id":"<key-id>"}
```

```sh
attestor_post "$SURVIVOR_BASE" /v1/keys/describe "$DESCRIBE_BODY"
```

Compare `curve`, `public_key`, `owner`, `account`, `epoch` and `participants`.
The four surviving records must agree. Build the add-share request from those
fields, with `quorum` containing exactly the four surviving IDs and
`new_participant_id` containing the replacement ID:

```json
{
  "session_id": "<add-share-session>",
  "key_id": "<key-id>",
  "curve": "<described-curve>",
  "public_key": "<described-public-key>",
  "owner": "<described-owner>",
  "account": "<described-account>",
  "epoch": 0,
  "quorum": ["<survivor-1>", "<survivor-2>", "<survivor-3>", "<survivor-4>"],
  "new_participant_id": "<replacement>"
}
```

Replace `epoch: 0` with the described current epoch. Submit the identical body
to `/v1/keys/addshare` on all five participants concurrently. Collect and check
every response: the public key and epoch remain unchanged, `participants`
contains the approved five holders, and `refreshed` is false.

Then [refresh the key](refresh-cadence.md) across those five holders. Confirm
that all report the unchanged public key and the same incremented epoch.
Update the signed inventory to approve that epoch before further signing.
Describe the key on the replacement and compare it with the survivors. Verify
a consented signature with a signing quorum that includes the replacement.

## Reconciliation

Health and share counts alone do not establish that a key combines with its
peers. Confirm each key's public identity, participant set, epoch and signature.
If an operation returns an uncertain result, describe the retained state on all
participants and reconcile it before starting another operation. Add-share and
refresh persist share state; a transport error does not establish that no node
changed state. Keep the old participant isolated after replacement and refresh.
