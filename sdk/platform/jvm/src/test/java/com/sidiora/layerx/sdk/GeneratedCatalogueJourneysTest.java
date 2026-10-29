package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.DatabindException;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.lang.reflect.Modifier;
import java.util.Arrays;
import java.util.List;
import java.util.Set;
import java.util.stream.Collectors;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.*;

public final class GeneratedCatalogueJourneysTest {
    @Test
    void humanOperationsHaveNoRetiredJourneys() throws Exception {
        List<String> retiredPrefixes = List.of("binding.", "deposit.", "exit.", "withdraw.");
        assertFalse(GeneratedSchema.HUMAN.isEmpty());
        for (var entry : GeneratedSchema.HUMAN.entrySet()) {
            assertTrue(retiredPrefixes.stream().noneMatch(entry.getKey()::startsWith), entry.getKey());
            assertEquals(entry.getKey(), entry.getValue().wireName());
            assertEquals(OperationCatalog.Plane.HUMAN, entry.getValue().plane());
        }
        for (var field : GeneratedSchema.HumanOperations.class.getDeclaredFields()) {
            if (Modifier.isStatic(field.getModifiers())
                    && SchemaTypes.TypedOperation.class.isAssignableFrom(field.getType())) {
                var operation = (SchemaTypes.TypedOperation<?, ?>) field.get(null);
                assertTrue(retiredPrefixes.stream().noneMatch(operation.wireName()::startsWith), field.getName());
                assertSame(operation, GeneratedSchema.HUMAN.get(operation.wireName()));
            }
        }
        assertSame(GeneratedSchema.HumanOperations.JOURNEY_GET, GeneratedSchema.HUMAN.get("journey.get"));
        assertSame(GeneratedSchema.HumanOperations.MOVE_COMMIT, GeneratedSchema.HUMAN.get("move.commit"));
        for (Class<?> container : List.of(GeneratedSchema.HumanModels.class,
                GeneratedSchema.HumanOperations.class)) {
            for (Class<?> type : container.getDeclaredClasses()) {
                assertFalse(List.of("Binding", "Deposit", "Exit", "Withdraw", "Rebinding", "WalletBinding")
                    .stream().anyMatch(type.getSimpleName()::startsWith), type.getName());
            }
        }

        ObjectMapper json = new ObjectMapper();
        assertEquals(Set.of("onboarding", "move", "agent-create", "agent-fund", "agent-pause", "agent-retire"),
            Arrays.stream(GeneratedSchema.HumanModels.JourneyKind.values())
                .map(GeneratedSchema.HumanModels.JourneyKind::wire).collect(Collectors.toSet()));
        for (var kind : GeneratedSchema.HumanModels.JourneyKind.values()) {
            assertSame(kind, GeneratedSchema.HumanModels.JourneyKind.fromWire(kind.wire()));
            assertEquals(kind, json.readValue(json.writeValueAsBytes(kind),
                GeneratedSchema.HumanModels.JourneyKind.class));
        }
        for (String retired : List.of("wallet-binding", "deposit", "exit", "withdraw")) {
            assertThrows(IllegalArgumentException.class,
                () -> GeneratedSchema.HumanModels.JourneyKind.fromWire(retired));
            assertThrows(DatabindException.class, () -> json.readValue(json.writeValueAsBytes(retired),
                GeneratedSchema.HumanModels.JourneyKind.class));
        }

        Set<String> errorCodes = Arrays.stream(SchemaErrors.HumanCode.values())
            .map(SchemaErrors.HumanCode::wire).collect(Collectors.toSet());
        assertEquals(Arrays.stream(GeneratedSchema.HumanModels.ErrorCode.values())
            .map(GeneratedSchema.HumanModels.ErrorCode::wire).collect(Collectors.toSet()), errorCodes);
        for (var code : SchemaErrors.HumanCode.values()) {
            assertSame(code, SchemaErrors.HumanCode.fromWire(code.wire()));
        }
        for (String retired : List.of("wallet-not-bound", "exit-unavailable")) {
            assertFalse(errorCodes.contains(retired));
            assertThrows(IllegalArgumentException.class, () -> SchemaErrors.HumanCode.fromWire(retired));
            assertThrows(IllegalArgumentException.class,
                () -> GeneratedSchema.HumanModels.ErrorCode.fromWire(retired));
        }
        assertEquals(Set.of("unverified", "receipt-verified", "block-committed"),
            Arrays.stream(GeneratedSchema.HumanModels.HumanVerificationLevel.values())
                .map(GeneratedSchema.HumanModels.HumanVerificationLevel::wire).collect(Collectors.toSet()));
        assertThrows(IllegalArgumentException.class,
            () -> GeneratedSchema.HumanModels.HumanVerificationLevel.fromWire("settlement-anchored"));
        assertThrows(DatabindException.class, () -> json.readValue("\"settlement-anchored\"",
            GeneratedSchema.HumanModels.HumanVerificationLevel.class));
    }
}
