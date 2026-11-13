# ICA availability contract

ICA is **unsupported on mainnet paxd**. This package supplies library controller
and host implementations for applications that explicitly integrate them. The
test application in `interchain/testing/simapp` is such an application; its
support does not describe mainnet availability.

`node/app.go` intentionally omits ICA controller and host stores, scoped
capabilities, keepers, the ICA module account, authentication middleware, module
services, genesis ordering and IBC routes. Its IBC router mounts transfer and
wasm. Mainnet does not accept ICA channel handshakes, account registration or
controller-to-host transactions, and does not serve ICA controller/host gRPC or
REST queries. Enabling library genesis parameters cannot activate these absent
components. Generic wasm IBC support does not mount the ICA host or controller.

The `interchain-accounts` / `ica` query commands are library tools. Every command's
help explains this availability boundary, including standalone controller/host
commands. Use them only with an application that explicitly wires ICA. The
`host packet-events` command searches historical ICA receive events; its presence
does not imply that mainnet accepts ICA packets. No mainnet ICA CLI command or
endpoint is promised by this package.

ICA genesis export captures and marshals state before `ExportGenesisStream`
returns. The returned channel contains one message and is closed, so consumers
may advance or close the source store, or abandon the stream. Imported state
still requires the receiving application's ICA and capability integration.
