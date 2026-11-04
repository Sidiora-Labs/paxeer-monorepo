from __future__ import annotations

from collections import OrderedDict
from collections.abc import Callable, Iterator
from dataclasses import dataclass
from math import isfinite
from threading import Event
from typing import Generic, Self, TypeVar, cast

from .production import PlatformSdkError, SdkErrorCode

_T = TypeVar("_T")
STREAM_REPLAY_WINDOW = 1024


class StreamCursor(str):
    def __new__(cls, value: str) -> Self:
        if not value or len(value) > 512 or "\0" in value:
            raise PlatformSdkError(SdkErrorCode.INVALID_ARGUMENT, "never")
        return cast(Self, str.__new__(cls, value))

    def _log_position(self) -> tuple[int, int, int]:
        if len(self) != 32 or any(char not in "0123456789abcdef" for char in self):
            raise PlatformSdkError(SdkErrorCode.DECODE_FAILURE, "never")
        return int(self[:16], 16), int(self[16:24], 16), int(self[24:], 16)


@dataclass(frozen=True)
class StreamEvent(Generic[_T]):
    event_id: str
    previous_cursor: StreamCursor
    cursor: StreamCursor
    value: _T


@dataclass(frozen=True)
class StreamPage(Generic[_T]):
    requested_cursor: StreamCursor
    events: tuple[StreamEvent[_T], ...]
    next_cursor: StreamCursor


class ResumableStream(Generic[_T]):
    """Reject duplicate IDs within the last 1024 accepted events and each page.

    Older identities are evicted in delivery order. The strictly increasing log
    cursor still rejects replayed positions after their identities are evicted.
    """

    def __init__(self, cursor: StreamCursor) -> None:
        self._cursor = cursor
        self._seen: OrderedDict[str, None] = OrderedDict()

    @property
    def cursor(self) -> StreamCursor:
        return self._cursor

    def accept(self, page: StreamPage[_T]) -> tuple[StreamEvent[_T], ...]:
        if page.requested_cursor != self._cursor:
            raise PlatformSdkError(SdkErrorCode.DECODE_FAILURE, "never")
        expected = self._cursor
        position = expected._log_position()
        accepted: list[StreamEvent[_T]] = []
        page_event_ids: set[str] = set()
        for event in page.events:
            event_position = event.cursor._log_position()
            if (
                not event.event_id
                or event.previous_cursor != expected
                or event.event_id in self._seen
                or event.event_id in page_event_ids
                or event_position <= position
            ):
                raise PlatformSdkError(SdkErrorCode.DECODE_FAILURE, "never")
            page_event_ids.add(event.event_id)
            accepted.append(event)
            expected = event.cursor
            position = event_position
        if page.next_cursor != expected:
            raise PlatformSdkError(SdkErrorCode.DECODE_FAILURE, "never")
        for event in accepted:
            self._seen[event.event_id] = None
            if len(self._seen) > STREAM_REPLAY_WINDOW:
                self._seen.popitem(last=False)
        self._cursor = page.next_cursor
        return tuple(accepted)

    def events(
        self,
        source: Callable[[StreamCursor], StreamPage[_T]],
        *,
        poll_interval: float = 1.0,
        cancel: Event | None = None,
    ) -> Iterator[StreamEvent[_T]]:
        """Wait poll_interval seconds after each validated empty page.

        Setting cancel interrupts an idle wait and prevents further source calls.
        A source already running must enforce its own transport deadline. Pages
        are validated and committed atomically, as with accept().
        """
        if (
            isinstance(poll_interval, bool)
            or not isinstance(poll_interval, (int, float))
            or not isfinite(poll_interval)
            or poll_interval <= 0
        ):
            raise PlatformSdkError(SdkErrorCode.INVALID_ARGUMENT, "never")
        stopped = cancel if cancel is not None else Event()
        while not stopped.is_set():
            accepted = self.accept(source(self._cursor))
            for event in accepted:
                if stopped.is_set():
                    return
                yield event
            if not accepted:
                stopped.wait(poll_interval)
