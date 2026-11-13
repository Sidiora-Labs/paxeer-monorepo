# RPC test overview

[`SetupTestServer`](utils.go) initializes an application, commits an empty block
with the supplied state initializers, then finalizes and commits the supplied
blocks. After `*testing.T`, it accepts `[][][]byte`: a list of blocks, each
containing a list of encoded transactions. Its remaining arguments are
`func(sdk.Context, *app.App)` initializers that set state through the keepers.
Here `app` aliases the repository's `node` package.

Call the returned server's `Run` with `func(port int)` to make HTTP requests
while the server is running. For example,
`sendRequestWithNamespace("eth", port, "getTransactionByHash", hash)` calls
`eth_getTransactionByHash` and returns the decoded JSON-RPC response.

Run the package tests from the repository root with
`CGO_ENABLED=1 go test -count=1 -timeout 35m ./rpc/tests/`. The tests read
`ERC20.bin` and other files relative to this directory, which `go test` uses
as the working directory.

## Transaction generation

1. Create unsigned transaction data, such as `send(nonce)` for an EVM transfer.
2. Sign it with `signTxWithMnemonic` for EVM data or `signCosmosTxWithMnemonic`
   for a Cosmos message.
3. Encode it with `encodeEvmTx(txData, signed)` or `encodeCosmosTx(tx)`.

`signAndEncodeTx(txData, mnemonic)` combines EVM signing and encoding.
`signAndEncodeCosmosTx(msg, mnemonic, accountNumber, sequence)` does the same
for a Cosmos message. `signTxWithMnemonic` alone returns a signed Ethereum
transaction, not the encoded bytes that `SetupTestServer` accepts.

## State initialization

Initializers have the signature `func(ctx sdk.Context, a *app.App)` and run
before the initial empty block is committed. The package provides:

- `mnemonicInitializer`: associates addresses and funds the mnemonic's account.
- `erc20Initializer`: deploys [`ERC20.bin`](ERC20.bin); `erc20Addr` holds its
  deterministic address.
- `cw20Initializer`: deploys
  [`contracts/wasm/cw20_base.wasm`](../../contracts/wasm/cw20_base.wasm), with
  optional EVM pointer registration controlled by its `pointer` argument.
