package com.sidiora.layerx.spring;

import com.sidiora.layerx.sdk.verify.LocalVerifier;
import com.sidiora.layerx.sdk.PlatformSdkException;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.regex.Pattern;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "layerx")
public class LayerXProperties {
    private static final Pattern KEY_ID = Pattern.compile("[A-Za-z0-9._-]{1,64}");
    private static final Pattern PATH_REJECT = Pattern.compile(".*[\\s?#].*");

    private String principal;
    private String protectedPath;
    private String storageDirectory;
    private final Resource resource = new Resource();
    private final Payment payment = new Payment();
    private final ReceiptKeys receiptKeys = new ReceiptKeys();
    private final Webhook webhook = new Webhook();

    public String getPrincipal() { return principal; }

    public void setPrincipal(String principal) { this.principal = principal; }

    public String getProtectedPath() { return protectedPath; }

    public void setProtectedPath(String protectedPath) { this.protectedPath = protectedPath; }

    public String getStorageDirectory() { return storageDirectory; }

    public void setStorageDirectory(String storageDirectory) { this.storageDirectory = storageDirectory; }

    public Resource getResource() { return resource; }

    public Payment getPayment() { return payment; }

    public ReceiptKeys getReceiptKeys() { return receiptKeys; }

    public Webhook getWebhook() { return webhook; }

    public static class Resource {
        private String url;
        private String description;
        private String mimeType;
        private String serviceName;

        public String getUrl() { return url; }

        public void setUrl(String url) { this.url = url; }

        public String getDescription() { return description; }

        public void setDescription(String description) { this.description = description; }

        public String getMimeType() { return mimeType; }

        public void setMimeType(String mimeType) { this.mimeType = mimeType; }

        public String getServiceName() { return serviceName; }

        public void setServiceName(String serviceName) { this.serviceName = serviceName; }
    }

    public static class Payment {
        private String scheme;
        private String network;
        private String price;
        private String asset;
        private String payTo;
        private long timeoutSeconds;

        public String getScheme() { return scheme; }

        public void setScheme(String scheme) { this.scheme = scheme; }

        public String getNetwork() { return network; }

        public void setNetwork(String network) { this.network = network; }

        public String getPrice() { return price; }

        public void setPrice(String price) { this.price = price; }

        public String getAsset() { return asset; }

        public void setAsset(String asset) { this.asset = asset; }

        public String getPayTo() { return payTo; }

        public void setPayTo(String payTo) { this.payTo = payTo; }

        public long getTimeoutSeconds() { return timeoutSeconds; }

        public void setTimeoutSeconds(long timeoutSeconds) { this.timeoutSeconds = timeoutSeconds; }
    }

    public static class ReceiptKeys {
        private final List<ReceiptKey> entries = new ArrayList<>();

        public List<ReceiptKey> getEntries() { return entries; }
    }

    public static class ReceiptKey {
        private String keyId;
        private String publicKey;
        private String firstHeight;
        private String lastHeight;

        public String getKeyId() { return keyId; }

        public void setKeyId(String keyId) { this.keyId = keyId; }

        public String getPublicKey() { return publicKey; }

        public void setPublicKey(String publicKey) { this.publicKey = publicKey; }

        public String getFirstHeight() { return firstHeight; }

        public void setFirstHeight(String firstHeight) { this.firstHeight = firstHeight; }

        public String getLastHeight() { return lastHeight; }

        public void setLastHeight(String lastHeight) { this.lastHeight = lastHeight; }
    }

    public static class Webhook {
        private String path;
        private final Map<String, String> publicKeys = new LinkedHashMap<>();
        private long maximumAgeMs = Webhooks.DEFAULT_MAXIMUM_AGE_MS;
        private long leaseMs = Webhooks.DEFAULT_LEASE_MS;

        public String getPath() { return path; }

        public void setPath(String path) { this.path = path; }

        public Map<String, String> getPublicKeys() { return publicKeys; }

        public long getMaximumAgeMs() { return maximumAgeMs; }

