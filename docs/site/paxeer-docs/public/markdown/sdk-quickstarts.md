<!-- Source: https://docs.paxeer.app/sdk-quickstarts/ -->

# SDK quickstarts

Register a principal, fund an account, send a payment, and independently verify receipts with the Paxeer X SDKs.

## Register, fund, send, verify

Choose a public principal flow or a human payment journey before selecting a client. `agent.register` registers an agent with the daemon; `px_register` creates the public principal binding. The retired `lx_*` gateway names return `-32601`. They serve different workflows. Fund through an authenticated faucet claim and submit signed canonical activities with a scoped LayerX key.

## Rust

Use the typed `RpcClient` for registration, funding requests, and canonical activity submission. Sign the binding digest returned by `register::binding` before calling register. Funding requires the identity session authorization on the gateway request. Note: `RpcClient::request_funds` currently still calls the retired `lx_requestFunds` name, which the gateway refuses; call `px_requestFunds` directly until the SDK is updated.

### Registration binding

```
use layerx_sdk::register::{binding, TENANT};
use layerx_sdk::rpc::RpcClient;

fn register(rpc: &RpcClient, public_key: [u8; 32], signature: &[u8; 64])
    -> Result<layerx_sdk::register::Registration, layerx_sdk::rpc::RpcError>
{
    let _digest = binding(TENANT, &public_key);
    rpc.register(public_key, signature)
}
```

### Authenticated funding

```
use layerx_sdk::rpc::{FaucetGrant, RpcClient, RpcError};

fn request_funds(rpc: &RpcClient, did: &str, public_key: &[u8; 32])
    -> Result<FaucetGrant, RpcError>
{
    rpc.request_funds(did, public_key)
}
```

### Client and payment example

```
use layerx_sdk::rpc::{RpcClient, RpcError};

fn send(rpc: &RpcClient, raw: &[u8]) -> Result<[u8; 32], RpcError> {
    rpc.send_raw_transaction(raw)
}
```

### Client and payment example

```
use layerx_sdk::production::SecretBytes;
use layerx_sdk::programs::LayerXKeyCredential;
use layerx_sdk::rpc::RpcClient;

fn connect(endpoint: &str, key_id: &str, secret: &[u8]) -> Result<RpcClient, layerx_sdk::rpc::RpcError> {
    let secret = SecretBytes::new(secret).map_err(|_| layerx_sdk::rpc::RpcError::InvalidRequest)?;
    let credential = LayerXKeyCredential::new(key_id, secret).map_err(layerx_sdk::rpc::RpcError::Configuration)?;
    RpcClient::connect(endpoint, Some(credential))
}
```

The Rust crate still exposes the kernel-batch verifier below. The platform SDKs in the other languages have retired this model; use their sequencer-receipt verifiers instead.

### Local receipt verification (Rust kernel batch)

```
use layerx_proof::receipt::AuthorizedBatch;
use layerx_sdk::production::verify_receipt;

fn verify(
    receipt: &[u8],
    batch_id: [u8; 32],
    asset: [u8; 32],
    previous_state_root: [u8; 32],
    resulting_state_root: [u8; 32],
    sequencer_public_key: [u8; 32],
) -> Result<layerx_proof::receipt::VerifiedReceipt, layerx_sdk::production::ReceiptVerificationFailure> {
    let authorised = AuthorizedBatch::new(
        batch_id,
        asset,
        previous_state_root,
        resulting_state_root,
        sequencer_public_key,
    );
    verify_receipt(receipt, &authorised)
}
```

## TypeScript

Use `JsonRpcClient` for native submission and reads. Use public JSON-RPC for registration and faucet methods; `ProductionClient` provides human and agent operation interfaces with an appropriate transport.

### Client and payment example

```
import { JsonRpcClient, LayerXKeyCredential, SecretBytes } from "@sidiora/layerx-sdk";
import type { Commitment } from "@sidiora/layerx-sdk";

export function openRpc(endpoint: string, keyId: string, secret: Uint8Array): JsonRpcClient {
  return new JsonRpcClient(endpoint, new LayerXKeyCredential(keyId, new SecretBytes(secret)));
}

export async function send(
  rpc: JsonRpcClient,
  canonical: Uint8Array,
  commitment: Commitment = "submitted",
): Promise<Record<string, unknown>> {
  const hash = await rpc.sendRawTransaction(canonical);
  return { hash, commitment };
}
```

### Client and payment example

```
import { ProductionClient, SecretBytes, idempotencyKey } from "@sidiora/layerx-sdk";

export async function pay(
  layerx: ProductionClient,
  source: string,
  destination: string,
  money: { amount: string; currency: string },
  paymentKey: string,
) {
  const quote = await layerx.human("move.quote", { source, destination, money });
  return layerx.human("move.commit", { quote_id: quote.quote_id }, { idempotencyKey: idempotencyKey(paymentKey) });
}
```

### Local receipt verification

