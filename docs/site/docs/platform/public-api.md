# Public payment API

Payments on Paxeer X Network use the public node JSON-RPC at the root path of
`https://api-mainnet-beta.paxeer.network` or `api1` through
`api14.mainnet-beta.paxeer.network`, EVM chain ID `125` (`0x7d`). A payment
is a locally signed EVM transaction to a native precompile; it is final once
its block is committed. Mainnet-beta uses real value and there is no public
faucet; fund an EVM account with a native chain transfer.

The former activity API (`lx_sendActivity`, `lx_getReceipt`, `lx_subscribe`
and the other `lx_*` methods) and its `/rpc` routes are retired. The node and
the hosted gateway answer `lx_*` with `-32601`.

## Payment flow

| Step | Call |
| --- | --- |
| Check the network | `eth_chainId` returns `0x7d` |
| Bind the EVM address to a DID | `bindLayerX` on the address precompile `0x0000000000000000000000000000000000001004` |
| Register an Asset, open an account, mint | `register`, `openAccount`, `mint` on the ledger precompile `0x000000000000000000000000000000000000101a` |
| Pay | `payExact` on the ledger precompile, submitted with `eth_sendRawTransaction` |
| Read balances and assets | `balanceOf` and `getAsset` through `eth_call`, or `px_getBalances` and `px_listAssets` |
| Read the outcome | `pax_sequencerReceipt` until the rung is `committed` with `status: "0x1"` |

The [Payments guide](../overview/payments.md) gives the exact ABI
methods, signing steps, request bodies and refusal codes. The ledger ABI is
[`precompiles/layerxledger/abi.json`](https://github.com/Sidiora-Labs/paxeer-monorepo/blob/main/precompiles/layerxledger/abi.json).

## Hosted operations

Registration (`px_register`) and faucet claims (`px_requestFunds`) are
operations of the operator-deployed hosted identity and faucet services. A
faucet claim requires a hosted identity session:

```http
Authorization: Bearer <session-token>
```

See [Hosted gateway JSON-RPC](gateway-rpc.md) for the `px` namespace and
[Faucet](faucet.md) for claim behavior.
