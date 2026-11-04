package com.sidiora.layerx.sdk;

import java.util.ArrayList;
import java.util.ArrayDeque;
import java.util.HashSet;
import java.util.List;
import java.util.Objects;
import java.util.Queue;
import java.util.Set;
import java.util.concurrent.CompletionException;
import java.util.concurrent.CompletionStage;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.Flow;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;

/** Cursor-checked stream state with atomic page acceptance and virtual-thread-friendly async I/O. */
public final class ResumableStream<T> {
    public static final int REPLAY_WINDOW_SIZE = 65_536;
    public record Replay(String eventId, Cursor cursor) {
        public Replay {
            if (eventId == null || eventId.isEmpty() || eventId.length() > 512 || eventId.indexOf('\0') >= 0)
                throw PlatformSdkException.invalidArgument();
            Objects.requireNonNull(cursor, "cursor");
        }
    }
    /** Persist the checkpoint after consuming events; it includes the bounded replay boundary. */
    public record Checkpoint(Cursor cursor, List<Replay> replay) {
        public Checkpoint {
            Objects.requireNonNull(cursor, "cursor");
            if (replay == null || replay.size() > REPLAY_WINDOW_SIZE) throw PlatformSdkException.invalidArgument();
            replay = List.copyOf(replay);
        }
    }
    /** Empty pages keep the cursor and demand unchanged; polling backs off until data arrives. */
    public record PollPolicy(java.time.Duration initialDelay, java.time.Duration maximumDelay) {
        public PollPolicy {
            if (initialDelay == null || maximumDelay == null || initialDelay.toMillis() < 1
                    || maximumDelay.compareTo(initialDelay) < 0 || maximumDelay.compareTo(java.time.Duration.ofSeconds(30)) > 0)
                throw PlatformSdkException.invalidArgument();
        }
        public static PollPolicy defaults() {
            return new PollPolicy(java.time.Duration.ofMillis(100), java.time.Duration.ofSeconds(2));
        }
    }
    public record Cursor(String value) {
        public Cursor {
            if (value == null || value.isEmpty() || value.length() > 512 || value.indexOf('\0') >= 0) {
                throw PlatformSdkException.invalidArgument();
            }
        }
        @Override public String toString() { return value; }
    }
    public record Event<T>(String eventId, Cursor previousCursor, Cursor cursor, T value) {}
    public record Page<T>(Cursor requestedCursor, List<Event<T>> events, Cursor nextCursor) {
        public Page {
            Objects.requireNonNull(requestedCursor, "requestedCursor");
            events = List.copyOf(events);
            Objects.requireNonNull(nextCursor, "nextCursor");
        }
    }
    @FunctionalInterface public interface PageSource<T> { CompletionStage<Page<T>> fetch(Cursor cursor); }

    private Cursor cursor;
    private final Set<String> seen = new HashSet<>();
    private final Set<Cursor> seenCursors = new HashSet<>();
    private final ArrayDeque<Replay> replay = new ArrayDeque<>();

    public ResumableStream(Cursor cursor) { this(new Checkpoint(cursor, List.of())); }
    public ResumableStream(Checkpoint checkpoint) {
        this.cursor = Objects.requireNonNull(checkpoint, "checkpoint").cursor();
        for (Replay entry : checkpoint.replay()) {
            if (!seen.add(entry.eventId()) || !seenCursors.add(entry.cursor())) throw PlatformSdkException.invalidArgument();
            replay.addLast(entry);
        }
        if (!replay.isEmpty() && !replay.getLast().cursor().equals(cursor)) throw PlatformSdkException.invalidArgument();
    }
    public synchronized Cursor cursor() { return cursor; }
    public synchronized Checkpoint checkpoint() { return new Checkpoint(cursor, List.copyOf(replay)); }

    public synchronized List<Event<T>> accept(Page<T> page) {
        validate(page);
        for (Event<T> event : page.events()) {
            seen.add(event.eventId()); seenCursors.add(event.cursor());
            replay.addLast(new Replay(event.eventId(), event.cursor()));
            if (replay.size() > REPLAY_WINDOW_SIZE) {
                Replay expired = replay.removeFirst();
                seen.remove(expired.eventId()); seenCursors.remove(expired.cursor());
            }
        }
        cursor = page.nextCursor();
        return List.copyOf(new ArrayList<>(page.events()));
    }

    private synchronized void validate(Page<T> page) {
        Objects.requireNonNull(page, "page");
        if (!page.requestedCursor().equals(cursor)) throw decodeFailure();
        Cursor expected = cursor;
        if (page.events().size() > REPLAY_WINDOW_SIZE) throw decodeFailure();
        Set<String> pageIds = new HashSet<>();
        Set<Cursor> pageCursors = new HashSet<>();
        for (Event<T> event : page.events()) {
            if (event == null || event.eventId() == null || event.eventId().isEmpty()
                    || event.eventId().length() > 512 || event.eventId().indexOf('\0') >= 0
                    || event.previousCursor() == null || event.cursor() == null
                    || !event.previousCursor().equals(expected)
                    || event.cursor().equals(expected) || event.cursor().equals(cursor)
                    || seenCursors.contains(event.cursor()) || !pageCursors.add(event.cursor())
                    || seen.contains(event.eventId()) || !pageIds.add(event.eventId())) throw decodeFailure();
            expected = event.cursor();
        }
        if (!page.nextCursor().equals(expected)) throw decodeFailure();
    }

