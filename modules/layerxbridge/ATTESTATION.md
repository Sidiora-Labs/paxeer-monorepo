# PaxeerX bridge attestation digests

Attestors sign one of two raw digests with secp256k1 ECDSA. The digest is
`keccak256` over the packed preimage below: no EIP-191 `"\x19Ethereum Signed Message"`
prefix and no EIP-712 domain separator or struct hashing. Every integer is
big-endian and fixed width; addresses are their 20 raw bytes; the domain is the
ASCII string with no length prefix and no terminator. This is exactly
Solidity `abi.encodePacked` of the listed types
([BridgeAttestation.sol](../../contracts/bridge/evm/src/BridgeAttestation.sol)).
This page documents the Paxeer side of the
[EVM vault attestation format](../../contracts/bridge/evm/ATTESTATION.md);
`types.InboundPreimage` / `types.OutboundPreimage` in
[types/attestation.go](types/attestation.go) build these bytes, and
[keeper/attestation_test.go](keeper/attestation_test.go) recomputes the vectors
below. The bridge module runs within the Paxeer network. In these digests,
`chainId` identifies the remote chain; the Go API accepts a `uint64` and packs
it as a 32-byte `uint256`.

## Outbound: Paxeer -> Ethereum (`PaxeerXVault.release`)

Domain `PAXEERX_BRIDGE_OUT_V1`. Preimage length 185 bytes.

| Offset | Size | Field           | Encoding                                        |
|-------:|-----:|-----------------|-------------------------------------------------|
|      0 |   21 | domain          | ASCII `PAXEERX_BRIDGE_OUT_V1`                   |
|     21 |   32 | chainId         | uint256, Ethereum chain id of the vault         |
|     53 |   20 | vault           | address of the `PaxeerXVault`                   |
|     73 |   32 | paxeerTxHash    | bytes32, Paxeer transaction hash of the burn    |
|    105 |    8 | paxeerNonce     | uint64, Paxeer bridge nonce of the burn         |
|    113 |   20 | recipient       | address receiving the release on Ethereum       |
|    133 |   20 | asset           | ERC20 address, or `0x00..00` for native ETH     |
|    153 |   32 | amount          | uint256, base units of `asset`                  |

`digest = keccak256(preimage)`; `PaxeerXVault.releaseDigest(...)` returns it for
the deployed vault and current chain.

Signature rules enforced by the vault:

- each signature is 65 bytes `r (32) || s (32) || v (1)` with `v` in {27, 28}
  and `s <= secp256k1n / 2`;
- signatures are ordered by strictly ascending recovered signer address, which
  also rejects a repeated signer;
- every recovered signer must be a current attestor and at least `threshold`
  signatures must be supplied;
- the nullifier `keccak256(paxeerTxHash (32) || paxeerNonce (8, big-endian))`
  is consumed by a successful release and can never be released again.

## Inbound: Ethereum -> Paxeer (`BridgeDeposit` log, verified by `modules/layerxbridge`)

Domain `PAXEERX_BRIDGE_IN_V1`. Preimage length 196 bytes. This mirrors the
outbound layout. `bridgeIn` on the
[layerxBridge precompile](../../precompiles/layerxbridge/layerxbridge.go) at
`0x0000000000000000000000000000000000001016` verifies it.

| Offset | Size | Field           | Encoding                                              |
|-------:|-----:|-----------------|-------------------------------------------------------|
|      0 |   20 | domain          | ASCII `PAXEERX_BRIDGE_IN_V1`                          |
|     20 |   32 | chainId         | uint256, Ethereum chain id of the vault               |
|     52 |   20 | vault           | address of the `PaxeerXVault` that emitted the log    |
|     72 |   32 | txHash          | bytes32, Ethereum transaction hash of the deposit     |
|    104 |    8 | logIndex        | uint64, log index of `BridgeDeposit` in its block     |
|    112 |   32 | recipient       | bytes32 `paxeerRecipient` from the deposit            |
|    144 |   20 | asset           | ERC20 address, or `0x00..00` for native ETH           |
|    164 |   32 | amount          | uint256, base units of `asset`                        |

`PaxeerXVault.depositDigest(...)` returns it for the deployed vault and current
chain.

The deposit event is:

```
event BridgeDeposit(address indexed asset, uint256 amount, address indexed sender,
                    bytes32 indexed paxeerRecipient, uint64 nonce);
```

