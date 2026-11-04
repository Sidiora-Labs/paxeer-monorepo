from __future__ import annotations

import json
from collections import deque
from collections.abc import Iterator, Sequence
from contextlib import contextmanager
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Event, Thread
from time import monotonic

import pytest

from layerx_sdk import PlatformSdkError, PxClient, SdkErrorCode
from layerx_sdk.px import HistoryRow, history_cursor
from layerx_sdk.stream import (
    STREAM_REPLAY_WINDOW,
    ResumableStream,
    StreamCursor,
    StreamEvent,
    StreamPage,
)

ACCOUNT = "0x" + "11" * 20
ORIGIN = StreamCursor(history_cursor(0, 0, 0))


def position(height: int) -> StreamCursor:
    return StreamCursor(history_cursor(height, 0, 0))


class HistoryStore:
    """An in-process history producer with the gateway's px_getHistory schema."""

    def __init__(self, pages: Sequence[Sequence[tuple[int, int]]]) -> None:
        self.pages = deque(pages)
        self.requests: list[tuple[float, str]] = []

    def page(self, cursor: str) -> dict[str, object]:
        self.requests.append((monotonic(), cursor))
        rows = []
        for height, identity in self.pages.popleft() if self.pages else ():
            rows.append({
                "id": str(identity),
                "height_or_seq": str(height),
                "chain": "paxeer",
                "kind": "native_transfer",
                "direction": "out",
                "account": ACCOUNT,
                "counterparty": None,
                "asset": "evm:native",
                "amount": str(height),
                "tx_id": f"native_transfer-{height}-0",
                "tx_hash": "0x" + f"{height:064x}",
                "ordinal": "0",
                "final": True,
                "decoded": {"height": height},
                "asset_metadata": None,
                "blockHeight": height,
                "txIndex": 0,
                "logIndex": 0,
            })
        return {
            "account": ACCOUNT,
            "items": rows,
            "next_cursor": str(position(rows[-1]["blockHeight"])) if rows else None,
        }


class HistorySource:
    """Adapt the real HTTP client and history codec to the stream's typed input."""

    def __init__(self, client: PxClient) -> None:
        self.client = client
        self.returned = Event()

    def __call__(self, cursor: StreamCursor) -> StreamPage[HistoryRow]:
        page = self.client.get_history(ACCOUNT, cursor=str(cursor), limit=100)
        previous = cursor
        events = []
        for row in page.items:
            current = StreamCursor(
                history_cursor(row.block_height, row.tx_index, row.log_index)
            )
            events.append(StreamEvent(row.id, previous, current, row))
            previous = current
        result = StreamPage(
            cursor, tuple(events), StreamCursor(page.next_cursor or str(cursor))
        )
        self.returned.set()
        return result


@contextmanager
def history_server(
    pages: Sequence[Sequence[tuple[int, int]]],
) -> Iterator[tuple[HistoryStore, HistorySource]]:
    store = HistoryStore(pages)

    class Handler(BaseHTTPRequestHandler):
        def do_POST(self) -> None:
            assert self.path == "/"
            assert self.headers["Content-Type"] == "application/json"
            assert self.headers["Accept"] == "application/json"
            assert self.headers["User-Agent"] == "layerx-python/0.1.0"
            request = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
            assert request["jsonrpc"] == "2.0"
            assert type(request["id"]) is int
            assert request["method"] == "px_getHistory"
            assert request["params"][0] == ACCOUNT
            assert request["params"][2:] == [100, None]
            payload = json.dumps({
                "jsonrpc": "2.0",
                "id": request["id"],
                "result": store.page(request["params"][1]),
            }, separators=(",", ":")).encode("utf-8")
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)

        def log_message(self, _format: str, *args: object) -> None:
            pass

    server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
    thread = Thread(target=server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True)
    thread.start()
    try:
        yield store, HistorySource(PxClient(f"http://127.0.0.1:{server.server_port}/", timeout=2))
    finally:
        server.shutdown()
        server.server_close()
        thread.join(timeout=2)
        assert not thread.is_alive()


