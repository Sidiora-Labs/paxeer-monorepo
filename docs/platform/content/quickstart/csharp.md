# C# quickstart

Add a payment to a .NET application. Four lines, no protocol vocabulary, no key handling.

## Before you start

Use .NET 10 or newer. Configure an authenticated human-payment service that supports
`/v1/moves/quote`, `/v1/moves` and journey reads. The local emulator retires
these payment-write routes. For native node transactions, follow the
[reference applications](../guide/reference-applications.md).

```sh
dotnet add package LayerX.Sdk
```

| Variable | What it is |
|---|---|
| `LAYERX_API_URL` | The base URL of your human-payment service |
| `LAYERX_API_TOKEN` | A bearer token identifying your account |

## The integration

```csharp sample=first-payment-csharp
using var token = new AccessToken(Encoding.UTF8.GetBytes(apiToken));
var layerx = new PlatformClient(new HumanHttpTransport(new Uri(apiUrl), accessToken: token));
var quote = await layerx.HumanMoveQuoteAsync(JsonValue.Object(new Dictionary<string, JsonValue> { ["source"] = JsonValue.String(source), ["destination"] = JsonValue.String(destination), ["money"] = money }));
var journey = await layerx.HumanMoveCommitAsync(JsonValue.Object(new Dictionary<string, JsonValue> { ["quote_id"] = JsonValue.String(Text(quote, "quote_id")) }), new IdempotencyKey(paymentKey));
```

`AccessToken` is `IDisposable` and zeroes its buffer on disposal, which is why it is declared `using`. `HumanMoveCommitAsync` takes an `IdempotencyKey` as a required parameter, so the compiler will not let you spell a money-moving call without one.

`JsonValue` is a discriminated union with no floating-point case. Amounts stay exact.

## Run the whole sample

Set `LAYERX_API_URL` and `LAYERX_API_TOKEN` for that service, plus
`LAYERX_SOURCE`, `LAYERX_DESTINATION`, `LAYERX_AMOUNT`, `LAYERX_CURRENCY`
and `LAYERX_PAYMENT_KEY` for the payment. Amounts use integer base units;
reuse the same payment key when reconciling the same operation.

```sh
cd services/platform/examples/samples/first-payment-csharp
dotnet build --configuration Release
dotnet run --configuration Release
```

The sample polls the journey up to 40 times at 250 ms intervals, prints a JSON report of the journey identifier, state and receipt references, and returns exit code 2 when the journey did not complete.

## Handling refusals

`PlatformSdkException` carries the machine code and the retry class. Branch on the code; never on the message.

| Code | What to do |
|---|---|
| `idempotency-required` | You omitted the key |
| `idempotency-conflict` | Same key, different body |
| `rate-limit` | Wait for the carried retry timing |
| `unknown-outcome` | Do not retry. Resolve by receipt lookup under your key |
| `budget-refusal` | The payment exceeded a funded budget |

## Enforced by

| Capability | Layer | What that means here |
|---|---|---|
| Atomic settlement | `protocol` | The committed move applies completely or not at all. |
| Replay refusal | `protocol` | A retried submission cannot apply twice. |
| Idempotent money moves | `service` | The commit signature makes the key impossible to omit. |
| Quote then commit | `service` | The quote you committed is the one that executes. |
| Done means verified | `service` | The `done` state is backed by receipt evidence. |
