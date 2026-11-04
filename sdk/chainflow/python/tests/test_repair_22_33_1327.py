"""Audit 22.30.71 finding 8: real token-authenticated SDK logging paths.

The committed loopback peer issues scoped WsToken models over HTTP and sends
Price models through actual WebSocket upgrades; the SDK transport is unchanged.
"""

from __future__ import annotations

import asyncio
import json
import logging
from urllib.parse import parse_qs, quote, urlsplit

import pytest
from websockets.exceptions import InvalidStatus

from chainflow_data_gateway import AsyncChainflowDataGateway, ChainflowDataGateway

from test_repair_22_33_1330 import (
    DEADLINE,
    MINT_KEY,
    REST_KEY,
    assert_exchange,
    assert_token,
    gateway as gateway,
)

WS_LOGGER = "chainflow_data_gateway.resources.web_sockets.ws"


def assert_safe_connection_logs(caplog, endpoint, tokens, count):
    records = [record for record in caplog.records if record.name.startswith("chainflow_data_gateway")]
    assert records
    diagnostics = "\n".join(record.getMessage() for record in records)
    raw_records = repr([(record.msg, record.args, record.exc_info) for record in records])
    for credential in [REST_KEY, MINT_KEY, *tokens]:
        assert credential not in diagnostics
        assert credential not in raw_records
        assert quote(credential, safe="") not in diagnostics
        assert quote(credential, safe="") not in raw_records

    connections = [record for record in records if record.name == WS_LOGGER and record.msg == "Connecting to %s"]
    assert len(connections) == count
    expected = urlsplit(endpoint)
    for record in connections:
        # Assert the stored logging arguments are already sanitized, rather
        # than relying on a handler or formatter to hide the credential.
        assert isinstance(record.args, tuple)
        assert len(record.args) == 1
        assert isinstance(record.args[0], str)
        url = urlsplit(record.args[0])
        assert url.scheme == expected.scheme
        assert url.netloc == expected.netloc
        assert url.path == "/ws/btc"
        assert parse_qs(url.query) == {"token": ["<redacted>"]}
    assert "Connecting to" in diagnostics
    assert "redacted" in diagnostics


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
def test_finding_8_connection_and_reconnect_logs_redact_tokens(gateway, caplog, asynchronous):
    server, options = gateway
    caplog.set_level(logging.DEBUG, logger="chainflow_data_gateway")
    tokens = []
    reconnects = []
    events = []
    connection_options = {
        "max_retries": 1,
        "initial_delay": 0,
        "websocket_connection_options": {"open_timeout": DEADLINE, "close_timeout": DEADLINE, "max_size": 4096},
    }

    def install(client, token):
        assert_token(token, "btc")
        tokens.append(token.token)
        client.ws_token = token.token

    def refresh(client, minter, event):
        assert event.attempt == 1
        assert event.max_attempts == 1
        assert event.close_code == 1012
        reconnects.append(event)
        install(client, minter.gateway.create_web_socket_token(symbol="btc"))

    async def run_async():
        async with AsyncChainflowDataGateway(**options) as client:
            install(client, await client.gateway.create_web_socket_token(symbol="btc"))
            with ChainflowDataGateway(**options) as minter:

                def reconnect(event):
                    refresh(client, minter, event)

                async with client.web_sockets.connect_price_feed(
                    "btc", on_reconnecting=reconnect, **connection_options
                ) as connection:

                    async def consume(event):
                        events.append(event)

                    connection.on("event", consume)
                    await asyncio.wait_for(connection.dispatch_events(), DEADLINE)

    if asynchronous:
        asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options) as client:
            install(client, client.gateway.create_web_socket_token(symbol="btc"))

            def reconnect(event):
                refresh(client, client, event)

            with client.web_sockets.connect_price_feed(
                "btc", on_reconnecting=reconnect, **connection_options
            ) as connection:
                connection.on("event", events.append)
                connection.dispatch_events()

    assert len(reconnects) == 1
    assert len(tokens) == 2
    assert tokens[0] != tokens[1]
    assert server.accepted == tokens
    assert events == [json.loads(frame.to_json()) for frame in server.frames]
    assert [event["price"] for event in events] == [65000.5, 65010.25]
    for token in tokens:
        assert_exchange(server, "POST", "/auth/ws-token", [MINT_KEY], body={"symbol": "btc"})
        assert_exchange(server, "WS", "/ws/btc", [], query={"token": [token]})
    assert server.requests.empty()
    assert_safe_connection_logs(caplog, options["websocket_base_url"], tokens, count=2)
    messages = [record.getMessage() for record in caplog.records if record.name == WS_LOGGER]
    assert any("Reconnecting to WebSocket API (attempt 1/1)" in message for message in messages)
    assert "Reconnected to WebSocket API" in messages
    option_logs = [message for message in messages if message.startswith("Connection options:")]
    assert len(option_logs) == 2
    assert all("max_size" in message and "4096" in message for message in option_logs)


@pytest.mark.parametrize("asynchronous", [False, True], ids=["sync", "async"])
@pytest.mark.parametrize("credential", ["wrong_scope", "expired"])
def test_finding_8_rejected_upgrade_logs_redact_tokens(gateway, caplog, asynchronous, credential):
    server, options = gateway
    caplog.set_level(logging.DEBUG, logger="chainflow_data_gateway")
    scope = "eth" if credential == "wrong_scope" else "btc"
    expected_status = 403 if credential == "wrong_scope" else 401

    def install(client, token):
        assert_token(token, scope)
        client.ws_token = token.token
        if credential == "expired":
            server.expire(token.token)
        return token.token

    async def run_async():
        async with AsyncChainflowDataGateway(**options) as client:
            token = install(client, await client.gateway.create_web_socket_token(symbol=scope))
            with pytest.raises(InvalidStatus) as refusal:
                async with client.web_sockets.connect_price_feed("btc"):
                    pytest.fail("rejected credential reached an open connection")
            assert refusal.value.response.status_code == expected_status
            return token

    if asynchronous:
        token = asyncio.run(run_async())
    else:
        with ChainflowDataGateway(**options) as client:
            token = install(client, client.gateway.create_web_socket_token(symbol=scope))
            with pytest.raises(InvalidStatus) as refusal:
                with client.web_sockets.connect_price_feed("btc"):
                    pytest.fail("rejected credential reached an open connection")
            assert refusal.value.response.status_code == expected_status

    assert server.accepted == []
    assert_exchange(server, "POST", "/auth/ws-token", [MINT_KEY], body={"symbol": scope})
    assert_exchange(server, "WS", "/ws/btc", [], query={"token": [token]})
    assert server.requests.empty()
    assert_safe_connection_logs(caplog, options["websocket_base_url"], [token], count=1)
