# Solana identity in the Paxeer X Network bridge attestations

Paxeer mainnet-beta is one BFT network, with EVM chain id `125` (`0x7d`). Its
kernel is part of the network and proposes blocks; a Paxeer transaction receipt
is final when its block is committed. Solana is a foreign custody destination
of the bridge.

The bridge uses the same attestation layouts across destinations. An attestor
signs `keccak256` of a packed preimage with secp256k1 and returns 65 bytes of
`r || s || v`. The EVM vault uses `ecrecover`, the `layerxBridge` precompile
delegates recovery to the keeper, and the Solana custody program checks the
native secp256k1 program's verification. The byte layouts and signature rules
are specified in [modules/layerxbridge/ATTESTATION.md](../../modules/layerxbridge/ATTESTATION.md)
and implemented by `InboundPreimage`, `OutboundPreimage` and `RecoverSigner` in
[the Paxeer types package](../../modules/layerxbridge/types/attestation.go).

The domains are `PAXEERX_BRIDGE_IN_V1` (196-byte preimage) and
`PAXEERX_BRIDGE_OUT_V1` (185-byte preimage). The digest is plain `keccak256` of
the preimage with no EIP-191 prefix and no EIP-712 domain. Signatures use `v` in
`{27, 28}` and `s` at or below half the curve order, with strictly ascending
recovered signer addresses, every signer a current attestor and at least
`threshold` of them.

For Solana release transactions, the relayer's
[`secp256k1_instruction_data`](../../services/interop/crates/layerx-bridge-relayer/src/solana/release.rs)
converts `v` to recovery id `0` or `1` and supplies the full 185-byte preimage
to the native secp256k1 instruction, which hashes it during verification. That
instruction must immediately precede the release. The custody program's
[`verify_attestation`](solana/src/attestation.rs) reads it through the
instructions sysvar and checks the message, low `s`, signer membership, order
and threshold. Solana ed25519 signatures authorize transactions and account
operations, including fee payment and recipient registration; they do not
serve as bridge attestations.

The Solana identity mapping below connects 32-byte keys to the 20-byte fields
in those shared layouts.

## The mapping

| Digest field | On Solana |
|--------------|-----------|
| chain id     | the reserved `uint64` `91600046870081` |
| vault        | the handle of the custody program's vault-authority PDA |
| asset        | the 20-byte asset id the program's asset registry holds for the mint, whose default is the handle of the mint |
| txHash (inbound) | `keccak256` of the 64-byte Solana transaction signature that carried the deposit |
| logIndex (inbound) | the deposit nonce recorded in the deposit-receipt PDA |
| recipient (inbound) | the 32-byte Paxeer recipient of the deposit, a nonzero EVM address in its low 20 bytes with its high 12 bytes zero |
| recipient (outbound) | the handle of the full 32-byte Solana pubkey the release instruction names |

### The handle of a 32-byte key

    handle(key) = keccak256(key)[12:32]

The last 20 bytes of `keccak256` of the key's 32 raw bytes. The Go vector
package implements this as [`Handle`](vectors/solana.go). The custody program
implements it as `identity::handle` and `pubkey_handle` in
[`solana/src/identity.rs`](solana/src/identity.rs); the relayer implements
`solana::handle` in its
[`solana` module](../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs).
These implementations use the same derivation and have tests against the
worked vectors below.

### The vault

The vault Paxeer registers for Solana is `handle(vault-authority PDA)`, where the
vault-authority PDA is the program address of the single seed `vault-authority`
under the custody program's id:

    vault_authority, bump = find_program_address([b"vault-authority"], program_id)
    vault                 = handle(vault_authority)

That PDA is the authority of the SPL token accounts holding the bridged tokens
and signs releases from them, so its handle identifies the custody authority
in the signed preimage. A deployment's program id, its vault-authority PDA and
that PDA's handle are written down by
[deploy-solana-program.sh](../../services/bridge/deploy/deploy-solana-program.sh).
Governance registers the handle through `MsgRegisterChain` like any other
chain's vault.

