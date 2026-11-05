import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { createHash, randomUUID } from 'node:crypto';
import { Worker } from 'node:worker_threads';
import { ethers } from 'ethers';
import { expect } from 'chai';
import { RuntimeStatePath, Endpoints } from '../config/endpoints';
import { EvmAccount } from './evmUtils';
import { JsonRpcEnvelope } from './chainUtils';
import { uint256Word } from './format';

/**
 * Runtime state captured once by _start/00_bootstrap.spec.ts and read by every other spec; the
 * typed contract turns a missing field into a TypeScript error, not a runtime undefined. Add fields
 * here, but never write back from a non-bootstrap spec — the bootstrap is the only writer.
 */
export interface RuntimeState {
    /** ISO timestamp when the state was written. */
    bootstrappedAt: string;

    /** Chain IDs as integers. */
    chainIds: {
        pax: number;
        eth: number;
    };

    /** Block numbers captured at well-defined points in the bootstrap. */
    blocks: {
        /** Pax block just before any contracts were deployed. */
        paxBeforeDeploy: number;
        /** Pax block in which TestERC20 was deployed. */
        paxErc20Deploy: number;
        /** Pax block just after the bootstrap finished. */
        paxAfterDeploy: number;
        /** geth reference head when the bootstrap ran. */
        ethAtBootstrap: number;
        /** geth block in which the mirrored TestERC20 was deployed. */
        ethErc20Deploy: number;
    };

    /** Deployed contract addresses, one per reference chain. */
    contracts: {
        /** TestERC20 on Pax. */
        erc20: string;
        /** The same TestERC20 deployed on the geth reference, for parity tests. */
        erc20Geth: string;
        /** SimpleAccount7702 delegation target on Pax (used by EIP-7702 specs). */
        simpleAccount7702: string;
        /** RealGasBurner on Pax: lets specs burn arbitrary gas to move the base fee. */
        gasBurner: string;
    };

    /**
     * CW20 + its EVM (ERC20) pointer, populated only when wasm is enabled on the chain.
     * Absent on wasm-disabled chains, so consumers must treat it as optional and skip the
     * dual-VM / pointer paths when it is undefined.
     */
    wasm?: {
        cw20: string;
        cw20Pointer: string;
        actor: { address: string; privateKey: string };
    };

    /** EVM addresses pre-funded with a small balance, ready for use by tests. */
    funded: {
        admin: string;
        adminMnemonic: string;
        /**
         * Deployer/owner of the geth-side TestERC20. Funded from geth's unlocked dev
         * account; this key is controlled client-side so specs can sign geth txs.
         */
        gethAdmin: { address: string; privateKey: string };
        /** Pool of fresh accounts the bootstrap funded but has not transacted from. */
        pool: { address: string; privateKey: string }[];
    };
}

interface ChainSnapshot {
    endpoint: string;
    chainId: string;
    genesis: string;
    anchor: { number: string; hash: string };
    codes: { address: string; code: string }[];
}
interface SnapshotRequest {
    endpoints: string[];
    contracts: string[][];
    anchors?: { number: string; hash: string }[];
}
interface RuntimeBinding {
    chains: { endpoint: string; chainId: string; genesis: string;
        anchor: { number: string; hash: string }; codes: { address: string; hash: string }[] }[];
}
interface StoredRuntime {
    schemaVersion: 1;
    state: RuntimeState;
    binding: RuntimeBinding;
}

function bootstrapRequired(): Error {
    return new Error('RPC fixture state is invalid or stale; bootstrap required: run yarn rpc:bootstrap');
}

/** The default secret store is outside the checkout and all uploaded report trees. */
function stateAbsPath(): string {
    const suite = createHash('sha256').update(process.cwd()).digest('hex').slice(0, 24);
    const abs = RuntimeStatePath === 'runtime/runtime.json'
        ? path.join(os.homedir(), '.pax-rpc-tests', suite, 'runtime.json')
        : path.resolve(process.cwd(), RuntimeStatePath);
    if (abs.split(path.sep).some(part => /^(reports?|artifacts?|test-results)$/i.test(part))) {
        throw new Error('RPC fixture secrets must be stored outside report and artifact directories');
    }
    const dir = path.dirname(abs);
    if ([process.cwd(), os.homedir(), os.tmpdir(), path.parse(dir).root].includes(dir)) {
        throw new Error('RPC fixture state requires a dedicated private directory');
    }
    return abs;
}

