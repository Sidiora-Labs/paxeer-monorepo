"""Task record 22.33.1330, audit 22.30.71 finding 21.

Proofs (both sync and async): test_scoped_feed_reconnect_refreshes_token proves
REST/mint credential separation, a lowercase feed upgrade, production Price
codec delivery and automatic reconnect with a freshly minted scoped WsToken.
test_upgrade_refuses_unusable_token proves wrong-scope, expired and unknown
credentials are refused before stream delivery over the actual handshake.

Baseline revision: 72c91077cb966f1c4c640ded61b4b259f41cad90. No baseline
verification ran: the explicit bounded gate order requires implementation
before testing; no baseline failure is claimed. Final revision and gate logs
are recorded by hf-build/hf-done, outside the candidate to keep it unchanged.
Wiring requests: none; this repair changes only test coverage and path fixtures.

The peer is an in-process contract server, not a patched SDK transport. Its
payloads use the production Price and WsToken codecs, as in the committed
credential contract test test_repair_22_31_341.py. Tokens are freshly issued
opaque credentials bound to scope and expiry in the peer's in-memory store.
"""

from __future__ import annotations

import asyncio
import json
import secrets
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from queue import Queue
from threading import Lock, Thread
from urllib.parse import parse_qs, urlsplit

import pytest
from websockets.exceptions import InvalidStatus
from websockets.sync.server import serve

from chainflow_data_gateway import AsyncChainflowDataGateway, ChainflowDataGateway
from chainflow_data_gateway.types.price import Price
from chainflow_data_gateway.types.ws_token import WsToken

REST_KEY = "repair-1330-rest-read-key"
MINT_KEY = "repair-1330-websocket-mint-key"
DEADLINE = 5


class FeedGateway(ThreadingHTTPServer):
    """Loopback peer with real HTTP, upgrades, JSON frames and scoped tokens."""

    def __init__(self):
        super().__init__(("127.0.0.1", 0), FeedHandler)
        self.requests = Queue()
        self.errors = Queue()
        self.lock = Lock()
        self.tokens = {}
        self.accepted = []
        self.frames = [
            Price(
                symbol="BTC",
                price=price,
                open=64900.0,
                high=65100.0,
                low=64800.0,
                timestamp=datetime(2026, 1, 1, 0, minute, tzinfo=timezone.utc),
            )
            for minute, price in enumerate((65000.5, 65010.25))
        ]

    def expire(self, value):
        with self.lock:
            self.tokens[value].expires_at = datetime.now(timezone.utc) - timedelta(seconds=1)

    def upgrade(self, connection, request):
        url = urlsplit(request.path)
        query = parse_qs(url.query)
        keys = request.headers.get_all("X-API-Key")
        self.requests.put(("WS", url.path, keys, query, None))
        if keys or request.headers.get_all("Authorization") or request.headers.get_all("Cookie"):
            return connection.respond(401, '{"error":"unexpected_credentials"}')
        if url.path != "/ws/btc":
            return connection.respond(404, '{"error":"unknown_feed"}')
        values = query.get("token", [])
        with self.lock:
            token = self.tokens.get(values[0]) if len(values) == 1 and set(query) == {"token"} else None
            if token is None or token.expires_at <= datetime.now(timezone.utc):
                return connection.respond(401, '{"error":"invalid_token"}')
            if token.symbol != "btc":
                return connection.respond(403, '{"error":"wrong_scope"}')
        assert request.headers["Upgrade"].lower() == "websocket"
        assert request.headers["Sec-WebSocket-Version"] == "13"
        return None

    def stream(self, connection):
        try:
            value = parse_qs(urlsplit(connection.request.path).query)["token"][0]
            with self.lock:
                index = len(self.accepted)
                self.accepted.append(value)
            # No invented subscription request or ticker envelope: serialize the
            # gateway's price model with its production wire field names.
            connection.send(self.frames[index].to_json())
            if index == 0:
                self.expire(value)
                connection.close(code=1012, reason="feed restarting")
            else:
                connection.close(code=1000, reason="transcript complete")
        except Exception as exc:
            self.errors.put(exc)
            connection.close(code=1011, reason="contract peer failed")