`nonce` is the vault's sequential deposit counter starting at 0; the digest
binds `logIndex` rather than `nonce` so it can be checked against the receipt.

Rules enforced by `bridgeIn` on Paxeer:

- signatures follow the vault's rules above: 65 bytes `r || s || v`, `v` in
  {27, 28}, low `s`, strictly ascending recovered signer, every signer a current
  attestor, at least `threshold` of them, with at most 64 signatures;
- `chainId` must be a registered, enabled chain and `vault` its registered vault;
- `(chainId, asset)` must have a registered bridged denom;
- `recipient` must be `bytes32(uint256(uint160(paxeerEvmAddress)))`: the high 12
  bytes zero and the low 20 bytes a non-zero Paxeer EVM address, which receives
  the mint;
- the nullifier `(chainId, txHash, logIndex)` is consumed once;
- the bridge must not be paused, and `amount` must be positive, below `2^255`,
  within the asset's per-transaction cap and keep its in-flight supply within
  its in-flight cap.

### Minted denom and the Sidiora exception

For every pair except canonical Sidiora, the minted tokenfactory denom is
`factory/{bridge module address}/lxb{hex(keccak256(uint64 chainId || asset)[:20])}`.
Here `chainId` is eight bytes, big-endian, and `asset` is 20 raw bytes.
The bridge module account administers the denom; governance `SetCap` creates
it when first setting the pair's caps.

Canonical Sidiora instead uses `factory/{bridge module address}/usid`, returned
by `types.SidioraDenom()`. Its pair is `types.SidioraHomeChainID`
(`0x0000534f4c414e41`, Solana) with remote asset
`0x21f7b20a555199fa73A238B1a91FD0f549068fEe`, as declared in
[types/keys.go](types/keys.go). Governance must register the chain, call
`RegisterSidioraPair`, and then set its caps, through
[keeper/msg_server.go](keeper/msg_server.go). `SetCap` refuses this pair until
it is registered with the canonical denom, and refuses an existing generic
denom that would require a governed migration.

`RegisterSidioraPair` uses `EnsureSidioraDenom` in
[keeper/sidiora.go](keeper/sidiora.go) to create or validate the module-owned
`usid` denom and install bank metadata: base exponent 0, display and symbol
`SID` at exponent 6. Both minting and burning use the registered asset record's
denom. [types/genesis.go](types/genesis.go) also enforces this exception through
`AssetDenom`: only the canonical pair may use `usid`; all other pairs must use
their derived `lxb…` denom.

## Outbound on Paxeer: `bridgeOut`

`bridgeOut(chain, asset, amount, recipient)` burns the caller's registered
bridged denom and emits, from `0x0000000000000000000000000000000000001016`,

```
event BridgeOut(uint64 indexed chain, address indexed asset, uint256 amount,
                address recipient, uint64 indexed nonce);
```

Attestors sign the outbound digest with `paxeerTxHash` = the hash of the Paxeer
transaction carrying that log and `paxeerNonce` = its `nonce`, a per-chain
counter starting at 1.

The chain must be registered and enabled, the asset registered, and the bridge
unpaused. The amount must be positive, below `2^255`, within the per-transaction
cap and no greater than the in-flight bridged supply; the recipient must be
non-zero. A successful burn reduces that supply.

## Test vectors

Common inputs: `chainId = 1`, `vault = 0x1111111111111111111111111111111111111111`,
`hash = 0x2222222222222222222222222222222222222222222222222222222222222222`,
nonce / logIndex `= 7`, `asset = 0x4444444444444444444444444444444444444444`,
`amount = 1000000000000000000`.

- Outbound with `recipient = 0x3333333333333333333333333333333333333333`:
  `0xbd35888e4b158986238ce7abe73957702e2f6e78fe6157197878ebd13edf5b37`
- Inbound with `recipient = 0x5555555555555555555555555555555555555555555555555555555555555555`:
  `0x511964ae9566f9536604258667400b0d76335e2a6e60ab0f700bf2433bc97918`

The inbound vector exercises digest encoding only: its all-`0x55` recipient
does not satisfy the padding rule required for an actual deposit or mint.
Both vectors are asserted by Go `TestDigestVectorsFromAttestationDoc` and
Solidity `test_DigestVectorsFromAttestationDoc`.