```
import { verifySequencerReceipt } from "@sidiora/layerx-sdk";
import type { SequencerReceiptKey, SequencerReceiptVerification } from "@sidiora/layerx-sdk";

export function verify(
  signedReceipt: Uint8Array,
  rawTransaction: Uint8Array,
  receiptKeys: readonly SequencerReceiptKey[],
): Promise<SequencerReceiptVerification> {
  return verifySequencerReceipt(signedReceipt, rawTransaction, receiptKeys);
}
```

## Python

`PaymentRpc.call` sends public method names directly. The endpoint must be HTTPS with path exactly `/rpc`. Registration uses lowercase hexadecimal key and signature values. Faucet calls use a Bearer identity session; signed activity calls use the LayerX-Key header.

### Public registration

```
from layerx_sdk import PaymentRpc

def register(rpc: PaymentRpc, signer_public_key: str, registration_signature: str) -> dict:
    return rpc.call("px_register", [signer_public_key, registration_signature])
```

### Authenticated funding

```
from layerx_sdk import PaymentRpc

def request_funds(endpoint: str, session_token: str, did: str, signer_public_key: str) -> dict:
    rpc = PaymentRpc(endpoint, {"Authorization": f"Bearer {session_token}"})
    return rpc.call("px_requestFunds", [did, signer_public_key])
```

### Client and payment example

```
from layerx_sdk import PaymentRpc

def send(rpc: PaymentRpc, raw_transaction: str) -> str:
    return rpc.send(raw_transaction)
```

### Client and payment example

```
from layerx_sdk import IdempotencyKey, ProductionClient

def pay(layerx: ProductionClient, source, destination, money, payment_key):
    quote = layerx.human("move.quote", {"source": source, "destination": destination, "money": money})
    return layerx.human("move.commit", {"quote_id": quote["quote_id"]}, idempotency_key=IdempotencyKey(payment_key))
```

### Local receipt verification

```
from layerx_sdk import ReceiptKey, SequencerReceiptVerification, verify_sequencer_receipt

def verify(
    signed_receipt: bytes,
    raw_transaction: bytes,
    receipt_keys: tuple[ReceiptKey, ...],
) -> SequencerReceiptVerification:
    return verify_sequencer_receipt(signed_receipt, raw_transaction, receipt_keys)
```

## Go

Use `NewHumanHTTPTransport` and `Client.Human` for a quote followed by an idempotent commit. Agent catalog operations require an agent-plane transport.

### Client and payment example

```
package main

import (
	"context"
	"net/http"

	layerx "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"
)

type Money struct {
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type MoveQuoteRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	Money       Money  `json:"money"`
}

type MoveQuote struct {
	QuoteID string `json:"quote_id"`
}

type MoveCommitRequest struct {
	QuoteID string `json:"quote_id"`
}

type Journey struct {
	JourneyID string `json:"journey_id"`
	State     string `json:"state"`
}

func pay(ctx context.Context, apiURL, apiToken, source, destination, paymentKey string, money Money) (Journey, error) {
	authorize := func(request *http.Request) error {
		request.Header.Set("Authorization", "Bearer "+apiToken)
		return nil
	}
	transport, err := layerx.NewHumanHTTPTransport(apiURL, nil, authorize)
	if err != nil {
		return Journey{}, err
	}
	client, err := layerx.NewClient(transport, nil)
	if err != nil {
		return Journey{}, err
	}
	key, err := layerx.NewIdempotencyKey(paymentKey)
	if err != nil {
		return Journey{}, err
	}
	var quote MoveQuote
	if err := client.Human(ctx, layerx.HumanOperationMoveQuote, MoveQuoteRequest{Source: source, Destination: destination, Money: money}, &quote, layerx.CallOptions{}); err != nil {
		return Journey{}, err
	}
	var journey Journey
	if err := client.Human(ctx, layerx.HumanOperationMoveCommit, MoveCommitRequest{QuoteID: quote.QuoteID}, &journey, layerx.CallOptions{IdempotencyKey: key}); err != nil {
		return Journey{}, err
	}
	return journey, nil
}
```

### Local receipt verification

```
package main

import layerx "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"

func verify(receiptBytes, rawTransaction []byte, keys layerx.ReceiptKeys) (layerx.SequencerReceiptVerification, error) {
	return layerx.VerifySequencerReceipt(receiptBytes, rawTransaction, keys)
}
```

## JVM

Configure `HttpProductionTransport` with the human origin and the agent origin. Human payments use `ProductionClient.human` and typed options.

### Client and payment example

```
import com.fasterxml.jackson.databind.JsonNode;
import com.sidiora.layerx.sdk.HttpProductionTransport;
import com.sidiora.layerx.sdk.IdempotencyKey;
import com.sidiora.layerx.sdk.ProductionClient;
import com.sidiora.layerx.sdk.SecretBytes;
import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.Map;

public final class Pay {
    public static JsonNode pay(
        String apiUrl,
        String apiToken,
        String source,
        String destination,
        Map<String, String> money,
        String paymentKey
    ) {
        var credential = new HttpProductionTransport.BearerCredential(
            new SecretBytes(apiToken.getBytes(StandardCharsets.UTF_8)));
        var layerx = new ProductionClient(
            HttpProductionTransport.create(URI.create(apiUrl), URI.create(apiUrl), credential));
        var quote = layerx.human(
            "move.quote",
            Map.of("source", source, "destination", destination, "money", money),
            JsonNode.class,
            ProductionClient.Options.none()).toCompletableFuture().join();
        return layerx.human(
            "move.commit",
            Map.of("quote_id", quote.path("quote_id").asText()),
            JsonNode.class,
            ProductionClient.Options.idempotent(new IdempotencyKey(paymentKey)))
            .toCompletableFuture().join();
    }

    private Pay() {}
}
```