### The asset

An asset's 20 bytes come from the program's owner-set asset registry, one PDA per
mint. Registering a mint without an explicit id defaults it to `handle(mint)`;
the owner may register an explicit nonzero 20-byte id instead. An unregistered or
disabled mint is refused, so no asset reaches a digest without the owner having
recorded it.

Sidiora requires the explicit id `0x21f7b20a555199fa73A238B1a91FD0f549068fEe`.
The custody program's `require_sidiora_binding` enforces this in both directions:
Sidiora's mint must use that id, and no other mint may use it. On Paxeer,
governance submits `MsgRegisterSidioraPair` after registering the Solana chain
and before setting the Sidiora cap. The handler calls `EnsureSidioraDenom` in
[`sidiora.go`](../../modules/layerxbridge/keeper/sidiora.go) to register the pair
against `SidioraDenom()`, the bridge-owned tokenfactory denom with subdenom
`usid` and display symbol `SID` (6 decimals). The mapping makes inbound SID
deposits resolve to that denom; Solana is its configured foreign home.

SOL uses the wrapped SOL mint, registered with its derived handle. Custody
deposits and releases transfer SPL tokens, so native SOL must be wrapped before
deposit and may be unwrapped after release. The
[Solana chain configuration](solana/chains/solana/config.json) lists wrapped
SOL and Sidiora; registration, caps, attestors and the Paxeer chain registration
must be configured before bridging. See the [bridge runbook](README.md) and
[Solana deployment page](solana/chains/solana/README.md).

### The inbound transaction hash and log index

Solana has no 32-byte transaction hash, and the custody program never learns the
signature of the transaction it runs in. The relayer supplies both halves of the
nullifier instead:

    txHash   = keccak256(transaction_signature)   // the 64 raw bytes, not the base58 text
    logIndex = deposit nonce from the deposit-receipt PDA

The deposit increments the config's nonce and writes a receipt PDA seeded by it,
so the nonce is monotonic, auditable and unique per deposit. `(chain id, txHash,
logIndex)` is the nullifier the precompile consumes once.

### The outbound recipient

A release instruction names the recipient's **full 32-byte pubkey** and the
program derives `handle(pubkey)` for the digest it verifies. It also checks that
the destination is an SPL token account for the specified mint owned by that
pubkey. Changing the pubkey changes the signed recipient handle unless the
keys collide under the 160-bit mapping.

The relayer cannot reverse a handle into a pubkey. The recipient first signs
`register_recipient` to publish its key in the recipient PDA derived from its
handle; anyone may pay the rent. The program checks that the declared handle
matches the registrant's key. The relayer reads this record to resolve the
Paxeer burn's 20-byte recipient to the full key used in the release.

### The chain id

Solana's chain id on the Paxeer side is the reserved `uint64`:

    "SOLANA" -> 0x53 0x4f 0x4c 0x41 0x4e 0x41
    left-padded to eight bytes -> 0x0000534f4c414e41
    big-endian                 -> 91600046870081

This is Solana's remote-chain identity in the bridge, distinct from Paxeer's
EVM chain id `125`. It differs from every EVM chain id in the bridge's current
configurations: 1, 10, 56, 137, 999, 8453, 42161 and 43114. The padded-ASCII
convention does not guarantee uniqueness against all possible EIP-155 ids;
operators must keep this reserved identity distinct in their configurations.
Governance registers it through `MsgRegisterChain` like other remote chains.

## Worked vectors

[`vectors/solana.go`](vectors/solana.go) holds these values.
[`vectors/solana_test.go`](vectors/solana_test.go) recomputes both digests through
the chain's `InboundPreimage` and `OutboundPreimage` builders and checks that
this page contains the pinned identities and digests. Rust coverage is also
implemented:

