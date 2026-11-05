import { expect } from 'chai';
import assert from 'node:assert/strict';
import { ethers } from 'ethers';
import { sleep } from './chainUtils';
import { ADDRESS, HASH32, HEX_QUANTITY, HEX_DATA, BLOOM256, NONCE8, OPAQUE_HEX_ID } from './format';
import { ZERO_HASH, ZERO_NONCE } from './constants';

interface PendingCall {
    resolve: (value: any) => void;
    reject: (error: any) => void;
    timer: ReturnType<typeof setTimeout>;
    method: string;
}

export class WsClient {
    private nextId = 1;
    private readonly pending = new Map<number, PendingCall>();
    private readonly buffers = new Map<string, any[]>();
    private failure: Error | undefined;

    private constructor(
        private readonly socket: any,
        private readonly callTimeoutMs: number,
        private readonly maxBuffer: number,
    ) {
        socket.onmessage = (ev: any) => this.onMessage(String(ev.data));
        socket.onclose = () => this.fail(new Error('WsClient: connection closed'));
        socket.onerror = () => this.fail(new Error('WsClient: connection error'));
    }

    static open(url: string, timeoutMs = 10_000, callTimeoutMs = 10_000, maxBuffer = 1024): Promise<WsClient> {
        if (![timeoutMs, callTimeoutMs, maxBuffer].every(n => Number.isSafeInteger(n) && n > 0)) {
            throw new Error('WsClient: deadlines and buffer capacity must be positive integers');
        }
        const Impl: any = (globalThis as any).WebSocket;
        if (!Impl) throw new Error('global WebSocket unavailable (Node >= 21 required)');
        return new Promise<WsClient>((resolve, reject) => {
            const socket = new Impl(url);
            const client = new WsClient(socket, callTimeoutMs, maxBuffer);
            const timer = setTimeout(() => {
                client.close();
                reject(new Error('WsClient.open: connection deadline exceeded'));
            }, timeoutMs);
            socket.onopen = () => { clearTimeout(timer); resolve(client); };
            socket.onerror = () => {
                clearTimeout(timer);
                client.close();
                reject(new Error('WsClient.open: connection error'));
            };
            socket.onclose = () => {
                clearTimeout(timer);
                client.fail(new Error('WsClient: connection closed'));
                reject(client.failure);
            };
        });
    }

    private fail(error: Error): void {
        if (!this.failure) this.failure = error;
        for (const pending of this.pending.values()) {
            clearTimeout(pending.timer);
            pending.reject(this.failure);
        }
        this.pending.clear();
        this.buffers.clear();
    }

    private onMessage(raw: string): void {
        if (this.failure) return;
        let msg: any;
        try { msg = JSON.parse(raw); } catch {
            this.fail(new Error('WsClient: invalid JSON message'));
            this.socket.close();
            return;
        }
        if (!msg || typeof msg !== 'object') {
            this.fail(new Error('WsClient: invalid RPC message'));
            this.socket.close();
            return;
        }
        if (msg.id !== undefined && this.pending.has(msg.id)) {
            const pending = this.pending.get(msg.id)!;
            clearTimeout(pending.timer);
            this.pending.delete(msg.id);
            if (msg.error) {
                pending.reject(msg.error);
            } else if (pending.method === 'eth_subscribe') {
                if (typeof msg.result !== 'string' || this.buffers.has(msg.result)) {
                    pending.reject(new Error('WsClient: invalid subscription id'));
                    this.fail(new Error('WsClient: invalid subscription id'));
                    this.socket.close();
                } else {
                    // Install before resolving: a notification may immediately follow the reply.
                    this.buffers.set(msg.result, []);
                    pending.resolve(msg.result);
                }
            } else {
                pending.resolve(msg.result);
            }
            return;
        }
        if (msg.method === 'eth_subscription' && msg.params) {
            const buf = this.buffers.get(msg.params.subscription);
            // Ignore late notifications after unsubscribe; never allocate an unsolicited buffer.
            if (!buf) return;
            if (buf.length >= this.maxBuffer) {
                this.fail(new Error('WsClient: subscription buffer overflow'));
                this.socket.close();
                return;
            }
            buf.push(msg.params.result);
        }
    }

    private call(method: string, params: unknown[]): Promise<any> {
        if (this.failure) return Promise.reject(this.failure);
        const id = this.nextId++;
        return new Promise((resolve, reject) => {
            const timer = setTimeout(() => {
                this.fail(new Error(`WsClient: ${method} deadline exceeded`));
                this.socket.close();
            }, this.callTimeoutMs);
            this.pending.set(id, { resolve, reject, timer, method });
            try {
                this.socket.send(JSON.stringify({ jsonrpc: '2.0', id, method, params }));
            } catch {
                this.fail(new Error(`WsClient: ${method} send failed`));
                this.socket.close();
            }
        });
    }

    /** eth_subscribe; starts buffering before its promise resolves. */
    subscribe(params: unknown[]): Promise<string> {
        return this.call('eth_subscribe', params);
    }

