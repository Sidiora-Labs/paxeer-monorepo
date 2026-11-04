# One account

Paxeer X Network is one chain, with EVM chain ID `125` (`0x7d`). An EVM
account uses a secp256k1 key; a LayerX identity uses an Ed25519 key whose DID
is `did:layerx:<64 lowercase hex characters of the Ed25519 public key>`.
Binding these identifiers records their relationship in the same chain.
It does not fund an account. Ledger accounts are opened per asset through
`openAccount(assetId)` at the LayerXLedger precompile; see
[Getting started](../overview/getting-started.md#ledger-assets-and-payments).

This page specifies how **one user secret yields both keys**, the same way in
every implementation, and how first use binds them on-chain so the network
shows one account.

| Implementation | Where |
| --- | --- |
| Rust | `services/agent/crates/layerx-crypto/src/account_derivation.rs`, `services/agent/crates/layerx-client/src/account_binding.rs` |
| TypeScript | `@sidiora/layerx-sdk`, `services/agent/sdk/typescript/src/account-derivation.ts` |
| Python | `services/agent/sdk/python/layerx_sdk/account_derivation.py` (install the `derivation` extra) |
| CLI | `layerx wallet derive` |
| Shared vectors | `sdk/platform/conformance/fixtures/account-derivation-v1.json` |

All of them load the same fixture in their tests. The fixture was produced by
an independent implementation (ethers, `ed25519-hd-key` and the Node HKDF), and
the SLIP-0010 code is also checked against the official SLIP-0010 Ed25519
vectors.

## Path 1: a seed phrase

For the LayerX wallet, the CLI and the SDKs, the secret is a BIP-39 English
mnemonic with an optional passphrase. BIP-39 turns it into a 64-byte seed.

| Key | Scheme | Path |
| --- | --- | --- |
| Paxeer EVM account | BIP-32, secp256k1 | `m/44'/60'/0'/0/i` |
| LayerX identity | SLIP-0010, Ed25519 | `m/44'/19544'/i'/0'` |

`i` is the account index, below 2^31, and it is the same on both sides: EVM
account `i` always pairs with LayerX identity `i`.

The EVM path matches wallets configured for that same path, phrase,
passphrase and index. SLIP-0010 only defines hardened derivation for Ed25519,
so every LayerX component is hardened.

### The coin type 19544

`19544` is `0x4c58`, the ASCII bytes `LX`. It is the implementation's
`LAYERX_COIN_TYPE`, separate from the EVM coin type `60`. The value is part
of version 1 of this derivation; changing it would change the derived key.

```sh
# The phrase is read from standard input or a file, never from arguments.
layerx wallet derive --index 0 < phrase.txt
layerx wallet derive --mnemonic-file phrase.txt --passphrase-file pass.txt
```

The command prints the EVM address and the DID. Private keys are written only
when `--export-private-keys <file>` is given, to a new file that only its
owner can read; an existing file is never overwritten.

## Path 2: an external wallet

MetaMask and similar wallets never reveal their seed. They can sign, so the
LayerX key is derived from **one signature over one fixed message**.

The wallet is asked, with `eth_signTypedData_v4`, to sign this EIP-712
document. Typed data is used instead of `personal_sign` so the wallet renders
every field instead of an opaque string.

```json
{
  "types": {
    "EIP712Domain": [
      { "name": "name", "type": "string" },
      { "name": "version", "type": "string" },
      { "name": "chainId", "type": "uint256" }
    ],
    "LayerXKeyDerivation": [
      { "name": "purpose", "type": "string" },
      { "name": "warning", "type": "string" },
      { "name": "address", "type": "address" },
      { "name": "index", "type": "uint32" },
      { "name": "version", "type": "uint32" }
    ]
  },
  "primaryType": "LayerXKeyDerivation",
  "domain": { "name": "Paxeer X Network", "version": "1", "chainId": 125 },
  "message": {
    "purpose": "Derive your LayerX account key",
    "warning": "Only sign this on https://paxportwallet.com. Anyone holding this signature controls your LayerX account.",
    "address": "0x<your address, lower case>",
    "index": 0,
    "version": 1
  }
}
```

`chainId` is the chain the wallet is connected to (at most 2^53 - 1, so it
is exact in JSON), `address` is the signing account and `index` is the account
index. Mainnet-beta uses `125`; the TypeScript automatic binding helper
requires that chain ID. The origin also affects the message as described below.

Older derivation vectors use chain ID `713714`. Those are historical test
inputs, not mainnet-beta configuration. The external-wallet derivation binds
the chain ID into both the signing document and HKDF context: deriving with
`125` produces a different key from deriving with `713714`. Preserve the
original chain ID, origin, index and version when recovering an existing
identity. Updating a connection setting does not migrate a key or its binding.
The mnemonic paths above do not depend on the chain ID.

The LayerX Ed25519 seed is then

```text
seed = HKDF-SHA256(
  ikm  = canonical 65-byte signature  r || s || v,
  salt = "paxeer-x-network/layerx-account-key/v1",
  info = "LX:ACCOUNT-KEY:v1" || chainId (32 bytes, big endian)
         || address (20 bytes) || index (4 bytes, big endian),
  L    = 32)
```

Three rules make that safe to repeat:

1. **Normalise first.** The same ECDSA signature has equivalent encodings.
   Before hashing, `s` is brought to the low half of the group order (flipping
   the recovery bit when it is reflected) and `v` is written as 27 or 28. A
   wallet that answers with `v` of 0 or 1, or with a high `s`, yields the same
   key.
2. **Verify before deriving.** The canonical signature must recover to the
   `address` named in the message. Anything else is refused
   (`wallet_signer_mismatch`), so a signature from the wrong account or the
   wrong chain never becomes a key.
3. **Ask twice and compare.** The key is only recoverable if the wallet signs
   deterministically (RFC 6979). `deriveFromBrowserWallet` in the TypeScript
   SDK requests the signature twice and refuses with
   `wallet_signature_not_deterministic` when the two canonical forms differ.
   Every client that implements this path must do the same.

```ts
import { deriveFromBrowserWallet, autoBindLayerX } from "@sidiora/layerx-sdk";

const chainId = 125;
const [address] = await window.ethereum.request({ method: "eth_requestAccounts" });
const account = await deriveFromBrowserWallet(window.ethereum, { chainId, address });
await autoBindLayerX(window.ethereum, account, chainId);
```

### The risk, stated plainly

**Anyone who obtains that signature controls the LayerX account**, for ever;
the signature is the secret. A phishing page that gets a user to sign the same
message learns the key. That is why the message says what it is and names the
only origin that should ask for it. Recognise it by all of these together:

- the domain name is `Paxeer X Network`, version `1`;
- the primary type is `LayerXKeyDerivation`;
- `purpose` is exactly `Derive your LayerX account key`;
- `warning` names the origin you are actually on.

If a site other than the one named in `warning` shows this message, reject
it. Applications must never log, store or transmit the signature; derive the
key and discard it. The SDK builders accept another `origin` for self-hosted
deployments, but a different origin is a different message and therefore a
**different LayerX key**, so a deployment must pick one origin and keep it.

## Binding on first use

The two keys become one account through the `addr` precompile at
`0x0000000000000000000000000000000000001004`:

1. read `layerXBindNonce(address)` and the identity currently bound to the
   address (the helpers read `getUnifiedAccount(address)`, which carries the
   same public key as `getLayerXDid(address)` and does not revert for an
   unbound address);
2. if the address is already bound to the derived DID, do nothing: the helper
   is idempotent;
3. if it is bound to a **different** DID, refuse with
   `bound_to_different_did`. Nothing is sent and nothing is overwritten;
   changing identities takes a deliberate `unbindLayerX` by the owner;
4. otherwise sign `"LX:PAXEER-BIND:v1" || chainId || address || nonce` with
   the LayerX key and call `bindLayerX(didPublicKey, signature)` **from the
   EVM address**. The transaction is the EVM account's consent and the
   Ed25519 signature is the LayerX identity's.

With a seed phrase both consents come from the same secret: `layerx wallet
derive --bind --rpc <endpoint>` signs and submits the transaction, and the
Rust and TypeScript helpers return the signed EIP-1559 transaction. With an
external wallet the helper hands the call to the wallet with
`eth_sendTransaction`. A returned transaction hash means submitted; wait for
its committed receipt and read the binding again before treating it as active.
The hosted gateway's `px_resolveAccount` accepts either the original EVM
address or explicit DID and reports their binding. Its `layerx_account` field
is a native reference, not a reverse-lookup selector. See
[Unified network](../overview/unified-network.md#public-node-json-rpc-and-the-hosted-gateway).

## Recovery

The same secret restores both keys. Re-entering the phrase (and passphrase)
with the same index yields the same pair. With an external wallet, signing the
same message again with the same account on the same chain yields the same
LayerX key; if the binding is already on-chain, the helper reports
`already_bound`. Retain the mnemonic passphrase and index, or the external
wallet's chain ID, origin, index and derivation version, along with access to
the secret needed for recovery.

## What is not covered

- **Wallets that cannot sign deterministically.** Some hardware and
  multi-party wallets use a fresh nonce for every signature. They fail the
  ask-twice check and cannot use Path 2. Use a seed phrase, or create a LayerX
  identity separately and bind it manually.
- **Smart-contract accounts.** Their signatures do not recover to the account
  address, so they are refused.
- **Identities created before this scheme.** A LayerX identity made from a
  random or imported Ed25519 seed keeps working unchanged. It can be bound to
  an EVM address manually with the same `bindLayerX` call
  (`layerx_client::paxeer_binding`); it simply cannot be re-derived from the
  EVM secret.
- **Other mnemonic languages.** Only the English BIP-39 word list is accepted.
- **Key rotation.** Rotating the LayerX key breaks the derivation link for
  that index; move to the next index instead.
