# Attestor security boundaries

The attestor daemon under `services/wallet/attestor` implements three-of-five
wallet custody for secp256k1 and Ed25519 keys. Its threshold-library version is
pinned in [go.mod](../../../services/wallet/attestor/go.mod). Key generation,
signing and refresh run through the implementations under
[`internal/tss`](../../../services/wallet/attestor/internal/tss).

## TLS and route authority

The API and peer listeners use authenticated TLS. Peer identity is bound to
configured SPKI pins. API routes distinguish gateway and operator certificate
authorities; see [server registration](../../../services/wallet/attestor/internal/server/server.go)
and [transport TLS](../../../services/wallet/attestor/internal/transport/tls.go).

| Authority | Routes |
| --- | --- |
| Gateway | Key generation, public-wallet reads, custody authority updates, ordinary signing, native-v2 signing, EVM and native reviews |
| Operator | Import, refresh, add-share, key description, ceremony verification signing and metrics |

`/health` is available on the authenticated API listener. Import additionally
requires `ATTESTOR_CEREMONY`. Operator verification signing uses the ceremony-bound
`operator_verification` request on `/v1/sign`; it is separate from ordinary
wallet signing. The daemon does not expose an API returning stored
share material. Operator key descriptions contain public identity and metadata.

## Custody authority and inventory

Startup loads a pinned signed custody authority and approved wallet inventory.
The [authority verifier](../../../services/wallet/attestor/internal/auth/agent/authority.go)
checks signed claims against configured issuer, tenant, audience and validity.
Signed state sequences are persisted to bind updates to retained state.

The [wallet inventory](../../../services/wallet/attestor/internal/server/authority.go)
requires protocol `wallet`, threshold three and exactly five distinct members
whose pins match the runtime configuration. Each key entry binds key ID, owner,
curve, public key, epoch and permitted operations. Generate, import, sign,
refresh and add-share apply the relevant inventory admission checks.

Add-share admits exactly four existing holders and one new participant matching
the approved five-member inventory. Existing holders must agree on the public
key, owner, account and epoch; the replacement must not already hold the key.
The resulting share set is bound into the protocol session. Refresh preserves
the public key and advances the share epoch. Subsequent signing requires the
inventory to approve that epoch.

## Identity and replay protection

The gateway verifies user access tokens, and each attestor verifies the token
independently. The attestor's [JWT verifier](../../../services/wallet/attestor/internal/auth/jwt/jwt.go)
accepts RS256 and ES256, checks issuer, audience, expiry,
maximum token age and ownership of the requested key, and bounds JWKS cache age.

Bearer replay protection is request-scoped. It persists the token hash together
with the canonical method, key ID and request digest. Repeating the same
authorized request is refused; distinct requests can use the same valid token.
Agent requests also bind their signed request, principal, nonce and expiry to
the configured custody authority and durable nonce state.

## Signing and policy

The ordinary [signing API](../../../services/wallet/attestor/internal/server/sign.go)
accepts `evm_tx`, `eip712`, `personal_message`, `eth_sign_digest`, `lx_bind`,
`lx_grant` and `lx_send_authorization`. Native-v2 signing and review have
separate registered handlers. Each operation applies its own authority,
identity, inventory and policy checks before running a threshold session.

EVM transaction inspection checks chain identity and transaction encoding.
Typed data and supported digest constructions are decoded and recomputed by
[`internal/policy/evm`](../../../services/wallet/attestor/internal/policy/evm).
`eth_sign_digest` accepts the supported authorization and sponsored-batch
constructions and recomputes their digests. Policy applies allowed kinds,
transaction and rolling spend limits, rate limits, destination and selector
rules, and approved
authorization delegates. Missing applicable policy refuses signing. The shipped
[policy](../../../services/wallet/deploy/attestor-policy.json) has an empty
`authorization_delegates` list, which admits no EIP-7702 delegate.

Native operations use the
[kernel policy](../../../services/wallet/attestor/internal/policy/lx) or
[native policy](../../../services/wallet/attestor/internal/policy/native)
appropriate to their handler. An unavailable required chain read does not
supply an empty balance or an approval. Session bindings include the held epoch;
shares from different epochs do not combine in a signing session.

## Storage and recovery

The [share store](../../../services/wallet/attestor/internal/store) encrypts
share payloads with the node key and authenticates key metadata as associated
data. Audit entries are durable. Per-key locking serializes key operations;
protocol sessions use authenticated peers, bounded transport and round deadlines.

Import binds the ceremony ID, session, key identity, participants, threshold,
epoch and share bundle. Exact durable replay uses the retained import identity.
Ceremony refresh retains staged state and commit decisions for reconciliation.
A lost response does not establish that no durable state changed.

The [snapshot writer](../../../services/wallet/attestor/internal/backup/writer.go)
encrypts backups with a separate backup key. Restore validates snapshot digest
and node identity and requires an empty destination. Optional SFTP replication
pins the remote host key and tracks shipping acknowledgments before retention.
See [backup restore](runbooks/backup-restore.md),
[node replacement](runbooks/node-replacement.md) and
[share refresh](runbooks/refresh-cadence.md) for recovery procedures.

The [ceremony tool](cutover.md) handles dealer imports and retains encrypted
journals. Dealer splitting handles the whole secret during import; normal
threshold signing uses shares held by the participants. Keep ceremony material,
node keys and backup keys under their configured custody boundaries.
