from dataclasses import replace

import pytest
from layerx_sdk import (
    ResumableStream,
    StreamPage,
    StreamCursor,
    StreamEvent,
    PlatformSdkError,
    SdkErrorCode,
    history_cursor,
)


def _log_event(
    previous_cursor: StreamCursor, position: tuple[int, int, int]
) -> StreamEvent[dict[str, object]]:
    block_height, tx_index, log_index = position
    cursor = StreamCursor(history_cursor(*position))
    return StreamEvent(
        event_id=f"log:{cursor}",
        previous_cursor=previous_cursor,
        cursor=cursor,
        value={
            "jsonrpc": "2.0",
            "method": "eth_subscription",
            "params": {
                "subscription": "0x1",
                "result": {
                    "blockNumber": hex(block_height),
                    "transactionIndex": hex(tx_index),
                    "logIndex": hex(log_index),
                    "address": "0x" + "00" * 20,
                    "topics": [],
                    "data": "0x",
                    "removed": False,
                },
            },
        },
    )


def test_log_cursor_resumes_after_last_delivered_log():
    initial_cursor = StreamCursor(history_cursor(9, 0, 0))
    stream = ResumableStream(initial_cursor)
    first = _log_event(initial_cursor, (10, 0, 0))
    second = _log_event(first.cursor, (10, 0, 1))
    delivered = stream.accept(
        StreamPage(initial_cursor, (first, second), second.cursor)
    )
    assert delivered == (first, second)
    assert stream.cursor == StreamCursor(history_cursor(10, 0, 1))

    reconnected = ResumableStream(stream.cursor)
    for position in ((10, 0, 0), (10, 0, 1)):
        replayed = _log_event(reconnected.cursor, position)
        with pytest.raises(PlatformSdkError) as exc_info:
            reconnected.accept(
                StreamPage(reconnected.cursor, (replayed,), replayed.cursor)
            )
        assert exc_info.value.code == SdkErrorCode.DECODE_FAILURE
        assert reconnected.cursor == second.cursor

    later = _log_event(reconnected.cursor, (11, 2, 1))
    earlier = _log_event(later.cursor, (11, 2, 0))
    with pytest.raises(PlatformSdkError) as exc_info:
        reconnected.accept(
            StreamPage(reconnected.cursor, (later, earlier), earlier.cursor)
        )
    assert exc_info.value.code == SdkErrorCode.DECODE_FAILURE
    assert reconnected.cursor == second.cursor

    resumed = _log_event(reconnected.cursor, (11, 2, 0))
    accepted = reconnected.accept(
        StreamPage(second.cursor, (resumed,), resumed.cursor)
    )
    assert accepted == (resumed,)
    assert [event.value for event in (*delivered, *accepted)] == [
        first.value,
        second.value,
        resumed.value,
    ]
    assert reconnected.cursor == StreamCursor(history_cursor(11, 2, 0))
    assert reconnected.accept(StreamPage(resumed.cursor, (), resumed.cursor)) == ()


@pytest.mark.parametrize(
    ("first_position", "regressing_position"),
    [
        (None, (11, 1, 5)),
        (None, (11, 2, 0)),
        (None, (10, 0xFFFFFFFF, 0xFFFFFFFF)),
        ((11, 2, 2), (11, 2, 1)),
        ((11, 2, 1), (11, 2, 1)),
        ((11, 3, 0), (11, 2, 5)),
        ((12, 0, 0), (11, 3, 5)),
    ],
)
def test_log_cursor_refuses_regressing_log_position(first_position, regressing_position):
    requested = StreamCursor(history_cursor(11, 2, 0))
    stream = ResumableStream(requested)
    prefix = ()
    if first_position is not None:
        prefix = (_log_event(requested, first_position),)
    previous = prefix[-1].cursor if prefix else requested
    regressing = replace(
        _log_event(previous, regressing_position), event_id="regressing"
    )
    page = StreamPage(requested, (*prefix, regressing), regressing.cursor)

    with pytest.raises(PlatformSdkError) as exc_info:
        stream.accept(page)
    assert exc_info.value.code == SdkErrorCode.DECODE_FAILURE
    assert stream.cursor == requested

    corrected = _log_event(previous, (12, 1, 0))
    accepted = stream.accept(
        StreamPage(requested, (*prefix, corrected), corrected.cursor)
    )
    assert accepted == (*prefix, corrected)
    assert stream.cursor == corrected.cursor


class TestStreamCursorHygiene:
    def test_constructs_valid_cursors(self):
        cursor = StreamCursor(history_cursor(10, 0, 0))
        assert cursor is not None

    def test_refuses_empty_cursors(self):
        with pytest.raises(PlatformSdkError) as exc_info:
            StreamCursor("")
        assert exc_info.value.code == SdkErrorCode.INVALID_ARGUMENT

    def test_refuses_nul_containing_cursors(self):
        with pytest.raises(PlatformSdkError):
            StreamCursor("has\0null")


