import assert from 'node:assert/strict';
import { once } from 'node:events';
import { AddressInfo } from 'node:net';
import { WebSocket, WebSocketServer } from 'ws';
import { assertNoAdditionalNotifications, WsClient } from './subscribeUtils';

async function subscriptionOverDeliveryRegression(): Promise<void> {
    const server = new WebSocketServer({ host: '127.0.0.1', port: 0 });
    const clients: WsClient[] = [];
    let nextSubscription = 1;
    server.on('connection', peer => {
        const subscriptions = new Set<string>();
        peer.on('message', raw => {
            const request = JSON.parse(raw.toString());
            if (request.method === 'eth_subscribe') {
                const subId = `0x${(nextSubscription++).toString(16)}`;
                subscriptions.add(subId);
                peer.send(JSON.stringify({ jsonrpc: '2.0', id: request.id, result: subId }));
            } else if (request.method === 'eth_unsubscribe') {
                const removed = subscriptions.delete(request.params[0]);
                peer.send(JSON.stringify({ jsonrpc: '2.0', id: request.id, result: removed }));
            }
        });
    });

    function notify(peer: WebSocket, subId: string, sequence: number): void {
        peer.send(JSON.stringify({
            jsonrpc: '2.0',
            method: 'eth_subscription',
            params: { subscription: subId, result: { sequence } },
        }));
    }

    async function connect(): Promise<{ client: WsClient; peer: WebSocket }> {
        const connected = new Promise<WebSocket>(resolve => server.once('connection', resolve));
        const port = (server.address() as AddressInfo).port;
        const client = await WsClient.open(`ws://127.0.0.1:${port}`, 1_000, 1_000);
        clients.push(client);
        return { client, peer: await connected };
    }

    const expected = [{ sequence: 1 }, { sequence: 2 }];
    const observationMs = 100;
    try {
        await once(server, 'listening');
        const { client, peer } = await connect();
        const subId = await client.subscribe(['logs']);
        const otherId = await client.subscribe(['logs']);

        notify(peer, subId, 1);
        notify(peer, subId, 2);
        assert.deepEqual(await client.waitFor(subId, expected.length, 1_000), expected);
        await assertNoAdditionalNotifications(client, subId, observationMs);

        // A notification on the other subscription is a receive barrier for the preceding batch.
        notify(peer, subId, 1);
        notify(peer, subId, 2);
        notify(peer, subId, 3);
        notify(peer, otherId, 10);
        assert.deepEqual(await client.waitFor(otherId, 1, 1_000), [{ sequence: 10 }]);
        assert.deepEqual(await client.waitFor(subId, expected.length, 1_000), expected);
        await assert.rejects(
            assertNoAdditionalNotifications(client, subId, observationMs),
            /unexpected additional notification/,
        );

        notify(peer, subId, 1);
        notify(peer, subId, 2);
        assert.deepEqual(await client.waitFor(subId, expected.length, 1_000), expected);
        const delayedGuard = assertNoAdditionalNotifications(client, subId, observationMs);
        const delayedExtra = setTimeout(() => notify(peer, subId, 3), 25);
        try {
            await assert.rejects(delayedGuard, /unexpected additional notification/);
        } finally {
            clearTimeout(delayedExtra);
        }

        notify(peer, subId, 1);
        notify(peer, subId, 2);
        assert.deepEqual(await client.waitFor(subId, expected.length, 1_000), expected);
        notify(peer, otherId, 11);
        const isolatedGuard = assertNoAdditionalNotifications(client, subId, observationMs);
        const unrelated = setTimeout(() => notify(peer, otherId, 12), 25);
        try {
            await isolatedGuard;
        } finally {
            clearTimeout(unrelated);
        }
        assert.deepEqual(await client.waitFor(otherId, 2, 1_000), [
            { sequence: 11 },
            { sequence: 12 },
        ]);

        for (const sequence of [20, 21, 22, 23]) notify(peer, subId, sequence);
        notify(peer, otherId, 13);
        assert.deepEqual(await client.waitFor(otherId, 1, 1_000), [{ sequence: 13 }]);
        assert.deepEqual(await client.waitFor(subId, 2, 1_000), [
            { sequence: 20 },
            { sequence: 21 },
        ]);
        assert.deepEqual(await client.waitFor(subId, 1, 1_000), [{ sequence: 22 }]);
        assert.deepEqual(await client.waitFor(subId, 1, 1_000), [{ sequence: 23 }]);
        await assertNoAdditionalNotifications(client, subId, observationMs);

        assert.equal(await client.unsubscribe(otherId), true);
        await assert.rejects(
            assertNoAdditionalNotifications(client, otherId, observationMs),
            /unknown subscription/,
        );
        await assert.rejects(
            assertNoAdditionalNotifications(client, '0xunknown', observationMs),
            /unknown subscription/,
        );

        const failedGuard = assertNoAdditionalNotifications(client, subId, observationMs);
        const disconnect = setTimeout(() => peer.terminate(), 25);
        try {
            await assert.rejects(failedGuard, /WsClient: connection (closed|error)/);
        } finally {
            clearTimeout(disconnect);
        }
        await assert.rejects(
            assertNoAdditionalNotifications(client, subId, observationMs),
            /WsClient: connection (closed|error)/,
        );

        const malformed = await connect();
        const malformedId = await malformed.client.subscribe(['logs']);
        const malformedGuard = assertNoAdditionalNotifications(
            malformed.client,
            malformedId,
            observationMs,
        );
        malformed.peer.send('invalid JSON');
        await assert.rejects(malformedGuard, /WsClient: invalid JSON message/);
    } finally {
        clients.forEach(client => client.close());
        server.clients.forEach(peer => peer.terminate());
        await new Promise<void>((resolve, reject) =>
            server.close(error => error ? reject(error) : resolve()),
        );
    }
}

describe('subscribeUtils', function () {
    it(subscriptionOverDeliveryRegression.name, subscriptionOverDeliveryRegression);
});
