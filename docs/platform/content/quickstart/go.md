# Go quickstart

Use the human-payment client from a Go service. The integration region has nine lines; the complete sample supplies request and response types, configuration and journey polling.

## Before you start

Use Go 1.25 or newer. Configure an authenticated human-payment service that supports
`/v1/moves/quote`, `/v1/moves` and journey reads. The local emulator does not implement
these payment-write routes. For native node transactions, follow the
[reference applications](../guide/reference-applications.md).

```sh
go get github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go
```

| Variable | What it is |
|---|---|
| `LAYERX_API_URL` | The base URL of your human-payment service |
| `LAYERX_API_TOKEN` | A bearer token identifying your account |

## The integration

```go sample=first-payment-go
authorize := func(request *http.Request) error { request.Header.Set("Authorization", "Bearer "+apiToken); return nil }
transport, err := layerx.NewHumanHTTPTransport(apiURL, nil, authorize)
exitOn(err)
client, err := layerx.NewClient(transport, nil)
exitOn(err)
var quote MoveQuote
exitOn(client.Human(ctx, layerx.HumanOperationMoveQuote, MoveQuoteRequest{Source: source, Destination: destination, Money: money}, &quote, layerx.CallOptions{}))
var journey Journey
exitOn(client.Human(ctx, layerx.HumanOperationMoveCommit, MoveCommitRequest{QuoteID: quote.QuoteID}, &journey, layerx.CallOptions{IdempotencyKey: key}))
```

The Go SDK ships its own human-plane HTTP transport, so the only thing you supply is how to authorise a request. The closure here sets a bearer header; if your deployment mints credentials per request, put that logic in the same place.

`layerx.CallOptions{IdempotencyKey: key}` is required on `HumanOperationMoveCommit`. Omit it and `Client.Human` refuses locally rather than sending an unprotected mutation.

## Run the whole sample

Set `LAYERX_API_URL` and `LAYERX_API_TOKEN` for that service, plus
`LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY`
and `LAYERX_PAYMENT_KEY` for the payment. Amounts use integer base units;
reuse the same payment key when reconciling the same operation.

```sh
cd services/platform/examples/samples/first-payment-go
go mod download
go run .
```

The sample declares its own request and response structs so the compiler checks the shapes you care about, polls `HumanOperationJourneyGet` with a path parameter up to 40 times at 250 ms intervals, and prints a report.

## Why the count is nine and not four

Go has no exceptions, so `exitOn(err)` appears after each fallible call. The measured region includes the calls to `exitOn`; its implementation and your struct definitions sit outside that region. The [samples page](../reference/samples.md) states the counting rule for every language.

## Handling refusals

Use `errors.As` to recover `*layerx.SDKError` and branch on its `Code` and `Retry` fields.

| Code | What to do |
|---|---|
| `idempotency-required` | You omitted `CallOptions.IdempotencyKey` |
| `idempotency-conflict` | Same key, different body |
| `rate-limit` | Wait for the carried retry timing |
| `unknown-outcome` | Reconcile the original journey and its evidence before another payment |
| `budget-refusal` | The service reported a budget or limit refusal; inspect its evidence for the enforcement layer |

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The committed move applies completely or not at all. |
| Replay refusal | `protocol` | Rebroadcasting the same signed transaction cannot apply twice. |
| Idempotent money moves | `service` | Repeating the commit with the same key returns the original journey. |
| Quote then commit | `service` | The quote you committed is the one that executes. |
| Done means verified | `service` | The `done` state is backed by receipt evidence. |

The sample reports the service's journey state and evidence references. It does not independently verify receipt bytes or block commitment.
