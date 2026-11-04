"""Bounded MCP framing shared by the two qualification clients."""
import os
import select
import time

MAX_LINE_BYTES = 1_048_576
RESPONSE_TIMEOUT = 30.0


def read_line(stream, pending, deadline):
    descriptor = stream.fileno()
    blocking = os.get_blocking(descriptor)
    os.set_blocking(descriptor, False)
    try:
        while True:
            if time.monotonic() >= deadline:
                raise ValueError('MCP response deadline: incomplete JSON line')
            newline = pending.find(b'\n')
            if newline >= 0:
                if newline + 1 > MAX_LINE_BYTES:
                    raise ValueError('MCP response exceeds byte bound')
                line = bytes(pending[:newline + 1])
                del pending[:newline + 1]
                return line
            if len(pending) >= MAX_LINE_BYTES:
                raise ValueError('MCP response exceeds byte bound without newline')
            remaining = deadline - time.monotonic()
            if remaining <= 0 or not select.select([descriptor], [], [], remaining)[0]:
                raise ValueError('MCP response deadline: incomplete JSON line')
            try:
                chunk = os.read(descriptor, min(65536, MAX_LINE_BYTES + 1 - len(pending)))
            except BlockingIOError:
                continue
            if not chunk:
                raise ValueError('MCP response EOF: incomplete JSON line')
            pending.extend(chunk)
    finally:
        os.set_blocking(descriptor, blocking)
