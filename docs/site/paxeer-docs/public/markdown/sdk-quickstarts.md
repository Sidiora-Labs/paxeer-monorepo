<!-- Source: https://docs.paxeer.app/sdk-quickstarts/ -->

# SDK quickstarts

Register a principal, fund an account, submit a signed transaction, and verify execution evidence with the Paxeer X SDKs.

## Register, fund, send, verify

Choose a node transaction flow or a Human payment journey before selecting a client. `eth_sendRawTransaction` submits signed EVM transactions to the node; `px_register` creates a principal binding through the hosted gateway. Retrieve progress with `pax_sequencerReceipt` and trusted signing-key data with `px_getReceiptKeys`. Faucet funding requires an authorized identity session.

## Rust

Use `RpcClient` for node access and hosted registration. Sign the digest from `register::binding` before registering. Node connections use `RpcClient::connect_node`; `RpcClient::get_balance` reads an EVM balance, and `RpcClient::send_raw_transaction` submits signed transaction bytes.

### Registration binding

```
use layerx_sdk::register::{binding, TENANT};
use layerx_sdk::rpc::{RpcClient, RpcError};

fn registration_digest(public_key: &[u8; 32]) -> [u8; 32] {
    binding(TENANT, public_key)
}

fn register(rpc: &RpcClient, public_key: [u8; 32], signature: &[u8; 64])
    -> Result<layerx_sdk::register::Registration, RpcError>
{
    rpc.register(public_key, signature)
}
```

### Read an EVM balance

```
use layerx_sdk::rpc::{RpcClient, RpcError};

fn balance(rpc: &RpcClient, address: &str) -> Result<serde_json::Value, RpcError> {
    rpc.get_balance(address)
}
```

### Submit a signed EVM transaction

```
use layerx_sdk::rpc::{RpcClient, RpcError};

fn send(rpc: &RpcClient, raw: &[u8]) -> Result<[u8; 32], RpcError> {
    rpc.send_raw_transaction(raw)
}
```

### Connect to a node

```
use layerx_sdk::rpc::{RpcClient, RpcError};

fn connect(endpoint: &str) -> Result<RpcClient, RpcError> {
    RpcClient::connect_node(endpoint, None)
}
```

The Rust committed-wait example uses a caller-supplied receipt policy. Configure the client with a runtime clock and independently pinned node consensus authority before waiting for committed inclusion.

### Verify committed inclusion (Rust)

```
use std::time::Duration;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::{ReceiptPolicy, VerifiedRpcReceipt};

fn wait_for_commit(
    rpc: &RpcClient,
    transaction_hash: [u8; 32],
    policy: &ReceiptPolicy,
) -> Result<VerifiedRpcReceipt, RpcError> {
    rpc.wait_for(transaction_hash, Commitment::Committed, policy, Duration::from_secs(30))
}
```

## TypeScript

Use `JsonRpcClient` for node reads and signed EVM transaction submission. `ProductionClient` provides Human operations through a configured Human transport. Registration and faucet calls use the hosted gateway contract.

### Connect and submit a transaction

```
import { JsonRpcClient } from "@sidiora/layerx-sdk";

export function openNode(endpoint: string): JsonRpcClient {
  return new JsonRpcClient(endpoint);
}

export function send(rpc: JsonRpcClient, signedTransaction: Uint8Array): Promise<string> {
  return rpc.sendRawTransaction(signedTransaction);
}
```

### Quote and commit a Human payment

```
import { ProductionClient, idempotencyKey } from "@sidiora/layerx-sdk";

type MoveRequest = {
  source: string;
  destination: string;
  money: { amount: string; currency: string };
};

export async function pay(
  client: ProductionClient,
  request: MoveRequest,
  paymentKey: string,
): Promise<Record<string, unknown>> {
  const quote = await client.human<MoveRequest, { quote_id: string }>("move.quote", request);
  return client.human<{ quote_id: string }, Record<string, unknown>>(
    "move.commit",
    { quote_id: quote.quote_id },
    { idempotencyKey: idempotencyKey(paymentKey) },
  );
}
```

### Verify canonical receipt bytes

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

`PaymentRpc` sends node methods at the root path `/`. The hosted gateway client handles registration and faucet methods at its configured service endpoint. Registration takes lowercase hexadecimal keys and signatures; funding requires a Bearer identity session.

### Hosted registration

```
from layerx_sdk.x402_rpc import HostedGatewayRpc

def register(rpc: HostedGatewayRpc, signer_public_key: str, registration_signature: str) -> dict:
    result = rpc.call("px_register", [signer_public_key, registration_signature])
    if not isinstance(result, dict):
        raise ValueError("Invalid registration response")
    return result
```

### Authenticated funding

```
from layerx_sdk.x402_rpc import HostedGatewayRpc

def request_funds(gateway_endpoint: str, session_token: str, did: str, signer_public_key: str) -> dict:
    rpc = HostedGatewayRpc(gateway_endpoint, {"Authorization": f"Bearer {session_token}"})
    result = rpc.call("px_requestFunds", [did, signer_public_key])
    if not isinstance(result, dict):
        raise ValueError("Invalid funding response")
    return result
```

### Submit a signed EVM transaction

```
from layerx_sdk import PaymentRpc

def send(rpc: PaymentRpc, raw_transaction: str) -> str:
    return rpc.send(raw_transaction)
```

### Quote and commit a Human payment

