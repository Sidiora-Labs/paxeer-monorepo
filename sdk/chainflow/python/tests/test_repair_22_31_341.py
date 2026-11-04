"""Finding 6: prove operation credentials over HTTP and WebSocket transports.

Task record 22.31.341:
- Finding 6 is covered by test_distinct_credentials, test_rest_key_cannot_mint,
  and test_mint_key_cannot_read, each for synchronous and asynchronous clients.
- Baseline verification was not run: the bounded gate order requires full implementation first.
- The previous repair's six tests passed at b58b1a62de61f4d6f207e43b10ca9089de973ec0;
  that attempt did not qualify because its build and final candidate differed.
- Wiring request (domain 15): Scalar's upstream Python authentication/resource templates and
  sdk/chainflow/python/scalar-sdk.manifest.json must preserve per-operation security on regeneration.
"""

from __future__ import annotations

import asyncio
import json
import secrets
from datetime import datetime, timedelta, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from queue import Queue
from threading import Thread
from urllib.parse import parse_qs, urlsplit

import pytest
from websockets.sync.server import serve

from chainflow_data_gateway import AsyncChainflowDataGateway, AuthenticationError, ChainflowDataGateway
from chainflow_data_gateway.types.price import Price
from chainflow_data_gateway.types.ws_token import WsToken

REST_KEY = "test-rest-read-credential"
MINT_KEY = "test-ws-mint-credential"


class ContractGateway(ThreadingHTTPServer):
    """In-process implementation of the declared gateway authentication contract."""

    def __init__(self):
        super().__init__(("127.0.0.1", 0), ContractHandler)
        self.requests = Queue()
        self.tokens = {}
        self.price = Price(
            symbol="BTC",
            price=65000.0,
            open=64900.0,
            high=65100.0,
            low=64800.0,
            timestamp=datetime.now(timezone.utc).replace(second=0, microsecond=0),
        )

    def upgrade(self, connection, request):
        url = urlsplit(request.path)
        self.requests.put(("WS", url.path, request.headers.get_all("X-API-Key"), parse_qs(url.query), None))
        values = parse_qs(url.query).get("token", [])
        token = self.tokens.get(values[0]) if len(values) == 1 else None
        if (
            request.headers.get_all("X-API-Key")
            or url.path != "/ws/btc"
            or token is None
            or token.symbol != "btc"
            or token.expires_at <= datetime.now(timezone.utc)
        ):
            return connection.respond(401, "unauthorized")
        return None

    def stream(self, connection):
        connection.send(self.price.to_json())


class ContractHandler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def respond(self, status, body, content_type="application/json"):
        data = body.encode()
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        url = urlsplit(self.path)
        keys = self.headers.get_all("X-API-Key") or []
        self.server.requests.put(("GET", url.path, keys, parse_qs(url.query), None))
        if url.path == "/health":
            if keys or url.query:
                return self.respond(400, '{"error":"unexpected_credentials"}')
            return self.respond(200, "ok", "text/plain")
        if url.path != "/api/btc/price":
            return self.respond(404, '{"error":"not_found"}')
        if keys != [REST_KEY] or url.query:
            return self.respond(401, '{"error":"invalid_rest_key"}')
        self.respond(200, self.server.price.to_json())

    def do_POST(self):
        url = urlsplit(self.path)
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        keys = self.headers.get_all("X-API-Key") or []
        self.server.requests.put(("POST", url.path, keys, parse_qs(url.query), body))
        if url.path != "/auth/ws-token":
            return self.respond(404, '{"error":"not_found"}')
        if keys != [MINT_KEY] or url.query:
            return self.respond(401, '{"error":"invalid_ws_mint_key"}')
        if self.headers.get_content_type() != "application/json" or body != {"symbol": "btc"}:
            return self.respond(400, '{"error":"invalid_scope"}')
        token = WsToken(
            token=secrets.token_urlsafe(32),
            symbol=body["symbol"],
            expiresAt=datetime.now(timezone.utc) + timedelta(seconds=300),
            expiresIn=300,
        )
        self.server.tokens[token.token] = token
        self.respond(200, token.to_json())