### Local receipt verification

```
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.LocalVerifier.ReceiptKeys;
import com.sidiora.layerx.sdk.verify.LocalVerifier.SequencerReceiptVerification;

public final class Verify {
    public static SequencerReceiptVerification verify(byte[] receiptBytes, byte[] rawTransaction, ReceiptKeys keys) {
        return LocalVerifier.verifySequencerReceipt(receiptBytes, rawTransaction, keys);
    }

    private Verify() {}
}
```

## Swift

Use `PlatformClient` with `HumanHTTPTransport` for a payment quote and commit. Access tokens and idempotency keys use dedicated types.

### Client and payment example

```
import Foundation
import LayerXSDK

func pay(
    serviceURL: URL,
    apiToken: String,
    source: String,
    destination: String,
    money: JSONValue,
    paymentKey: IdempotencyKey
) async throws -> JSONValue {
    let token = try AccessToken(Data(apiToken.utf8))
    let layerx = PlatformClient(transport: try HumanHTTPTransport(baseURL: serviceURL, accessToken: token))
    let quote = try await layerx.humanMoveQuote(.object([
        "source": .string(source),
        "destination": .string(destination),
        "money": money,
    ]))
    guard let quoteID = quote.objectValue?["quote_id"]?.stringValue else {
        throw PlatformSDKError(code: .invalidArgument, retry: .never)
    }
    return try await layerx.humanMoveCommit(.object(["quote_id": .string(quoteID)]), idempotencyKey: paymentKey)
}
```

### Local receipt verification

```
import Foundation
import LayerXSDK

func verify(receiptBytes: Data, rawTransaction: Data, keys: [ReceiptKeys], receiptJSON: [String: JSONValue], prevSeqHash: Data) throws -> SequencerReceiptVerification {
    try LocalVerifier.verifySequencerReceipt(receiptBytes, rawTransaction: rawTransaction, keys: keys, receiptJSON: receiptJSON, prevSeqHash: prevSeqHash)
}
```

## .NET

Use `PlatformClient` with `HumanHttpTransport`, `AccessToken`, and `IdempotencyKey` for the human payment flow.

### Client and payment example

```
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
    IReadOnlyDictionary<string, JsonValue> fields = quote is JsonValue.ObjectValue record
        ? record.Value
        : new Dictionary<string, JsonValue>();
    string quoteId = fields.TryGetValue("quote_id", out var found) && found is JsonValue.StringValue text
        ? text.Value
        : string.Empty;
    return await layerx.HumanMoveCommitAsync(
        JsonValue.Object(new Dictionary<string, JsonValue> { ["quote_id"] = JsonValue.String(quoteId) }),
        new IdempotencyKey(paymentKey));
}
```

### Local receipt verification

```
using LayerX.Sdk;

static SequencerReceiptVerification Verify(
    ReadOnlyMemory<byte> receiptBytes,
    ReadOnlyMemory<byte> rawTransaction,
    ReceiptKeys keys) =>
    LocalVerifier.VerifySequencerReceipt(receiptBytes, rawTransaction, keys);
```

## Verify sequencer receipts against trusted keys

The platform verifiers take the signed receipt, the raw transaction, and independently trusted receipt keys (key ID, public key, and valid height range). They check the canonical encoding, the sequencer signature, and that the Keccak-256 hash of the raw transaction matches the receipt. Supplying keys from the same untrusted response as the receipt does not establish independent trust.

The TypeScript `Commitment` type is `"submitted" | "receipted" | "committed"`. The values `executed`, `batched`, and `finalised` are retired and are rejected as invalid; the only verification rungs are `receipted` and `committed`. The Rust kernel-batch verifier above is the exception and follows its own `AuthorizedBatch` contract.

## Programs terminal evidence

A Programs CALL receipt commits to one runtime terminal. Verify the receipt first, then check the terminal payload against its receipt commitments before acting on a program outcome.

There is no public cross-SDK terminal-verifier function with a single shared name. The previously documented entry points (`decode_and_verify_program_terminal`, `decodeAndVerifyProgramTerminal`, `verifyProgramTerminal`, `ProgramsClient.verifyTerminal`, `ProgramsClient.VerifyTerminal`, and Swift `verifyTerminal`) are retired, unexported, package-private, private, or internal. On the JVM the public entry point is `ProgramsClient.verifyTerminalV5`. In other languages, verify the sequencer receipt and the commitments it carries, and confirm any terminal-decoding helper in your SDK source before depending on it.

Continue with [XDK API families](https://docs.paxeer.app/xdk), [Programs](https://docs.paxeer.app/programs), and [MCP tools](https://docs.paxeer.app/mcp).