```
from layerx_sdk import IdempotencyKey, ProductionClient

def pay(layerx: ProductionClient, source, destination, money, payment_key):
    quote = layerx.human("move.quote", {"source": source, "destination": destination, "money": money})
    return layerx.human("move.commit", {"quote_id": quote["quote_id"]}, idempotency_key=IdempotencyKey(payment_key))
```

### Verify canonical receipt bytes

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

Use `NewHumanHTTPTransport` and `Client.Human` for a quote followed by an idempotent commit. The Go package is in sdk/platform/go; its import path is declared by that directory's go.mod.

### Quote and commit a Human payment

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

### Verify the JSON receipt view

```
package main

import layerx "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"

func verify(receiptJSON, rawTransaction []byte, keys layerx.ReceiptKeys) (layerx.SequencerReceiptVerification, error) {
	return layerx.VerifySequencerReceipt(receiptJSON, rawTransaction, keys)
}
```

## JVM

Configure `HttpProductionTransport` with the Human and agent origins. Human payments use `ProductionClient.human` and typed options. The JVM SDK source is in sdk/platform/jvm.

### Quote and commit a Human payment

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
        String humanOrigin,
        String agentOrigin,
        String apiToken,
        String source,
        String destination,
        Map<String, String> money,
        String paymentKey
    ) {
        var credential = new HttpProductionTransport.BearerCredential(
            new SecretBytes(apiToken.getBytes(StandardCharsets.UTF_8)));
        var layerx = new ProductionClient(
            HttpProductionTransport.create(URI.create(humanOrigin), URI.create(agentOrigin), credential));
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

### Verify bytes and JSON execution evidence

```
import com.fasterxml.jackson.databind.JsonNode;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.LocalVerifier.ReceiptKeys;
import com.sidiora.layerx.sdk.verify.LocalVerifier.SequencerReceiptVerification;

public final class Verify {
    public static SequencerReceiptVerification verify(
        byte[] receiptBytes,
        byte[] rawTransaction,
        ReceiptKeys keys,
        JsonNode receiptJSON,
        byte[] previousSequenceHash
    ) {
        return LocalVerifier.verifySequencerReceipt(
            receiptBytes, rawTransaction, keys, receiptJSON, previousSequenceHash);
    }

    private Verify() {}
}
```

## Swift

Use `PlatformClient` with `HumanHTTPTransport` for a payment quote and commit. Access tokens and idempotency keys use dedicated types. The Swift package is in sdk/platform/swift.

### Quote and commit a Human payment

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

### Verify bytes and JSON execution evidence

```
import Foundation
import LayerXSDK

func verify(receiptBytes: Data, rawTransaction: Data, keys: [ReceiptKeys], receiptJSON: [String: JSONValue], prevSeqHash: Data) throws -> SequencerReceiptVerification {
    try LocalVerifier.verifySequencerReceipt(receiptBytes, rawTransaction: rawTransaction, keys: keys, receiptJSON: receiptJSON, prevSeqHash: prevSeqHash)
}
```

## .NET

Use `PlatformClient` with `HumanHttpTransport`, `AccessToken`, and `IdempotencyKey` for the Human payment flow. The .NET project is in sdk/platform/dotnet.

### Quote and commit a Human payment

```
using System;
using System.Collections.Generic;
using System.Text;
using System.Threading.Tasks;
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
        !record.Value.TryGetValue("quote_id", out var value) ||
        value is not JsonValue.StringValue quoteId)
        throw new InvalidOperationException("Invalid quote response");
    return await layerx.HumanMoveCommitAsync(
        JsonValue.Object(new Dictionary<string, JsonValue> { ["quote_id"] = JsonValue.String(quoteId.Value) }),
        new IdempotencyKey(paymentKey));
}
```

### Verify the JSON receipt view

```
using System;
using LayerX.Sdk;

static SequencerReceiptVerification Verify(
    ReadOnlyMemory<byte> receiptJSON,
    ReadOnlyMemory<byte> rawTransaction,
    ReceiptKeys keys) =>
    LocalVerifier.VerifySequencerReceipt(receiptJSON, rawTransaction, keys);
```

## Verify sequencer receipts against trusted keys

Receipt verifiers check the signed receipt, raw transaction hash, and independently trusted receipt-key identifiers, public keys, and height ranges. TypeScript and Python take canonical receipt bytes; Go and .NET take the JSON receipt view. The JVM and Swift examples take canonical bytes, the JSON view, and authenticated previous-sequence context. A receipt signature proves execution under that key; committed finality additionally requires authenticated block inclusion and consensus evidence.

The TypeScript `Commitment` type is `"submitted" | "receipted" | "committed"`. The node reports progress as `submitted`, `receipted`, or `committed`. Verify signatures at the `receipted` stage and consensus inclusion for `committed`. Rust exposes its committed wait through a separate typed policy API.

## Program execution evidence

A program call returns execution status, return data, and event evidence committed by its transaction receipt. Verify those receipt commitments and the expected program outcome before acting on a result.

Receipt verification uses language-specific APIs: TypeScript `verifySequencerReceipt`, Python `verify_sequencer_receipt`, Go `VerifySequencerReceipt`, JVM `LocalVerifier.verifySequencerReceipt`, .NET `LocalVerifier.VerifySequencerReceipt`, and Swift `LocalVerifier.verifySequencerReceipt`. The JVM Programs client additionally exposes `ProgramsClient.verifyTerminalV5` for terminal evidence. Supply the trust inputs required by the selected API.

Continue with [XDK API families](https://docs.paxeer.app/xdk), [Programs](https://docs.paxeer.app/programs), and [MCP tools](https://docs.paxeer.app/mcp).
