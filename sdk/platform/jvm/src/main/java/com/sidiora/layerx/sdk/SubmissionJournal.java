package com.sidiora.layerx.sdk;

import com.sidiora.layerx.sdk.verify.PortableReceipt;
import java.io.DataInputStream;
import java.io.DataOutputStream;
import java.io.IOException;
import java.nio.channels.FileChannel;
import java.nio.channels.FileLock;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.StandardCopyOption;
import java.nio.file.StandardOpenOption;
import java.util.HexFormat;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.Objects;

/**
 * Caller-owned, bounded signed-transaction retention. Unresolved and merely receipted entries
 * are never evicted. Admission fails before a broadcast when either budget is exhausted.
 * Export signedTransaction(hash) to caller storage or reopen a durable journal after restart.
 * A journal has one owner at a time; sharing the same instance between clients is supported.
 * Durable writes use a forced temporary file, atomic replacement and directory fsync. A storage
 * error poisons the handle: close and reopen it to resolve any uncertain disk write.
 */
public final class SubmissionJournal implements AutoCloseable {
    private static final int MAGIC = 0x4c585331;
    private static final int MAX_TRANSACTION_BYTES = ProgramsClient.MAX_CALLDATA_BYTES + 65536;
    private record Entry(String sender, String nonce, byte[] signed) {}
    private final int maximumEntries;
    private final long maximumBytes;
    private final Path directory;
    private final FileChannel lockChannel;
    private final FileLock lock;
    private Map<String, Entry> entries = new LinkedHashMap<>();
    private long retainedBytes;
    private boolean closed;
    private boolean broken;

    private SubmissionJournal(int maximumEntries, long maximumBytes, Path directory) throws IOException {
        if (maximumEntries < 1 || maximumEntries > 65536 || maximumBytes < 1 || maximumBytes > 64L * 1024 * 1024)
            throw PlatformSdkException.invalidArgument();
        this.maximumEntries = maximumEntries;
        this.maximumBytes = maximumBytes;
        this.directory = directory;
        if (directory == null) { lockChannel = null; lock = null; return; }
        Files.createDirectories(directory);
        lockChannel = FileChannel.open(directory.resolve("owner.lock"), StandardOpenOption.CREATE, StandardOpenOption.WRITE);
        FileLock acquired = null;
        try {
            acquired = lockChannel.tryLock();
            if (acquired == null) throw new IOException("Submission journal is already open");
            lock = acquired;
            load();
        } catch (IOException | RuntimeException error) {
            if (acquired != null) acquired.release();
            lockChannel.close();
            throw error;
        }
    }

    public static SubmissionJournal memory(int maximumEntries, long maximumBytes) {
        try { return new SubmissionJournal(maximumEntries, maximumBytes, null); }
        catch (IOException error) { throw new IllegalStateException(error); }
    }

    /** Reopen with the same budgets; the directory and its backing filesystem must survive restart. */
    public static SubmissionJournal open(Path directory, int maximumEntries, long maximumBytes) throws IOException {
        return new SubmissionJournal(maximumEntries, maximumBytes, Objects.requireNonNull(directory, "directory").toAbsolutePath());
    }

    /** The producer validates signed intent and sender before storing; this journal never broadcasts. */
    public synchronized void retain(String sender, String nonce, byte[] signedTransaction) {
        ensureOpen();
        if (sender == null || !sender.matches("0x[0-9a-f]{40}")
                || nonce == null || !nonce.matches("0x(?:0|[1-9a-f][0-9a-f]{0,15})")
                || signedTransaction == null || signedTransaction.length == 0
                || signedTransaction.length > MAX_TRANSACTION_BYTES) throw PlatformSdkException.invalidArgument();
        String hash = hash(signedTransaction);
        Entry prior = entries.get(hash);
        if (prior != null) {
            if (!prior.sender().equals(sender) || !prior.nonce().equals(nonce)) throw PlatformSdkException.invalidArgument();
            return;
        }
        for (Entry entry : entries.values())
            if (entry.sender().equals(sender) && entry.nonce().equals(nonce))
                throw new PlatformSdkException(PlatformSdkException.Code.IDEMPOTENCY_CONFLICT,
                    PlatformSdkException.Retry.NEVER, null, null, null);
        if (entries.size() == maximumEntries || signedTransaction.length > maximumBytes - retainedBytes)
            throw new PlatformSdkException(PlatformSdkException.Code.BUDGET_REFUSAL,
                PlatformSdkException.Retry.NEVER, null, null, null);
        var next = new LinkedHashMap<>(entries);
        next.put(hash, new Entry(sender, nonce, signedTransaction.clone()));
        replace(next, retainedBytes + signedTransaction.length);
    }

    public synchronized byte[] signedTransaction(byte[] transactionHash) {
        ensureOpen();
        if (transactionHash == null || transactionHash.length != 32) throw PlatformSdkException.invalidArgument();
        Entry entry = entries.get(HexFormat.of().formatHex(transactionHash));
        return entry == null ? null : entry.signed().clone();
    }