- The custody program's [`core` tests](solana/tests/core.rs) check identity and
  vault-PDA derivation. Its [`attestation` tests](solana/src/attestation.rs)
  check the outbound preimage and digest, and its
  [`release` tests](solana/src/release.rs) check the outbound nullifier.
- The relayer's [`solana` module tests](../../services/interop/crates/layerx-bridge-relayer/src/solana/mod.rs)
  check the pinned handles. Its [`inbound tests](../../services/interop/crates/layerx-bridge-relayer/tests/solana.rs)
  check the observed deposit against the inbound digest, and its
  [`release tests](../../services/interop/crates/layerx-bridge-relayer/tests/solana_release.rs)
  check the outbound digest, nullifier and program-derived addresses.

The tables retain the shared inputs and expected outputs. Integers in the
packed preimages are big-endian, with `chainId` and `amount` widened to 32 bytes
and nonces encoded in 8 bytes. Solana token amounts are bounded by `uint64`
even though the attestation amount field is 32 bytes.

The synthetic program, transaction signature, recipient and burn inputs are
derived from documented ASCII labels with `keccak256`, so implementations can
recompute them. The mint identities are the configured values in the identity
table. The labels and their derived keys and signature are test inputs, not
deployment values:

| Label | Derives |
|-------|---------|
| `PAXEERX_BRIDGE_SOLANA_VECTOR_PROGRAM` | the program id these vectors are computed against |
| `PAXEERX_BRIDGE_SOLANA_VECTOR_DEPOSIT_SIGNATURE` | the 64-byte deposit transaction signature, as `keccak256(label \|\| 0x01) \|\| keccak256(label \|\| 0x02)` |
| `PAXEERX_BRIDGE_SOLANA_VECTOR_PAXEER_RECIPIENT` | the Paxeer EVM address credited by the inbound deposit, as `keccak256(label)[12:32]` |
| `PAXEERX_BRIDGE_SOLANA_VECTOR_RECIPIENT` | the 32-byte Solana pubkey the outbound release pays |
| `PAXEERX_BRIDGE_SOLANA_VECTOR_BURN` | the Paxeer transaction hash of the burn the outbound release answers |

### Identities

| Identity | Value | 20 bytes in the digest |
|----------|-------|------------------------|
| Sidiora mint, 6 decimals | `5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump` | `0x21f7b20a555199fa73A238B1a91FD0f549068fEe`, the id the chain fixed, **not** the mint's handle `0x9232467d43fd9edd0bf250fe24ca0c9380b3ca86` |
| wrapped SOL mint, 9 decimals | `So11111111111111111111111111111111111111112` | `0xcf996523b5d068a26f0aa8a116602fe5033ee3a1`, its derived handle |
| vector program id | `A7SZbByPYuHpunZ9pyMDMrhMYvK44ANT1AVqb8U1FpM9` | not in a digest |
| vault-authority PDA, bump 255 | `GxxA9Cs9v5pAGVsaCe2jjDrtmieeBijcY4S5HHTY8Vq6` | `0x334121a65b47bd45c3f6381537d9180e98e445bc`, the registered vault |
| outbound recipient pubkey | `59TLtNdRpCZysEHkGDMPFHkHiHXkBqQVqAVxAupzNoQb` | `0xfb02125a3275d53a9f6538626b49894d2aae80cc` |

### Inbound: a Sidiora deposit on Solana, 12.345678 SID

Domain `PAXEERX_BRIDGE_IN_V1`, preimage 196 bytes.

