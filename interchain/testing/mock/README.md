# Mock IBC Application

This package supplies a small application for testing core IBC, light clients
and middleware through the [IBC testing harness](../README.md). It supports
channel behavior that the transfer application restricts. The application
implementation imports neither transfer nor simapp. The
[test simapp](../simapp/README.md) imports this package, and external tests
compose the mock application with transfer and simapp.

## Registration and callbacks

[`MockIBCApp`](ibc_app.go) holds a port ID, a scoped capability keeper and optional
function fields for the IBC application callbacks. Create it with
`NewMockIBCApp(portID, scopedKeeper)`, then register it with
`NewIBCModule(appModule, app)`, where `appModule` is a `*AppModule` and `app`
is the `*MockIBCApp`. During `AppModule.InitGenesis`, registered applications with a
non-empty, unbound port ID bind that port and claim its capability. An empty
port ID leaves port binding to the surrounding application or middleware.

Each non-nil callback replaces the corresponding default behavior in
[`IBCModule`](ibc_module.go). Overrides can return errors or application
acknowledgements, so tests must check their results. The defaults also perform
state changes and can fail:

- `OnChanOpenInit` claims the channel capability and returns any claim error.
- `OnChanOpenTry` claims the channel capability and returns `Version`
  (`mock-version`) on success, or an empty version and the claim error on failure.
  Its callback returns `(string, error)`, whereas open-init returns only `error`.
- Open-ack, open-confirm, close-init and close-confirm return nil when no
  override is installed.
- Receive, acknowledgement and timeout callbacks create per-packet canary
  capabilities so tests can detect whether state changes were retained or
  reverted. A capability creation failure, including a duplicate canary, panics.
  The default acknowledgement and timeout callbacks return nil after creating
  their canaries.

An open-init or open-try override bypasses the default capability claim; it must
claim the channel capability itself if subsequent operations require it. See the
[full middleware callback example](../README.md#middleware-testing).

## Packet acknowledgements

After creating its receive canary, the default `OnRecvPacket` selects a result by
comparing packet data with the values declared in [mock.go](mock.go):

| Packet data | Result |
| --- | --- |
| `MockPacketData` | `MockAcknowledgement`, a successful result acknowledgement |
| `MockAsyncPacketData` | nil, indicating asynchronous acknowledgement |
| Any other data, including `MockFailPacketData` | `MockFailAcknowledgement`, an error acknowledgement |

An error acknowledgement is an application-level refusal; it is not a Go error
returned by `OnRecvPacket`. Core IBC discards the application's callback state
changes on an unsuccessful acknowledgement while recording receipt and the
synchronous acknowledgement. A nil acknowledgement requires a later explicit
write by the application. This package does not promise that every callback is
a no-op or that every packet succeeds.
