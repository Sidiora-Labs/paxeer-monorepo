# Wallet custody cutover

The [ceremony command](../../../services/wallet/ceremony/cmd/ceremony/main.go)
moves wallet records to the gateway database and delivers eligible keys to the
attestor network. Funded-wallet rows are archived separately; they are not
converted into supported funded gateway accounts.

Run ceremony commands from `services/wallet/ceremony` with their configuration
already supplied. Keep private keys, database credentials, owner tokens,
archives and journals outside published files.

## Configuration

| Purpose | Variables |
| --- | --- |
| Source and destination databases | `CEREMONY_SOURCE_DATABASE_URL`, `CEREMONY_DATABASE_URL` |
| Gateway schema | `CEREMONY_GATEWAY_MIGRATIONS_DIR` |
| Archive | `CEREMONY_ARCHIVE_PATH`, `CEREMONY_ARCHIVE_PASSPHRASE` |
| Source key decryption | `CEREMONY_MASTER_KEY` |
| Durable delivery | `CEREMONY_ID`, `CEREMONY_JOURNAL_DIR`, `CEREMONY_JOURNAL_KEY_FILE`, `CEREMONY_REHEARSAL_RECEIPT`, `CEREMONY_RPC_URL` |
| Attestor membership | `CEREMONY_NODES`, `CEREMONY_NODE_PINS` |
| Operator TLS | `CEREMONY_TLS_CERT_FILE`, `CEREMONY_TLS_KEY_FILE`, `CEREMONY_TLS_CA_FILE` |
| Gateway TLS and owner verification | `CEREMONY_GATEWAY_TLS_CERT_FILE`, `CEREMONY_GATEWAY_TLS_KEY_FILE`, `CEREMONY_OWNER_TOKENS_FILE` |

`CEREMONY_NODES` and `CEREMONY_NODE_PINS` are matching comma-separated
`id=value` maps for five nodes. The client verifies each node's SPKI pin.
Operator and gateway credentials have separate route authorities.

The [delivery options](../../../services/wallet/ceremony/internal/migrate/migrate.go)
require an absolute journal directory owned by the effective user with mode
`0700`, a protected `0600` file containing the raw 32-byte journal key, and an
absolute rehearsal receipt path. The ceremony ID is retained across recovery.
The attestors need the configured signed custody authority, inventory, policies
and ceremony import setting; see [security boundaries](security-review.md).

## Rehearse

Configure `CEREMONY_REHEARSAL_ADMIN_URL` for temporary database creation and
`CEREMONY_GATEWAY_MIGRATIONS_DIR` for the gateway migrations. The source role
must be read-only. The rehearsal copies a consistent source snapshot, applies
the gateway migrations and operates against the temporary database, which it
drops afterwards.

Supply a separate set of attestor client variables with the prefix
`CEREMONY_REHEARSAL_`, including nodes, pins, operator and gateway TLS files,
and owner tokens. Also set `CEREMONY_REHEARSAL_JOURNAL_DIR` and
`CEREMONY_REHEARSAL_RPC_URL`. The
[rehearsal](../../../services/wallet/ceremony/internal/rehearsal/rehearsal.go)
uses these configured attestors; their HTTPS authorities must differ from the
delivery endpoints while member IDs and pins match. Rehearsal and delivery
journal directories must be distinct.

```sh
go run ./cmd/ceremony rehearse --report-only-counts
```

The counts report includes wallet eligibility, archive, import, refresh,
verification and copied-table counts. Successful counts-only rehearsal writes
the sealed rehearsal receipt used by delivery. Keep that receipt and the
journal key with the retained ceremony material.

## Move, plan and archive

With the source service quiesced for the final copy and the destination selected:

```sh
go run ./cmd/ceremony move
go run ./cmd/ceremony plan
go run ./cmd/ceremony archive --report-only-counts
```

The initial move requires an empty destination wallet schema, copies the source
through a read-only snapshot and checks per-table counts and digests. It applies
the remaining gateway migrations. `move --delta` supports the command's checked
final-delta path; it is not an unrestricted overwrite of gateway state.

`plan` reports `wallets`, `eligible`, `funded` and `already_migrated`. Archive
writes and verifies the funded rows under `CEREMONY_ARCHIVE_PASSPHRASE`.
Delivery verifies that archive again before importing eligible keys. Preserve
the archive and decryption material separately.

## Deliver and resume

Enable `ATTESTOR_CEREMONY` on the intended attestors and ensure their current
signed inventory admits each required key operation. With the same plan,
ceremony identity, pins, journal and rehearsal receipt configured:

```sh
go run ./cmd/ceremony deliver --report-only-counts
```

Delivery retains per-wallet encrypted journals, imports or reconciles key
material, refreshes shares and verifies possession before recording migration.
Public EVM and DID identities must match the original wallet. Owner verification
uses the configured credentials; chain checks use `CEREMONY_RPC_URL`.

Resume an interrupted delivery with the same ceremony identity and retained
journal. The tool reconciles durable import and refresh descriptions; do not
clear `migrated_at`, delete journals, replace the journal key or re-import under
an unrelated identity to restart a partially delivered wallet. Verify all
eligible wallets are matched and the gateway can reconcile their signing state
before directing traffic to it. Disable ceremony imports when delivery ends.

## Endpoint switch

The endpoint scripts under
[`deploy/cutover`](../../../services/wallet/deploy/cutover) support an existing
Caddy proxy and Docker-managed service. Configure their `CUTOVER_` variables
for the actual deployment. `preflight.sh` checks the gateway;
`apply.sh` changes only the selected site's reverse proxy, validates and reloads
Caddy, and verifies gateway readiness through the public path. It retains the
previous Caddyfile and restores it if reload or verification fails.

After `CUTOVER_SOAK_SECONDS` and successful public health and readiness checks,
`retire.sh` stops the selected old service and disables its restart policy.
Compose mode uses `CUTOVER_COMPOSE_DIR` and `CUTOVER_RETIRE_SERVICES`; standalone
container mode uses `CUTOVER_CONTAINER`. The proxy remains running.

A proxy rollback does not reverse key delivery or database migrations. Route
migrated wallets only to code supporting their attestor-backed records. For
application image changes, use [gateway rollback](runbooks/gateway-rollback.md).
