package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.DatabindException;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ObjectNode;
import java.lang.reflect.ParameterizedType;
import java.lang.reflect.RecordComponent;
import java.math.BigInteger;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.HexFormat;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public final class GeneratedCatalogueIntentsTest {
    private static final ObjectMapper JSON = new ObjectMapper();

    @Test
    void intentPlanProducesPerLegCalls() throws Exception {
        JsonNode vector = fixture("intent-plan-v1.json");
        JsonNode expected = vector.required("plan_result");
        var request = JSON.treeToValue(vector.required("plan_request"),
            GeneratedSchema.HumanOperations.INTENT_PLAN.requestType());
        assertEquals("0x9858effd232b4033e47d90003d41ec34ecaeda94", request.source().evm_address());
        assertEquals(BigInteger.valueOf(125), request.source().resolution().chainId());
        var plan = JSON.treeToValue(expected, GeneratedSchema.HumanOperations.INTENT_PLAN.responseType());
        var model = JSON.treeToValue(expected, GeneratedSchema.HumanModels.IntentPlan.class);
        assertEquals(2, plan.legs().size());
        assertEquals(model.legs(), plan.legs());
        assertEquals(expected, JSON.readTree(JSON.writeValueAsBytes(plan)));
        assertEquals(expected, JSON.readTree(JSON.writeValueAsBytes(model)));
        for (int i = 0; i < plan.legs().size(); i++) {
            var leg = plan.legs().get(i);
            JsonNode wire = expected.required("legs").get(i);
            assertEquals(i, leg.index());
            assertEquals(wire.required("to").asText(), leg.to());
            assertEquals(wire.required("data").asText(), leg.data());
            assertEquals(wire.required("chainId").bigIntegerValue(), leg.chainId());
            assertEquals(GeneratedSchema.HumanModels.IntentDomain.PAXEER, leg.domain());
            assertEquals(leg.payment().paymentReference(), plan.signing_requirements().get(i).action_key());
            assertTrue(leg.data().startsWith("0x78ea11db"));
            assertThrows(UnsupportedOperationException.class, () -> plan.legs().add(leg));
            for (String field : List.of("to", "data", "chainId")) {
                ObjectNode missing = wire.deepCopy();
                missing.remove(field);
                assertThrows(DatabindException.class,
                    () -> JSON.treeToValue(missing, GeneratedSchema.HumanModels.IntentLeg.class), field);
            }
            for (BigInteger invalid : List.of(BigInteger.valueOf(-1), BigInteger.ONE.shiftLeft(64))) {
                ObjectNode invalidChain = wire.deepCopy();
                invalidChain.put("chainId", invalid);
                assertThrows(DatabindException.class,
                    () -> JSON.treeToValue(invalidChain, GeneratedSchema.HumanModels.IntentLeg.class));
            }
        }
        assertNotEquals(plan.legs().get(0).data(), plan.legs().get(1).data());
        assertThrows(IllegalArgumentException.class,
            () -> GeneratedSchema.HumanModels.IntentDomain.fromWire("layerx"));

        ObjectNode submitted = JSON.createObjectNode().put("journey_id", "journey-per-leg")
            .put("plan_digest", plan.plan_digest()).put("state", "submitted")
            .put("state_copy_key", "intent.submitted");
        submitted.set("legs", expected.required("legs"));
        var submission = JSON.treeToValue(submitted, GeneratedSchema.HumanOperations.INTENT_SUBMIT.responseType());
        var submissionModel = JSON.treeToValue(submitted, GeneratedSchema.HumanModels.IntentSubmission.class);
        assertEquals(plan.legs(), submission.legs());
        assertEquals(plan.legs(), submissionModel.legs());
        assertEquals(submitted, JSON.readTree(JSON.writeValueAsBytes(submission)));
        assertThrows(UnsupportedOperationException.class, () -> submission.legs().clear());
        for (Class<?> type : List.of(GeneratedSchema.HumanModels.IntentPlan.class,
                GeneratedSchema.HumanOperations.IntentPlanResponse.class,
                GeneratedSchema.HumanModels.IntentSubmission.class,
                GeneratedSchema.HumanOperations.IntentSubmitResponse.class)) {
            RecordComponent legs = Arrays.stream(type.getRecordComponents())
                .filter(component -> component.getName().equals("legs")).findFirst().orElseThrow();
            assertEquals(List.class, legs.getType());
            assertEquals(GeneratedSchema.HumanModels.IntentLeg.class,
                ((ParameterizedType) legs.getGenericType()).getActualTypeArguments()[0]);
        }
        assertTrue(components(GeneratedSchema.HumanModels.IntentLeg.class)
            .containsAll(Set.of("to", "data", "chainId")));
        nativeRequestsCarryProgramCalldata();
    }

    @Test
    void verificationLevelsUseBlockCommitted() throws Exception {
        var committed = GeneratedSchema.HumanModels.HumanVerificationLevel.BLOCK_COMMITTED;
        assertEquals(Set.of("unverified", "receipt-verified", "block-committed"),
            Arrays.stream(GeneratedSchema.HumanModels.HumanVerificationLevel.values())
                .map(GeneratedSchema.HumanModels.HumanVerificationLevel::wire).collect(Collectors.toSet()));
        for (var level : GeneratedSchema.HumanModels.HumanVerificationLevel.values()) {
            assertSame(level, GeneratedSchema.HumanModels.HumanVerificationLevel.fromWire(level.wire()));
            assertEquals(level, JSON.readValue(JSON.writeValueAsBytes(level),
                GeneratedSchema.HumanModels.HumanVerificationLevel.class));
        }
        for (String retired : List.of("checkpoint-finalised", "paxeer-finalised", "settlement-anchored")) {
            assertThrows(IllegalArgumentException.class,
                () -> GeneratedSchema.HumanModels.HumanVerificationLevel.fromWire(retired));
            assertThrows(DatabindException.class, () -> JSON.readValue(JSON.writeValueAsBytes(retired),
                GeneratedSchema.HumanModels.HumanVerificationLevel.class));
        }
        ObjectNode evidence = JSON.createObjectNode().put("evidence_id", "receipt-1")
            .put("class", "layerx-receipt").put("verification", "block-committed");
        var reference = JSON.treeToValue(evidence, GeneratedSchema.HumanModels.EvidenceRef.class);
        assertEquals(committed, reference.verification());
        assertEquals(evidence, JSON.valueToTree(reference));
        ObjectNode material = evidence.deepCopy().put("content_type", "application/octet-stream")
            .put("bytes_base64", "AQID");
        var exported = JSON.treeToValue(material, GeneratedSchema.HumanModels.EvidenceMaterial.class);
        var response = JSON.treeToValue(material, GeneratedSchema.HumanOperations.EVIDENCE_GET.responseType());
        assertEquals(committed, exported.verification());
        assertEquals(committed, response.verification());
        assertEquals(material, JSON.valueToTree(exported));
        assertEquals(material, JSON.valueToTree(response));
        for (Class<?> type : List.of(GeneratedSchema.HumanModels.EvidenceRef.class,
                GeneratedSchema.HumanModels.EvidenceMaterial.class,
                GeneratedSchema.HumanOperations.EvidenceGetResponse.class)) {
            assertFalse(components(type).stream().anyMatch(name -> name.contains("settlement")));
            ObjectNode legacy = (type == GeneratedSchema.HumanModels.EvidenceRef.class ? evidence : material)
                .deepCopy().put("settlement_domain", "paxeer");
            assertThrows(DatabindException.class, () -> JSON.treeToValue(legacy, type));
        }
        assertFalse(GeneratedSchema.HUMAN.keySet().stream().anyMatch(name -> name.startsWith("binding.")));
        for (Class<?> container : List.of(GeneratedSchema.AgentModels.class,
                GeneratedSchema.HumanModels.class, GeneratedSchema.HumanOperations.class)) {
            Set<String> names = Arrays.stream(container.getDeclaredClasses()).map(Class::getSimpleName)
                .collect(Collectors.toSet());
            assertFalse(names.contains("SettlementDomain"));
            assertFalse(names.contains("WalletBinding"));
            assertFalse(names.stream().anyMatch(name -> name.startsWith("Binding")));
        }
        assertFalse(Arrays.stream(GeneratedSchema.HumanModels.JourneyKind.values())
            .anyMatch(kind -> kind.wire().equals("wallet-binding")));
        assertFalse(Arrays.stream(GeneratedSchema.HumanModels.NotificationClass.values())
            .anyMatch(kind -> kind.wire().equals("security-wallet-rebinding")));
    }

    private static void nativeRequestsCarryProgramCalldata() throws Exception {
        JsonNode vector = fixture("native-program-call-v4.json");
        JsonNode evm = vector.required("evm");
        ObjectNode wire = JSON.createObjectNode().put("to", evm.required("to").asText())
            .put("data", evm.required("calldata").asText())
            .put("chainId", evm.required("chainId").bigIntegerValue());
        var fields = JSON.treeToValue(wire, GeneratedSchema.AgentModels.NativeProgramCallFields.class);
        assertEquals(ProgramsClient.PROGRAMS_ADDRESS, fields.to());
        assertEquals(BigInteger.valueOf(125), fields.chainId());
        assertEquals(wire, JSON.valueToTree(fields));
        var decoded = NativeProgramCall.decodeCalldata(HexFormat.of().parseHex(fields.data().substring(2)));
        assertArrayEquals(HexFormat.of().parseHex(vector.required("payload_hex").asText()), decoded.encode());
        assertArrayEquals(HexFormat.of().parseHex(fields.data().substring(2)),
            decoded.calldata(new BigInteger(vector.required("fee_limit").asText())));
        for (String field : List.of("to", "data", "chainId")) {
            ObjectNode missing = wire.deepCopy();
            missing.remove(field);
            assertThrows(DatabindException.class,
                () -> JSON.treeToValue(missing, GeneratedSchema.AgentModels.NativeProgramCallFields.class));
        }
        ObjectNode wrongTarget = wire.deepCopy().put("to", "0x000000000000000000000000000000000000101a");
        assertThrows(DatabindException.class,
            () -> JSON.treeToValue(wrongTarget, GeneratedSchema.AgentModels.NativeProgramCallFields.class));
        for (Class<?> type : List.of(GeneratedSchema.AgentModels.NativeProgramCallRequest.class,
                GeneratedSchema.AgentModels.NativeProgramDeployRequest.class,
                GeneratedSchema.AgentModels.NativeProgramUpgradeRequest.class,
                GeneratedSchema.AgentModels.NativeProgramWindDownRequest.class)) {
            assertEquals(Set.of("native_call", "signed_transaction"), components(type));
            assertEquals(GeneratedSchema.AgentModels.NativeProgramCallFields.class,
                type.getMethod("native_call").getReturnType());
            ObjectNode kernel = JSON.createObjectNode().put("signed_activity",
                vector.required("signed_activity_hex").asText());
            assertThrows(DatabindException.class, () -> JSON.treeToValue(kernel, type));
        }
    }

    private static Set<String> components(Class<?> type) {
        return Arrays.stream(type.getRecordComponents()).map(RecordComponent::getName).collect(Collectors.toSet());
    }

    private static JsonNode fixture(String name) throws Exception {
        Path root = Path.of("").toAbsolutePath();
        while (root != null) {
            Path path = root.resolve("sdk/platform/conformance/fixtures/" + name);
            if (Files.isRegularFile(path)) return JSON.readTree(Files.readString(path));
            root = root.getParent();
        }
        throw new IllegalStateException("fixture not found: " + name);
    }
}
