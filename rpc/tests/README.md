## RPC Test Overview
An RPC test under `rpc/tests` involves generating the chain state as if the chain started from a specified genesis state and processed a number of specified blocks. This is encapsulated in one `SetupTestServer` call (in `utils.go`). After the `*testing.T`, its second argument is a `[][][]byte` which represents a list of blocks (note that `[]byte` represents a transaction and `[][]byte` represents a block), and the remaining arguments are a series of `func(sdk.Context, *app.App)` which represent the genesis state initializers and allows test writers to set genesis state directly through keeper functions without having to write it as a json file. Once a test server is set up, test writers can call `Run` with a function parameter `func(port int)`, within which requests can be made via calls like `sendRequestWithNamespace(<namespace e.g. eth>, port, <endpoint e.g. getTransactionByHash>, parameters...)`. 

Run the inventory sweep from the repository root with `CGO_ENABLED=1 go vet ./rpc/tests/ && CGO_ENABLED=1 go test -count=1 -timeout 35m ./rpc/tests/`. The tests read `ERC20.bin` and other files by paths relative to this directory, which `go test` uses as the working directory.

### Transaction Generation
To generate a transaction to feed into `SetupTestServer`, it follows a three-step process:
1. create unsigned message (e.g. `send(<nonce>)` generates a simple EVM send message)
2. sign the message (e.g. `signTxWithMnemonic` signs an EVM message, and `signCosmosTxWithMnemonic` signs a Cosmos message)
3. encode the message into bytes (e.g. `encodeEvmTx` encodes an EVM tx, `encodeCosmosTx` encodes a Cosmos tx)
There are also helper functions that consolidate these three steps into one line (e.g. `signAndEncodeCosmosTx(bankSendMsg(...),...)` or `signTxWithMnemonic(send(...),...)`), but there might be times when explicit steps are preferred (e.g. record transaction hash before it's encoded).

### Genesis State Initialization
The "genesis state" of the chain backing a test server can be initialized via a variable number of functions with signature `func(ctx sdk.Context, a *app.App)`, where `*app.App` provides keeper access to allow directly setting state with the familiar keeper functions. Some of the common initializers are already predefined:
- `mnemonicInitializer` sets address association and initial funds for an account specified by the mnemonic.
- `erc20Initializer` creates an ERC20 contract with `ERC20.bin`
    - the ERC20 created this way has a deterministic address stored in `erc20Addr`
- `cw20Initializer` creates a CW20 contract with `contracts/wasm/cw20_base.wasm`
    - the CW20 created this way has a deterministic address `pax18cszlvm6pze0x9sz32qnjq4vtd45xehqs8dq7cwy8yhq35wfnn3qcm2ty5`