    /**
     * Explicit retention policy: release only after PortableReceipt verified inclusion, signatures
     * and caller-pinned consensus trust. RPC committed metadata alone is insufficient. Archive
     * PortableReceipt.export(proof) first if later offline lookups are required after release.
     */
    public synchronized boolean releaseCommitted(PortableReceipt.Verification proof) {
        ensureOpen();
        if (proof == null || proof.committed() == null) throw PlatformSdkException.verificationFailure();
        String hash;
        try {
            // Use the immutable verified bundle, not mutable array accessors on receipt records.
            String encoded = new com.fasterxml.jackson.databind.ObjectMapper()
                .readTree(PortableReceipt.export(proof)).required("rawTransaction").textValue();
            hash = hash(HexFormat.of().parseHex(encoded.substring(2)));
        } catch (IOException error) { throw PlatformSdkException.verificationFailure(); }
        Entry prior = entries.get(hash);
        if (prior == null) return false;
        var next = new LinkedHashMap<>(entries);
        next.remove(hash);
        replace(next, retainedBytes - prior.signed().length);
        return true;
    }

    public synchronized int size() { ensureOpen(); return entries.size(); }
    public synchronized long retainedBytes() { ensureOpen(); return retainedBytes; }

    private void replace(Map<String, Entry> next, long bytes) {
        if (directory != null) {
            Path temporary = null;
            try {
                temporary = Files.createTempFile(directory, "journal-", ".tmp");
                try (var output = new DataOutputStream(Files.newOutputStream(temporary))) {
                    output.writeInt(MAGIC); output.writeInt(next.size());
                    for (var item : next.entrySet()) {
                        Entry entry = item.getValue();
                        output.writeUTF(item.getKey()); output.writeUTF(entry.sender()); output.writeUTF(entry.nonce());
                        output.writeInt(entry.signed().length); output.write(entry.signed());
                    }
                }
                try (var file = FileChannel.open(temporary, StandardOpenOption.WRITE)) { file.force(true); }
                Files.move(temporary, directory.resolve("journal.bin"), StandardCopyOption.ATOMIC_MOVE, StandardCopyOption.REPLACE_EXISTING);
                try (var parent = FileChannel.open(directory, StandardOpenOption.READ)) { parent.force(true); }
            } catch (IOException error) {
                broken = true;
                throw new PlatformSdkException(PlatformSdkException.Code.INTERNAL_FAULT,
                    PlatformSdkException.Retry.NEVER, null, null, null);
            } finally {
                if (temporary != null) {
                    try { Files.deleteIfExists(temporary); }
                    catch (IOException error) {
                        broken = true;
                        throw new PlatformSdkException(PlatformSdkException.Code.INTERNAL_FAULT,
                            PlatformSdkException.Retry.NEVER, null, null, null);
                    }
                }
            }
        }
        entries = next; retainedBytes = bytes;
    }

    private void load() throws IOException {
        Path file = directory.resolve("journal.bin");
        if (!Files.exists(file)) return;
        if (Files.size(file) > maximumBytes + maximumEntries * 256L + 8) throw new IOException("Submission journal exceeds budget");
        try (var input = new DataInputStream(Files.newInputStream(file))) {
            if (input.readInt() != MAGIC) throw new IOException("Invalid submission journal version");
            int count = input.readInt();
            if (count < 0 || count > maximumEntries) throw new IOException("Submission journal exceeds entry budget");
            var nonces = new java.util.HashSet<String>();
            for (int i = 0; i < count; i++) {
                String hash = input.readUTF(), sender = input.readUTF(), nonce = input.readUTF();
                int length = input.readInt();
                if (!hash.matches("[0-9a-f]{64}") || !sender.matches("0x[0-9a-f]{40}")
                        || !nonce.matches("0x(?:0|[1-9a-f][0-9a-f]{0,15})")
                        || length < 1 || length > MAX_TRANSACTION_BYTES || length > maximumBytes - retainedBytes)
                    throw new IOException("Invalid submission journal entry");
                byte[] raw = input.readNBytes(length);
                if (raw.length != length || !hash(raw).equals(hash) || entries.containsKey(hash)
                        || !nonces.add(sender + ":" + nonce)) throw new IOException("Invalid submission journal evidence");
                entries.put(hash, new Entry(sender, nonce, raw)); retainedBytes += length;
            }
            if (input.read() != -1) throw new IOException("Trailing submission journal bytes");
        }
    }

    private static String hash(byte[] raw) { return HexFormat.of().formatHex(ProgramsClient.Programs.transactionHash(raw)); }
    private void ensureOpen() {
        if (closed || broken) throw new IllegalStateException("Submission journal must be reopened");
    }
    @Override public synchronized void close() throws IOException {
        if (closed) return;
        closed = true;
        entries.clear(); retainedBytes = 0;
        if (lock != null) lock.release();
        if (lockChannel != null) lockChannel.close();
    }
}