    /** Consume exactly `want` notifications, rejecting disconnects and released subscriptions. */
    async waitFor(subId: string, want: number, timeoutMs = 30_000): Promise<any[]> {
        if (!Number.isSafeInteger(want) || want < 1 || want > this.maxBuffer) {
            throw new Error('WsClient.waitFor: invalid notification count');
        }
        const deadline = Date.now() + timeoutMs;
        for (;;) {
            if (this.failure) throw this.failure;
            const buf = this.buffers.get(subId);
            if (!buf) throw new Error('WsClient.waitFor: unknown subscription');
            if (buf.length >= want) return buf.splice(0, want);
            if (Date.now() >= deadline) {
                throw new Error(`WsClient.waitFor: got ${buf.length}/${want} notifications before timeout`);
            }
            await sleep(Math.min(25, Math.max(1, deadline - Date.now())));
        }
    }

    async unsubscribe(subId: string): Promise<boolean> {
        try {
            return await this.call('eth_unsubscribe', [subId]);
        } finally {
            this.buffers.delete(subId);
        }
    }

    close(): void {
        this.fail(new Error('WsClient: connection closed'));
        this.socket.close();
    }
}

/** Subscription ids are opaque random hex, not minimally-encoded quantities. */
export const SUBSCRIPTION_ID = OPAQUE_HEX_ID;

/** Every field Pax pushes in an eth_newHeads notification (rpc/subscribe.go encodeCommittedBlock). */
export const NEW_HEAD_FIELDS = [
    'baseFeePerGas',
    'blobGasUsed',
    'difficulty',
    'excessBlobGas',
    'extraData',
    'gasLimit',
    'gasUsed',
    'hash',
    'logsBloom',
    'miner',
    'mixHash',
    'nonce',
    'number',
    'parentBeaconBlockRoot',
    'parentHash',
    'receiptsRoot',
    'sha3Uncles',
    'stateRoot',
    'size',
    'timestamp',
    'transactionsRoot',
    'withdrawalsRoot',
] as const;

const HEAD_QUANTITY_FIELDS = [
    'size',
    'number',
    'gasLimit',
    'gasUsed',
    'timestamp',
    'difficulty',
    'baseFeePerGas',
    'blobGasUsed',
    'excessBlobGas',
] as const;

const HEAD_HASH_FIELDS = [
    'hash',
    'parentHash',
    'receiptsRoot',
    'transactionsRoot',
    'stateRoot',
    'sha3Uncles',
    'mixHash',
    'parentBeaconBlockRoot',
    'withdrawalsRoot',
] as const;

/** Assert a newHeads payload carries exactly the documented fields, each canonically encoded. */
export function assertNewHeadSchema(head: any): void {
    assert.deepEqual(Object.keys(head).sort(), [...NEW_HEAD_FIELDS].sort(), 'complete newHeads schema');
    HEAD_QUANTITY_FIELDS.forEach(f =>
        assert.match(head[f], HEX_QUANTITY, `newHead.${f} is a canonical quantity`),
    );
    HEAD_HASH_FIELDS.forEach(f => assert.match(head[f], HASH32, `newHead.${f} is a 32-byte hash`));
    assert.match(head.miner, ADDRESS, 'newHead.miner');
    assert.match(head.logsBloom, BLOOM256, 'newHead.logsBloom');
    assert.match(head.nonce, NONCE8, 'newHead.nonce');
    assert.match(head.extraData, HEX_DATA, 'newHead.extraData');
}

/** The PoW/PoS-only header fields Pax cannot populate must be their canonical zero values. */
export function assertNewHeadInapplicableZeros(head: any): void {
    expect(BigInt(head.difficulty), 'difficulty is 0 (inapplicable)').to.equal(0n);
    expect(head.sha3Uncles, 'sha3Uncles is the zero hash').to.equal(ZERO_HASH);
    expect(head.mixHash, 'mixHash is the zero hash').to.equal(ZERO_HASH);
    expect(head.nonce, 'nonce is the zero block nonce').to.equal(ZERO_NONCE);
    expect(head.extraData, 'extraData is empty').to.equal('0x');
}

/** Compare every canonical header field at the notification's own height. */
export function assertNewHeadMatchesBlock(head: any, block: any): void {
    assertNewHeadSchema(head);
    for (const field of NEW_HEAD_FIELDS) {
        assert.deepEqual(head[field], block[field], `newHead.${field} matches the same-height canonical block`);
    }
}

/** A successor's fee cannot prove the fee of the block that was announced. */
export async function assertNewHeadBaseFeeTracksChain(
    head: any,
    provider: ethers.JsonRpcProvider,
): Promise<void> {
    const canonical = await provider.send('eth_getBlockByNumber', [head.number, false]);
    assert.notEqual(canonical, null, `canonical block exists for ${head.number}`);
    assertNewHeadMatchesBlock(head, canonical);
}
