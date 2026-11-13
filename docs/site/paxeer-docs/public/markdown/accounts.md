<!-- Source: https://docs.paxeer.app/accounts/ -->

# Accounts & Unified Identity

An EVM address bound to a native identity on Paxeer mainnet-beta, with explicit asset accounts and financial subaccounts.

Paxeer X presents a unified account while preserving distinct cryptographic roles on one network. An EVM account uses secp256k1. A LayerX identity uses Ed25519 and the key-derived identifier `did:layerx:<64 hex characters of the public key>`. An on-chain binding connects the two, and named LayerX accounts hold native assets, issued assets, escrow, budgets, streams, and margin.

## One secret, two keys

A BIP-39 English mnemonic and optional passphrase produce a 64-byte seed. The same account index selects a matching EVM account and LayerX identity.

| Key | Derivation | Path |
| --- | --- | --- |
| Paxeer EVM | BIP-32 secp256k1 | `m/44'/60'/0'/0/i` |
| LayerX identity | SLIP-0010 Ed25519 | `m/44'/19544'/i'/0'` |

The index is below 2³¹ and is identical on both paths. LayerX derivation is entirely hardened. Coin type `19544` is the fixed version-1 branch value `0x4c58`, the ASCII bytes LX. Rust, TypeScript, Python, and the CLI use shared derivation vectors.

```
# Read the mnemonic through standard input or a file.
layerx wallet derive --index 0 < phrase.txt
layerx wallet derive --mnemonic-file phrase.txt --passphrase-file pass.txt

# Bind the derived pair using the configured Paxeer RPC.
layerx wallet derive --bind --rpc <endpoint> < phrase.txt
```

Derivation prints the EVM address and DID. Private-key export requires the explicit `--export-private-keys <file>` option; it creates an owner-readable file and refuses to overwrite an existing file.

### External-wallet derivation

A browser wallet that keeps its seed private can derive the LayerX key from a fixed EIP-712 signature. The domain is `Paxeer X Network`, version `1`, with the connected chain identifier. The primary type is `LayerXKeyDerivation`; the message binds purpose, warning, address, index, and version.

```
seed = HKDF-SHA256(
  ikm  = canonical signature r || s || v,
  salt = "paxeer-x-network/layerx-account-key/v1",
  info = "LX:ACCOUNT-KEY:v1" || chainId_u256_be
         || address_20_bytes || index_u32_be,
  L    = 32
)
```

The SDK normalizes the signature to low-s form with v equal to 27 or 28, verifies that it recovers the requested signing address, asks the wallet twice, and compares the canonical signatures. A mismatch refuses derivation with `wallet_signature_not_deterministic`. A signer mismatch refuses with `wallet_signer_mismatch`.

The derivation signature is itself secret: anyone holding it can derive the LayerX key. The message names the authorized origin, selected by `ACCOUNT_DERIVATION_ORIGIN` in the TypeScript SDK. Keep the signature out of application logs and storage. A self-hosted origin changes the message and the resulting key, so the chosen origin remains stable for recovery.

```
import { deriveFromBrowserWallet, autoBindLayerX } from '@sidiora/layerx-sdk/browser'

const account = await deriveFromBrowserWallet(window.ethereum, {
  chainId: 125,
  address,
})
await autoBindLayerX(window.ethereum, account, 125)
```

## Binding both identities

The address precompile at `0x0000000000000000000000000000000000001004` records mutual consent.

1. Read `layerXBindNonce(address)` and `getUnifiedAccount(address)`.
2. If the derived DID is already bound, return the existing binding without sending another transaction.
3. If a different DID is bound, refuse with `bound_to_different_did`. Changing identity requires the owner’s deliberate `unbindLayerX`.
4. Sign the domain-separated `LX:PAXEER-BIND:v1` preimage containing chain identifier, EVM address, and nonce with the LayerX key.
5. Call `bindLayerX(didPublicKey, signature)` from the EVM address. The transaction supplies EVM consent and the Ed25519 signature supplies LayerX consent.