class FeedHandler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def respond(self, status, body):
        data = body.encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        url = urlsplit(self.path)
        keys = self.headers.get_all("X-API-Key") or []
        self.server.requests.put(("GET", url.path, keys, parse_qs(url.query), None))
        if url.path != "/api/btc/price":
            return self.respond(404, '{"error":"unknown_feed"}')
        if keys != [REST_KEY] or url.query:
            return self.respond(401, '{"error":"invalid_rest_key"}')
        self.respond(200, self.server.frames[0].to_json())

    def do_POST(self):
        url = urlsplit(self.path)
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        keys = self.headers.get_all("X-API-Key") or []
        self.server.requests.put(("POST", url.path, keys, parse_qs(url.query), body))
        if url.path != "/auth/ws-token":
            return self.respond(404, '{"error":"not_found"}')
        if keys != [MINT_KEY] or url.query:
            return self.respond(401, '{"error":"invalid_ws_mint_key"}')
        if (
            self.headers.get_content_type() != "application/json"
            or set(body) != {"symbol"}
            or body["symbol"] not in ("btc", "eth", "markets", "explorer")
        ):
            return self.respond(400, '{"error":"invalid_scope"}')
        token = WsToken(
            token=secrets.token_urlsafe(32),
            symbol=body["symbol"],
            expiresAt=datetime.now(timezone.utc) + timedelta(seconds=300),
            expiresIn=300,
        )
        with self.server.lock:
            self.server.tokens[token.token] = token
        self.respond(200, token.to_json())


@pytest.fixture
def gateway(monkeypatch):
    for name in ("REST_API_KEY", "WS_MINT_KEY", "WS_TOKEN", "SIDIORALABS_CUSTOM_HEADERS"):
        monkeypatch.delenv(name, raising=False)
    with FeedGateway() as server:
        http_thread = Thread(target=server.serve_forever, daemon=True)
        http_thread.start()
        with serve(
            server.stream,
            "127.0.0.1",
            0,
            process_request=server.upgrade,
            open_timeout=DEADLINE,
            close_timeout=DEADLINE,
        ) as ws_server:
            ws_thread = Thread(target=ws_server.serve_forever, daemon=True)
            ws_thread.start()
            try:
                yield (
                    server,
                    {
                        "base_url": f"http://127.0.0.1:{server.server_port}",
                        "websocket_base_url": f"ws://127.0.0.1:{ws_server.socket.getsockname()[1]}",
                        "rest_api_key": REST_KEY,
                        "ws_mint_key": MINT_KEY,
                        "timeout": DEADLINE,
                        "max_retries": 0,
                        "_strict_response_validation": True,
                    },
                )
            finally:
                ws_server.shutdown()
                ws_thread.join(timeout=DEADLINE)
                server.shutdown()
                http_thread.join(timeout=DEADLINE)
            assert not ws_thread.is_alive()
            assert not http_thread.is_alive()
            assert server.errors.empty(), list(server.errors.queue)


def assert_exchange(server, method, path, keys, query=None, body=None):
    assert server.requests.get(timeout=DEADLINE) == (method, path, keys, query or {}, body)


