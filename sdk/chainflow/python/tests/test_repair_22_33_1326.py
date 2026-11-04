"""Task record 22.33.1326, audit 22.30.71 (domain 15).

Finding 7: test_finding_7_construction_modes_and_operation_validation.
Finding 9: test_finding_9_mixed_rest_and_websocket_credentials.
Finding 10: test_finding_10_request_diagnostics_redact_secrets.
Finding 22: test_finding_22_sync_failed_send_reserves_bytes and
            test_finding_22_async_failed_send_reserves_bytes.
Baseline was not run: the explicit bounded gate order requires implementation first.
The WsToken security selector was already present; finding 9 receives regression coverage.
Wiring request (domain 15): preserve these repairs in Scalar's upstream Python
runtime/auth templates and sdk/chainflow/python/scalar-sdk.manifest.json on regeneration.
The runner's hf-done evidence records the exact qualified revision and command.
"""

from __future__ import annotations

import asyncio
import json
import logging
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timedelta, timezone
from threading import Event, Thread
from urllib.parse import parse_qs, urlsplit

import pytest
from websockets.exceptions import ConnectionClosed
from websockets.sync.server import serve

from chainflow_data_gateway import AsyncChainflowDataGateway, ChainflowDataGateway
from chainflow_data_gateway._base_client import make_request_options
from chainflow_data_gateway._exceptions import WebSocketQueueFullError
from chainflow_data_gateway._types import omit
from chainflow_data_gateway.resources.web_sockets.ws import _serialize_event
from chainflow_data_gateway.types.ws_token import WsToken

# Reuse the committed HTTP contract peer, which checks real headers, JSON, scopes,
# token expiry, and generated Price/WsToken codecs. No transport is replaced.
from test_repair_22_31_341 import ContractGateway, ContractHandler, MINT_KEY, REST_KEY, assert_exchange

TOKEN = "opaque-ws-test-token-1326"
HEADER_SECRET = "override-header-secret-1326"
QUERY_SECRET = "override-query-secret-1326"
URL_SECRET = "embedded-url-secret-1326"
EVENTS = [
    {"type": "subscribe", "channels": ["ticker"]},
    {"type": "subscribe", "channels": ["trades"]},
    {"type": "subscribe", "channels": ["stats"]},
]


class DiagnosticHandler(ContractHandler):
    def do_GET(self):
        url = urlsplit(self.path)
        if url.path != "/diagnostics":
            return super().do_GET()
        self.server.requests.put(("GET", url.path, self.headers.get_all("X-API-Key"), parse_qs(url.query), None))
        self.server.diagnostic_requests += 1
        if self.server.diagnostic_requests == 1:
            self.send_response(503)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", "4")
            self.send_header("Retry-After", "0")
            self.end_headers()
            self.wfile.write(b"busy")
        else:
            self.respond(200, "ok", "text/plain")


class EchoGateway(ContractGateway):
    def __init__(self):
        super().__init__()
        self.RequestHandlerClass = DiagnosticHandler
        self.diagnostic_requests = 0
        self.tokens[TOKEN] = WsToken(
            token=TOKEN,
            symbol="btc",
            expiresAt=datetime.now(timezone.utc) + timedelta(seconds=300),
            expiresIn=300,
        )

    def stream(self, connection):
        connection.send(self.price.to_json())
        for data in connection:
            # Consume the actual serialized event and echo it over the wire.
            connection.send(json.dumps(json.loads(data)))