/** Validate every field consumed by the specs before exposing keys or contract addresses. */
function validateRuntimeState(state: any): asserts state is RuntimeState {
    const address = (value: any) => typeof value === 'string' && /^0x[0-9a-fA-F]{40}$/.test(value);
    const height = (value: any) => Number.isSafeInteger(value) && value >= 0;
    const account = (value: any) => value && address(value.address) && typeof value.privateKey === 'string' &&
        /^0x[0-9a-fA-F]{64}$/.test(value.privateKey) &&
        new ethers.Wallet(value.privateKey).address.toLowerCase() === value.address.toLowerCase();
    if (!state || typeof state !== 'object' || typeof state.bootstrappedAt !== 'string' ||
        !Number.isFinite(Date.parse(state.bootstrappedAt)) || !state.chainIds ||
        !Number.isSafeInteger(state.chainIds.pax) || state.chainIds.pax < 1 ||
        !Number.isSafeInteger(state.chainIds.eth) || state.chainIds.eth < 1 || !state.blocks ||
        !['paxBeforeDeploy', 'paxErc20Deploy', 'paxAfterDeploy', 'ethAtBootstrap', 'ethErc20Deploy']
            .every(field => height(state.blocks[field])) ||
        state.blocks.paxAfterDeploy < state.blocks.paxErc20Deploy ||
        state.blocks.paxErc20Deploy < state.blocks.paxBeforeDeploy || !state.contracts ||
        !['erc20', 'erc20Geth', 'simpleAccount7702', 'gasBurner'].every(field => address(state.contracts[field])) ||
        !state.funded || !address(state.funded.admin) || typeof state.funded.adminMnemonic !== 'string' ||
        !ethers.Mnemonic.isValidMnemonic(state.funded.adminMnemonic) || !account(state.funded.gethAdmin) ||
        !Array.isArray(state.funded.pool) || state.funded.pool.length === 0 || !state.funded.pool.every(account) ||
        (state.wasm !== undefined && (!state.wasm || !/^pax1[0-9a-z]+$/.test(state.wasm.cw20) ||
            !address(state.wasm.cw20Pointer) || !account(state.wasm.actor)))) {
        throw bootstrapRequired();
    }
}

/** The worker's sole snapshot producer, kept as JavaScript to avoid loader closure helpers. */
const RUNTIME_SNAPSHOT_SOURCE = String.raw`async function runtimeChainSnapshot(request) {
    return Promise.all(request.endpoints.map(async (endpoint, index) => {
        let id = 0;
        const rpc = async (method, params) => {
            const response = await fetch(endpoint, {
                method: 'POST', headers: { 'content-type': 'application/json' },
                body: JSON.stringify({ jsonrpc: '2.0', id: ++id, method, params }),
                signal: AbortSignal.timeout(10_000),
            });
            if (!response.ok) throw new Error('chain snapshot transport failed');
            const body = await response.json();
            if (body.jsonrpc !== '2.0' || body.id !== id || body.error || body.result === undefined) {
                throw new Error('chain snapshot RPC failed');
            }
            return body.result;
        };
        const chainId = await rpc('eth_chainId', []);
        // Height one exists on both the Cosmos-backed EVM and the geth reference.
        const genesis = await rpc('eth_getBlockByNumber', ['0x1', false]);
        const anchor = await rpc('eth_getBlockByNumber', [request.anchors?.[index].number ?? 'latest', false]);
        if (!/^0x[0-9a-fA-F]{64}$/.test(genesis?.hash) ||
            !/^0x[0-9a-fA-F]{64}$/.test(anchor?.hash) ||
            !/^0x[0-9a-f]+$/.test(anchor?.number) || !/^0x[0-9a-f]+$/.test(chainId)) {
            throw new Error('chain snapshot identity is unavailable');
        }
        const codes = [];
        // Keep request ids serial and verify each deployed object's current code.
        for (const address of request.contracts[index]) {
            const code = await rpc('eth_getCode', [address, 'latest']);
            if (typeof code !== 'string' || !/^0x(?:[0-9a-fA-F]{2})+$/.test(code)) {
                throw new Error('chain snapshot contract code is unavailable');
            }
            codes.push({ address, code });
        }
        return { endpoint, chainId, genesis: genesis.hash, anchor: { number: anchor.number, hash: anchor.hash }, codes };
    }));
}`;