    public Flow.Publisher<Event<T>> publisher(PageSource<T> source) {
        return publisher(source, PollPolicy.defaults());
    }

    public Flow.Publisher<Event<T>> publisher(PageSource<T> source, PollPolicy polling) {
        Objects.requireNonNull(source, "source");
        Objects.requireNonNull(polling, "polling");
        AtomicBoolean subscribed = new AtomicBoolean();
        return subscriber -> {
            Objects.requireNonNull(subscriber, "subscriber");
            if (!subscribed.compareAndSet(false, true)) {
                subscriber.onSubscribe(new Flow.Subscription() {
                    @Override public void request(long count) {}
                    @Override public void cancel() {}
                });
                subscriber.onError(new IllegalStateException("a resumable stream publisher permits one subscriber"));
                return;
            }
            var subscription = new StreamSubscription(subscriber, source, polling);
            subscriber.onSubscribe(subscription);
            subscription.start();
        };
    }

    private final class StreamSubscription implements Flow.Subscription, Runnable {
        private final Flow.Subscriber<? super Event<T>> subscriber;
        private final PageSource<T> source;
        private final PollPolicy polling;
        private final AtomicLong demand = new AtomicLong();
        private final AtomicBoolean cancelled = new AtomicBoolean();
        private final Queue<Event<T>> pending = new ArrayDeque<>();
        private final Object signal = new Object();
        private volatile Thread worker;

        private StreamSubscription(Flow.Subscriber<? super Event<T>> subscriber, PageSource<T> source, PollPolicy polling) {
            this.subscriber = subscriber;
            this.source = source;
            this.polling = polling;
        }

        private void start() {
            worker = Thread.ofVirtual().name("layerx-stream").unstarted(this);
            worker.start();
        }

        @Override public void request(long count) {
            if (count <= 0) {
                synchronized (ResumableStream.this) {
                    if (cancelled.compareAndSet(false, true)) {
                        subscriber.onError(new IllegalArgumentException("demand must be positive"));
                    }
                }
                Thread running = worker;
                if (running != null) running.interrupt();
                wake();
                return;
            }
            demand.getAndUpdate(current -> current > Long.MAX_VALUE - count ? Long.MAX_VALUE : current + count);
            wake();
        }

        @Override public void cancel() {
            synchronized (ResumableStream.this) { cancelled.set(true); }
            Thread running = worker;
            if (running != null) running.interrupt();
            wake();
        }

        @Override public void run() {
            try {
                long idleDelay = polling.initialDelay().toMillis();
                while (!cancelled.get()) {
                    awaitDemand();
                    if (cancelled.get()) return;
                    if (pending.isEmpty()) {
                        Page<T> page = Objects.requireNonNull(source.fetch(cursor()), "page stage")
                            .toCompletableFuture().get();
                        synchronized (ResumableStream.this) {
                            if (cancelled.get()) return;
                            validate(page);
                            pending.addAll(page.events());
                        }
                        if (page.events().isEmpty()) {
                            Thread.sleep(idleDelay);
                            idleDelay = Math.min(polling.maximumDelay().toMillis(), idleDelay * 2);
                            continue;
                        }
                        idleDelay = polling.initialDelay().toMillis();
                    }
                    while (demand.get() > 0 && !pending.isEmpty() && !cancelled.get()) {
                        synchronized (ResumableStream.this) {
                            if (cancelled.get()) return;
                            Event<T> event = pending.remove();
                            accept(new Page<>(cursor, List.of(event), event.cursor()));
                            demand.decrementAndGet();
                            subscriber.onNext(event);
                        }
                    }
                }
            } catch (Throwable failure) {
                if (cancelled.compareAndSet(false, true)) subscriber.onError(unwrap(failure));
            }
        }

        private void awaitDemand() throws InterruptedException {
            synchronized (signal) {
                while (demand.get() == 0 && !cancelled.get()) signal.wait();
            }
        }

        private void wake() {
            synchronized (signal) { signal.notifyAll(); }
        }

        private Throwable unwrap(Throwable failure) {
            if ((failure instanceof CompletionException || failure instanceof ExecutionException)
                    && failure.getCause() != null) {
                return failure.getCause();
            }
            return failure;
        }
    }

    private static PlatformSdkException decodeFailure() {
        return new PlatformSdkException(PlatformSdkException.Code.DECODE_FAILURE,
            PlatformSdkException.Retry.NEVER, null, null, null);
    }
}