def assert_token(token, scope):
    assert isinstance(token, WsToken)
    assert token.symbol == scope
    assert token.expires_in == 300
    assert datetime.now(timezone.utc) < token.expires_at <= datetime.now(timezone.utc) + timedelta(seconds=300)
    assert token.token not in (REST_KEY, MINT_KEY)


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
def test_scoped_feed_reconnect_refreshes_token(gateway, asynchronous):
    server, options = gateway
    tokens = []
    reconnects = []
    events = []

    def refreshed(client, event, token):
        reconnects.append(event)
        assert event.attempt == 1
        assert event.max_attempts == 1
        assert event.close_code == 1012
        assert server.tokens[tokens[0].token].expires_at < datetime.now(timezone.utc)
        assert_token(token, "btc")
        assert token.token != tokens[0].token
        tokens.append(token)
        client.ws_token = token.token

    connection_options = {
        "max_retries": 1,
        "initial_delay": 0,
        "websocket_connection_options": {"open_timeout": DEADLINE, "close_timeout": DEADLINE},
    }

    async def run_async():
        async with AsyncChainflowDataGateway(**options) as client:
            assert await client.price_feeds.list("btc") == server.frames[0]
            token = await client.gateway.create_web_socket_token(symbol="btc")
            assert_token(token, "btc")
            tokens.append(token)
            client.ws_token = token.token

            # The production callback is synchronous; mint through the real
            # sync HTTP client while the async transport's peer runs separately.
            with ChainflowDataGateway(**options) as minter:

                def refresh(event):
                    refreshed(client, event, minter.gateway.create_web_socket_token(symbol="btc"))

                manager = client.web_sockets.connect_price_feed("btc", on_reconnecting=refresh, **connection_options)
                async with manager as connection:

                    async def on_event(event):
                        events.append(event)

                    connection.on("event", on_event)
                    await asyncio.wait_for(connection.dispatch_events(), timeout=DEADLINE)

    if asynchronous:
        asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options) as client:
            assert client.price_feeds.list("btc") == server.frames[0]
            token = client.gateway.create_web_socket_token(symbol="btc")
            assert_token(token, "btc")
            tokens.append(token)
            client.ws_token = token.token

            def refresh(event):
                refreshed(client, event, client.gateway.create_web_socket_token(symbol="btc"))

            manager = client.web_sockets.connect_price_feed("btc", on_reconnecting=refresh, **connection_options)
            with manager as connection:
                connection.on("event", events.append)
                connection.dispatch_events()

    assert len(reconnects) == 1
    assert len(tokens) == 2
    assert events == [json.loads(price.to_json()) for price in server.frames]
    assert [event["symbol"] for event in events] == ["BTC", "BTC"]
    assert [event["price"] for event in events] == [65000.5, 65010.25]
    assert all(set(event) == {"symbol", "price", "timestamp", "open", "high", "low"} for event in events)
    assert server.accepted == [token.token for token in tokens]
    assert_exchange(server, "GET", "/api/btc/price", [REST_KEY])
    for token in tokens:
        assert_exchange(server, "POST", "/auth/ws-token", [MINT_KEY], body={"symbol": "btc"})
        assert_exchange(server, "WS", "/ws/btc", [], query={"token": [token.token]})
    assert server.requests.empty()


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
@pytest.mark.parametrize("credential", ["eth", "markets", "explorer", "expired", "unknown"])
def test_upgrade_refuses_unusable_token(gateway, asynchronous, credential):
    server, options = gateway
    scope = credential if credential in ("eth", "markets", "explorer") else "btc"
    expected_status = 403 if credential in ("eth", "markets", "explorer") else 401

    def install(client, token):
        assert_token(token, scope)
        if credential == "expired":
            server.expire(token.token)
        client.ws_token = "unissued-opaque-token" if credential == "unknown" else token.token
        return client.ws_token

    async def run_async():
        async with AsyncChainflowDataGateway(**options) as client:
            token = await client.gateway.create_web_socket_token(symbol=scope)
            value = install(client, token)
            with pytest.raises(InvalidStatus) as refusal:
                async with client.web_sockets.connect_price_feed("btc"):
                    pytest.fail("unusable token reached the stream")
            assert refusal.value.response.status_code == expected_status
            return value

    if asynchronous:
        value = asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options) as client:
            token = client.gateway.create_web_socket_token(symbol=scope)
            value = install(client, token)
            with pytest.raises(InvalidStatus) as refusal:
                with client.web_sockets.connect_price_feed("btc"):
                    pytest.fail("unusable token reached the stream")
            assert refusal.value.response.status_code == expected_status
    assert server.accepted == []
    assert_exchange(server, "POST", "/auth/ws-token", [MINT_KEY], body={"symbol": scope})
    assert_exchange(server, "WS", "/ws/btc", [], query={"token": [value]})
    assert server.requests.empty()
