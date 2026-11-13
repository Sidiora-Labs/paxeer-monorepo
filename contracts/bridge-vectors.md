# Bridge attestation vectors

The native bridge module and foreign custody implementations share one byte
contract. [modules/layerxbridge/types/attestation.go](../modules/layerxbridge/types/attestation.go)
builds the preimages; [ATTESTATION.md](../modules/layerxbridge/ATTESTATION.md)
specifies them. The EVM implementation is
[BridgeAttestation.sol](bridge/evm/src/BridgeAttestation.sol), and
[bridge/vectors/solana.go](bridge/vectors/solana.go) owns the Go mapping from
Solana identities. These attestations authorize cross-chain minting and
release; Paxeer receipt finality comes from committed BFT blocks on chain ID
125, with the kernel participating in the same network.

## Packed preimages

Concatenate the fields below without ABI offsets, dynamic-length prefixes or
padding between fields. Integers are unsigned big-endian; uint256 occupies 32
bytes and uint64 occupies 8. An address is exactly 20 raw bytes.

| Direction | Ordered fields | Length |
| --- | --- | --- |
| Inbound | ASCII `PAXEERX_BRIDGE_IN_V1`, uint256 chainId, address vault, bytes32 txHash, uint64 logIndex, bytes32 recipient, address asset, uint256 amount | 196 bytes |
| Outbound | ASCII `PAXEERX_BRIDGE_OUT_V1`, uint256 chainId, address vault, bytes32 paxeerTxHash, uint64 paxeerNonce, address recipient, address asset, uint256 amount | 185 bytes |

The digest is `keccak256(preimage)`. Sign the raw digest with secp256k1;
there is no EIP-191 prefix or EIP-712 wrapper. An attestation signature is 65
bytes, `r || s || v`, with `v` equal to 27 or 28 and canonical low `s`.
Recovered signers must be strictly ascending, current attestors, and meet
the configured threshold. The inbound nullifier is
`(chainId, txHash, logIndex)`. The inbound recipient is a nonzero EVM address
in the low 20 bytes of a 32-byte word, with its high 12 bytes zero.
Amounts are asset base units. Solana custody amounts must also fit uint64,
even though the shared preimage encodes the amount as uint256.

## Chain config

The nine committed chain configurations are inputs to the
[deployment schema](bridge-deploy.md#chainconfig), not deployed address records.
Their chain IDs are Ethereum 1, Base 8453, Arbitrum 42161, Optimism 10, BNB 56,
Polygon 137, Avalanche 43114, HyperEVM 999, and Solana 91600046870081.
The eight EVM configs are under [bridge/evm/chains](bridge/evm/chains/);
Solana's is under [bridge/solana/chains](bridge/solana/chains/).
For an EVM asset the digest's asset ID is its address, including address zero
for the native coin. Do not substitute Paxeer's chain ID for the foreign
chain ID in a bridge attestation.

## Solana

Decode base58 keys into 32 raw bytes before hashing. The mapping implemented
by `Key`, `Handle`, `InboundTxHash`, `PaxeerRecipient`, `VaultAuthority`, and
`VaultHandle` is:

| Digest input | Mapping |
| --- | --- |
| chainId | `91600046870081`, the uint64 value of ASCII `SOLANA` left-padded to eight bytes (`0x0000534f4c414e41`), then encoded as uint256 in the preimage |
| vault | `handle(vault-authority PDA)` for the deployed custody program |
| handle(key) | last 20 bytes of `keccak256(key[0:32])` |
| txHash | `keccak256` of the 64 raw bytes of the observed transaction signature, not its base58 text |
| logIndex | deposit nonce in the deposit-receipt PDA |
| inbound recipient | EVM address left-padded to 32 bytes with zero |
| outbound recipient | handle of the full Solana recipient public key supplied to the release instruction |
| asset | registered 20-byte mint ID; normally `handle(mint)`, with Sidiora's explicit ID below |

The vault-authority PDA uses the single seed `vault-authority`. The derivation
hashes seeds, bump, program ID and `ProgramDerivedAddress` with SHA-256,
rejects points on the ed25519 curve, and searches bumps from 255 downwards.
That PDA's handle, rather than the program ID's handle, is registered as the
vault. The deployed program ID is taken from
[deployment records](bridge-deploy.md#records); the vector program below is
only a reproducible test identity.

| Mint | Decimals | Asset ID |
| --- | --- | --- |
| wrapped SOL `So11111111111111111111111111111111111111112` | 9 | `0xcf996523b5d068a26f0aa8a116602fe5033ee3a1` |
| Sidiora `5w3wVdJaESaJKyLmStM6Hv9UyUkmZ1b9DLQquAqqpump` | 6 | `0x21f7b20a555199fa73A238B1a91FD0f549068fEe` |

Sidiora's ID is fixed by
[modules/layerxbridge/types/keys.go](../modules/layerxbridge/types/keys.go)
and the keeper's [Sidiora registration](../modules/layerxbridge/keeper/sidiora.go)
against `usid`; it is **not** the derived handle of the mint. Native SOL uses
wrapped SOL so the custody token path is shared. Solana transaction signatures
are ed25519 transport signatures; bridge attestations remain secp256k1.

### Pinned vectors

These values are declared in `solana.go` and rebuilt by the existing
[solana_test.go](bridge/vectors/solana_test.go) through the native module's
preimage builders. They are deterministic test values, not public deployment
addresses or evidence of a transfer.

| Common input | Hex value |
| --- | --- |
| program ID | `875f91f2b7ddd837d8584366fa8d7a8c73905b2212713a4826d4a89faa8d40fc` |
| vault-authority PDA, bump 255 | `ed349d8496a5f534b3bb8f5d240d8633acf502427eca37504ec7cf68766aa71d` |
| vault handle | `334121a65b47bd45c3f6381537d9180e98e445bc` |

Both vectors use Solana's chain ID and Sidiora's asset ID with six decimals.

| Inbound input/output | Value |
| --- | --- |
| txHash | `4219bc1e7d357618e0c662c494981e70d02920d8f7c3f850dd25712ef87e48d3` |
| logIndex | 7 |
| recipient | `000000000000000000000000b65aa00b0baa8fe2abc2d188312fb0a6d00e3ed5` |
| amount | `12345678` |
| digest, 196-byte preimage | `3333b122e2a4e61ad324d5c875a8a9c72cf98a2724da26c211235e30a8d294ef` |

| Outbound input/output | Value |
| --- | --- |
| Paxeer transaction hash | `6f79d9a61a77030bbeba5f435907f53b09321779310326b9faaebb391b3b5d5f` |
| Paxeer nonce | 11 |
| full recipient key | `3d992e1dc9d1cc1c5311d79c69a3801d40099046094d125fc687c6c57c395e88` |
| recipient handle | `fb02125a3275d53a9f6538626b49894d2aae80cc` |
| amount | `4200000` |
| digest, 185-byte preimage | `c583652dd9b59e0fcef102cfc8866a52beadb77b82de25445d86900baf6d1c4e` |

Use the [bridge precompile ABI](../precompiles/layerxbridge/abi.json) at
[`0x1016`](address-inventory.md#activation) for Paxeer-side calls over
[node JSON-RPC](evm-rpc.md). Explorer display configuration is separate from
these signed byte layouts: see [backend](explorer-config.md) and
[frontend](explorer-frontend-config.md).
