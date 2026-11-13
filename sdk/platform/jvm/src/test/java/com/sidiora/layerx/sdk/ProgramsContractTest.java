package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JavaType;
import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sun.net.httpserver.HttpServer;
import java.net.InetSocketAddress;
import java.util.concurrent.CopyOnWriteArrayList;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.math.BigInteger;
import java.net.URI;
import java.net.http.HttpClient;
import java.nio.ByteBuffer;
import java.nio.file.Files;
import java.nio.file.Path;
import com.fasterxml.jackson.databind.JsonNode;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertArrayEquals;
import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertInstanceOf;
import static org.junit.jupiter.api.Assertions.assertThrows;
import static org.junit.jupiter.api.Assertions.assertTrue;

public final class ProgramsContractTest {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final String HEX32 = "01".repeat(32);
    private static final String OTHER_HEX32 = "02".repeat(32);
    private static final String LAYERX_SECRET = "lxp_live_" + "a1".repeat(32);

    @Test
    void productionTransportRejectsRedirects() {
        assertThrows(PlatformSdkException.class, () -> new HttpProductionTransport(
            HttpClient.newBuilder().followRedirects(HttpClient.Redirect.ALWAYS).build(), JSON,
            URI.create("http://localhost:8080"), URI.create("http://localhost:9090"),
            Duration.ofSeconds(1), null));
    }

    @Test
    void generatedSourceMetadataAndPrecompileInterfaceUseTheirDeclaredContracts() throws Exception {
        long now = System.currentTimeMillis();
        ObjectNode source = JSON.createObjectNode().put("status", "verified")
            .put("source_digest", HEX32).put("environment_digest", OTHER_HEX32)
            .put("pipeline", "sha256-source-artifact-reproducible-build-v1");
        ObjectNode interfaceValue = JSON.createObjectNode().put("program_id", HEX32).put("version", 7)
            .put("code_hash", HEX32).put("abi_version", 2).put("interface", "00")
            .put("interface_digest", "6e340b9cffb37a989ca544e6bb780a2c78901d3fb33738768511a30617afa01d")
            .put("receipt_digest", OTHER_HEX32).put("state_root", HEX32)
            .put("observed_sequence", "9").put("observed_at", Long.toString(now))
            .put("valid_through", Long.toString(now + 60_000))
            .put("verification", "deployment-interface-and-current-head-verified").set("source", source);

        var generated = JSON.treeToValue(interfaceValue,
            GeneratedSchema.AgentModels.VerifiedProgramInterface.class);
        assertEquals("verified", generated.source().status());
        assertEquals(HEX32, generated.source().source_digest());

        NativeProgramDeploy deploy = NativeProgramDeploy.decode(fixtureBytes(
            fixture("native-program-deploy-v4.json"), "payload_hex"));
        byte[] encoding = deploy.programInterface();
        byte[] digest = sha256(encoding);
        byte[] head = concatenate(ProgramsClient.Programs.word(BigInteger.ONE, 8),
            deploy.authority(), deploy.newHash(), ProgramsClient.Programs.word(BigInteger.valueOf(deploy.guestAbi()), 16),
            ProgramsClient.Programs.word(BigInteger.valueOf(7), 32), ProgramsClient.Programs.word(BigInteger.ONE, 1));
        try (RpcEndpoint endpoint = new RpcEndpoint(request -> {
            String data = request.required("params").get(0).required("data").asText();
            byte[] value = data.startsWith("0x8efc7240")
                ? ProgramsClient.Programs.abi("", new Object[] {
                    ProgramsClient.Programs.word(BigInteger.valueOf(7), 32), digest, encoding}, 2)
                : head;
            return JSON.getNodeFactory().textNode("0x" + HexFormat.of().formatHex(value));
        })) {
            ProgramsClient.Interface typed = client(endpoint).interfaceAt(deploy.programId())
                .toCompletableFuture().join();
            assertEquals(7, typed.version());
            assertArrayEquals(encoding, typed.interfaceBytes());
            assertArrayEquals(digest, typed.interfaceDigest());
            assertArrayEquals(deploy.newHash(), typed.codeHash());
            assertEquals(deploy.guestAbi(), typed.abiVersion());
            org.junit.jupiter.api.Assertions.assertNull(typed.source());
            assertEquals("pending", typed.verification());
            assertEquals(2, endpoint.requests.size());
            for (JsonNode request : endpoint.requests) {
                assertEquals("eth_call", request.required("method").asText());
                assertEquals("pending", request.required("params").get(1).asText());
                assertEquals(ProgramsClient.PROGRAMS_ADDRESS, request.required("params").get(0).required("to").asText());
            }
            assertTrue(endpoint.requests.get(0).required("params").get(0).required("data").asText().startsWith("0x8efc7240"));
            assertTrue(endpoint.requests.get(1).required("params").get(0).required("data").asText().startsWith("0xb1ffb3d4"));
        }
    }

