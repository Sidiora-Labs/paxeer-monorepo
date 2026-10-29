package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.net.URI;
import java.net.http.HttpClient;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.List;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

public final class AgentEnvelopeMajor2Test {
    private static final ObjectMapper JSON = new ObjectMapper();
    private static final URI ENDPOINT = URI.create("http://127.0.0.1:8080");

    @Test
    void agentEnvelopeEncodesVersionTwo() throws Exception {
        try (var client = HttpClient.newHttpClient();
             var session = new HttpProductionTransport.AgentSessionCredential("tenant", "11".repeat(32),
                 new SecretBytes("22".repeat(32).getBytes(StandardCharsets.US_ASCII)), "1")) {
            var transport = new HttpProductionTransport(client, JSON, ENDPOINT, ENDPOINT,
                Duration.ofSeconds(1), null, session);
            assertFalse(OperationCatalog.AGENT_OPERATIONS.isEmpty());
            for (String operation : OperationCatalog.AGENT_OPERATIONS) {
                var descriptor = OperationCatalog.agent(operation);
                boolean mutating = OperationCatalog.requiresIdempotency(descriptor);
                var key = mutating ? new IdempotencyKey("33".repeat(32)) : null;
                var request = JSON.createObjectNode();
                var call = new ProductionTransport.Call(descriptor, request, null, key);
                JsonNode envelope = JSON.readTree(transport.agentEnvelope(call, "7", mutating));
                assertTrue(envelope.get("version").isIntegralNumber(), operation);
                assertEquals(2, envelope.get("version").intValue(), operation);
                assertEquals(operation, envelope.get("operation").textValue());
                assertEquals("7", envelope.get("request_id").textValue());
                assertEquals(request, envelope.get("request"));
                assertEquals(mutating ? key.value() : null, envelope.get("idempotency_key").textValue());
                if (operation.equals("agent.register") || operation.equals("session.open")) {
                    assertTrue(envelope.get("credential").isNull(), operation);
                } else {
                    assertEquals("tenant", envelope.get("credential").get("tenant").textValue(), operation);
                }
            }
        }
    }

    @Test
    void agentEnvelopeRefusesVersionOneResponse() throws Exception {
        try (var client = HttpClient.newHttpClient()) {
            var transport = new HttpProductionTransport(client, JSON, ENDPOINT, ENDPOINT,
                Duration.ofSeconds(1), null);
            var response = JSON.createObjectNode().put("version", 2).put("request_id", "7");
            response.set("value", JSON.createObjectNode().put("accepted", true));
            response.set("verification_status", JSON.createObjectNode()
                .put("state", "achieved").put("level", "Unverified"));
            var type = JSON.constructType(JsonNode.class);
            JsonNode accepted = transport.decodeAgent(200, JSON.writeValueAsBytes(response), "7", type);
            assertEquals(response.get("value"), accepted);

            for (String encodedVersion : List.of("1", "0", "3", "2.0", "\"2\"", "null", "4294967298")) {
                response.set("version", JSON.readTree(encodedVersion));
                PlatformSdkException error = assertThrows(PlatformSdkException.class,
                    () -> transport.decodeAgent(200, JSON.writeValueAsBytes(response), "7", type), encodedVersion);
                assertEquals(PlatformSdkException.Code.PROTOCOL_INCOMPATIBILITY, error.code(), encodedVersion);
                assertEquals(PlatformSdkException.Retry.NEVER, error.retry(), encodedVersion);
            }
            response.remove("version");
            PlatformSdkException missing = assertThrows(PlatformSdkException.class,
                () -> transport.decodeAgentReply(200, JSON.writeValueAsBytes(response), "7"));
            assertEquals(PlatformSdkException.Code.PROTOCOL_INCOMPATIBILITY, missing.code());

            var refusal = JSON.createObjectNode().put("version", 1).put("request_id", "7")
                .put("class", "PolicyRefusal").putNull("protocol_result_code")
                .put("retriability", "Terminal").put("reason", "refused");
            PlatformSdkException error = assertThrows(PlatformSdkException.class,
                () -> transport.decodeAgentReply(403, JSON.writeValueAsBytes(refusal), "7"));
            assertEquals(PlatformSdkException.Code.PROTOCOL_INCOMPATIBILITY, error.code());
            assertEquals(PlatformSdkException.Retry.NEVER, error.retry());
            refusal.put("version", 2);
            PlatformSdkException compatible = assertThrows(PlatformSdkException.class,
                () -> transport.decodeAgentReply(403, JSON.writeValueAsBytes(refusal), "7"));
            assertEquals(PlatformSdkException.Code.POLICY_REFUSAL, compatible.code());
            assertEquals(SchemaErrors.AgentClass.POLICY_REFUSAL, compatible.agentClass());
        }
    }
}
