package com.sidiora.layerx.sdk;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

final class TerminalV4Test {
    private static byte[] bytes(JsonNode node, String name) { return HexFormat.of().parseHex(node.get(name).asText()); }

    @Test void nativeAccountAuthorizationVectors() throws Exception {
        JsonNode vectors = new ObjectMapper().readTree(Files.readString(Path.of(System.getProperty("layerx.repo.root", "../../.."), "programs/fixtures/pay5/account-authorization-vectors.json")));
        for (JsonNode vector : vectors) {
            byte[] encoded = bytes(vector, "encoded"), root = bytes(vector, "root");
            if (vector.get("accept").asBoolean()) assertDoesNotThrow(() -> ProgramsClient.verifyAuthorizationRoot(encoded, root), vector.get("name").asText());
            else assertThrows(IllegalArgumentException.class, () -> ProgramsClient.verifyAuthorizationRoot(encoded, root), vector.get("name").asText());
        }
    }

}