class TestResumableStreamNoGapNoDuplicate:
    def test_accepts_empty_page(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        page = StreamPage(
            requested_cursor=initial_cursor,
            events=(),
            next_cursor=initial_cursor,
        )
        accepted = stream.accept(page)
        assert len(accepted) == 0
        assert stream.cursor == initial_cursor

    def test_accepts_single_event(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        next_cursor = StreamCursor(history_cursor(10, 0, 0))
        page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=initial_cursor,
                    cursor=next_cursor,
                    value="first",
                ),
            ),
            next_cursor=next_cursor,
        )
        accepted = stream.accept(page)
        assert len(accepted) == 1
        assert accepted[0].value == "first"
        assert stream.cursor == next_cursor

    def test_accepts_multiple_events_forming_chain(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        c1 = StreamCursor(history_cursor(10, 0, 0))
        c2 = StreamCursor(history_cursor(10, 0, 1))
        c3 = StreamCursor(history_cursor(11, 2, 0))
        page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=initial_cursor,
                    cursor=c1,
                    value="first",
                ),
                StreamEvent(
                    event_id="e2",
                    previous_cursor=c1,
                    cursor=c2,
                    value="second",
                ),
                StreamEvent(
                    event_id="e3",
                    previous_cursor=c2,
                    cursor=c3,
                    value="third",
                ),
            ),
            next_cursor=c3,
        )
        accepted = stream.accept(page)
        assert len(accepted) == 3
        assert stream.cursor == c3

    def test_refuses_page_with_wrong_requested_cursor(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        wrong_cursor = StreamCursor(history_cursor(8, 0, 0))
        page = StreamPage(
            requested_cursor=wrong_cursor,
            events=(),
            next_cursor=wrong_cursor,
        )
        with pytest.raises(PlatformSdkError) as exc_info:
            stream.accept(page)
        assert exc_info.value.code == SdkErrorCode.DECODE_FAILURE

    def test_refuses_gap_in_cursor_chain(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        c1 = StreamCursor(history_cursor(10, 0, 0))
        c2 = StreamCursor(history_cursor(10, 0, 1))
        page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=c1,
                    cursor=c2,
                    value="skipped",
                ),
            ),
            next_cursor=c2,
        )
        with pytest.raises(PlatformSdkError):
            stream.accept(page)

    def test_refuses_duplicate_event_ids_across_pages(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        c1 = StreamCursor(history_cursor(10, 0, 0))
        c2 = StreamCursor(history_cursor(10, 0, 1))
        first_page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=initial_cursor,
                    cursor=c1,
                    value="first",
                ),
            ),
            next_cursor=c1,
        )
        stream.accept(first_page)
        duplicate_page = StreamPage(
            requested_cursor=c1,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=c1,
                    cursor=c2,
                    value="duplicate",
                ),
            ),
            next_cursor=c2,
        )
        with pytest.raises(PlatformSdkError):
            stream.accept(duplicate_page)

    def test_refuses_duplicate_event_ids_within_page(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        c1 = StreamCursor(history_cursor(10, 0, 0))
        c2 = StreamCursor(history_cursor(10, 0, 1))
        page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=initial_cursor,
                    cursor=c1,
                    value="first",
                ),
                StreamEvent(
                    event_id="e1",
                    previous_cursor=c1,
                    cursor=c2,
                    value="duplicate",
                ),
            ),
            next_cursor=c2,
        )
        with pytest.raises(PlatformSdkError):
            stream.accept(page)

    def test_refuses_empty_event_id(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        c1 = StreamCursor(history_cursor(10, 0, 0))
        page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="",
                    previous_cursor=initial_cursor,
                    cursor=c1,
                    value="no-id",
                ),
            ),
            next_cursor=c1,
        )
        with pytest.raises(PlatformSdkError):
            stream.accept(page)

    def test_refuses_mismatched_next_cursor(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        c1 = StreamCursor(history_cursor(10, 0, 0))
        c2 = StreamCursor(history_cursor(10, 0, 1))
        page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=initial_cursor,
                    cursor=c1,
                    value="first",
                ),
            ),
            next_cursor=c2,
        )
        with pytest.raises(PlatformSdkError):
            stream.accept(page)

    def test_supports_resumption_after_disconnection(self):
        initial_cursor = StreamCursor(history_cursor(9, 0, 0))
        stream = ResumableStream(initial_cursor)
        c1 = StreamCursor(history_cursor(10, 0, 0))
        c2 = StreamCursor(history_cursor(10, 0, 1))
        first_page = StreamPage(
            requested_cursor=initial_cursor,
            events=(
                StreamEvent(
                    event_id="e1",
                    previous_cursor=initial_cursor,
                    cursor=c1,
                    value="before-disconnect",
                ),
            ),
            next_cursor=c1,
        )
        stream.accept(first_page)
        resumed_page = StreamPage(
            requested_cursor=c1,
            events=(
                StreamEvent(
                    event_id="e2",
                    previous_cursor=c1,
                    cursor=c2,
                    value="after-reconnect",
                ),
            ),
            next_cursor=c2,
        )
        accepted = stream.accept(resumed_page)
        assert len(accepted) == 1
        assert accepted[0].value == "after-reconnect"
        assert stream.cursor == c2