def assert_decode_failure(
    stream: ResumableStream[HistoryRow], page: StreamPage[HistoryRow]
) -> None:
    before = stream.cursor, tuple(stream._seen)
    with pytest.raises(PlatformSdkError) as error:
        stream.accept(page)
    assert error.value.code == SdkErrorCode.DECODE_FAILURE
    assert (stream.cursor, tuple(stream._seen)) == before


def test_stream_retained_identities_are_bounded_over_real_history_transport() -> None:
    assert STREAM_REPLAY_WINDOW == 1024
    total = STREAM_REPLAY_WINDOW * 3 + 17
    pages = [
        [(height, height) for height in range(start, min(start + 100, total + 1))]
        for start in range(1, total + 1, 100)
    ]
    with history_server(pages) as (store, source):
        stream = ResumableStream[HistoryRow](ORIGIN)
        received = []
        events = stream.events(source)
        for _ in range(total):
            event = next(events)
            received.append(event.value.block_height)
            assert event.value.amount == event.value.block_height
            assert len(stream._seen) <= STREAM_REPLAY_WINDOW
        events.close()
        assert received == list(range(1, total + 1))
        assert stream.cursor == position(total)
        assert tuple(stream._seen) == tuple(str(i) for i in range(total - 1023, total + 1))
        assert [cursor for _, cursor in store.requests] == [
            str(position(start - 1)) for start in range(1, total + 1, 100)
        ]


def test_stream_rejects_recent_ids_and_old_positions_after_eviction() -> None:
    total = STREAM_REPLAY_WINDOW + 1
    initial = [
        [(i, i) for i in range(start, min(start + 100, total + 1))]
        for start in range(1, total + 1, 100)
    ]
    with history_server([*initial, [(total + 1, total)], [(1, 1)], [(total + 1, 1)]]) as (_, source):
        stream = ResumableStream[HistoryRow](ORIGIN)
        for _ in initial:
            stream.accept(source(stream.cursor))
        assert "1" not in stream._seen
        assert_decode_failure(stream, source(stream.cursor))
        assert_decode_failure(stream, source(stream.cursor))
        # Reusing an evicted identity at a new position is the documented window contract.
        accepted = stream.accept(source(stream.cursor))
        assert [event.event_id for event in accepted] == ["1"]
        assert stream.cursor == position(total + 1)
        assert len(stream._seen) == STREAM_REPLAY_WINDOW


def test_stream_rejects_within_page_duplicates_before_committing_any_event() -> None:
    with history_server([[(1, 1), (2, 1)], [(1, 1), (1, 2)], [(1, 1), (2, 2)]]) as (_, source):
        stream = ResumableStream[HistoryRow](ORIGIN)
        assert_decode_failure(stream, source(stream.cursor))
        assert_decode_failure(stream, source(stream.cursor))
        assert [event.event_id for event in stream.accept(source(stream.cursor))] == ["1", "2"]


def test_stream_large_page_retains_delivery_order_and_rejects_early_duplicate() -> None:
    total = STREAM_REPLAY_WINDOW + 1
    pages = [
        [(i, i) for i in range(start, min(start + 100, total + 1))]
        for start in range(1, total + 1, 100)
    ]
    # Rebatch decoded transport pages into one stream page larger than its window.
    with history_server([*pages, [(total + 1, 1)]]) as (_, source):
        cursor = ORIGIN
        produced = []
        for _ in range(len(pages) + 1):
            page = source(cursor)
            produced.extend(page.events)
            cursor = page.next_cursor
    events = tuple(produced[:-1])
    duplicate = produced[-1]
    stream = ResumableStream[HistoryRow](ORIGIN)
    assert_decode_failure(stream, StreamPage(ORIGIN, (*events, duplicate), duplicate.cursor))
    assert stream.accept(StreamPage(ORIGIN, events, events[-1].cursor)) == events
    assert tuple(stream._seen) == tuple(str(i) for i in range(2, STREAM_REPLAY_WINDOW + 2))