/** Preserve the synchronous API while the worker owns all asynchronous network I/O. */
function queryRuntimeSnapshot(request: SnapshotRequest): ChainSnapshot[] {
    const shared = new SharedArrayBuffer(256 * 1024);
    const control = new Int32Array(shared, 0, 2);
    const worker = new Worker(`
        const { workerData } = require('node:worker_threads');
        const control = new Int32Array(workerData.shared, 0, 2);
        const finish = (value) => {
            let bytes = new TextEncoder().encode(JSON.stringify(value));
            if (bytes.length > workerData.shared.byteLength - 8) {
                bytes = new TextEncoder().encode(JSON.stringify({ error: true }));
            }
            new Uint8Array(workerData.shared, 8, bytes.length).set(bytes);
            Atomics.store(control, 1, bytes.length);
            Atomics.store(control, 0, 1);
            Atomics.notify(control, 0);
        };
        (${RUNTIME_SNAPSHOT_SOURCE})(workerData.request).then(
            chains => finish({ chains }), () => finish({ error: true }),
        );
    `, { eval: true, workerData: { shared, request } });
    // A startup failure must not become an unhandled worker error or leak request details.
    worker.on('error', () => undefined);
    try {
        if (Atomics.wait(control, 0, 0, 15_000) === 'timed-out') throw bootstrapRequired();
        const size = Atomics.load(control, 1);
        if (size < 1 || size > shared.byteLength - 8) throw bootstrapRequired();
        const result = JSON.parse(new TextDecoder().decode(new Uint8Array(shared, 8, size)));
        if (result.error || !Array.isArray(result.chains)) throw bootstrapRequired();
        return result.chains;
    } finally {
        void worker.terminate();
    }
}

function captureRuntimeBinding(state: RuntimeState, previous?: RuntimeBinding): RuntimeBinding {
    const endpoints = [Endpoints.pax.evmRpc, Endpoints.eth.geth];
    if (previous && (previous.chains?.length !== 2 ||
        previous.chains.some((chain, index) => chain.endpoint !== endpoints[index]))) throw bootstrapRequired();
    const contracts = [
        [state.contracts.erc20, state.contracts.simpleAccount7702, state.contracts.gasBurner,
            ...(state.wasm ? [state.wasm.cw20Pointer] : [])],
        [state.contracts.erc20Geth],
    ];
    const snapshots = queryRuntimeSnapshot({ endpoints, contracts, anchors: previous?.chains.map(chain => chain.anchor) });
    const chains = snapshots.map((chain, index) => {
        if (chain.chainId !== ethers.toQuantity(index === 0 ? state.chainIds.pax : state.chainIds.eth)) throw bootstrapRequired();
        return { ...chain, codes: chain.codes.map(entry => ({ address: entry.address, hash: ethers.keccak256(entry.code) })) };
    });
    const binding = { chains };
    if (previous && JSON.stringify(previous) !== JSON.stringify(binding)) throw bootstrapRequired();
    return binding;
}

/** Reject symlinks and repair modes on legacy files before reading or replacing them. */
function secureRuntimeDirectory(abs: string): void {
    const dir = path.dirname(abs);
    // Check every existing ancestor before mkdir or chmod can follow a symlink.
    let ancestor = dir;
    for (;;) {
        if (fs.existsSync(ancestor) && fs.lstatSync(ancestor).isSymbolicLink()) throw bootstrapRequired();
        const parent = path.dirname(ancestor);
        if (parent === ancestor) break;
        ancestor = parent;
    }
    fs.mkdirSync(dir, { recursive: true, mode: 0o700 });
    fs.chmodSync(dir, 0o700);
    if (fs.existsSync(abs)) {
        const stat = fs.lstatSync(abs);
        if (!stat.isFile() || stat.isSymbolicLink() || stat.nlink !== 1) throw bootstrapRequired();
        fs.chmodSync(abs, 0o600);
    }
}

