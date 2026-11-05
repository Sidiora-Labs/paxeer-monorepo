from collections.abc import Callable, Iterator
from dataclasses import dataclass
from threading import Event
from typing import Generic, Self, TypeVar

_T = TypeVar("_T")
STREAM_REPLAY_WINDOW: int
class StreamCursor(str):
    def __new__(cls, value: str) -> Self: ...
    def _log_position(self) -> tuple[int, int, int]: ...

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
    def __init__(self, cursor: StreamCursor) -> None: ...
    @property
    def cursor(self) -> StreamCursor: ...
    def accept(self, page: StreamPage[_T]) -> tuple[StreamEvent[_T], ...]: ...
    def events(
        self,
        source: Callable[[StreamCursor], StreamPage[_T]],
        *,
        poll_interval: float = 1.0,
        cancel: Event | None = None,
    ) -> Iterator[StreamEvent[_T]]: ...