After binding, the hosted gateway method `px_resolveAccount` resolves the EVM address and DID to the same unified account. It is part of the gateway's `px_` API, defined by its OpenRPC contract. Obtain the gateway URL and credentials from the operator. The CLI bind flow requires EVM chain ID 125. Seed-based helpers construct a signed EIP-1559 transaction; browser-wallet binding uses `eth_sendTransaction` with the pending EVM nonce on that chain.

## Named financial accounts

Native escrow, budget, margin, and stream commitments hold value in separate accounts. Each module authorizes transfers from its own accounts.

| Namespace | Role |
| --- | --- |
| `agent:<did>:main` | The identity’s native-asset account. |
| `agent:<did>:asset:<asset_id>` | A separate account for an issued asset. |
| `agent:<did>:budget:<id>` | Budget-controlled value. |
| `agent:<did>:escrow:<id>` | Escrow-held value. |
| `agent:<did>:stream:<id>` | Stream-controlled value. |
| `agent:<did>:margin:<position>` | Position margin. |
| `system:liquidity:<market>` | Market liquidity. |
| `system:funding:<market>:long\|short` | Protocol-3 market funding accounts. |
| `system:insurance`, `system:fees` | Insurance pool and fee treasury. |
| `system:paxeer-reserve`, `system:paxeer-withdrawals` | System reserve and withdrawal identifiers recognized by the account namespace decoder. |
| `module:asset:value:<issuance-account-id>` | Asset issuance account records. |

### Canonical identifiers

```
account_id32 = SHA-256(
  "LX:ACCOUNT:v1" || u32_be(name_length) || name_bytes
)
```

The hash above derives ordinary named accounts. A module value name ends in its existing 32-byte account ID as 64 lowercase hex characters, which is decoded directly. Hexadecimal identifiers in names use lowercase text; asset and protocol-3 object identifiers use 64 hex characters. Signed Escrow OPEN, Budget CREATE, and Stream OPEN derive subaccounts from the signed actor DID and object identifier. Signed Perps MARKET_CREATE derives market liquidity and funding accounts. System accounts are created by genesis or authorized governance.

## Authority and sequence ownership

An owner’s ordinary signature or session key cannot directly debit a budget, escrow, margin, or stream subaccount. The owning escrow authority, budget allowance, or protocol-module capability authorizes release. This makes the account boundary enforce the financial commitment.

| Counter | Scope |
| --- | --- |
| Activity `account_sequence` | The actor DID’s identity counter. |
| Transfer-set `actor_sequence` | The designated sequence account, normally the debited record and therefore per DID and asset. |
| Send, mint, and burn | Send advances the source account counter; mint uses the issuance account counter; burn uses the source asset account counter. |

An EVM transaction also carries its own sender nonce, read with eth_getTransactionCount at pending. A new per-asset account starts at zero independently of the main account’s counter. Account identifiers are protocol namespace values, not arbitrary application strings. Agent SDK type boundaries reject identifiers outside the supported namespace.

## Recovery and supported wallet paths

The same phrase, passphrase, and index restore the same key pair. External-wallet recovery requires the same account, chain, index, origin, and deterministic signing message; the existing binding returns `already_bound`. Wallets with nondeterministic signatures cannot use signature derivation. Smart-contract accounts whose signatures do not recover to the account address are refused on this path.

An existing independently generated Ed25519 identity remains valid and can bind manually, though it cannot be re-derived from the EVM secret. Mnemonic derivation accepts the English BIP-39 list. Rotating a derived LayerX key breaks the original derivation link; selecting the next account index preserves a reproducible new pair.

## Continue reading

- [Identity, grants, and recovery](https://docs.paxeer.app/identity)

- [Issued assets](https://docs.paxeer.app/assets)

- [Activity sequences](https://docs.paxeer.app/activities)

- [Paxeer precompiles](https://docs.paxeer.app/precompiles)

- [Escrow accounts](https://docs.paxeer.app/escrow)

- [Budget accounts](https://docs.paxeer.app/agent-budgets)