def test_stream_empty_page_loop_backs_off_then_delivers_real_history() -> None:
    with history_server([[], [], [], [(1, 1)]]) as (store, source):
        stream = ResumableStream[HistoryRow](ORIGIN)
        events = stream.events(source, poll_interval=0.12)
        event = next(events)
        events.close()
        assert event.value.block_height == 1
        assert event.value.decoded == {"height": 1}
        assert len(store.requests) == 4
        assert all(cursor == ORIGIN for _, cursor in store.requests)
        assert all(
            after[0] - before[0] >= 0.12
            for before, after in zip(store.requests, store.requests[1:])
        )
        assert stream.cursor == position(1)


def test_stream_default_polling_waits_after_empty_page() -> None:
    with history_server([[], [(1, 1)]]) as (store, source):
        stream = ResumableStream[HistoryRow](ORIGIN)
        events = stream.events(source)
        assert next(events).event_id == "1"
        events.close()
        assert len(store.requests) == 2
        assert store.requests[1][0] - store.requests[0][0] >= 1.0


def test_stream_empty_page_loop_cancellation_interrupts_idle_wait() -> None:
    with history_server([[]]) as (store, source):
        stream = ResumableStream[HistoryRow](ORIGIN)
        cancel = Event()
        events = stream.events(source, poll_interval=30, cancel=cancel)
        outcomes: list[object] = []

        def consume() -> None:
            try:
                outcomes.extend(events)
            except BaseException as error:
                outcomes.append(error)

        consumer = Thread(target=consume, daemon=True)
        consumer.start()
        try:
            assert source.returned.wait(2)
            cancel.set()
            consumer.join(timeout=1)
            assert not consumer.is_alive()
            assert outcomes == []
            assert len(store.requests) == 1
            assert stream.cursor == ORIGIN
        finally:
            cancel.set()
            consumer.join(timeout=2)


def test_stream_pre_cancelled_iterator_never_calls_real_client() -> None:
    with history_server([[(1, 1)]]) as (store, source):
        cancel = Event()
        cancel.set()
        assert list(
            ResumableStream[HistoryRow](ORIGIN).events(source, cancel=cancel)
        ) == []
        assert store.requests == []


@pytest.mark.parametrize("interval", [0, -1, float("inf"), float("nan"), True, "1"])
def test_stream_rejects_poll_intervals_that_cannot_bound_idle_requests(
    interval: object,
) -> None:
    with history_server([[]]) as (store, source):
        with pytest.raises(PlatformSdkError) as error:
            next(ResumableStream[HistoryRow](ORIGIN).events(source, poll_interval=interval))
        assert error.value.code == SdkErrorCode.INVALID_ARGUMENT
        assert store.requests == []


def test_stream_cursor_validation_is_atomic_including_empty_pages() -> None:
    with history_server([[(1, 1)]]) as (_, source):
        page = source(ORIGIN)
    stream = ResumableStream[HistoryRow](ORIGIN)
    assert_decode_failure(stream, StreamPage(position(1), page.events, page.next_cursor))
    assert_decode_failure(stream, StreamPage(ORIGIN, page.events, position(2)))
    assert_decode_failure(stream, StreamPage(ORIGIN, (), position(1)))
    event = page.events[0]
    assert_decode_failure(
        stream,
        StreamPage(
            ORIGIN,
            (StreamEvent(event.event_id, position(1), event.cursor, event.value),),
            event.cursor,
        ),
    )
    invalid = StreamCursor("A" * 32)
    assert_decode_failure(
        stream,
        StreamPage(
            ORIGIN,
            (StreamEvent(event.event_id, ORIGIN, invalid, event.value),),
            invalid,
        ),
    )
    assert stream.accept(page) == page.events
