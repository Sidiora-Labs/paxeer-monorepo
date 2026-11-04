package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import com.sidiora.layerx.sdk.verify.ConsensusTrust;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.verify.PortableReceipt;
import com.sun.net.httpserver.HttpExchange;
import com.sun.net.httpserver.HttpHandler;
import com.sun.net.httpserver.HttpServer;
import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.math.BigInteger;
import java.net.InetSocketAddress;
import java.net.URI;
import java.net.http.HttpClient;
import java.nio.file.Files;
import java.nio.file.Path;
import java.time.Duration;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Base64;
import java.util.HexFormat;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Flow;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicReference;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.Timeout;
import org.junit.jupiter.api.io.TempDir;
import static org.junit.jupiter.api.Assertions.*;

@Timeout(60)
public final class Repair22331390Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final IdempotencyKey KEY = new IdempotencyKey("44".repeat(32));
    @TempDir Path temporary;

    @Test
    void programRpcRefusalsPreserveProducerCodesAndMethodAwareRetry() throws Exception {
        // Wire refusals from rpc/rpcstack_test.go and geth/rpc/testdata/reqresp-nomethod.js.
        List<JsonNode> recorded = List.of(
            JSON.readTree("{\"code\":-32002,\"message\":\"request timed out\"}"),
            JSON.readTree("{\"code\":-32601,\"message\":\"the method invalid_method does not exist/is not available\"}"),
            JSON.readTree("{\"code\":-32603,\"message\":\"method handler crashed\"}"),
            JSON.readTree("{\"code\":-32005,\"message\":\"rate limit exceeded\"}"),
            JSON.readTree("{\"code\":-32000,\"message\":\"already known\"}"));
        var current = new AtomicReference<>(recorded.getFirst());
        try (var server = new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody());
            ObjectNode reply = envelope(request); reply.set("error", current.get());
            send(exchange, 200, reply);
        }); var rpc = new ProgramsClient.Programs(server.uri())) {
            var params = JSON.createArrayNode();
            for (int index = 0; index < recorded.size(); index++) {
                current.set(recorded.get(index));
                var read = refused(rpc.request("eth_call", params));
                assertEquals(List.of(PlatformSdkException.Code.TRANSPORT_FAILURE,
                    PlatformSdkException.Code.UNAVAILABLE_CAPABILITY, PlatformSdkException.Code.TRANSPORT_FAILURE,
                    PlatformSdkException.Code.RATE_LIMIT, PlatformSdkException.Code.CORE_REJECTION).get(index), read.code());
                assertEquals(index == 1 || index == 4 ? PlatformSdkException.Retry.NEVER : PlatformSdkException.Retry.SAFE, read.retry());
                assertEquals(recorded.get(index).required("code").intValue(), read.protocolResultCode());
                assertFalse(JSON.writeValueAsString(read.safeDetails()).contains("message"));
                var write = refused(rpc.request("eth_sendRawTransaction", params));
                assertEquals(index == 1 ? PlatformSdkException.Code.UNAVAILABLE_CAPABILITY : PlatformSdkException.Code.UNKNOWN_OUTCOME, write.code());
                assertEquals(index == 1 ? PlatformSdkException.Retry.NEVER : PlatformSdkException.Retry.UNKNOWN_OUTCOME, write.retry());
            }
            JsonNode revert = vector("sequencer-receipt-v1.json", "V2-layerxledger-revert");
            // geth/internal/ethapi/errors.go emits code 3 and the EVM return bytes as data.
            current.set(JSON.createObjectNode().put("code", 3).put("message", "execution reverted")
                .set("data", revert.required("receiptJson").required("returnData")));
            var refusal = refused(rpc.request("eth_call", params));
            assertEquals(PlatformSdkException.Code.REVERT, refusal.code());
            assertEquals(PlatformSdkException.Retry.NEVER, refusal.retry());
            assertEquals(revert.required("receiptJson").required("returnData").textValue(), refusal.safeDetails().get("rpcData"));
            current.set(JSON.createObjectNode().put("code", 3).put("message", "secret bearer value")
                .put("data", "0x" + "ab".repeat(4096)));
            var bounded = refused(rpc.request("eth_call", params));
            assertEquals(514, ((String) bounded.safeDetails().get("rpcData")).length());
            assertEquals(4096, bounded.safeDetails().get("rpcDataBytes"));
            assertFalse(JSON.writeValueAsString(bounded.safeDetails()).contains("secret"));
            ObjectNode objectData = JSON.createObjectNode().put("code", 3).put("message", "secret");
            objectData.putObject("data").put("token", "private"); current.set(objectData);
            assertFalse(refused(rpc.request("eth_call", params)).safeDetails().containsKey("rpcData"));
        }
    }

    @Test
    void programRpcHttpFailuresAndLostBroadcastRemainUnknown() throws Exception {
        for (int status : new int[] {503, 429, 200}) {
            try (var server = new Loopback(exchange -> {
                exchange.getRequestBody().readAllBytes();
                if (status == 200) { exchange.sendResponseHeaders(200, 200); exchange.getResponseBody().write('{'); }
                else send(exchange, status, JSON.createObjectNode().put("message", "node unavailable"));
            }); var rpc = new ProgramsClient.Programs(server.uri(), HttpClient.newHttpClient(), Duration.ofMillis(500))) {
                assertEquals(PlatformSdkException.Code.UNKNOWN_OUTCOME,
                    refused(rpc.request("eth_sendRawTransaction", JSON.createArrayNode())).code());
                var read = refused(rpc.request("eth_getTransactionCount", JSON.createArrayNode()));
                assertEquals(PlatformSdkException.Code.TRANSPORT_FAILURE, read.code());
                assertEquals(PlatformSdkException.Retry.SAFE, read.retry());
            }
        }
    }

    @Test
    void programRestartLookupReopensUnknownBytesAndReconcilesWithoutResubmission() throws Exception {
        JsonNode vector = program();
        byte[] raw = bytes(vector, "rawTransaction"), hash = ProgramsClient.Programs.transactionHash(raw);
        var methods = new CopyOnWriteArrayList<String>();
        var receipt = new AtomicReference<JsonNode>(JSON.createObjectNode().put("rung", "submitted").put("txHash", hex(hash)));
        Path directory = temporary.resolve("unknown");
        try (var server = new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody());
            String method = request.required("method").textValue(); methods.add(method);
            if (method.equals("eth_sendRawTransaction")) {
                assertEquals(hex(raw), request.required("params").required(0).textValue());
                // The producer accepted the bytes but its response was lost after real HTTP headers.
                exchange.sendResponseHeaders(200, 500); exchange.getResponseBody().write('{');
            } else {
                assertEquals("pax_sequencerReceipt", method);
                assertEquals(hex(hash), request.required("params").required(0).textValue());
                rpcReply(exchange, request, receipt.get());
            }
        })) {
            try (var journal = SubmissionJournal.open(directory, 2, 1024 * 1024);
                    var rpc = new ProgramsClient.Programs(server.uri(), HttpClient.newHttpClient(), Duration.ofMillis(500))) {
                ProgramsClient client = client(rpc, vector, journal);
                var nativeCall = NativeProgramCall.decodeSignedActivity(raw);
                BigInteger fee = ProgramsClient.Programs.number(ProgramsClient.Programs.transactionData(raw), 452, 128);
                var failure = refused(client.submit(new ProgramsClient.Call(nativeCall, fee, raw), KEY));
                assertEquals(PlatformSdkException.Code.UNKNOWN_OUTCOME, failure.code());
                assertEquals(PlatformSdkException.Retry.UNKNOWN_OUTCOME, failure.retry());
                assertArrayEquals(raw, journal.signedTransaction(hash));
                assertEquals(1, journal.size());
            }
            try (var journal = SubmissionJournal.open(directory, 2, 1024 * 1024);
                    var rpc = new ProgramsClient.Programs(server.uri())) {
                ProgramsClient fresh = client(rpc, vector, journal);
                assertEquals(PlatformSdkException.Code.UNKNOWN_OUTCOME, refused(fresh.receipt(KEY, hash)).code());
                assertArrayEquals(raw, journal.signedTransaction(hash));
                receipt.set(vector.required("receiptJson"));
                var result = await(fresh.receipt(KEY, hash));
                byte[] exported = fresh.retainedSignedTransaction(hash);
                assertArrayEquals(raw, exported);
                exported[0] ^= 1;
                assertArrayEquals(raw, fresh.retainedSignedTransaction(hash), "exports cannot mutate durable retention");
                try (var importedJournal = SubmissionJournal.memory(2, 1024 * 1024)) {
                    ProgramsClient imported = client(rpc, vector, importedJournal);
                    assertArrayEquals(hash, imported.retainSignedTransaction(fresh.retainedSignedTransaction(hash)));
                    assertArrayEquals(result.execution().receiptDigest(), await(imported.receipt(KEY, hash)).execution().receiptDigest());
                }
                assertEquals(ProgramsClient.SubmissionState.EXECUTED, result.state());
                assertArrayEquals(bytes(vector, "receiptDigest"), result.execution().receiptDigest());
                assertArrayEquals(raw, result.retainedSignedTransaction());
                byte[] portable = PortableReceipt.export(raw, JSON.writeValueAsBytes(receipt.get()), key(vector));
                var verified = PortableReceipt.verify(portable, keys(vector));
                assertArrayEquals(bytes(vector, "signedReceipt"), verified.receipted().canonicalBytes());
                assertThrows(PlatformSdkException.class, () -> journal.releaseCommitted(verified));
                assertEquals(1, journal.size(), "receipted bytes are not finality eligible");
                byte[] wrong = bytes(vector, "signedReceipt"); wrong[wrong.length - 1] ^= 1;
                assertThrows(PlatformSdkException.class, () -> ProgramsClient.verifyReceipt(wrong, raw, keys(vector), receipt.get()));
            }
            assertEquals(List.of("eth_sendRawTransaction", "pax_sequencerReceipt", "pax_sequencerReceipt", "pax_sequencerReceipt"), methods);
        }
    }

    @Test
    void programRestartLookupFindsRecordedCommittedReceiptAndFinalityReleasesDurably() throws Exception {
        JsonNode vector = fixture("sdk/platform/conformance/fixtures/vector-v4-committed.json").required("positive");
        JsonNode bundle = fixture("sdk/platform/conformance/fixtures/vector-v5-portable.json").required("bundles").required(1);
        JsonNode committed = fixture("daemon/committed-fixture/vectors/vector-v4-committed.json").required("settlement").required("receipt");
        var proof = PortableReceipt.verify(JSON.writeValueAsBytes(bundle), keys(vector), trust());
        assertEquals("committed", proof.verificationLevel());
        assertArrayEquals(bytes(vector, "rawTransaction"), bytes(bundle, "rawTransaction"));
        byte[] raw = bytes(vector, "rawTransaction"), hash = ProgramsClient.Programs.transactionHash(raw);
        Path directory = temporary.resolve("committed");
        try (var journal = SubmissionJournal.open(directory, 2, 1024 * 1024)) {
            journal.retain(committed.required("from").textValue(), committed.required("nonce").textValue(), raw);
        }
        var requests = new AtomicInteger();
        try (var server = new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody()); requests.incrementAndGet();
            assertEquals("pax_sequencerReceipt", request.required("method").textValue());
            assertEquals(hex(hash), request.required("params").required(0).textValue());
            rpcReply(exchange, request, committed);
        }); var rpc = new ProgramsClient.Programs(server.uri());
                var journal = SubmissionJournal.open(directory, 2, 1024 * 1024)) {
            ProgramsClient fresh = client(rpc, vector, journal);
            for (int cycle = 0; cycle < 64; cycle++) {
                var result = await(fresh.receipt(KEY, hash));
                assertArrayEquals(bytes(vector, "receiptDigest"), result.execution().receiptDigest());
                assertEquals(ProgramsClient.SubmissionState.EXECUTED, result.state());
                assertEquals(1, journal.size()); assertEquals(raw.length, journal.retainedBytes());
                assertTrue(fresh.releaseCommitted(proof));
                assertEquals(0, journal.size()); assertEquals(0, journal.retainedBytes());
                assertFalse(journal.releaseCommitted(proof));
                if (cycle != 63) journal.retain(committed.required("from").textValue(), committed.required("nonce").textValue(), raw);
            }
            assertEquals(64, requests.get(), "reconciliation never rebroadcasts");
        }
        try (var reopened = SubmissionJournal.open(directory, 2, 1024 * 1024)) {
            assertEquals(0, reopened.size()); assertNull(reopened.signedTransaction(hash));
        }
    }

    @Test
    void programMemoryGrowthIsBoundedBeforeBroadcastAndUnresolvedNoncesSurviveRestart() throws Exception {
        JsonNode vector = program();
        NativeProgramCall nativeCall = NativeProgramCall.decode(bytes(fixture("sdk/platform/conformance/fixtures/native-program-call-v4.json"), "payload_hex"));
        var broadcasts = new AtomicInteger();
        var nonces = new AtomicInteger();
        Path directory = temporary.resolve("bounded");
        long budget = 1024 * 1024;
        List<byte[]> retained = new ArrayList<>();
        try (var server = new Loopback(exchange -> {
            JsonNode request = JSON.readTree(exchange.getRequestBody());
            switch (request.required("method").textValue()) {
                case "eth_getTransactionCount" -> rpcReply(exchange, request, JSON.getNodeFactory().textNode("0x" + Integer.toHexString(nonces.getAndIncrement())));
                case "eth_estimateGas" -> rpcReply(exchange, request, JSON.getNodeFactory().textNode("0x5208"));
                case "eth_sendRawTransaction" -> {
                    byte[] raw = bytes(request.required("params").required(0)); broadcasts.incrementAndGet();
                    rpcReply(exchange, request, JSON.getNodeFactory().textNode(hex(ProgramsClient.Programs.transactionHash(raw))));
                }
                default -> throw new AssertionError("unexpected method");
            }
        }); var rpc = new ProgramsClient.Programs(server.uri())) {
            try (var journal = SubmissionJournal.open(directory, 32, budget)) {
                ProgramsClient client = new ProgramsClient(rpc, ReceiptFixtureTest.SENDER, BigInteger.ONE, BigInteger.TWO,
                    keys(vector), key(vector).keyId(), journal);
                long total = 0;
                for (int index = 0; index < 32; index++) {
                    var prepared = await(client.call(nativeCall, BigInteger.valueOf(1000)));
                    byte[] raw = sign(prepared); retained.add(raw);
                    var result = await(client.submit(prepared, raw));
                    assertTrue(result.pending()); total += raw.length;
                    assertEquals(index + 1, journal.size()); assertEquals(total, journal.retainedBytes());
                    assertTrue(total < budget);
                }
                var prepared = await(client.call(nativeCall, BigInteger.valueOf(1000)));
                byte[] refused = sign(prepared);
                assertEquals(PlatformSdkException.Code.BUDGET_REFUSAL,
                    assertThrows(PlatformSdkException.class, () -> client.submit(prepared, refused)).code());
                assertEquals(32, broadcasts.get());
                ObjectNode conflicting = prepared.transaction(); conflicting.put("nonce", "0x0").put("maxFeePerGas", "0x3");
                var replacement = new ProgramsClient.ProgramCall(conflicting);
                assertEquals(PlatformSdkException.Code.IDEMPOTENCY_CONFLICT,
                    assertThrows(PlatformSdkException.class, () -> client.submit(replacement, sign(replacement))).code());
                assertEquals(32, broadcasts.get());
            }
            try (var journal = SubmissionJournal.open(directory, 32, budget)) {
                assertEquals(32, journal.size());
                for (byte[] raw : retained) assertArrayEquals(raw, journal.signedTransaction(ProgramsClient.Programs.transactionHash(raw)));
                assertThrows(Exception.class, () -> SubmissionJournal.open(directory, 32, budget));
                ProgramsClient restarted = new ProgramsClient(rpc, ReceiptFixtureTest.SENDER, BigInteger.ONE, BigInteger.TWO,
                    keys(vector), key(vector).keyId(), journal);
                ObjectNode conflicting = ReceiptFixtureTest.transaction(nativeCall.calldata(BigInteger.valueOf(1000))).transaction();
                conflicting.put("nonce", "0x0").put("maxFeePerGas", "0x3");
                var replacement = new ProgramsClient.ProgramCall(conflicting);
                assertEquals(PlatformSdkException.Code.IDEMPOTENCY_CONFLICT,
                    assertThrows(PlatformSdkException.class, () -> restarted.submit(replacement, sign(replacement))).code());
                assertEquals(32, broadcasts.get(), "restart preserves unresolved nonce reservations");
            }
            try (var bytesBound = SubmissionJournal.memory(100, retained.getFirst().length)) {
                bytesBound.retain(ReceiptFixtureTest.SENDER, "0x0", retained.getFirst());
                assertEquals(PlatformSdkException.Code.BUDGET_REFUSAL,
                    assertThrows(PlatformSdkException.class, () -> bytesBound.retain(ReceiptFixtureTest.SENDER, "0x1", retained.get(1))).code());
                assertEquals(1, bytesBound.size());
            }
        }
    }

    @Test
    void streamLifetimeWindowAcceptsMoreThan65536EventsAndRestartsAtPersistedBoundary() throws Exception {
        int total = 65_538;
        JsonNode recorded = fixture("services/human/schema/human-api/golden/stream.next.response.json");
        JsonNode notification = recorded.required("body").required("result").required("events").required(1);
        var reads = new AtomicInteger();
        try (var server = new Loopback(exchange -> {
            assertEquals("GET", exchange.getRequestMethod());
            int previous = Integer.parseInt(exchange.getRequestURI().getPath().substring("/v1/stream/c".length()));
            reads.incrementAndGet();
            ObjectNode response = recorded.required("body").deepCopy();
            ObjectNode result = (ObjectNode) response.required("result");
            ArrayNode events = result.putArray("events");
            int end = Math.min(previous + 4096, total + 1);
            for (int next = previous + 1; next <= end; next++) {
                ObjectNode event = notification.deepCopy(); event.put("cursor", "c" + next); events.add(event);
            }
            result.put("next_cursor", "c" + end);
            send(exchange, 200, response);
        }); var http = HttpClient.newHttpClient()) {
            ProductionClient client = humanClient(server.uri(), http);
            var stream = new ResumableStream<GeneratedSchema.HumanModels.StreamEvent>(cursor(0));
            var count = new AtomicInteger();
            var completed = new CompletableFuture<Void>();
            client.humanStreamPublisher(stream).subscribe(new Flow.Subscriber<>() {
                private Flow.Subscription subscription;
                @Override public void onSubscribe(Flow.Subscription value) { subscription = value; value.request(total); }
                @Override public void onNext(ResumableStream.Event<GeneratedSchema.HumanModels.StreamEvent> event) {
                    try {
                        int index = count.incrementAndGet();
                        assertEquals(cursor(index), event.cursor());
                        assertEquals(GeneratedSchema.HumanModels.StreamEventKind.NOTIFICATION, event.value().kind());
                        assertEquals(notification.required("observed_at").textValue(), event.value().observed_at());
                        if (index == total) { subscription.cancel(); completed.complete(null); }
                    } catch (Throwable error) { subscription.cancel(); completed.completeExceptionally(error); }
                }
                @Override public void onError(Throwable error) { completed.completeExceptionally(error); }
                @Override public void onComplete() { completed.completeExceptionally(new AssertionError("live stream completed")); }
            });
            await(completed);
            assertEquals(total, count.get()); assertEquals(cursor(total), stream.cursor());
            assertEquals(17, reads.get());
            var checkpoint = stream.checkpoint();
            assertEquals(ResumableStream.REPLAY_WINDOW_SIZE, checkpoint.replay().size());
            assertEquals("c3", checkpoint.replay().getFirst().eventId());
            assertEquals("c65538", checkpoint.replay().getLast().eventId());
            var boundaryReplay = new ResumableStream.Event<>("c3", stream.cursor(), cursor(total + 1),
                JSON.treeToValue(notification, GeneratedSchema.HumanModels.StreamEvent.class));
            assertEquals(PlatformSdkException.Code.DECODE_FAILURE,
                assertThrows(PlatformSdkException.class, () -> stream.accept(
                    new ResumableStream.Page<>(stream.cursor(), List.of(boundaryReplay), boundaryReplay.cursor()))).code());
            assertEquals(checkpoint, stream.checkpoint(), "a refused page is atomic");
            Path path = temporary.resolve("stream-checkpoint.json");
            JSON.writeValue(path.toFile(), checkpoint);
            var reopened = new ResumableStream<GeneratedSchema.HumanModels.StreamEvent>(
                JSON.readValue(path.toFile(), ResumableStream.Checkpoint.class));
            assertEquals(checkpoint, reopened.checkpoint());
            assertThrows(PlatformSdkException.class, () -> reopened.accept(
                new ResumableStream.Page<>(reopened.cursor(), List.of(boundaryReplay), boundaryReplay.cursor())));
            // An old requested cursor remains invalid even after that event has left the window.
            assertThrows(PlatformSdkException.class, () -> reopened.accept(new ResumableStream.Page<>(cursor(0), List.of(), cursor(0))));
            var recovered = new CompletableFuture<ResumableStream.Event<GeneratedSchema.HumanModels.StreamEvent>>();
            client.humanStreamPublisher(reopened).subscribe(oneEvent(recovered));
            assertEquals(cursor(total + 1), await(recovered).cursor());
            assertEquals(cursor(total + 1), reopened.cursor());
            assertEquals(18, reads.get());
            assertEquals(ResumableStream.REPLAY_WINDOW_SIZE, reopened.checkpoint().replay().size());
        }
    }

    @Test
    void streamEmptyPollBacksOffRetainsDemandAndDeliversLaterRealPage() throws Exception {
        JsonNode recorded = fixture("services/human/schema/human-api/golden/stream.next.response.json");
        String first = "cur-000000000000000042";
        var times = new CopyOnWriteArrayList<Long>();
        var paths = new CopyOnWriteArrayList<String>();
        try (var server = new Loopback(exchange -> {
            times.add(System.nanoTime()); paths.add(exchange.getRequestURI().getPath());
            ObjectNode body = recorded.required("body").deepCopy();
            // Replay the retained notification event; the other golden event references retired deposit journeys.
            ObjectNode page = (ObjectNode) body.required("result");
            page.putArray("events").add(recorded.required("body").required("result").required("events").required(1));
            if (times.size() <= 3) {
                ObjectNode result = (ObjectNode) body.required("result");
                result.putArray("events"); result.put("next_cursor", first);
            }
            send(exchange, 200, body);
        }); var http = HttpClient.newHttpClient()) {
            var stream = new ResumableStream<GeneratedSchema.HumanModels.StreamEvent>(new ResumableStream.Cursor(first));
            var received = new CompletableFuture<ResumableStream.Event<GeneratedSchema.HumanModels.StreamEvent>>();
            humanClient(server.uri(), http).humanStreamPublisher(stream,
                new ResumableStream.PollPolicy(Duration.ofMillis(100), Duration.ofMillis(200))).subscribe(oneEvent(received));
            var event = await(received);
            assertEquals(recorded.required("body").required("result").required("events").required(1).required("cursor").textValue(), event.cursor().value());
            assertEquals(GeneratedSchema.HumanModels.StreamEventKind.NOTIFICATION, event.value().kind());
            assertEquals(4, times.size());
            assertEquals(List.of("/v1/stream/" + first, "/v1/stream/" + first, "/v1/stream/" + first, "/v1/stream/" + first), paths);
            assertTrue(times.get(1) - times.get(0) >= TimeUnit.MILLISECONDS.toNanos(95));
            assertTrue(times.get(2) - times.get(1) >= TimeUnit.MILLISECONDS.toNanos(195));
            assertTrue(times.get(3) - times.get(2) >= TimeUnit.MILLISECONDS.toNanos(195));
            assertEquals(event.cursor(), stream.cursor(), "only the demanded event advances the checkpoint");
        }
    }

    @Test
    void streamEmptyPollCancellationInterruptsIdleWaitWithoutFurtherFetchesOrSignals() throws Exception {
        JsonNode recorded = fixture("services/human/schema/human-api/golden/stream.next.response.json");
        var responseSent = new CountDownLatch(1);
        var polls = new AtomicInteger();
        var signals = new AtomicInteger();
        var subscription = new AtomicReference<Flow.Subscription>();
        try (var server = new Loopback(exchange -> {
            polls.incrementAndGet();
            ObjectNode body = recorded.required("body").deepCopy();
            ObjectNode result = (ObjectNode) body.required("result"); result.putArray("events"); result.put("next_cursor", "c0");
            send(exchange, 200, body); responseSent.countDown();
        }); var http = HttpClient.newHttpClient()) {
            var stream = new ResumableStream<GeneratedSchema.HumanModels.StreamEvent>(cursor(0));
            humanClient(server.uri(), http).humanStreamPublisher(stream,
                new ResumableStream.PollPolicy(Duration.ofMillis(1500), Duration.ofMillis(1500))).subscribe(new Flow.Subscriber<>() {
                    @Override public void onSubscribe(Flow.Subscription value) { subscription.set(value); value.request(1); }
                    @Override public void onNext(ResumableStream.Event<GeneratedSchema.HumanModels.StreamEvent> value) { signals.incrementAndGet(); }
                    @Override public void onError(Throwable error) { signals.incrementAndGet(); }
                    @Override public void onComplete() { signals.incrementAndGet(); }
                });
            assertTrue(responseSent.await(5, TimeUnit.SECONDS));
            Thread.sleep(200);
            assertEquals(1, polls.get());
            long start = System.nanoTime(); subscription.get().cancel();
            assertTrue(System.nanoTime() - start < TimeUnit.MILLISECONDS.toNanos(200));
            subscription.get().request(5);
            Thread.sleep(1700);
            assertEquals(1, polls.get()); assertEquals(0, signals.get()); assertEquals(cursor(0), stream.cursor());
        }
    }

    private static Flow.Subscriber<ResumableStream.Event<GeneratedSchema.HumanModels.StreamEvent>> oneEvent(
            CompletableFuture<ResumableStream.Event<GeneratedSchema.HumanModels.StreamEvent>> received) {
        return new Flow.Subscriber<>() {
            private Flow.Subscription subscription;
            @Override public void onSubscribe(Flow.Subscription value) { subscription = value; value.request(1); }
            @Override public void onNext(ResumableStream.Event<GeneratedSchema.HumanModels.StreamEvent> event) {
                subscription.cancel(); received.complete(event);
            }
            @Override public void onError(Throwable error) { received.completeExceptionally(error); }
            @Override public void onComplete() { received.completeExceptionally(new AssertionError("live stream completed")); }
        };
    }
    private static ResumableStream.Cursor cursor(int index) { return new ResumableStream.Cursor("c" + index); }
    private static ProductionClient humanClient(URI uri, HttpClient http) {
        return new ProductionClient(new HttpProductionTransport(http, JSON, uri, uri, Duration.ofSeconds(10), null));
    }
    private static ProgramsClient client(ProgramsClient.Programs rpc, JsonNode vector, SubmissionJournal journal) {
        return new ProgramsClient(rpc, vector.required("receiptJson").required("from").textValue(),
            BigInteger.ONE, BigInteger.TWO, keys(vector), key(vector).keyId(), journal);
    }
    private static LocalVerifier.ReceiptKey key(JsonNode vector) {
        JsonNode pin = vector.required("pinnedKey");
        return new LocalVerifier.ReceiptKey(bytes(pin, "keyId"), bytes(pin, "publicKey"),
            pin.required("firstHeight").bigIntegerValue(), pin.required("lastHeight").bigIntegerValue());
    }
    private static LocalVerifier.ReceiptKeys keys(JsonNode vector) { return new LocalVerifier.ReceiptKeys(List.of(key(vector))); }
    private static ConsensusTrust trust() {
        return new ConsensusTrust("committed-fixture", 10,
            Base64.getDecoder().decode("Zc6vQFr0ZD2oXZPWyqjKikf4CkF01FQdfT9YQBmBrKI="));
    }
    private static JsonNode program() throws IOException { return vector("sequencer-receipt-v1.json", "V1-layerxprograms-call"); }
    private static JsonNode vector(String name, String id) throws IOException {
        for (JsonNode vector : fixture("sdk/platform/conformance/fixtures/" + name).required("cases"))
            if (id.equals(vector.required("id").textValue())) return vector;
        throw new AssertionError("missing producer vector " + id);
    }
    private static JsonNode fixture(String relative) throws IOException {
        Path root = Path.of("").toAbsolutePath();
        while (root != null) {
            Path path = root.resolve(relative);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readAllBytes(path));
            root = root.getParent();
        }
        throw new IOException("missing committed fixture " + relative);
    }
    private static byte[] bytes(JsonNode node, String field) { return bytes(node.required(field)); }
    private static byte[] bytes(JsonNode node) {
        String text = node.textValue(); return HexFormat.of().parseHex(text.startsWith("0x") ? text.substring(2) : text);
    }
    private static String hex(byte[] data) { return "0x" + HexFormat.of().formatHex(data); }
    private static <T> T await(CompletionStage<T> pending) throws Exception { return pending.toCompletableFuture().get(30, TimeUnit.SECONDS); }
    private static PlatformSdkException refused(CompletionStage<?> pending) {
        var failure = assertThrows(ExecutionException.class, () -> pending.toCompletableFuture().get(10, TimeUnit.SECONDS));
        Throwable cause = failure.getCause();
        while (cause instanceof java.util.concurrent.CompletionException && cause.getCause() != null) cause = cause.getCause();
        return assertInstanceOf(PlatformSdkException.class, cause);
    }
    private static ObjectNode envelope(JsonNode request) {
        ObjectNode result = JSON.createObjectNode().put("jsonrpc", "2.0"); result.set("id", request.required("id")); return result;
    }
    private static void rpcReply(HttpExchange exchange, JsonNode request, JsonNode result) throws IOException {
        ObjectNode reply = envelope(request); reply.set("result", result); send(exchange, 200, reply);
    }
    private static void send(HttpExchange exchange, int status, JsonNode body) throws IOException {
        byte[] encoded = JSON.writeValueAsBytes(body); exchange.getResponseHeaders().set("Content-Type", "application/json");
        exchange.sendResponseHeaders(status, encoded.length); exchange.getResponseBody().write(encoded);
    }

    /** Deterministic secp256k1 producer signing the real ProgramCall signing payload. */
    private static byte[] sign(ProgramsClient.ProgramCall call) {
        var curve = org.bouncycastle.crypto.ec.CustomNamedCurves.getByName("secp256k1");
        var domain = new org.bouncycastle.crypto.params.ECDomainParameters(curve.getCurve(), curve.getG(), curve.getN());
        var signer = new org.bouncycastle.crypto.signers.ECDSASigner(
            new org.bouncycastle.crypto.signers.HMacDSAKCalculator(new org.bouncycastle.crypto.digests.SHA256Digest()));
        signer.init(true, new org.bouncycastle.crypto.params.ECPrivateKeyParameters(BigInteger.ONE, domain));
        byte[] digest = ProgramsClient.Programs.transactionHash(call.signingPayload());
        BigInteger[] signature = signer.generateSignature(digest);
        BigInteger lowS = signature[1].min(curve.getN().subtract(signature[1]));
        signer.init(false, new org.bouncycastle.crypto.params.ECPublicKeyParameters(curve.getG(), domain));
        assertTrue(signer.verifySignature(digest, signature[0], lowS));
        ObjectNode tx = call.transaction();
        for (int parity = 0; parity < 2; parity++) {
            var fields = new ByteArrayOutputStream();
            for (String field : List.of("chainId", "nonce", "maxPriorityFeePerGas", "maxFeePerGas", "gas"))
                fields.writeBytes(ProgramsClient.Programs.rlp(magnitude(ProgramsClient.Programs.quantity(tx.required(field))), false));
            fields.writeBytes(ProgramsClient.Programs.rlp(bytes(tx.required("to")), false));
            fields.writeBytes(ProgramsClient.Programs.rlp(magnitude(ProgramsClient.Programs.quantity(tx.required("value"))), false));
            fields.writeBytes(ProgramsClient.Programs.rlp(bytes(tx.required("data")), false));
            fields.writeBytes(ProgramsClient.Programs.rlp(new byte[0], true));
            fields.writeBytes(ProgramsClient.Programs.rlp(magnitude(BigInteger.valueOf(parity)), false));
            fields.writeBytes(ProgramsClient.Programs.rlp(magnitude(signature[0]), false));
            fields.writeBytes(ProgramsClient.Programs.rlp(magnitude(lowS), false));
            byte[] raw = ProgramsClient.Programs.concat(new byte[] {2}, ProgramsClient.Programs.rlp(fields.toByteArray(), true));
            try { ProgramsClient.Programs.bind(call, raw); return raw; }
            catch (PlatformSdkException error) { assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, error.code()); }
        }
        throw new AssertionError("neither signature recovery parity matches");
    }
    private static byte[] magnitude(BigInteger number) {
        if (number.signum() == 0) return new byte[0];
        byte[] value = number.toByteArray(); return value[0] == 0 ? Arrays.copyOfRange(value, 1, value.length) : value;
    }
    private static final class Loopback implements AutoCloseable {
        private final HttpServer server;
        private final ExecutorService executor = Executors.newVirtualThreadPerTaskExecutor();
        private final AtomicReference<Throwable> failure = new AtomicReference<>();
        Loopback(HttpHandler handler) throws IOException {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0); server.setExecutor(executor);
            server.createContext("/", exchange -> {
                try { handler.handle(exchange); }
                catch (IOException disconnected) { /* The real HTTP client may cancel or lose a response. */ }
                catch (Throwable error) { failure.compareAndSet(null, error); }
                finally { exchange.close(); }
            });
            server.start();
        }
        URI uri() { return URI.create("http://127.0.0.1:" + server.getAddress().getPort()); }
        @Override public void close() {
            server.stop(0); executor.close();
            if (failure.get() != null) throw new AssertionError("loopback peer failed", failure.get());
        }
    }
}
