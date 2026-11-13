# .NET SDK quickstart

NuGet: `LayerX.Sdk` at `sdk/platform/dotnet`. There is no public
JSON-RPC client in this package.

---

## Register

No `px_register` wrapper. Generated
`PlatformClient.AgentAgentRegisterAsync` /
`MutateAsync(PlatformOperation.AgentAgentRegister, …)` sends
`"agent.register"`. `HumanHttpTransport` is the human plane. Use
[Public JSON-RPC](PublicRpc.md) or the Rust SDK for self-service
principal creation.

---

## Fund from faucet

Use `px_requestFunds` on the configured hosted gateway as documented on
[Hosted faucet](HostedFaucet.md).

---

## Send

Human-plane quote then commit, as in
`services/platform/examples/samples/first-payment-csharp`:

```csharp
using System.Text;
using LayerX.Sdk;

static async Task<JsonValue> Pay(
    Uri apiUrl,
    string apiToken,
    string source,
    string destination,
    JsonValue money,
    string paymentKey)
{
    using var token = new AccessToken(Encoding.UTF8.GetBytes(apiToken));
    var layerx = new PlatformClient(new HumanHttpTransport(apiUrl, accessToken: token));
    var quote = await layerx.HumanMoveQuoteAsync(JsonValue.Object(new Dictionary<string, JsonValue>
    {
        ["source"] = JsonValue.String(source),
        ["destination"] = JsonValue.String(destination),
        ["money"] = money,
    }));
    if (quote is not JsonValue.ObjectValue record ||
        !record.Value.TryGetValue("quote_id", out var found) ||
        found is not JsonValue.StringValue text || string.IsNullOrEmpty(text.Value))
    {
        throw new FormatException("Invalid quote response");
    }
    string quoteId = text.Value;
    return await layerx.HumanMoveCommitAsync(
        JsonValue.Object(new Dictionary<string, JsonValue> { ["quote_id"] = JsonValue.String(quoteId) }),
        new IdempotencyKey(paymentKey));
}
```

`HumanMoveCommitAsync` returns a journey. Follow it with
`HumanJourneyGetAsync` using the returned `journey_id` until it reaches its
outcome, as the sample does. Agent `prepare` / `submit` require an agent-plane
`IPlatformTransport`, such as `AgentEnvelopeTransport`.

---

## Verify a receipt

```csharp
using LayerX.Sdk;

static SequencerReceiptVerification Verify(
    ReadOnlyMemory<byte> receiptJson,
    ReadOnlyMemory<byte> rawTransaction,
    ReceiptKeys keys) =>
    LocalVerifier.VerifySequencerReceipt(receiptJson, rawTransaction, keys);
```

`receiptJson` contains the JSON result object from `pax_sequencerReceipt`,
including its canonical signed `receipt` field. `ReceiptKeys` holds `Pinned`
keys and `PreviousSequenceHash`; each `ReceiptKey` has `KeyId`, `PublicKey`,
`FirstHeight` and `LastHeight`. Authenticate those pins and the predecessor
independently of the response, and retain the exact signed EVM transaction.

The verifier checks receipt, transaction, RPC fields, logs and sequence
bindings. Inspect `Status` and `Revert` before treating execution as successful.
Use the evidence and trust interfaces in
[`Committed.cs`](../../sdk/platform/dotnet/Committed.cs) to establish block
inclusion and BFT commitment separately.

[SDK quickstarts](SdkQuickstarts.md) · [Home](Home.md)
