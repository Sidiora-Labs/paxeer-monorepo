# Wallet inventory

The wallet inventory is the signed admission document shared by the gateway
and all five attestors. It identifies the custody members and the wallet and
account keys they may generate, sign, import or refresh. The sequence advances
for each accepted document, and every member must hold that sequence before it
becomes current at the gateway.

The `inventory` command has four maintenance subcommands: `init` creates the
five-member document, `approve` applies a request manifest, `renew` extends its
lifetime, and `show` displays its contents. The `serve` subcommand runs the
unattended sweep beside the gateway. It reads administrator requests, admits
wallet `secp256k1` and account `ed25519` generation IDs within the hourly and
per-owner limits, and admits signing only after the matching generation entry
is present. Each approved entry is recorded in the state journal with its time,
sequence, key ID, owner, curve and operations; each round records its count of
unapproved requests.

Configure `serve` with `WALLET_INVENTORY_GATEWAY_URL`,
`WALLET_INVENTORY_ADMIN_TOKEN_FILE`, `WALLET_INVENTORY_PRIVATE_KEY_FILE`,
`WALLET_INVENTORY_ISSUER`, `WALLET_INVENTORY_TENANT` and
`WALLET_INVENTORY_STATE_DIR`. The bearer file contains the gateway's
administrator token; the signing key is an owned P-256 PEM file with mode
`0600`. Seed the persistent state directory with the current signed document
as `inventory.jwt`, using the same issuer, tenant and signing key as the
gateway and attestors. The journal is `journal.ndjson` in that directory.

| Setting | Default | Behaviour |
| --- | --- | --- |
| `WALLET_INVENTORY_INTERVAL` | `1m` | Time between sweeps |
| `WALLET_INVENTORY_RENEWAL_MARGIN` | `240h` | Renew within ten days of expiry |
| `WALLET_INVENTORY_MAX_GENERATIONS_PER_HOUR` | `100` | New generation approvals in a rolling hour |
| `WALLET_INVENTORY_MAX_KEYS_PER_OWNER` | `1000` | Total admitted key IDs for one owner |
| `PORT` | `8080` | Health listener port |

Durations are positive values such as `1m` and `240h`; limits are integers
from 1 through 65536. `WALLET_INVENTORY_LISTEN_ADDR` overrides the listener
address. `/healthz` succeeds while a completed round is at most three
intervals old. The image materialises its base64-encoded
`WALLET_INVENTORY_PRIVATE_KEY` and `WALLET_INVENTORY_ADMIN_TOKEN` inputs as
the configured secret files and runs `serve` as an unprivileged user.

The unattended service renews the document before its lifetime ends, preserving
its members and keys. A renewal is published only after the gateway confirms
that all five members hold the new sequence. A failed confirmation leaves the
current inventory and sequence in place, so the next sweep retries the same
sequence.

Renewal gives the inventory a new 30-day lifetime. Keep the service running
with a renewal margin shorter than that lifetime so each renewal is followed
by ordinary sweeps.

To replace the signing key, create the new public key and give it to each of
the five attestors and to the gateway as the inventory authority key. Keep the
existing signed inventory in place until every service has the replacement
key and the gateway accepts a document signed by it.

If a request remains unapproved, inspect its key ID, owner, curve, operation
and the applicable limits. Correct the gateway's provisioning record or wait
for the hourly limit to clear, then let a later sweep evaluate the request.