export function writeRuntimeState(state: RuntimeState): void {
    validateRuntimeState(state);
    const abs = stateAbsPath();
    secureRuntimeDirectory(abs);
    const binding = captureRuntimeBinding(state);
    const tmp = path.join(path.dirname(abs), `.runtime-${randomUUID()}.tmp`);
    let fd: number | undefined;
    try {
        fd = fs.openSync(tmp, fs.constants.O_WRONLY | fs.constants.O_CREAT | fs.constants.O_EXCL | fs.constants.O_NOFOLLOW, 0o600);
        fs.fchmodSync(fd, 0o600);
        fs.writeFileSync(fd, JSON.stringify({ schemaVersion: 1, state, binding }), 'utf-8');
        fs.fsyncSync(fd);
        fs.closeSync(fd);
        fd = undefined;
        fs.renameSync(tmp, abs);
        const directory = fs.openSync(path.dirname(abs), fs.constants.O_RDONLY | fs.constants.O_DIRECTORY);
        try { fs.fsyncSync(directory); } finally { fs.closeSync(directory); }
    } finally {
        if (fd !== undefined) fs.closeSync(fd);
        if (fs.existsSync(tmp)) fs.unlinkSync(tmp);
    }
}

/** No process cache: every read revalidates schema, endpoint, chain instance and deployed code. */
export function readRuntimeState(): RuntimeState {
    try {
        const abs = stateAbsPath();
        secureRuntimeDirectory(abs);
        const fd = fs.openSync(abs, fs.constants.O_RDONLY | fs.constants.O_NOFOLLOW);
        let stored: StoredRuntime;
        try { stored = JSON.parse(fs.readFileSync(fd, 'utf-8')); } finally { fs.closeSync(fd); }
        if (stored.schemaVersion !== 1) throw bootstrapRequired();
        validateRuntimeState(stored.state);
        if (!stored.binding) throw bootstrapRequired();
        captureRuntimeBinding(stored.state, stored.binding);
        return stored.state;
    } catch {
        // Never include parsed state, RPC errors, mnemonics or keys in reporter output.
        throw bootstrapRequired();
    }
}

/**
 * Assert two JSON-RPC envelopes carry byte-identical errors (code, message and data).
 * Used by the parity specs to prove Pax and the geth reference fail the exact same way.
 */
export function expectSameError(s: JsonRpcEnvelope, g: JsonRpcEnvelope): void {
    expect(g.error, `geth must error, got result ${JSON.stringify(g.result)}`).to.not.equal(
        undefined,
    );
    expect(s.error, `pax must error, got result ${JSON.stringify(s.result)}`).to.not.equal(
        undefined,
    );
    expect(s.error!.code, 'error.code parity').to.equal(g.error!.code);
    expect(s.error!.message, 'error.message parity').to.equal(g.error!.message);
    expect(s.error!.data, 'error.data parity').to.deep.equal(g.error!.data);
}

// The suite runs serially in a single process (see .mocharc.run.json), so a module-level
// cursor can hand every claimPool call a fresh, non-overlapping range of the pool.
let poolCursor = 0;

/**
 * Claim `count` accounts from the pre-funded pool, allocating a disjoint slice on every call so no
 * two specs ever share a key (the old salted-offset scheme could wrap and overlap, reusing accounts
 * and causing nonce collisions, balance drain and flaky heavy-block / fee-market failures). Throws
 * when exhausted rather than wrapping (which would reintroduce overlap); bump POOL_SIZE in
 * _start/00_bootstrap.spec.ts when adding hungry specs. `label` is diagnostics-only; accounts are
 * returned connected to `provider`.
 */
export function claimPool(
    runtime: RuntimeState,
    provider: ethers.JsonRpcProvider,
    count: number,
    label: string,
): EvmAccount[] {
    const pool = runtime.funded.pool;
    if (poolCursor + count > pool.length) {
        throw new Error(
            `claimPool('${label}', count=${count}): pre-funded pool exhausted ` +
                `(used ${poolCursor}/${pool.length}). Increase POOL_SIZE in _start/00_bootstrap.spec.ts.`,
        );
    }
    const start = poolCursor;
    poolCursor += count;
    return Array.from({ length: count }, (_, i) =>
        EvmAccount.fromPrivateKey(pool[start + i].privateKey, provider),
    );
}

/** Left-pad a uint into its canonical 32-byte ABI word. */
export const encodeUint = uint256Word;

/** Calldata encoders and result decoders bound to a specific ERC20 ABI. */
export class Erc20Calldata {
    constructor(private readonly iface: ethers.Interface) {}

    balanceOf(holder: string): string {
        return this.iface.encodeFunctionData('balanceOf', [holder]);
    }

    transfer(to: string, amount: bigint): string {
        return this.iface.encodeFunctionData('transfer', [to, amount]);
    }

    decodeBalance(hex: string): bigint {
        return this.iface.decodeFunctionResult('balanceOf', hex)[0] as bigint;
    }
}