@pytest.fixture
def gateway(monkeypatch):
    for name in ("REST_API_KEY", "WS_MINT_KEY", "WS_TOKEN", "SIDIORALABS_CUSTOM_HEADERS"):
        monkeypatch.delenv(name, raising=False)
    with ContractGateway() as server:
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
                        "rest_api_key": REST_KEY,
                        "max_retries": 0,
                        "_strict_response_validation": True,
                    },
                )
            finally:
                ws_server.shutdown()
                ws_thread.join(timeout=5)
                server.shutdown()
                http_thread.join(timeout=5)


def assert_exchange(server, method, path, keys, query=None, body=None):
    assert server.requests.get(timeout=5) == (method, path, keys, query or {}, body)


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
def test_distinct_credentials(gateway, asynchronous):
    server, options = gateway

    async def run_async():
        async with AsyncChainflowDataGateway(**options, ws_mint_key=MINT_KEY, ws_token="unused-token") as client:
            assert await client.gateway.list_health() == "ok"
            price = await client.price_feeds.list("btc")
            assert price == server.price
            token = await client.gateway.create_web_socket_token(symbol="btc")
            assert token.expires_in == 300
            assert token.symbol == "btc"
            assert server.tokens[token.token] == token
            client.ws_token = token.token
            async with client.web_sockets.connect_price_feed("btc") as connection:
                assert await connection.recv() == json.loads(server.price.to_json())
            return token

    if asynchronous:
        token = asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options, ws_mint_key=MINT_KEY, ws_token="unused-token") as client:
            assert client.gateway.list_health() == "ok"
            assert client.price_feeds.list("btc") == server.price
            token = client.gateway.create_web_socket_token(symbol="btc")
            assert token.expires_in == 300
            assert token.symbol == "btc"
            assert server.tokens[token.token] == token
            client.ws_token = token.token
            with client.web_sockets.connect_price_feed("btc") as connection:
                assert connection.recv() == json.loads(server.price.to_json())
    assert_exchange(server, "GET", "/health", [])
    assert_exchange(server, "GET", "/api/btc/price", [REST_KEY])
    assert_exchange(server, "POST", "/auth/ws-token", [MINT_KEY], body={"symbol": "btc"})
    assert_exchange(server, "WS", "/ws/btc", [], query={"token": [token.token]})
    assert server.requests.empty()


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
@pytest.mark.parametrize("ws_token", [None, "token-is-not-a-mint-key"], ids=["without-token", "with-token"])
def test_rest_key_cannot_mint(gateway, asynchronous, ws_token):
    server, options = gateway

    async def run_async():
        async with AsyncChainflowDataGateway(**options, ws_token=ws_token) as client:
            assert await client.price_feeds.list("btc") == server.price
            with pytest.raises(TypeError, match="Could not resolve authentication"):
                await client.gateway.create_web_socket_token(symbol="btc")
        async with AsyncChainflowDataGateway(**options, ws_mint_key=REST_KEY) as client:
            with pytest.raises(AuthenticationError):
                await client.gateway.create_web_socket_token(symbol="btc")

    if asynchronous:
        asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options, ws_token=ws_token) as client:
            assert client.price_feeds.list("btc") == server.price
            with pytest.raises(TypeError, match="Could not resolve authentication"):
                client.gateway.create_web_socket_token(symbol="btc")
        with ChainflowDataGateway(**options, ws_mint_key=REST_KEY) as client:
            with pytest.raises(AuthenticationError):
                client.gateway.create_web_socket_token(symbol="btc")
    assert_exchange(server, "GET", "/api/btc/price", [REST_KEY])
    assert_exchange(server, "POST", "/auth/ws-token", [REST_KEY], body={"symbol": "btc"})
    assert server.requests.empty()


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
def test_mint_key_cannot_read(gateway, asynchronous):
    server, options = gateway
    options["rest_api_key"] = MINT_KEY

    async def run_async():
        async with AsyncChainflowDataGateway(**options, ws_mint_key=MINT_KEY) as client:
            with pytest.raises(AuthenticationError):
                await client.price_feeds.list("btc")

    if asynchronous:
        asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options, ws_mint_key=MINT_KEY) as client:
            with pytest.raises(AuthenticationError):
                client.price_feeds.list("btc")
    assert_exchange(server, "GET", "/api/btc/price", [MINT_KEY])
    assert server.requests.empty()