@pytest.fixture
def gateway(monkeypatch):
    for name in ("REST_API_KEY", "WS_MINT_KEY", "WS_TOKEN", "SIDIORALABS_CUSTOM_HEADERS"):
        monkeypatch.delenv(name, raising=False)
    with EchoGateway() as server:
        http_thread = Thread(target=server.serve_forever, daemon=True)
        http_thread.start()
        with serve(server.stream, "127.0.0.1", 0, process_request=server.upgrade) as ws_server:
            ws_thread = Thread(target=ws_server.serve_forever, daemon=True)
            ws_thread.start()
            try:
                yield (
                    server,
                    {
                        "base_url": f"http://127.0.0.1:{server.server_port}",
                        "websocket_base_url": f"ws://127.0.0.1:{ws_server.socket.getsockname()[1]}",
                        "max_retries": 0,
                        "_strict_response_validation": True,
                    },
                )
            finally:
                ws_server.shutdown()
                ws_thread.join(timeout=5)
                server.shutdown()
                http_thread.join(timeout=5)


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
@pytest.mark.parametrize("mode", ["token", "mint", "rest", "public"])
def test_finding_7_construction_modes_and_operation_validation(gateway, asynchronous, mode):
    server, options = gateway
    credentials = {
        "token": {"ws_token": TOKEN},
        "mint": {"ws_mint_key": MINT_KEY},
        "rest": {"rest_api_key": REST_KEY},
        "public": {},
    }[mode]

    async def run_async():
        async with AsyncChainflowDataGateway(**options, **credentials) as client:
            assert await client.gateway.list_health() == "ok"
            if mode == "rest":
                assert await client.price_feeds.list("btc") == server.price
            else:
                with pytest.raises(TypeError, match="requires rest_api_key"):
                    await client.price_feeds.list("btc", extra_query={"token": TOKEN})
                with pytest.raises(TypeError, match="requires rest_api_key"):
                    await client.price_feeds.list("btc", extra_headers={"X-API-Key": omit})
                assert await client.price_feeds.list("btc", extra_headers={"x-api-key": REST_KEY}) == server.price
            if mode == "mint":
                token = await client.gateway.create_web_socket_token(symbol="btc")
                assert token == server.tokens[token.token]
            else:
                with pytest.raises(TypeError, match="requires ws_mint_key"):
                    await client.gateway.create_web_socket_token(symbol="btc", extra_query={"token": TOKEN})
            if mode == "token":
                async with client.web_sockets.connect_price_feed("btc") as connection:
                    assert await connection.recv() == json.loads(server.price.to_json())
            else:
                with pytest.raises(TypeError, match="requires ws_token"):
                    async with client.web_sockets.connect_price_feed("btc", extra_headers={"X-API-Key": REST_KEY}):
                        pytest.fail("upgrade without a token must fail before transport")
            with pytest.raises(TypeError, match="requires ws_token"):
                async with client.web_sockets.connect_price_feed("btc", extra_query={"token": ""}):
                    pytest.fail("empty token must fail before transport")

    if asynchronous:
        asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options, **credentials) as client:
            assert client.gateway.list_health() == "ok"
            if mode == "rest":
                assert client.price_feeds.list("btc") == server.price
            else:
                with pytest.raises(TypeError, match="requires rest_api_key"):
                    client.price_feeds.list("btc", extra_query={"token": TOKEN})
                with pytest.raises(TypeError, match="requires rest_api_key"):
                    client.price_feeds.list("btc", extra_headers={"X-API-Key": omit})
                assert client.price_feeds.list("btc", extra_headers={"x-api-key": REST_KEY}) == server.price
            if mode == "mint":
                token = client.gateway.create_web_socket_token(symbol="btc")
                assert token == server.tokens[token.token]
            else:
                with pytest.raises(TypeError, match="requires ws_mint_key"):
                    client.gateway.create_web_socket_token(symbol="btc", extra_query={"token": TOKEN})
            if mode == "token":
                with client.web_sockets.connect_price_feed("btc") as connection:
                    assert connection.recv() == json.loads(server.price.to_json())
            else:
                with pytest.raises(TypeError, match="requires ws_token"):
                    with client.web_sockets.connect_price_feed("btc", extra_headers={"X-API-Key": REST_KEY}):
                        pytest.fail("upgrade without a token must fail before transport")
            with pytest.raises(TypeError, match="requires ws_token"):
                with client.web_sockets.connect_price_feed("btc", extra_query={"token": ""}):
                    pytest.fail("empty token must fail before transport")
    assert_exchange(server, "GET", "/health", [])
    assert_exchange(server, "GET", "/api/btc/price", [REST_KEY])
    if mode == "mint":
        assert_exchange(server, "POST", "/auth/ws-token", [MINT_KEY], body={"symbol": "btc"})
    if mode == "token":
        assert_exchange(server, "WS", "/ws/btc", [], query={"token": [TOKEN]})
    assert server.requests.empty()


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
def test_finding_9_mixed_rest_and_websocket_credentials(gateway, asynchronous):
    server, options = gateway
    options.update(rest_api_key=REST_KEY, ws_mint_key=MINT_KEY, ws_token=TOKEN)

    async def run_async():
        async with AsyncChainflowDataGateway(**options) as client:
            assert await client.gateway.list_health() == "ok"
            assert await client.price_feeds.list("btc") == server.price
            token = await client.gateway.create_web_socket_token(symbol="btc")
            assert server.tokens[token.token] == token
            async with client.web_sockets.connect_price_feed("btc") as connection:
                assert await connection.recv() == json.loads(server.price.to_json())
            assert await client.price_feeds.list("btc") == server.price

    if asynchronous:
        asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options) as client:
            assert client.gateway.list_health() == "ok"
            assert client.price_feeds.list("btc") == server.price
            token = client.gateway.create_web_socket_token(symbol="btc")
            assert server.tokens[token.token] == token
            with client.web_sockets.connect_price_feed("btc") as connection:
                assert connection.recv() == json.loads(server.price.to_json())
            assert client.price_feeds.list("btc") == server.price
    assert_exchange(server, "GET", "/health", [])
    assert_exchange(server, "GET", "/api/btc/price", [REST_KEY])
    assert_exchange(server, "POST", "/auth/ws-token", [MINT_KEY], body={"symbol": "btc"})
    assert_exchange(server, "WS", "/ws/btc", [], query={"token": [TOKEN]})
    assert_exchange(server, "GET", "/api/btc/price", [REST_KEY])
    assert server.requests.empty()


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
def test_finding_10_request_diagnostics_redact_secrets(gateway, caplog, asynchronous):
    server, options = gateway
    options.update(rest_api_key=REST_KEY, ws_token=TOKEN, max_retries=1)
    caplog.set_level(logging.DEBUG, logger="chainflow_data_gateway")
    request_options = make_request_options(
        extra_headers={"x-aPi-kEy": HEADER_SECRET}, extra_query={"ToKeN": QUERY_SECRET, "page": "2"}
    )
    path = f"/diagnostics?%74oken={URL_SECRET}&token={URL_SECRET}"

    async def run_async():
        async with AsyncChainflowDataGateway(**options) as client:
            assert await client.get(path, cast_to=str, options=request_options) == "ok"
            async with client.web_sockets.connect_price_feed("btc") as connection:
                assert await connection.recv() == json.loads(server.price.to_json())

    if asynchronous:
        asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options) as client:
            assert client.get(path, cast_to=str, options=request_options) == "ok"
            with client.web_sockets.connect_price_feed("btc") as connection:
                assert connection.recv() == json.loads(server.price.to_json())
    expected_query = {"token": [URL_SECRET], "ToKeN": [QUERY_SECRET], "page": ["2"]}
    for _ in range(2):
        assert_exchange(server, "GET", "/diagnostics", [HEADER_SECRET], query=expected_query)
    assert_exchange(server, "WS", "/ws/btc", [], query={"token": [TOKEN]})
    assert server.requests.empty()
    diagnostics = caplog.text + repr([(record.msg, record.args) for record in caplog.records])
    for secret in (REST_KEY, TOKEN, HEADER_SECRET, QUERY_SECRET, URL_SECRET):
        assert secret not in diagnostics
    for expected in ("Request options", "Sending HTTP Request", "HTTP Response", "Retrying request", "Connecting to"):
        assert expected in diagnostics
    assert "redacted" in diagnostics
    assert "page" in diagnostics
    assert "diagnostics" in diagnostics