        public void setMaximumAgeMs(long maximumAgeMs) { this.maximumAgeMs = maximumAgeMs; }

        public long getLeaseMs() { return leaseMs; }

        public void setLeaseMs(long leaseMs) { this.leaseMs = leaseMs; }
    }

    public LayerXDeclaredConfig toDeclaredConfig() {
        X402.PaymentRequirements requirements = new X402.PaymentRequirements(
            required(payment.getScheme()),
            required(payment.getNetwork()),
            required(payment.getPrice()),
            required(payment.getAsset()),
            required(payment.getPayTo()),
            positive(payment.getTimeoutSeconds()),
            null);
        X402.ResourceInfo info = new X402.ResourceInfo(
            required(resource.getUrl()),
            optional(resource.getDescription()),
            optional(resource.getMimeType()),
            optional(resource.getServiceName()),
            null,
            null);
        X402.PaymentRequired paymentRequired =
            X402.parsePaymentRequired(new X402.PaymentRequired(info, List.of(requirements), null, null).toNode());
        Map<String, byte[]> keys = new LinkedHashMap<>();
        for (Map.Entry<String, String> entry : webhook.getPublicKeys().entrySet()) {
            if (!KEY_ID.matcher(entry.getKey()).matches()) invalid();
            byte[] key = X402.decodeBase64(required(entry.getValue()),
                MiddlewareException.Code.INVALID_DECLARED_KEY);
            if (key.length != 32) invalid();
            keys.put(entry.getKey(), key);
        }
        if (keys.isEmpty() || keys.size() > 32) invalid();
        if (webhook.getMaximumAgeMs() <= 0 || webhook.getLeaseMs() <= 0) invalid();
        return new LayerXDeclaredConfig(
            required(principal),
            mountPath(required(protectedPath)),
            paymentRequired,
            paymentRequired.accepts().get(0),
            declaredReceiptKeys(),
            mountPath(required(webhook.getPath())),
            keys,
            webhook.getMaximumAgeMs(),
            webhook.getLeaseMs());
    }

    private LocalVerifier.ReceiptKeys declaredReceiptKeys() {
        try {
            List<LocalVerifier.ReceiptKey> entries = new ArrayList<>();
            for (ReceiptKey entry : receiptKeys.getEntries()) {
                if (entry == null || entry.getKeyId() == null || entry.getKeyId().isEmpty()
                        || entry.getPublicKey() == null || entry.getPublicKey().isEmpty()) invalid();
                entries.add(new LocalVerifier.ReceiptKey(hex32(entry.getKeyId()), hex32(entry.getPublicKey()),
                    unsignedHeight(entry.getFirstHeight()), unsignedHeight(entry.getLastHeight())));
            }
            return new LocalVerifier.ReceiptKeys(entries);
        } catch (PlatformSdkException | IllegalArgumentException error) {
            throw MiddlewareException.of(MiddlewareException.Code.INVALID_DECLARED_KEY);
        }
    }

    private static BigInteger unsignedHeight(String value) {
        if (value == null || !value.matches("0|[1-9][0-9]{0,19}")) invalid();
        BigInteger height = new BigInteger(value);
        if (height.bitLength() > 64) invalid();
        return height;
    }

    private static String required(String value) {
        if (value == null || value.isEmpty()) {
            throw MiddlewareException.of(MiddlewareException.Code.MISSING_DECLARED_KEY);
        }
        return value;
    }

    private static String optional(String value) {
        return value == null || value.isEmpty() ? null : value;
    }

    private static long positive(long value) {
        if (value <= 0 || value > 0xffff_ffffL) invalid();
        return value;
    }

    private static String mountPath(String value) {
        if (!value.startsWith("/") || value.length() > 512 || PATH_REJECT.matcher(value).matches()) invalid();
        return value;
    }

    private static byte[] hex32(String value) {
        return X402.parseHex32(required(value), MiddlewareException.Code.INVALID_DECLARED_KEY);
    }

    private static void invalid() {
        throw MiddlewareException.of(MiddlewareException.Code.INVALID_DECLARED_KEY);
    }
}