| Offset | Size | Field     | Value |
|-------:|-----:|-----------|-------|
|      0 |   20 | domain    | ASCII `PAXEERX_BRIDGE_IN_V1` |
|     20 |   32 | chainId   | `91600046870081` |
|     52 |   20 | vault     | `0x334121a65b47bd45c3f6381537d9180e98e445bc` |
|     72 |   32 | txHash    | `0x4219bc1e7d357618e0c662c494981e70d02920d8f7c3f850dd25712ef87e48d3` |
|    104 |    8 | logIndex  | `7`, the deposit nonce in the receipt PDA |
|    112 |   32 | recipient | `0x000000000000000000000000b65aa00b0baa8fe2abc2d188312fb0a6d00e3ed5` |
|    144 |   20 | asset     | `0x21f7b20a555199fa73A238B1a91FD0f549068fEe` |
|    164 |   32 | amount    | `12345678` base units |

The transaction signature the txHash hashes is
`0xbe5dcb8ac1c1153020e5002e376da27e2c817c11725d6b37ea46deadb92b55efedb2c44ad7ec9132c7685ed84b08fcefb65b76a9912b1f0cc8f82eeb379b96e2`,
base58 `4okVsRAC31AiwNv6HUY1upyAF5hqTn6VownC1j7CKXcau7KovRyFXnnEkpUibUm1E2LQU5zxKwrGp8XVCZEBegsB`.

    preimage = 504158454552585f4252494447455f494e5f5631
               0000000000000000000000000000000000000000000000000000534f4c414e41
               334121a65b47bd45c3f6381537d9180e98e445bc
               4219bc1e7d357618e0c662c494981e70d02920d8f7c3f850dd25712ef87e48d3
               0000000000000007
               000000000000000000000000b65aa00b0baa8fe2abc2d188312fb0a6d00e3ed5
               21f7b20a555199fa73a238b1a91fd0f549068fee
               0000000000000000000000000000000000000000000000000000000000bc614e

    digest   = 0x3333b122e2a4e61ad324d5c875a8a9c72cf98a2724da26c211235e30a8d294ef

### Outbound: a release of 4.2 SID to a Solana account

Domain `PAXEERX_BRIDGE_OUT_V1`, preimage 185 bytes.

| Offset | Size | Field        | Value |
|-------:|-----:|--------------|-------|
|      0 |   21 | domain       | ASCII `PAXEERX_BRIDGE_OUT_V1` |
|     21 |   32 | chainId      | `91600046870081` |
|     53 |   20 | vault        | `0x334121a65b47bd45c3f6381537d9180e98e445bc` |
|     73 |   32 | paxeerTxHash | `0x6f79d9a61a77030bbeba5f435907f53b09321779310326b9faaebb391b3b5d5f` |
|    105 |    8 | paxeerNonce  | `11` |
|    113 |   20 | recipient    | `0xfb02125a3275d53a9f6538626b49894d2aae80cc`, the handle of `59TLtNdRpCZysEHkGDMPFHkHiHXkBqQVqAVxAupzNoQb` |
|    133 |   20 | asset        | `0x21f7b20a555199fa73A238B1a91FD0f549068fEe` |
|    153 |   32 | amount       | `4200000` base units |

    preimage = 504158454552585f4252494447455f4f55545f5631
               0000000000000000000000000000000000000000000000000000534f4c414e41
               334121a65b47bd45c3f6381537d9180e98e445bc
               6f79d9a61a77030bbeba5f435907f53b09321779310326b9faaebb391b3b5d5f
               000000000000000b
               fb02125a3275d53a9f6538626b49894d2aae80cc
               21f7b20a555199fa73a238b1a91fd0f549068fee
               0000000000000000000000000000000000000000000000000000000000401640

    digest   = 0xc583652dd9b59e0fcef102cfc8866a52beadb77b82de25445d86900baf6d1c4e

The release consumes the nullifier `keccak256(paxeerTxHash || paxeerNonce)` =
`0xd653f4968eb9b70e1eaef15fb134f2f7da8c15c38335af0069c94585e521ea3a`, which the
custody program records as a PDA. Here `paxeerNonce` is an 8-byte big-endian
integer. A replayed release is rejected with `BridgeError::Replayed` when that
PDA is already owned by the custody program or has non-empty data.