    @Test
    void programsDecoderRefusesRetiredRestEnvelopes() {
        var transport = new HttpProductionTransport(HttpClient.newHttpClient(), JSON,
            URI.create("http://localhost:8080"), URI.create("http://localhost:9090"),
            Duration.ofSeconds(1), null);
        JavaType object = JSON.constructType(ObjectNode.class);
        byte[] result = "{\"result\":{}}".getBytes(StandardCharsets.UTF_8);
        PlatformSdkException deploy = assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.deploy", 200, result, object));
        assertEquals(PlatformSdkException.Code.DECODE_FAILURE, deploy.code());
        PlatformSdkException receipt = assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.receipt", 200, result, object));
        assertEquals(PlatformSdkException.Code.DECODE_FAILURE, receipt.code());

        byte[] error = "{\"error\":{\"code\":\"policy_refusal\",\"retry\":\"never\"}}"
            .getBytes(StandardCharsets.UTF_8);
        PlatformSdkException windDown = assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.wind-down", 400, error, object));
        assertEquals(PlatformSdkException.Code.DECODE_FAILURE, windDown.code());
    }

    @Test
    void programsAgentEnvelopeUsesExactOperationStatusMatrix() throws Exception {
        var credential = new HttpProductionTransport.LayerXKeyCredential("test",
            new SecretBytes(LAYERX_SECRET.getBytes(StandardCharsets.US_ASCII)));
        var transport = new HttpProductionTransport(HttpClient.newHttpClient(), JSON,
            URI.create("http://localhost:8080"), URI.create("http://localhost:9090"),
            Duration.ofSeconds(1), credential);
        JavaType object = JSON.constructType(ObjectNode.class);
        byte[] achieved = ("{\"request_id\":\"1\",\"value\":{},"
            + "\"verification_status\":{\"state\":\"Achieved\",\"level\":\"SequencerSigned\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertEquals(0, transport.<ObjectNode>decodePrograms("program.simulate", 200, achieved, object).size());

        byte[] terminal = ("{\"request_id\":\"1\",\"value\":{\"state\":\"executed\"},"
            + "\"verification_status\":{\"state\":\"Achieved\",\"level\":\"SequencerSigned\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertEquals("executed", transport.<ObjectNode>decodePrograms(
            "program.call", 200, terminal, object).path("state").textValue());

        byte[] downgraded = ("{\"request_id\":\"1\",\"value\":{},"
            + "\"verification_status\":{\"state\":\"Unverified\",\"level\":\"SequencerSigned\"}}")
            .getBytes(StandardCharsets.UTF_8);
        PlatformSdkException downgrade = assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.simulate", 200, downgraded, object));
        assertEquals(PlatformSdkException.Code.DECODE_FAILURE, downgrade.code());

        byte[] extra = ("{\"request_id\":\"1\",\"value\":{},"
            + "\"verification_status\":{\"state\":\"Achieved\",\"level\":\"SequencerSigned\"},"
            + "\"extra\":true}").getBytes(StandardCharsets.UTF_8);
        assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.simulate", 200, extra, object));

        byte[] serverVerifiedOnly = ("{\"request_id\":\"1\",\"value\":{},"
            + "\"verification_status\":{\"state\":\"Unverified\",\"requested\":\"SequencerSigned\","
            + "\"achieved\":\"Unverified\","
            + "\"reason\":\"server_side_receipt_verification_only\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertEquals(0, transport.<ObjectNode>decodePrograms(
            "program.discover", 200, serverVerifiedOnly, object).size());
        assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.discover", 200, achieved, object));

