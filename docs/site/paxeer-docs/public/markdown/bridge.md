<!-- Source: https://docs.paxeer.app/bridge/ -->

# Bridging and custody

Move assets between external chains, Paxeer X settlement, and LayerX execution with explicit proofs, replay protection, and withdrawal guarantees.

## Choose the asset path

Paxeer X connects two distinct asset boundaries. External-chain bridging moves value between a foreign vault and a Paxeer X representation. The LayerX custody bridge moves value between Paxeer X custody and the LayerX execution ledger. An application can compose these paths while retaining a separate identifier and acceptance proof for each step.

| Boundary | Asset backing | Acceptance evidence |
| --- | --- | --- |
| External chain ↔ Paxeer X | Foreign-chain vault or Solana custody program | Threshold attestor signatures, registered chain and asset, caps, unique source event |
| Paxeer X ↔ LayerX | Paxeer X custody with a LayerX reserve mirror | Tendermint light-client proof for deposits; finalised checkpoint and membership evidence for withdrawals |

## External-chain connectors

The external bridge uses `PaxeerXVault` on EVM chains and a custody program on Solana. Its Paxeer X interface is the `LayerXBridge` precompile at `0x0000000000000000000000000000000000001016`. Governance controls chain registration, vault identity, finality depth, attestor sets, per-asset limits, and pause state.

The bridge ships dormant. Its default genesis registers no chains, sets no attestors, and sets no caps; only a governance registration brings a chain up. The nine connectors below are the chains the deployment tooling is built for, not chains a deposit can use today. The chain configurations committed with that tooling also carry placeholder owner, attestor, and program-id values by design, and the deployability gate refuses them until the owner supplies real values. Confirm the chain, asset, attestor set, and caps are registered on-chain before treating any connector as a live deposit path.

| Connector | Chain identifier | Native counterpart |
| --- | --- | --- |
| Ethereum | 1 | ETH |
| Base | 8453 | ETH |
| Arbitrum | 42161 | ETH |
| Optimism | 10 | ETH |
| BNB Chain | 56 | BNB |
| Polygon | 137 | POL |
| Avalanche | 43114 | AVAX |
| HyperEVM | 999 | HYPE |
| Solana | 91600046870081 | SOL; also Sidiora (SID) |

Resolve the registered vault and asset configuration for the selected connector before depositing. Solana uses the bridge's mapped handles for its keys, mints, and transaction identities. Do not treat a Solana chain identifier as an EVM RPC chain ID.

### Deposit from an external chain

Each step below presumes the chain and asset have been registered by governance.

1. Choose the registered chain, asset, amount, and Paxeer X recipient. Check pause state and the asset's per-transaction and in-flight limits.
2. Lock the asset in the configured foreign vault. EVM native-coin deposits use `depositNative(bytes32 paxeerRecipient)`.
3. The relayer observes the deposit after the connector's configured finality depth. Attestors sign the deposit digest binding chain, vault, transaction hash, log index, recipient, asset, and amount.
4. Submit `bridgeIn` with the event and threshold signatures. The precompile credits the bridged denomination and emits `BridgeIn`.
5. Retain the source event and Paxeer X receipt. The source tuple `(chain, txHash, logIndex)` is consumable once.

### Withdraw to an external chain

`bridgeOut(chain, asset, amount, recipient)` burns the caller's bridged denomination and returns a nonce. The `BridgeOut` event identifies the outbound transfer. Attestors sign the foreign vault's release digest, and the relayer submits the release to the destination custody contract. Track both the Paxeer X burn and foreign release; a burn receipt alone is not the destination payout receipt.

### Precompile reference

```
interface ILayerXBridge {
  function bridgeIn(uint64 chain, address vault, bytes32 txHash,
    uint64 logIndex, bytes32 recipient, address asset,
    uint256 amount, bytes[] calldata signatures)
    external returns (string memory denom);
  function bridgeOut(uint64 chain, address asset, uint256 amount,
    address recipient) external returns (uint64 nonce);
}
```

Use `getChain`, `getAttestors`, `getCap`, `isPaused`, and `isNullified` for preflight and reconciliation. Inbound signatures are 65-byte `r || s || v` signatures over the raw bridge digest, with low s, v equal to 27 or 28, and strictly ascending signer addresses. Signing a prefixed personal message produces different evidence.

## Deposit into LayerX execution

Paxeer X holds custody while LayerX maintains the reserve mirror. A successful deposit credit moves value from the reserve to the beneficiary on the LayerX ledger.

1. Create the custody deposit with the intended asset, beneficiary, owner key, amount, and nonce.
2. Obtain a proof of the custody `Deposit` record against a trusted Paxeer X header.
3. Build the `LXDC3` credit head and attach its `LXLB1` light-client bundle.
4. Submit the signed credit activity and verify its LayerX receipt. Keep the deposit identifier for idempotent reconciliation.

### Proof acceptance

The credit head is 363 bytes. `LXDC3` occupies its first five bytes, the SHA-256 of the attached proof bundle sits at bytes 327–359, and the `LXLB1` bundle begins at byte 363. The head also binds the network, protocol version, deposit identifier, asset, beneficiary, owner key, depositor, amount, and nonce, and verification checks the Tendermint proof in the bundle.

The proof binds state height H to signed header H+1, its application root, and validator hash. These fields describe consensus state evidence. This custody path proves the deposit record directly; it does not accept an external bridge attestor signature as a substitute. Reused deposit identifiers, mismatched assets, incorrect beneficiaries, invalid signatures, and non-final proofs are refused.

## Withdraw from LayerX custody

A withdrawal request debits the LayerX account into the withdrawals holding account before payout and binds the asset recorded at request time. Finalisation requires a finalised checkpoint, guarantor certificate, membership proof, and a closed challenge window.

1. Submit the withdrawal request and preserve its receipt and requested destination.
2. Obtain checkpoint and membership evidence for the request.
3. Wait for checkpoint finality and challenge-window closure.
4. Finalise on Paxeer X and reconcile the payout with the request's nullifier.

| Condition | Result |
| --- | --- |
| Challenge window remains open | Finalisation is refused until the window closes |
| Claim is cancelled | Finalisation is refused |
| Nullifier has already settled | A second payout is refused |
| Caller supplies a different asset | Request is refused without a transfer |

## Emergency exit and reconciliation

When the sequencer is unavailable or its liveness bound is exceeded, emergency exit uses the last finalised checkpoint, a balance proof, and recorded guarantor signatures. The library exposes `lxp_exit_eligibility` for the eligibility decision. Emergency exit is a dedicated recovery path rather than another bridge activity.

Keep source evidence, admission acknowledgement, execution receipt, checkpoint evidence, and payout receipt as distinct records. A retry first looks up the deposit identifier, operation receipt, or withdrawal nullifier; it does not create a fresh claim for the same economic transfer.

## Continue building

Read [interoperability](https://docs.paxeer.app/interop) for foreign protocol connectors, [the unified architecture](https://docs.paxeer.app/paxeer-vs-layerx) for execution and settlement responsibilities, and [precompiles](https://docs.paxeer.app/precompiles) for EVM integration.
