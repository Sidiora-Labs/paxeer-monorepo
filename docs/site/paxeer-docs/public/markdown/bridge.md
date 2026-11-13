<!-- Source: https://docs.paxeer.app/bridge/ -->

# Bridging and balances

Move assets between registered external chains and Paxeer with attestor signatures, asset limits, and replay protection.

## Choose the asset path

External-chain bridging moves value between a foreign vault and a Paxeer bank denomination. Paxeer mainnet-beta is one BFT chain with EVM chain ID 125; its kernel executes within the network. A successful Paxeer receipt is final when its block is committed. An external payout also requires the destination chain receipt.

| Boundary | Asset backing | Acceptance evidence |
| --- | --- | --- |
| External chain ↔ Paxeer X | Foreign-chain vault or Solana custody program | Threshold attestor signatures, registered chain and asset, caps, unique source event |
| Transfers within Paxeer | Native bank balances and derived sub-accounts | Successful transaction execution in a committed block |

## External-chain connectors

The external bridge uses `PaxeerXVault` on EVM chains and a custody program on Solana. Its Paxeer X interface is the `LayerXBridge` precompile at `0x0000000000000000000000000000000000001016`. Governance controls chain registration, vault identity, finality depth, attestor sets, per-asset limits, and pause state.

Default bridge genesis contains no registered chains, attestors or asset caps. Governance registers and enables each chain, sets the attestor threshold, and registers asset limits before deposits can be accepted. The connector configurations are under contracts/bridge/evm/chains/ and contracts/bridge/solana/chains/solana/. Read the active on-chain configuration for the selected chain before depositing.

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

Resolve the registered vault and asset configuration for the selected connector before depositing. Solana uses chain-specific encodings for its keys, mints, and transaction identities; the SID asset identifier is declared explicitly in its connector configuration. Do not treat a Solana chain identifier as an EVM RPC chain ID.

### Deposit from an external chain

Each step below presumes the chain and asset have been registered by governance.

1. Choose the registered chain, asset, amount, and Paxeer EVM recipient encoded as a left-padded bytes32 value. Check pause state and the asset's per-transaction and in-flight limits.
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

## Read bridge configuration

The bridge ABI is in precompiles/layerxbridge/LayerXBridge.sol. Its view methods expose the chain registration, attestor set, caps and replay state used by the keeper.

1. Read the registered vault, finality depth and enabled flag for the selected chain.
2. Read the current attestor identities and signature threshold.
3. Read the asset denomination, per-transaction cap and in-flight supply cap.
4. Check pause and source-event replay state before submitting a deposit.

### Signature acceptance

The inbound digest binds the remote chain, registered vault, source transaction hash, log index, Paxeer recipient, asset and amount. The recipient is a nonzero EVM address in the low 20 bytes of a bytes32 value, with its high 12 bytes zero.

The signature format is specified in modules/layerxbridge/ATTESTATION.md. The inbound domain is PAXEERX_BRIDGE_IN_V1; the EVM outbound domain is PAXEERX_BRIDGE_OUT_V1. Attestors sign the raw Keccak-256 digest. Each accepted signature must recover a current attestor, and the recovered addresses must be strictly ascending.

## Track an external withdrawal

A successful bridgeOut burns the caller's bridged denomination and advances the registered chain's outbound nonce. The foreign vault releases the corresponding asset after checking its outbound signatures, cap, pause state and replay protection.

1. Preserve the Paxeer transaction hash, chain, asset, recipient, amount and outbound nonce.
2. Obtain signatures for that exact transfer using the registered foreign custody configuration.
3. Submit the release and wait for the destination chain receipt.
4. Reconcile the Paxeer burn with the foreign release event before reporting payout.

| Condition | Result |
| --- | --- |
| Bridge is paused | Inbound and outbound transfers are refused |
| Chain is unregistered or disabled | Transfers for that chain are refused |
| Source event has already been credited | A second inbound credit is refused |
| Transfer exceeds its per-transaction cap | The transfer is refused |

## Recovery and reconciliation

Reconcile a timed-out deposit against its original source event. The bridge exposes `isNullified` to check whether the inbound event has been consumed. For EVM withdrawals, the foreign vault consumes a nullifier derived from the Paxeer transaction hash and outbound nonce.

Keep the foreign deposit, Paxeer credit, Paxeer burn and destination release as separate records. Inspect the existing transfer before retrying. Native account funding uses bank transfers within Paxeer; it does not require a separate custody deposit or checkpoint settlement step.

## Continue building

Read [interoperability](https://docs.paxeer.app/interop) for foreign protocol connectors, [the unified architecture](https://docs.paxeer.app/paxeer-vs-layerx) for execution and settlement responsibilities, and [precompiles](https://docs.paxeer.app/precompiles) for EVM integration.
