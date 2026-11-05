package com.sidiora.layerx.spring;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.PosixFilePermissions;
import java.util.Arrays;
import java.util.Base64;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.AnnotatedElementContext;
import org.junit.jupiter.api.extension.ExtensionContext;
import org.junit.jupiter.api.io.TempDir;
import org.junit.jupiter.api.io.TempDirFactory;
import org.springframework.boot.SpringBootVersion;
import org.springframework.boot.autoconfigure.AutoConfiguration;
import org.springframework.boot.autoconfigure.EnableAutoConfiguration;
import org.springframework.boot.test.util.TestPropertyValues;
import org.springframework.context.annotation.AnnotationConfigApplicationContext;
import org.springframework.context.annotation.Configuration;

import static org.junit.jupiter.api.Assertions.*;

public final class Repair2231452Test {
    private static final String GHSA_WWPQ_F5C3_7HVX_MINIMUM = "4.0.6";
    private static final String GHSA_GGG2_9786_HWC8_MINIMUM = "4.0.7";

    @TempDir(factory = OwnerStorage.class)
    Path storageDirectory;

    public static final class OwnerStorage implements TempDirFactory {
        @Override
        public Path createTempDirectory(AnnotatedElementContext element, ExtensionContext context)
                throws IOException {
            Path target = Files.createDirectories(Path.of("target").toAbsolutePath()).toRealPath();
            return Files.createTempDirectory(target, "spring-boot-repair-",
                PosixFilePermissions.asFileAttribute(PosixFilePermissions.fromString("rwx------")));
        }
    }

    @Test
    void springBootResolvesAboveItsAdvisoryMinimum() {
        assertAtLeast(SpringBootVersion.getVersion(), GHSA_WWPQ_F5C3_7HVX_MINIMUM);
    }

    @Test
    void autoConfigureResolvesAboveItsAdvisoryMinimumAndMatchesBoot() {
        String version = AutoConfiguration.class.getPackage().getImplementationVersion();
        assertAtLeast(version, GHSA_GGG2_9786_HWC8_MINIMUM);
        assertEquals(SpringBootVersion.getVersion(), version);
    }

    @Test
    void springDiscoversAndLoadsTheIntegrationAutoConfiguration() {
        try (AnnotationConfigApplicationContext context = new AnnotationConfigApplicationContext()) {
            TestPropertyValues.of(
                "layerx.storage-directory=" + storageDirectory,
                "layerx.principal=merchant",
                "layerx.protected-path=/paid",
                "layerx.resource.url=https://merchant.example/paid",
                "layerx.payment.scheme=exact",
                "layerx.payment.network=eip155:125",
                "layerx.payment.price=100",
                "layerx.payment.asset=0x" + "55".repeat(32),
                "layerx.payment.pay-to=0x" + "66".repeat(32),
                "layerx.payment.timeout-seconds=120",
                "layerx.receipt-keys.entries[0].key-id=0x" + "11".repeat(32),
                "layerx.receipt-keys.entries[0].public-key=0x" + "22".repeat(32),
                "layerx.receipt-keys.entries[0].first-height=1",
                "layerx.receipt-keys.entries[0].last-height=100",
                "layerx.webhook.path=/layerx/webhooks",
                "layerx.webhook.public-keys.primary="
                    + Base64.getEncoder().encodeToString(new byte[32]))
                .applyTo(context);
            context.register(TestApplication.class);
            context.refresh();

            assertTrue(context.isActive());
            assertNotNull(context.getBean(LayerXAutoConfiguration.class));
            assertEquals("merchant", context.getBean(LayerXProperties.class).getPrincipal());
            LayerXDeclaredConfig config = context.getBean(LayerXDeclaredConfig.class);
            assertEquals("eip155:125", config.requirements().network());
            assertEquals(1, config.receiptKeys().entries().size());
            assertInstanceOf(SellerMiddleware.ReceiptPayloadAuthority.class,
                context.getBean(SellerMiddleware.PaymentAuthority.class));
            assertNotNull(context.getBean(SellerMiddleware.class));
            assertNotNull(context.getBean(Fulfillments.FulfillmentRepository.class));
            assertNotNull(context.getBean(Webhooks.DeliveryStore.class));
            assertNotNull(context.getBean(Webhooks.VerifiedWebhookConsumer.class));
            assertTrue(Files.isDirectory(storageDirectory.resolve("fulfillments")));
            assertTrue(Files.isDirectory(storageDirectory.resolve("webhooks")));
        }
    }

    @Configuration(proxyBeanMethods = false)
    @EnableAutoConfiguration
    static class TestApplication {}

    private static void assertAtLeast(String actual, String minimum) {
        assertNotNull(actual, "Spring Boot artifacts must expose their resolved runtime version");
        assertTrue(actual.matches("[0-9]+\\.[0-9]+\\.[0-9]+"),
            "Expected a stable Spring Boot release, got " + actual);
        int[] resolved = Arrays.stream(actual.split("\\.")).mapToInt(Integer::parseInt).toArray();
        int[] fixed = Arrays.stream(minimum.split("\\.")).mapToInt(Integer::parseInt).toArray();
        assertTrue(Arrays.compare(resolved, fixed) >= 0,
            "Resolved " + actual + " must be at or above advisory minimum " + minimum);
    }
}