        byte[] oldUnverified = ("{\"request_id\":\"1\",\"value\":{},"
            + "\"verification_status\":{\"state\":\"Unverified\",\"level\":\"SequencerSigned\","
            + "\"reason\":\"server_side_receipt_verification_only\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.discover", 200, oldUnverified, object));

        byte[] pendingUnknown = ("{\"request_id\":\"1\",\"value\":{\"state\":\"unknown\"},"
            + "\"verification_status\":{\"state\":\"Unverified\",\"requested\":\"SequencerSigned\","
            + "\"achieved\":\"Unverified\","
            + "\"reason\":\"receipt_pending\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertEquals("unknown", transport.<ObjectNode>decodePrograms(
            "program.receipt", 200, pendingUnknown, object).path("state").textValue());

        byte[] pendingInFlight = ("{\"request_id\":\"1\",\"value\":{\"state\":\"pending\"},"
            + "\"verification_status\":{\"state\":\"Unverified\",\"requested\":\"SequencerSigned\","
            + "\"achieved\":\"Unverified\",\"reason\":\"receipt_pending\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertEquals("pending", transport.<ObjectNode>decodePrograms(
            "program.activity", 200, pendingInFlight, object).path("state").textValue());

        byte[] achievedUnknown = ("{\"request_id\":\"1\",\"value\":{\"state\":\"unknown\"},"
            + "\"verification_status\":{\"state\":\"Achieved\",\"level\":\"SequencerSigned\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.receipt", 200, achievedUnknown, object));

        byte[] achievedPending = ("{\"request_id\":\"1\",\"value\":{\"state\":\"pending\"},"
            + "\"verification_status\":{\"state\":\"Achieved\",\"level\":\"SequencerSigned\"}}")
            .getBytes(StandardCharsets.UTF_8);
        assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.call", 200, achievedPending, object));

        byte[] serviceError = ("{\"class\":\"PolicyRefusal\",\"protocol_result_code\":null,"
            + "\"retriability\":\"Terminal\",\"request_id\":\"2\",\"reason\":\"policy_refusal\"}")
            .getBytes(StandardCharsets.UTF_8);
        PlatformSdkException error = assertThrows(PlatformSdkException.class,
            () -> transport.decodePrograms("program.discover", 403, serviceError, object));
        assertEquals(PlatformSdkException.Code.POLICY_REFUSAL, error.code());
        assertEquals("2", error.requestId());
        credential.close();
    }

    @Test
    void receiptSelectorBindsUnknownActivityAndIdempotency() throws Exception {
        NativeProgramCall nativeCall = NativeProgramCall.decode(fixtureBytes(fixture("native-program-call-v4.json"), "payload_hex"));
        byte[] signed = ReceiptFixtureTest.signedTransaction(nativeCall.calldata(BigInteger.valueOf(1000)));
        byte[] hash = ProgramsClient.Programs.transactionHash(signed);
        ObjectNode view = JSON.createObjectNode().put("rung", "submitted")
            .put("txHash", "0x" + HexFormat.of().formatHex(hash)).put("nonce", "0x3");
        try (RpcEndpoint endpoint = new RpcEndpoint(request -> view.deepCopy())) {
            ProgramsClient programs = client(endpoint);
            IdempotencyKey key = new IdempotencyKey(OTHER_HEX32);
            var call = new ProgramsClient.Call(nativeCall, BigInteger.valueOf(1000), signed);
            var submitted = programs.submit(call, key).toCompletableFuture().join();
            assertTrue(submitted.unknown());
            assertEquals(key.value(), submitted.idempotencyKey());
            PlatformSdkException pending = assertInstanceOf(PlatformSdkException.class,
                assertThrows(java.util.concurrent.CompletionException.class,
                    () -> programs.receipt(key, hash).toCompletableFuture().join()).getCause());
            assertEquals(PlatformSdkException.Code.UNKNOWN_OUTCOME, pending.code());
            JsonNode request = endpoint.requests.getLast();
            assertEquals("pax_sequencerReceipt", request.required("method").asText());
            assertEquals("0x" + HexFormat.of().formatHex(hash), request.required("params").get(0).asText());
            assertEquals(1, request.required("params").size());
            view.put("txHash", "0x" + OTHER_HEX32);
            PlatformSdkException mismatch = assertInstanceOf(PlatformSdkException.class,
                assertThrows(java.util.concurrent.CompletionException.class,
                    () -> programs.receipt(key, hash).toCompletableFuture().join()).getCause());
            assertEquals(PlatformSdkException.Code.VERIFICATION_FAILURE, mismatch.code());
            int requests = endpoint.requests.size();
            assertEquals(PlatformSdkException.Code.INVALID_ARGUMENT,
                assertThrows(PlatformSdkException.class, () -> programs.receipt(key,
                    HexFormat.of().parseHex(HEX32))).code());
            assertEquals(requests, endpoint.requests.size());

            JsonNode receiptVector = fixture("sequencer-receipt-v1.json").required("cases").get(0);
            view.removeAll();
            view.setAll((ObjectNode) receiptVector.required("receiptJson"));
            byte[] retained = prefixedBytes(receiptVector, "rawTransaction");
            var receipted = programs.receipt(new ProgramsClient.ProgramSubmission(
                ProgramsClient.Programs.transactionHash(retained), retained, null)).toCompletableFuture().join();
            assertFalse(receipted.pending());
            assertEquals(LocalVerifier.VerificationLevel.RECEIPTED, receipted.receipt().level());
            assertArrayEquals(prefixedBytes(receiptVector, "receiptDigest"), receipted.receipt().receiptDigest());
            assertArrayEquals(retained, receipted.signedTransaction());
        }
    }

    @Test
    void programCallBoundsAndCanonicalCapabilitiesFailClosed() throws Exception {
        var budget = new ProgramsClient.Budget(BigInteger.ONE, BigInteger.ZERO);
        byte[] programId = new byte[32];
        programId[0] = 1;
        assertThrows(PlatformSdkException.class, () -> new ProgramsClient.Call(programId,
            new byte[ProgramsClient.MAX_CALLDATA_BYTES + 1], budget, List.of(), new byte[] {1}));
        assertThrows(PlatformSdkException.class, () -> new ProgramsClient.Call(programId,
            new byte[0], budget, List.of(ProgramsClient.Capability.TRANSFER,
                ProgramsClient.Capability.STORAGE_READ), new byte[] {1}));
        NativeProgramCall nativeCall = NativeProgramCall.decode(fixtureBytes(fixture("native-program-call-v4.json"), "payload_hex"));
        byte[] signed = ReceiptFixtureTest.signedTransaction(nativeCall.calldata(BigInteger.valueOf(1000)));
        try (RpcEndpoint endpoint = new RpcEndpoint(request -> JSON.nullNode())) {
            ProgramsClient programs = client(endpoint);
            var call = new ProgramsClient.Call(nativeCall, BigInteger.valueOf(1000), signed);
            PlatformSdkException invalid = assertThrows(PlatformSdkException.class,
                () -> programs.submit(call, new IdempotencyKey("not-a-bytes32")));
            assertEquals(PlatformSdkException.Code.INVALID_ARGUMENT, invalid.code());
            var wrongFee = new ProgramsClient.Call(nativeCall, BigInteger.valueOf(999), signed);
            assertEquals(PlatformSdkException.Code.INVALID_ARGUMENT,
                assertThrows(PlatformSdkException.class, () -> programs.submit(wrongFee,
                    new IdempotencyKey(OTHER_HEX32))).code());
            assertEquals(PlatformSdkException.Code.INVALID_ARGUMENT,
                assertThrows(PlatformSdkException.class, () -> programs.receipt(wrongFee,
                    new IdempotencyKey(OTHER_HEX32))).code());
            assertTrue(endpoint.requests.isEmpty());
        }
    }

    @Test
    void submittedSignedActivityBindsCanonicalCallActivityAndIdempotency() throws Exception {
        NativeProgramCall nativeCall = NativeProgramCall.decode(fixtureBytes(fixture("native-program-call-v4.json"), "payload_hex"));
        byte[] signed = ReceiptFixtureTest.signedTransaction(nativeCall.calldata(BigInteger.valueOf(1000)));
        byte[] transactionHash = ProgramsClient.Programs.transactionHash(signed);
        try (RpcEndpoint endpoint = new RpcEndpoint(request -> JSON.nullNode())) {
            ProgramsClient programs = client(endpoint);
            var call = new ProgramsClient.Call(nativeCall, BigInteger.valueOf(1000), signed);
            ProgramsClient.Submission result = programs.submit(call, new IdempotencyKey(OTHER_HEX32))
                .toCompletableFuture().join();
            assertTrue(result.unknown());
            assertEquals(OTHER_HEX32, result.idempotencyKey());
            assertArrayEquals(transactionHash, result.activityId());
            assertArrayEquals(signed, result.retainedSignedTransaction());
            assertArrayEquals(transactionHash, nativeCall.bindSignedActivity(result.retainedSignedTransaction()));
            assertEquals(1, endpoint.requests.size());
            JsonNode request = endpoint.requests.getFirst();
            assertEquals("eth_sendRawTransaction", request.required("method").asText());
            assertEquals("0x" + HexFormat.of().formatHex(signed), request.required("params").get(0).asText());
        }
    }

    @Test
    void programTransferAuthorizationRecomputesV1AndV2RootsAndRejectsMutation() {
        byte[] program = filled(0x11);
        byte[] principal = filled(0x22);
        byte[] asset = filled(0x33);
        byte[] destination = filled(0x44);
        byte[] v1 = transferAuthorization(false, program, principal, asset, destination);
        byte[] v2 = transferAuthorization(true, program, principal, asset, destination);
        byte[] root = transferRoot(principal, asset, destination, BigInteger.valueOf(7));
        ProgramsClient.verifyAuthorizationRoot(v1, root);
        ProgramsClient.verifyAuthorizationRoot(v2, root);

        byte[] mutatedAuthorization = v2.clone();
        mutatedAuthorization[mutatedAuthorization.length - 65] ^= 1;
        assertThrows(IllegalArgumentException.class,
            () -> ProgramsClient.verifyAuthorizationRoot(mutatedAuthorization, root));
        byte[] mutatedRoot = root.clone();
        mutatedRoot[0] ^= 1;
        assertThrows(IllegalArgumentException.class,
            () -> ProgramsClient.verifyAuthorizationRoot(v2, mutatedRoot));
        assertThrows(IllegalArgumentException.class,
            () -> ProgramsClient.verifyAuthorizationRoot(concatenate(v2, new byte[] {0}), root));
    }

    @Test
    void programDestinationAuthorizationVectors() throws Exception {
        JsonNode vectors = JSON.readTree(Files.readString(Path.of(
            System.getProperty("layerx.repo.root", "../../.."),
            "programs/fixtures/pay5/destination-authorization-vectors.json")));
        assertTrue(vectors.isArray());
        assertEquals(1271, vectors.size());
        int accepted = 0;
        int refused = 0;
        for (JsonNode vector : vectors) {
            String name = vector.required("name").asText();
            byte[] encoded = fixtureBytes(vector, "encoded");
            byte[] root = fixtureBytes(vector, "root");
            if (vector.required("accept").asBoolean()) {
                assertDoesNotThrow(() -> ProgramsClient.verifyAuthorizationRoot(encoded, root), name);
                byte[] mutatedRoot = root.clone();
                mutatedRoot[0] ^= 1;
                assertThrows(IllegalArgumentException.class,
                    () -> ProgramsClient.verifyAuthorizationRoot(encoded, mutatedRoot), name);
                accepted++;
            } else {
                assertThrows(IllegalArgumentException.class,
                    () -> ProgramsClient.verifyAuthorizationRoot(encoded, root), name);
                refused++;
            }
        }
        assertEquals(2, accepted);
        assertEquals(1269, refused);
    }

    @Test
    void occupancyV1V2V3BindsCountersFeesAssetAndTransferRoot() {
        byte[] program = filled(0x11);
        byte[] payer = filled(0x77);
        byte[] asset = filled(0x66);
        byte[] root = occupancyRoot(payer, asset, BigInteger.valueOf(6));
        byte[] v1 = legacyOccupancy(false, program, payer);
        byte[] v2 = legacyOccupancy(true, program, payer);
        byte[] v3 = occupancyV3(program, payer);
        for (byte[] evidence : List.of(v1, v2, v3)) {
            ProgramsClient.verifyOccupancyBinding(evidence, asset, BigInteger.valueOf(3),
                BigInteger.valueOf(6), root);
        }

        assertThrows(IllegalArgumentException.class, () -> ProgramsClient.verifyOccupancyBinding(
            v3, asset, BigInteger.valueOf(4), BigInteger.valueOf(6), root));
        assertThrows(IllegalArgumentException.class, () -> ProgramsClient.verifyOccupancyBinding(
            v3, asset, BigInteger.valueOf(3), BigInteger.valueOf(7), root));
        byte[] mutatedAsset = asset.clone();
        mutatedAsset[0] ^= 1;
        assertThrows(IllegalArgumentException.class, () -> ProgramsClient.verifyOccupancyBinding(
            v3, mutatedAsset, BigInteger.valueOf(3), BigInteger.valueOf(6), root));
        byte[] mutatedRoot = root.clone();
        mutatedRoot[0] ^= 1;
        assertThrows(IllegalArgumentException.class, () -> ProgramsClient.verifyOccupancyBinding(
            v3, asset, BigInteger.valueOf(3), BigInteger.valueOf(6), mutatedRoot));
        byte[] mutatedEvidence = v3.clone();
        int declaredUnitsLowByte = "LXP/storage-occupancy-settlement/v3\0"
            .getBytes(StandardCharsets.UTF_8).length + 8 + 4 + 7 * 8 + 15;
        mutatedEvidence[declaredUnitsLowByte] ^= 1;
        assertThrows(IllegalArgumentException.class, () -> ProgramsClient.verifyOccupancyBinding(
            mutatedEvidence, asset, BigInteger.valueOf(3), BigInteger.valueOf(6), root));
        assertThrows(IllegalArgumentException.class, () -> ProgramsClient.verifyOccupancyBinding(
            concatenate(v3, new byte[] {0}), asset, BigInteger.valueOf(3),
            BigInteger.valueOf(6), root));
    }

    @Test
    void terminalAttachmentWrappersEnforceAuthorityThenOccupancyAndFullConsumption() {
        byte[] program = filled(0x11);
        byte[] principal = filled(0x22);
        byte[] asset = filled(0x33);
        byte[] destination = filled(0x44);
        byte[] authorization = transferAuthorization(true, program, principal, asset, destination);
        byte[] root = transferRoot(principal, asset, destination, BigInteger.valueOf(7));
        byte[] inner = new byte[] {1, 2, 3};
        byte[] occupancy = occupancyV3(program, filled(0x77));
        byte[] canonical = authorityWrapper(occupancyWrapper(inner, occupancy), authorization, root);
        assertArrayEquals(inner, ProgramsClient.unwrapTerminal(canonical).inner());

        byte[] wrongOrder = occupancyWrapper(authorityWrapper(inner, authorization, root), occupancy);
        assertThrows(IllegalArgumentException.class, () -> ProgramsClient.unwrapTerminal(wrongOrder));
        byte[] duplicateAuthority = authorityWrapper(authorityWrapper(inner, authorization, root),
            authorization, root);
        assertThrows(IllegalArgumentException.class,
            () -> ProgramsClient.unwrapTerminal(duplicateAuthority));
        byte[] duplicateOccupancy = occupancyWrapper(occupancyWrapper(inner, occupancy), occupancy);
        assertThrows(IllegalArgumentException.class,
            () -> ProgramsClient.unwrapTerminal(duplicateOccupancy));
        assertThrows(IllegalArgumentException.class,
            () -> ProgramsClient.unwrapTerminal(concatenate(canonical, new byte[] {0})));
    }

    @Test
    void nativeKernelCallFixturesPreserveCanonicalPayloadsAndRejectUnsupportedAbis() throws Exception {
        for (String name : List.of("native-program-call-v3.json", "native-program-call-v4.json")) {
            JsonNode fixture = fixture(name);
            byte[] payload = fixtureBytes(fixture, "payload_hex");
            byte[] signed = fixtureBytes(fixture, "signed_activity_hex");
            NativeProgramCall nativeCall = NativeProgramCall.decode(payload);
            assertArrayEquals(payload, nativeCall.encode());
            assertArrayEquals(fixtureBytes(fixture, "idempotency_key_hex"),
                NativeProgramLifecycleRequest.bind(3, payload, signed));
            for (int unsupported : new int[] {0, 6, 65535}) {
                byte[] invalidPayload = payload.clone();
                ByteBuffer.wrap(invalidPayload).putShort(32, (short) unsupported);
                assertThrows(IllegalArgumentException.class, () -> NativeProgramCall.decode(invalidPayload));
            }
        }
    }

    private static JsonNode fixture(String name) throws Exception {
        return JSON.readTree(Files.readString(Path.of(System.getProperty("layerx.repo.root", "../../.."),
            "sdk/platform/conformance/fixtures", name)));
    }

    private static byte[] fixtureBytes(JsonNode value, String name) {
        return HexFormat.of().parseHex(value.path(name).asText());
    }

    private static byte[] transferAuthorization(boolean candidate, byte[] program, byte[] principal,
                                                byte[] asset, byte[] destination) {
        byte[] domain = (candidate ? "LayerX/programs/402LXP/transfer-set/v2\0"
            : "LayerX/programs/402LXP/transfer-set/v1\0").getBytes(StandardCharsets.UTF_8);
        byte[] events = concatenate("LayerX/programs/events/v1\0".getBytes(StandardCharsets.UTF_8),
            integer(BigInteger.ZERO, 4));
        return concatenate(domain, program, principal, filled(0x55), new byte[9], sized(events),
            integer(BigInteger.ZERO, 8), integer(BigInteger.ONE, 8), new byte[9],
            candidate ? concatenate(new byte[] {1}, principal) : new byte[0], asset, destination,
            integer(BigInteger.valueOf(7), 16), program);
    }

    private static byte[] occupancyV3(byte[] program, byte[] payer) {
        byte[] namespace = concatenate(new byte[] {65}, program, new byte[] {0}, payer);
        return concatenate("LXP/storage-occupancy-settlement/v3\0".getBytes(StandardCharsets.UTF_8),
            integer(BigInteger.valueOf(2), 8), integer(BigInteger.ONE, 4),
            integer(BigInteger.ZERO, 8), integer(BigInteger.ZERO, 8), integer(BigInteger.ZERO, 8),
            integer(BigInteger.ZERO, 8), integer(BigInteger.ZERO, 8), integer(BigInteger.ZERO, 8),
            integer(BigInteger.valueOf(2), 8), integer(BigInteger.valueOf(3), 16),
            integer(BigInteger.valueOf(6), 16), integer(BigInteger.valueOf(6), 16),
            integer(BigInteger.ZERO, 16), integer(BigInteger.ONE, 4), namespace, payer, program,
            filled(0x88), integer(BigInteger.ONE, 8), integer(BigInteger.valueOf(2), 8),
            integer(BigInteger.valueOf(3), 8), integer(BigInteger.valueOf(3), 8),
            integer(BigInteger.valueOf(3), 16), integer(BigInteger.valueOf(2), 8),
            integer(BigInteger.valueOf(6), 16), integer(BigInteger.ZERO, 16),
            integer(BigInteger.valueOf(6), 16), integer(BigInteger.ZERO, 16), new byte[] {1},
            integer(BigInteger.ZERO, 16), integer(BigInteger.valueOf(3), 8),
            integer(BigInteger.valueOf(2), 8), integer(BigInteger.ZERO, 16), filled(0x99));
    }

    private static byte[] legacyOccupancy(boolean versioned, byte[] program, byte[] payer) {
        byte[] domain = (versioned ? "LXP/storage-occupancy-settlement/v2\0"
            : "LXP/storage-occupancy-settlement/v1\0").getBytes(StandardCharsets.UTF_8);
        byte[] namespace = concatenate(new byte[] {65}, program, new byte[] {0}, payer);
        return concatenate(domain, integer(BigInteger.valueOf(2), 8),
            versioned ? integer(BigInteger.ONE, 4) : new byte[0], integer(BigInteger.ZERO, 8),
            integer(BigInteger.ZERO, 8), integer(BigInteger.ZERO, 8), integer(BigInteger.ZERO, 8),
            integer(BigInteger.ZERO, 8), integer(BigInteger.ZERO, 8),
            integer(BigInteger.valueOf(2), 8), integer(BigInteger.valueOf(3), 16),
            integer(BigInteger.valueOf(6), 16), integer(BigInteger.ONE, 8), namespace, payer,
            integer(BigInteger.ONE, 8), integer(BigInteger.valueOf(2), 8),
            integer(BigInteger.valueOf(3), 8), integer(BigInteger.valueOf(3), 8),
            integer(BigInteger.valueOf(3), 16), integer(BigInteger.valueOf(2), 8),
            integer(BigInteger.valueOf(6), 16));
    }

    private static byte[] transferRoot(byte[] principal, byte[] asset, byte[] destination,
                                       BigInteger amount) {
        byte[] leg = concatenate(new byte[] {0}, principal, destination, asset, integer(amount, 16),
            integer(BigInteger.ONE, 2));
        return sha256("LXP/v1/merkle-leaf\0".getBytes(StandardCharsets.UTF_8), leg);
    }

    private static byte[] occupancyRoot(byte[] payer, byte[] asset, BigInteger amount) {
        byte[] treasury = sha256("LX:ACCOUNT:v1".getBytes(StandardCharsets.UTF_8),
            integer(BigInteger.valueOf(11), 4), "system:fees".getBytes(StandardCharsets.UTF_8));
        byte[] leg = concatenate(new byte[] {0}, payer, treasury, asset, integer(amount, 16),
            integer(BigInteger.valueOf(23), 2));
        return sha256("LXP/v1/merkle-leaf\0".getBytes(StandardCharsets.UTF_8), leg);
    }

    private static byte[] authorityWrapper(byte[] inner, byte[] authorization, byte[] root) {
        return concatenate("LXP/program-execution-with-transfer-authority/v2\0"
            .getBytes(StandardCharsets.UTF_8), sized(inner), sized(authorization), root);
    }

    private static byte[] occupancyWrapper(byte[] inner, byte[] evidence) {
        return concatenate("LXP/program-execution-with-occupancy/v1\0"
            .getBytes(StandardCharsets.UTF_8), sized(inner), sized(evidence));
    }

    private static byte[] sized(byte[] value) {
        return concatenate(integer(BigInteger.valueOf(value.length), 4), value);
    }

    private static byte[] integer(BigInteger value, int length) {
        byte[] raw = value.toByteArray();
        if (value.signum() < 0 || value.bitLength() > length * 8) throw new AssertionError();
        byte[] result = new byte[length];
        int copy = Math.min(raw.length, length);
        System.arraycopy(raw, raw.length - copy, result, length - copy, copy);
        return result;
    }

    private static byte[] concatenate(byte[]... values) {
        int length = 0;
        for (byte[] value : values) length += value.length;
        byte[] result = new byte[length];
        int offset = 0;
        for (byte[] value : values) {
            System.arraycopy(value, 0, result, offset, value.length);
            offset += value.length;
        }
        return result;
    }

    private static byte[] filled(int value) {
        byte[] result = new byte[32];
        java.util.Arrays.fill(result, (byte) value);
        return result;
    }

    private static byte[] sha256(byte[]... values) {
        try {
            var digest = java.security.MessageDigest.getInstance("SHA-256");
            for (byte[] value : values) digest.update(value);
            return digest.digest();
        } catch (java.security.GeneralSecurityException impossible) {
            throw new AssertionError(impossible);
        }
    }

    private static ProgramsClient client(RpcEndpoint endpoint) throws Exception {
        JsonNode key = fixture("sequencer-receipt-v1.json").required("cases").get(0).required("pinnedKey");
        var pin = new LocalVerifier.ReceiptKey(prefixedBytes(key, "keyId"), prefixedBytes(key, "publicKey"),
            key.required("firstHeight").bigIntegerValue(), key.required("lastHeight").bigIntegerValue());
        return new ProgramsClient(new ProgramsClient.Programs(endpoint.uri()), ReceiptFixtureTest.SENDER,
            BigInteger.ONE, BigInteger.TWO, new LocalVerifier.ReceiptKeys(List.of(pin)), pin.keyId());
    }

    private static byte[] prefixedBytes(JsonNode value, String field) {
        return HexFormat.of().parseHex(value.required(field).asText().substring(2));
    }

    private static final class RpcEndpoint implements AutoCloseable {
        private final HttpServer server;
        private final List<JsonNode> requests = new CopyOnWriteArrayList<>();

        RpcEndpoint(java.util.function.Function<JsonNode, JsonNode> responseValue) throws Exception {
            server = HttpServer.create(new InetSocketAddress("127.0.0.1", 0), 0);
            server.createContext("/rpc", exchange -> {
                try (exchange) {
                    JsonNode request = JSON.readTree(exchange.getRequestBody());
                    requests.add(request);
                    ObjectNode response = JSON.createObjectNode().put("jsonrpc", "2.0");
                    response.set("id", request.required("id"));
                    if ("eth_sendRawTransaction".equals(request.required("method").asText())) {
                        byte[] raw = HexFormat.of().parseHex(request.required("params").get(0).asText().substring(2));
                        response.put("result", "0x" + HexFormat.of().formatHex(ProgramsClient.Programs.transactionHash(raw)));
                    } else {
                        response.set("result", responseValue.apply(request));
                    }
                    byte[] encoded = JSON.writeValueAsBytes(response);
                    exchange.getResponseHeaders().set("Content-Type", "application/json");
                    exchange.sendResponseHeaders(200, encoded.length);
                    exchange.getResponseBody().write(encoded);
                }
            });
            server.start();
        }

        URI uri() { return URI.create("http://127.0.0.1:" + server.getAddress().getPort() + "/rpc"); }
        @Override public void close() { server.stop(0); }
    }
}