def _queue_budget():
    return sum(len(_serialize_event(event).encode("utf-8")) for event in EVENTS)


def test_finding_22_sync_failed_send_reserves_bytes(gateway):
    server, options = gateway
    with ChainflowDataGateway(**options, ws_token=TOKEN) as client:
        with client.web_sockets.connect_price_feed("btc", max_queue_size=_queue_budget()) as connection:
            assert connection.recv() == json.loads(server.price.to_json())
            queue = connection._send_queue
            connection._is_reconnecting = True
            connection.send(EVENTS[0])
            connection.send(EVENTS[1])
            entered, release = Event(), Event()
            attempts = 0

            def send(data):
                nonlocal attempts
                attempts += 1
                if attempts == 1:
                    connection._connection.send(data)
                    assert connection.recv() == EVENTS[0]
                    connection.close()
                else:
                    entered.set()
                    assert release.wait(5)
                    connection._connection.send(data)  # Real send on a closed socket.

            with ThreadPoolExecutor(max_workers=1) as executor:
                flush = executor.submit(queue.flush_sync, send)
                try:
                    assert entered.wait(5)
                    connection.send(EVENTS[2])
                    connection.send(EVENTS[0])  # Reuse only the successfully sent bytes.
                    with pytest.raises(WebSocketQueueFullError):
                        connection.send(EVENTS[1])
                    assert queue._bytes == _queue_budget()
                finally:
                    release.set()
                with pytest.raises(ConnectionClosed):
                    flush.result(timeout=5)
            assert len(queue) == 3
            assert queue._bytes == _queue_budget()
            with pytest.raises(ConnectionClosed):
                queue.flush_sync(connection._connection.send)
            assert queue._bytes == _queue_budget()
        # Open a new real transport and deliver the retained batch in order.
        with client.web_sockets.connect_price_feed("btc") as recovered:
            assert recovered.recv() == json.loads(server.price.to_json())
            recovered._send_queue = queue
            recovered._flush_send_queue()
            assert [recovered.recv() for _ in range(3)] == [EVENTS[1], EVENTS[2], EVENTS[0]]
            assert queue._bytes == 0
            assert not queue
            queue.enqueue("é" * (_queue_budget() // 2))
            assert queue._bytes == len(("é" * (_queue_budget() // 2)).encode("utf-8"))
            assert queue.drain() == ["é" * (_queue_budget() // 2)]
            assert queue._bytes == 0


def test_finding_22_async_failed_send_reserves_bytes(gateway):
    server, options = gateway

    async def run():
        async with AsyncChainflowDataGateway(**options, ws_token=TOKEN) as client:
            async with client.web_sockets.connect_price_feed("btc", max_queue_size=_queue_budget()) as connection:
                assert await connection.recv() == json.loads(server.price.to_json())
                queue = connection._send_queue
                connection._is_reconnecting = True
                await connection.send(EVENTS[0])
                await connection.send(EVENTS[1])
                entered, release = asyncio.Event(), asyncio.Event()
                attempts = 0

                async def send(data):
                    nonlocal attempts
                    attempts += 1
                    if attempts == 1:
                        await connection._connection.send(data)
                        assert await connection.recv() == EVENTS[0]
                        await connection.close()
                    else:
                        entered.set()
                        await asyncio.wait_for(release.wait(), 5)
                        await connection._connection.send(data)

                flush = asyncio.create_task(queue.flush_async(send))
                try:
                    await asyncio.wait_for(entered.wait(), 5)
                    await connection.send(EVENTS[2])
                    await connection.send(EVENTS[0])
                    with pytest.raises(WebSocketQueueFullError):
                        await connection.send(EVENTS[1])
                    assert queue._bytes == _queue_budget()
                finally:
                    release.set()
                with pytest.raises(ConnectionClosed):
                    await asyncio.wait_for(flush, 5)
                assert len(queue) == 3
                assert queue._bytes == _queue_budget()
                with pytest.raises(ConnectionClosed):
                    await queue.flush_async(connection._connection.send)
                assert queue._bytes == _queue_budget()
            async with client.web_sockets.connect_price_feed("btc") as recovered:
                assert await recovered.recv() == json.loads(server.price.to_json())
                recovered._send_queue = queue
                await recovered._flush_send_queue()
                assert [await recovered.recv() for _ in range(3)] == [EVENTS[1], EVENTS[2], EVENTS[0]]
                assert queue._bytes == 0
                assert not queue
                queue.enqueue("é" * (_queue_budget() // 2))
                assert queue._bytes == len(("é" * (_queue_budget() // 2)).encode("utf-8"))
                assert queue.drain() == ["é" * (_queue_budget() // 2)]
                assert queue._bytes == 0

    asyncio.run(run())
