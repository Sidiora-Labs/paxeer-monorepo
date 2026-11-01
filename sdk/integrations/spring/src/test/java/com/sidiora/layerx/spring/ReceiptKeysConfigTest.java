package com.sidiora.layerx.spring;

import com.sidiora.layerx.sdk.verify.LocalVerifier.ReceiptKeys;
import java.lang.reflect.Field;
import java.math.BigInteger;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.Base64;
import java.util.HexFormat;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.io.TempDir;
import org.springframework.boot.context.properties.bind.Binder;
import org.springframework.boot.test.util.TestPropertyValues;
import org.springframework.context.annotation.AnnotationConfigApplicationContext;
import org.springframework.core.env.StandardEnvironment;
import static org.junit.jupiter.api.Assertions.*;

public final class ReceiptKeysConfigTest {
    private static final String KEY_ID = "0x" + "11".repeat(32);
    private static final String PUBLIC_KEY = "0x" + "22".repeat(32);
    private static final String NEXT_KEY_ID = "0x" + "33".repeat(32);
    private static final String NEXT_PUBLIC_KEY = "0x" + "44".repeat(32);
    private static final String MAX_HEIGHT = "18446744073709551615";

    @TempDir
    Path storageDirectory;

    private StandardEnvironment environment() {
        StandardEnvironment environment = new StandardEnvironment();
        TestPropertyValues.of(
            "layerx.storage-directory=" + storageDirectory,
            "layerx.principal=merchant",
            "layerx.protected-path=/paid",
            "layerx.resource.url=https://merchant.example/paid",
            "layerx.payment.scheme=exact",
            "layerx.payment.network=layerx:mainnet",
            "layerx.payment.price=100",
            "layerx.payment.asset=0x" + "55".repeat(32),
            "layerx.payment.pay-to=0x" + "66".repeat(32),
            "layerx.payment.timeout-seconds=120",
            "layerx.receipt-keys.entries[0].key-id=" + KEY_ID,
            "layerx.receipt-keys.entries[0].public-key=" + PUBLIC_KEY,
            "layerx.receipt-keys.entries[0].first-height=1",
            "layerx.receipt-keys.entries[0].last-height=100",
            "layerx.receipt-keys.entries[1].key-id=" + NEXT_KEY_ID,
            "layerx.receipt-keys.entries[1].public-key=" + NEXT_PUBLIC_KEY,
            "layerx.receipt-keys.entries[1].first-height=101",
            "layerx.receipt-keys.entries[1].last-height=" + MAX_HEIGHT,
            "layerx.webhook.path=/layerx/webhooks",
            "layerx.webhook.public-keys.primary="
                + Base64.getEncoder().encodeToString(HexFormat.of().parseHex("77".repeat(32))))
            .applyTo(environment);
        return environment;
    }

    @Test
    void declaredConfigBindsReceiptKeys() {
        StandardEnvironment environment = environment();
        LayerXProperties properties = Binder.get(environment).bind("layerx", LayerXProperties.class).get();
        LayerXDeclaredConfig config = new LayerXAutoConfiguration()
            .layerXDeclaredConfig(properties, environment);

        assertEquals(ReceiptKeys.class, Arrays.stream(LayerXDeclaredConfig.class.getRecordComponents())
            .filter(component -> component.getName().equals("receiptKeys"))
            .findFirst().orElseThrow().getType());
        assertFalse(Arrays.stream(LayerXDeclaredConfig.class.getRecordComponents())
            .anyMatch(component -> component.getName().equals("authorizedBatch")));
        assertPinnedKeys(config.receiptKeys());
        for (String invalid : new String[] {
                "key-id=", "key-id=0x" + "00".repeat(32), "public-key=not-a-key",
                "first-height=-1", "first-height=+1", "first-height=101",
                "last-height=18446744073709551616", "last-height=1.5"}) {
            StandardEnvironment invalidEnvironment = environment();
            TestPropertyValues.of("layerx.receipt-keys.entries[0]." + invalid).applyTo(invalidEnvironment);
            LayerXProperties invalidProperties = Binder.get(invalidEnvironment)
                .bind("layerx", LayerXProperties.class).get();
            assertEquals(MiddlewareException.Code.INVALID_DECLARED_KEY,
                assertThrows(MiddlewareException.class, invalidProperties::toDeclaredConfig).code());
        }
        properties.getReceiptKeys().getEntries().clear();
        assertEquals(MiddlewareException.Code.INVALID_DECLARED_KEY,
            assertThrows(MiddlewareException.class, properties::toDeclaredConfig).code());
    }

    @Test
    void autoConfigurationBuildsAuthorityFromReceiptKeys() throws Exception {
        try (AnnotationConfigApplicationContext context = new AnnotationConfigApplicationContext()) {
            context.setEnvironment(environment());
            context.register(LayerXAutoConfiguration.class);
            context.refresh();

            LayerXDeclaredConfig config = context.getBean(LayerXDeclaredConfig.class);
            SellerMiddleware.PaymentAuthority authority = context.getBean(SellerMiddleware.PaymentAuthority.class);
            assertInstanceOf(SellerMiddleware.ReceiptPayloadAuthority.class, authority);
            assertFalse(context.containsBean("layerXAuthorizedBatchResolver"));
            assertFalse(Arrays.stream(LayerXAutoConfiguration.class.getDeclaredMethods())
                .anyMatch(method -> method.getName().equals("layerXAuthorizedBatchResolver")));
            Field keys = Arrays.stream(authority.getClass().getDeclaredFields())
                .filter(field -> field.getType().equals(ReceiptKeys.class))
                .findFirst().orElseThrow();
            keys.setAccessible(true);
            assertSame(config.receiptKeys(), keys.get(authority));
            assertPinnedKeys(config.receiptKeys());
        }
    }

    private static void assertPinnedKeys(ReceiptKeys keys) {
        assertNotNull(keys);
        assertEquals(2, keys.entries().size());
        var first = keys.entries().get(0);
        assertArrayEquals(HexFormat.of().parseHex(KEY_ID.substring(2)), first.keyId());
        assertArrayEquals(HexFormat.of().parseHex(PUBLIC_KEY.substring(2)), first.publicKey());
        assertEquals(BigInteger.ONE, first.firstHeight());
        assertEquals(BigInteger.valueOf(100), first.lastHeight());
        var next = keys.entries().get(1);
        assertArrayEquals(HexFormat.of().parseHex(NEXT_KEY_ID.substring(2)), next.keyId());
        assertArrayEquals(HexFormat.of().parseHex(NEXT_PUBLIC_KEY.substring(2)), next.publicKey());
        assertEquals(BigInteger.valueOf(101), next.firstHeight());
        assertEquals(new BigInteger(MAX_HEIGHT), next.lastHeight());
    }
}
