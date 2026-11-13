<!-- Source: https://docs.paxeer.app/interchain/ -->

# Interchain (IBC)

Connect Paxeer X to other chains through IBC packets, native token transfers, and the attested bridge.

**Source:** `Paxeer-X-Network/interchain/`, `precompiles/ibc/`, and the IBC keeper and router wiring in `node/app.go`.

## Cross-chain routes in Paxeer X

Paxeer X supports IBC transfers through its Cosmos protocol stack and attested external-chain transfers through the [native bridge](https://docs.paxeer.app/bridge). IBC authenticates packets with light clients, connections, channel state, and packet proofs. The bridge authenticates external deposits with a configured attestor threshold and enforces registered chains, asset caps, nullifiers, and pause controls.

The bridge precompile at `0x0000000000000000000000000000000000001016` exposes `bridgeIn`, `bridgeOut`, chain and attestor queries, cap queries, replay status, and pause state. IBC transfers use `0x1009`. Choose the route supported by the destination and asset; an IBC channel identifier is separate from the bridge chain registry identifier.

Record the source transaction, destination identifier, asset denomination, amount, and transfer status. Treat packet acknowledgement or timeout separately from bridge attestation and destination release. Continue with [assets](https://docs.paxeer.app/assets), [settlement](https://docs.paxeer.app/settlement), and the [unified network](https://docs.paxeer.app/unified-network).

## Protocol and implementation

The in-tree IBC implementation provides clients, connections, channels, packet commitments, and application routing. Relayers submit proofs and carry packets and acknowledgements between chains. A working transfer requires compatible live clients, an open channel, and relayer delivery.

Query the target network for the port, channel, counterparty, and supported denomination before sending funds.

| Source directory | Purpose |
| --- | --- |
| `interchain/modules/core/` | Client, connection, channel, port, and commitment protocol logic. |
| `interchain/modules/apps/transfer/` | ICS-20 fungible-token transfer messages, packets, and keeper. |
| `interchain/modules/apps/27-interchain-accounts/` | Interchain-account implementation. |
| `interchain/modules/light-clients/` | Tendermint (`07-tendermint`), solo-machine (`06-solomachine`), and localhost (`09-localhost`) light-client code. |
| `precompiles/ibc/` | EVM interface to native ICS-20 transfers. |

## Inspect clients and channels

Configure the CLI's Cosmos RPC endpoint for the Paxeer node being queried, then inspect its state:

```
paxd query ibc client states
paxd query ibc connection connections
paxd query ibc channel channels
```

Verify the channel's port, connection, remote chain, ordering, and open state. Channel identifiers such as `channel-0` are local to a chain; they are not universal destinations.

## Transfer through the CLI

This command template sends `1,000,000 uhpx`, one PAX. Set `IBC_CHANNEL` to a verified channel and `IBC_RECIPIENT` to an address valid on its destination chain.

```
paxd tx ibc-transfer transfer \
  transfer "$IBC_CHANNEL" "$IBC_RECIPIENT" 1000000uhpx \
  --from mykey --chain-id hyperpax_125-1
```

Use the actual bank denomination for other assets. An IBC voucher has its own denomination trace; do not substitute a display ticker for a bank denomination.

## Transfer directly from the EVM

The precompile at `0x0000000000000000000000000000000000001009` exposes two nonpayable transfer methods. Use the full `precompiles/ibc/abi.json` for client bindings.

```
interface IBC {
    function transfer(
        string calldata toAddress,
        string calldata port,
        string calldata channel,
        string calldata denom,
        uint256 amount,
        uint64 revisionNumber,
        uint64 revisionHeight,
        uint64 timeoutTimestamp,
        string calldata memo
    ) external returns (bool success);

    function transferWithDefaultTimeout(
        string calldata toAddress,
        string calldata port,
        string calldata channel,
        string calldata denom,
        uint256 amount,
        string calldata memo
    ) external returns (bool success);
}
```

The immediate EVM caller must have a Paxeer address association and sufficient native-denomination balance. A wrapper uses the wrapper's associated account. `amount` is a bank base-unit integer, not payable EVM value; delegatecall and staticcall are rejected.

The explicit method takes an absolute remote timeout height and timestamp. IBC timestamps use nanoseconds. The default-timeout method derives its timeout from the channel's connection and client state: the height is the client's latest height plus the relative default `0-1000` (revision 0, height 1000), and the timestamp is ten minutes in nanoseconds beyond the later of the client consensus timestamp and the current block time. It can fail when required channel or client state is absent.

The ABI submits transfers; it does not expose channel enumeration or a general incoming-packet callback. Use the native query interfaces to inspect IBC state.

## Track the whole transfer

A successful local transaction means the source-chain transfer was accepted. It does not mean the destination has already received the assets. Retain the source transaction and packet identifiers, then track relay, acknowledgement, or timeout/refund handling on both chains.

Configure your relayer for both networks, check that its clients are current, and monitor packet delivery and acknowledgement processing.

## Implementation references

- `precompiles/ibc/IBC.sol`: exact public selectors.

- `precompiles/ibc/ibc.go`: associated sender, amount conversion, timeout derivation, and keeper invocation.

- `interchain/modules/apps/transfer/`: packet lifecycle and native transfer behavior.

- [Precompile reference](https://docs.paxeer.app/precompiles) and [native query interfaces](https://docs.paxeer.app/rest-grpc).
