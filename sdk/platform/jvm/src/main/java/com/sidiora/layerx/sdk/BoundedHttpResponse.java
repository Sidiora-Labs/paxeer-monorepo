package com.sidiora.layerx.sdk;

import java.io.ByteArrayOutputStream;
import java.io.IOException;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.net.http.HttpTimeoutException;
import java.nio.ByteBuffer;
import java.time.Duration;
import java.util.List;
import java.util.concurrent.CompletableFuture;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.Flow;
import java.util.concurrent.ScheduledThreadPoolExecutor;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;

final class BoundedHttpResponse {
    private static final ScheduledThreadPoolExecutor DEADLINES = new ScheduledThreadPoolExecutor(1, task -> {
        Thread thread = new Thread(task, "layerx-http-deadline");
        thread.setDaemon(true);
        return thread;
    });
    static { DEADLINES.setRemoveOnCancelPolicy(true); }

    private BoundedHttpResponse() {}

    static final class LimitExceeded extends IOException {
        LimitExceeded() { super("HTTP response exceeds byte limit"); }
    }

    static CompletableFuture<HttpResponse<byte[]>> send(HttpClient client, HttpRequest request,
                                                        int maximum, Duration timeout) {
        CompletableFuture<HttpResponse<byte[]>> result = new CompletableFuture<>();
        AtomicReference<Subscriber> subscriber = new AtomicReference<>();
        AtomicReference<CompletableFuture<HttpResponse<byte[]>>> pending = new AtomicReference<>();
        var deadline = DEADLINES.schedule(() -> result.completeExceptionally(
            new HttpTimeoutException("HTTP response deadline exceeded")), timeout.toNanos(), TimeUnit.NANOSECONDS);
        result.whenComplete((response, failure) -> {
            deadline.cancel(false);
            if (failure != null || result.isCancelled()) {
                Subscriber body = subscriber.get();
                if (body != null) body.abort(failure);
                var exchange = pending.get();
                if (exchange != null) exchange.cancel(true);
            }
        });
        try {
            var exchange = client.sendAsync(request, info -> {
                Subscriber body = new Subscriber(maximum);
                subscriber.set(body);
                if (result.isDone()) body.abort(new HttpTimeoutException("HTTP response deadline exceeded"));
                else if (info.headers().firstValueAsLong("Content-Length").orElse(0) > maximum)
                    body.abort(new LimitExceeded());
                return body;
            });
            pending.set(exchange);
            if (result.isDone()) exchange.cancel(true);
            exchange.whenComplete((response, failure) -> {
                if (failure == null) result.complete(response);
                else result.completeExceptionally(failure);
            });
        } catch (RuntimeException failure) {
            result.completeExceptionally(failure);
        }
        return result;
    }

    private static final class Subscriber implements HttpResponse.BodySubscriber<byte[]> {
        private final int maximum;
        private final CompletableFuture<byte[]> body = new CompletableFuture<>();
        private ByteArrayOutputStream bytes = new ByteArrayOutputStream();
        private Flow.Subscription subscription;

        Subscriber(int maximum) { this.maximum = maximum; }
        @Override public CompletionStage<byte[]> getBody() { return body; }
        @Override public synchronized void onSubscribe(Flow.Subscription value) {
            if (subscription != null || body.isDone()) { value.cancel(); return; }
            subscription = value;
            value.request(1);
        }
        @Override public synchronized void onNext(List<ByteBuffer> buffers) {
            if (body.isDone()) return;
            long size = bytes.size();
            for (ByteBuffer buffer : buffers) {
                size += buffer.remaining();
                if (size > maximum) { abort(new LimitExceeded()); return; }
            }
            for (ByteBuffer buffer : buffers) {
                byte[] chunk = new byte[Math.min(buffer.remaining(), 8192)];
                while (buffer.hasRemaining()) {
                    int length = Math.min(buffer.remaining(), chunk.length);
                    buffer.get(chunk, 0, length);
                    bytes.write(chunk, 0, length);
                }
            }
            subscription.request(1);
        }
        @Override public synchronized void onError(Throwable failure) { abort(failure); }
        @Override public synchronized void onComplete() {
            if (!body.isDone()) { body.complete(bytes.toByteArray()); bytes = null; }
        }
        synchronized void abort(Throwable failure) {
            if (body.isDone()) return;
            bytes = null;
            body.completeExceptionally(failure == null ? new IOException("HTTP response cancelled") : failure);
            if (subscription != null) subscription.cancel();
        }
    }
}
