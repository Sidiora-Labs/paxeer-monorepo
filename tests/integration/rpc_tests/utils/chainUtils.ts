import { ethers } from 'ethers';
import { Endpoints } from '../config/endpoints';
import { QueryParamsRequest, QueryParamsResponse } from 'cosmjs-types/cosmos/params/v1beta1/query';
import { assertMergedEvmChainId } from './constants';

const POLLING_INTERVAL_MS = Number(process.env.RPC_POLLING_INTERVAL_MS ?? 100);

const makeProvider = (url: string): ethers.JsonRpcProvider =>
    new ethers.JsonRpcProvider(url, undefined, {
        batchMaxCount: 1,
        staticNetwork: true,
        pollingInterval: POLLING_INTERVAL_MS,
    });

let paxProvider: ethers.JsonRpcProvider | undefined;
let gethProvider: ethers.JsonRpcProvider | undefined;

export function paxRpc(): ethers.JsonRpcProvider {
    if (!paxProvider) paxProvider = makeProvider(Endpoints.pax.evmRpc);
    return paxProvider;
}

/** Primary Ethereum reference: local geth --dev. */
export function gethRpc(): ethers.JsonRpcProvider {
    if (!gethProvider) gethProvider = makeProvider(Endpoints.eth.geth);
    return gethProvider;
}

/**
 * Pax + the primary geth reference. `eth` aliases the geth provider so existing
 * specs keep working after the fork→geth reference switch.
 */
export function bothProviders(): {
    pax: ethers.JsonRpcProvider;
    geth: ethers.JsonRpcProvider;
    eth: ethers.JsonRpcProvider;
} {
    const pax = paxRpc();
    const geth = gethRpc();
    return { pax, geth, eth: geth };
}

export async function isReachable(url: string, timeoutMs = 2_500): Promise<boolean> {
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), timeoutMs);
    try {
        const res = await fetch(url, {
            method: 'POST',
            headers: { 'content-type': 'application/json' },
            body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'eth_chainId', params: [] }),
            signal: controller.signal,
        });
        if (!res.ok) return false;
        const body = (await res.json()) as { result?: string };
        return typeof body.result === 'string';
    } catch {
        return false;
    } finally {
        clearTimeout(timer);
    }
}

export interface JsonRpcError {
    code: number;
    message: string;
    data?: unknown;
}

export interface JsonRpcEnvelope<T = unknown> {
    jsonrpc: '2.0';
    id: number | string | null;
    result?: T;
    error?: JsonRpcError;
}

/**
 * Raw JSON-RPC POST that bypasses ethers' client-side validation. Ethers v6 normalizes
 * addresses, hexlifies `data`, and re-wraps non-array `params`; negative tests need the
 * malformed bytes to reach the node untouched to verify the *node's* validation, not the
 * client's. Returns the raw envelope.
 */
export interface RequestDeadline {
    timeoutMs?: number;
    deadline?: number;
    signal?: AbortSignal;
}

export async function requestJson<T = unknown>(
    url: string,
    init: RequestInit = {},
    options: RequestDeadline = {},
): Promise<T> {
    const timeout = Math.min(
        options.timeoutMs ?? 10_000,
        (options.deadline ?? Infinity) - Date.now(),
    );
    if (!(timeout > 0) || options.signal?.aborted) throw new Error('RPC request deadline expired');
    const controller = new AbortController();
    const abort = () => controller.abort();
    options.signal?.addEventListener('abort', abort, { once: true });
    const timer = setTimeout(abort, timeout);
    try {
        const res = await fetch(url, { ...init, signal: controller.signal });
        if (!res.ok) throw new Error(`RPC HTTP ${res.status} from ${url}`);
        return (await res.json()) as T;
    } finally {
        clearTimeout(timer);
        options.signal?.removeEventListener('abort', abort);
    }
}

export async function rawJsonRpc<T = unknown>(
    url: string,
    method: string,
    params: unknown,
    id: number | string = 1,
    options: RequestDeadline = {},
): Promise<JsonRpcEnvelope<T>> {
    return requestJson<JsonRpcEnvelope<T>>(
        url,
        {
            method: 'POST',
            headers: { 'content-type': 'application/json' },
            body: JSON.stringify({ jsonrpc: '2.0', id, method, params }),
        },
        options,
    );
}

export const rawPax = <T = unknown>(
    method: string,
    params: unknown,
    options: RequestDeadline = {},
) => rawJsonRpc<T>(Endpoints.pax.evmRpc, method, params, 1, options);

/** Raw POST to the primary geth reference. */
export const rawGeth = <T = unknown>(
    method: string,
    params: unknown,
    options: RequestDeadline = {},
) => rawJsonRpc<T>(Endpoints.eth.geth, method, params, 1, options);

/** Raw POST to a keyless node (hosted RPC) — used to observe the empty-account case. */
export const rawAccountless = <T = unknown>(
    method: string,
    params: unknown,
    options: RequestDeadline = {},
) => rawJsonRpc<T>(Endpoints.accountless, method, params, 1, options);

/**
 * Resolve a promise expected to throw an ethers RPC error and return the underlying
 * JSON-RPC envelope. Unwraps both `e.info.error` (ethers v6 default) and `e.error`
 * (older shapes). Throws if the promise resolved or the error carried no RPC envelope.
 */
export async function captureRpcError(promise: Promise<unknown>): Promise<JsonRpcError> {
    try {
        await promise;
    } catch (e: any) {
        const env = e?.info?.error ?? e?.error;
        if (env && typeof env.code === 'number') {
            return env as JsonRpcError;
        }
        throw new Error(
            `captureRpcError: thrown error did not carry an RPC envelope: ${e?.message ?? e}`,
        );
    }
    throw new Error('captureRpcError: expected promise to reject but it resolved');
}

/**
 * Assert a raw JSON-RPC envelope carries an error matching `code` and (optionally)
 * `messagePattern`; returns the error. Throws a descriptive Error (not a chai assertion)
 * so the message includes the whole envelope. Use this for raw-transport negative tests
 * where you POST malformed payloads directly.
 */
export function expectJsonRpcError(
    envelope: JsonRpcEnvelope,
    code: number,
    messagePattern?: RegExp,
): JsonRpcError {
    const err = envelope.error;
    if (!err) {
        throw new Error(
            `expectJsonRpcError: expected an error but got result: ${JSON.stringify(
                envelope.result,
            )}`,
        );
    }
    if (err.code !== code) {
        throw new Error(
            `expectJsonRpcError: expected code ${code} but got ${err.code} (message: ${err.message})`,
        );
    }
    if (messagePattern && !messagePattern.test(err.message)) {
        throw new Error(
            `expectJsonRpcError: message ${JSON.stringify(
                err.message,
            )} did not match ${messagePattern}`,
        );
    }
    return err;
}

export const sleep = (ms: number): Promise<void> =>
    new Promise((resolve) => setTimeout(resolve, ms));

/**
 * Poll `fn` until it returns truthy or the timeout elapses; returns the truthy value or
 * throws. For short, deterministic guards (wait for the next Pax block, etc.), not retries.
 */
export async function waitUntil<T>(
    fn: (signal: AbortSignal, deadline: number) => Promise<T | undefined | null>,
    opts: { timeoutMs: number; intervalMs?: number; label?: string } = { timeoutMs: 30_000 },
): Promise<T> {
    const interval = opts.intervalMs ?? 250;
    const deadline = Date.now() + opts.timeoutMs;
    const controller = new AbortController();
    let lastError: unknown;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const expired = new Promise<never>((_, reject) => {
        timer = setTimeout(() => {
            controller.abort();
            reject(
                new Error(
                    `waitUntil(${opts.label ?? 'condition'}) timed out after ${opts.timeoutMs}ms` +
                        (lastError ? `: ${String(lastError)}` : ''),
                ),
            );
        }, opts.timeoutMs);
    });
    try {
        while (Date.now() < deadline) {
            try {
                const result = await Promise.race([fn(controller.signal, deadline), expired]);
                if (
                    Date.now() < deadline &&
                    result !== undefined &&
                    result !== null &&
                    result !== false
                ) {
                    return result as T;
                }
            } catch (e) {
                if (controller.signal.aborted) throw e;
                lastError = e;
            }
            await Promise.race([
                sleep(Math.min(interval, Math.max(0, deadline - Date.now()))),
                expired,
            ]);
        }
        return await expired;
    } finally {
        clearTimeout(timer);
        controller.abort();
    }
}

/** EIP-1559 fee-market parameters as the chain applies them. */
export interface Eip1559Params {
    blockGasLimit: number;
    targetGasUsedPerBlock: number;
    maxUpwardAdjustment: number;
    maxDownwardAdjustment: number;
    minFeePerGas: number;
    maxFeePerGas: number;
}

/** Read one successful JSON-RPC result, keeping transport and protocol failures mandatory. */
export async function rpcResult(
    url: string,
    method: string,
    params: unknown,
    options: RequestDeadline = {},
): Promise<any> {
    const envelope = await rawJsonRpc(url, method, params, 1, options);
    if (envelope.error || envelope.result === undefined || envelope.result === null) {
        throw new Error(`${method}: ${JSON.stringify(envelope.error ?? envelope)}`);
    }
    return envelope.result;
}

/** Bind the configured EVM and Cosmos transports to the same committed block. */
export async function queryPaxChainHeight(
    height?: number,
    options: RequestDeadline = {},
): Promise<number> {
    assertMergedEvmChainId(await rpcResult(Endpoints.pax.evmRpc, 'eth_chainId', [], options));
    const params = height === undefined ? {} : { height: String(height) };
    const cosmos = await rpcResult(Endpoints.pax.cosmosRpc, 'block', params, options);
    const pinned = Number(cosmos.block.header.height);
    if (
        !Number.isSafeInteger(pinned) ||
        pinned <= 0 ||
        (height !== undefined && pinned !== height)
    ) {
        throw new Error('Invalid Cosmos block height');
    }
    const evm = await rpcResult(
        Endpoints.pax.evmRpc,
        'eth_getBlockByNumber',
        [ethers.toQuantity(pinned), false],
        options,
    );
    const hash = cosmos.block_id.hash;
    if (
        typeof hash !== 'string' ||
        !/^[0-9a-fA-F]{64}$/.test(hash) ||
        String(evm.hash).toLowerCase() !== `0x${hash.toLowerCase()}` ||
        Number(evm.number) !== pinned
    ) {
        throw new Error('Configured Cosmos and EVM endpoints do not share a committed block');
    }
    return pinned;
}

export function validateEip1559Params(p: Eip1559Params | null): Eip1559Params {
    if (!p) throw new Error('Pax fee-formula checks require validated EIP-1559 parameters');
    for (const key of [
        'blockGasLimit',
        'targetGasUsedPerBlock',
        'minFeePerGas',
        'maxFeePerGas',
    ] as const) {
        if (!Number.isSafeInteger(p[key]) || p[key] <= 0)
            throw new Error(`Invalid EIP-1559 ${key}`);
    }
    for (const key of ['maxUpwardAdjustment', 'maxDownwardAdjustment'] as const) {
        if (!Number.isFinite(p[key]) || p[key] < 0 || p[key] > 1)
            throw new Error(`Invalid EIP-1559 ${key}`);
    }
    if (p.targetGasUsedPerBlock >= p.blockGasLimit || p.minFeePerGas > p.maxFeePerGas) {
        throw new Error('Inconsistent EIP-1559 bounds');
    }
    return p;
}

/** Query the configured Cosmos endpoint at a height proven to belong to the EVM endpoint. */
export async function queryEip1559Params(height?: number): Promise<Eip1559Params> {
    const options = { deadline: Date.now() + 30_000 };
    const pinned = await queryPaxChainHeight(height, options);
    const param = async (subspace: string, key: string): Promise<any> => {
        const data = QueryParamsRequest.encode({ subspace, key }).finish();
        const result = await rpcResult(
            Endpoints.pax.cosmosRpc,
            'abci_query',
            {
                path: '/cosmos.params.v1beta1.Query/Params',
                data: Buffer.from(data).toString('hex'),
                height: String(pinned),
                prove: false,
            },
            options,
        );
        const response = result.response;
        if (
            !response ||
            Number(response.code ?? 0) !== 0 ||
            Number(response.height) !== pinned ||
            !response.value
        ) {
            throw new Error(
                `Parameter query failed for ${subspace}/${key}: ${JSON.stringify(response)}`,
            );
        }
        const decoded = QueryParamsResponse.decode(Buffer.from(response.value, 'base64')).param;
        if (!decoded || decoded.subspace !== subspace || decoded.key !== key) {
            throw new Error(`Parameter response mismatch for ${subspace}/${key}`);
        }
        return JSON.parse(decoded.value);
    };
    const [block, minFee, maxFee, upward, downward, target] = await Promise.all([
        param('baseapp', 'BlockParams'),
        param('evm', 'KeyMinFeePerGas'),
        param('evm', 'KeyMaximumFeePerGas'),
        param('evm', 'KeyMaxDynamicBaseFeeUpwardAdjustment'),
        param('evm', 'KeyMaxDynamicBaseFeeDownwardAdjustment'),
        param('evm', 'KeyTargetGasUsedPerBlock'),
    ]);
    const numeric = (value: unknown): number => {
        if (
            (typeof value !== 'string' && typeof value !== 'number') ||
            String(value).trim() === '' ||
            !/^[0-9]+(?:\.[0-9]+)?$/.test(String(value))
        )
            throw new Error('Invalid numeric fee parameter');
        return Number(value);
    };
    return validateEip1559Params({
        blockGasLimit: numeric(block.max_gas),
        targetGasUsedPerBlock: numeric(target),
        maxUpwardAdjustment: numeric(upward),
        maxDownwardAdjustment: numeric(downward),
        minFeePerGas: numeric(minFee),
        maxFeePerGas: numeric(maxFee),
    });
}

/**
 * Pax's dynamic base fee for the next block. Pax does not use geth's 1/8 rule: it
 * nudges by up to `maxUpwardAdjustment` when a block is over `targetGasUsedPerBlock`
 * (scaled by how full the block is relative to the gas limit) and down by
 * `maxDownwardAdjustment` when under target (scaled by how empty it is), then clamps
 * to [minFeePerGas, maxFeePerGas]. Mirrors modules/evm's CalculateNextBaseFee.
 */
export function nextBaseFeePax(
    prevBaseFee: number,
    blockGasUsed: number,
    p: Eip1559Params,
): number {
    validateEip1559Params(p);
    let next: number;
    if (blockGasUsed > p.targetGasUsedPerBlock) {
        const fullness =
            (blockGasUsed - p.targetGasUsedPerBlock) / (p.blockGasLimit - p.targetGasUsedPerBlock);
        next = prevBaseFee * (1 + p.maxUpwardAdjustment * fullness);
    } else {
        const emptiness = (p.targetGasUsedPerBlock - blockGasUsed) / p.targetGasUsedPerBlock;
        next = prevBaseFee * (1 - p.maxDownwardAdjustment * emptiness);
    }
    next = Math.floor(next);
    if (next < p.minFeePerGas) return p.minFeePerGas;
    if (next > p.maxFeePerGas) return p.maxFeePerGas;
    return next;
}

const GETH_ELASTICITY = 2n;
const GETH_BASE_FEE_CHANGE_DENOMINATOR = 8n;

/**
 * go-ethereum's London CalcBaseFee (all integer arithmetic): target = gasLimit/2,
 * base fee moves by at most 1/8 toward fullness each block, with a minimum delta of
 * 1 wei when over target. Exact, so feeHistory's predicted next base fee can be
 * matched byte-for-byte.
 */
export function nextBaseFeeGeth(prevBaseFee: bigint, gasUsed: bigint, gasLimit: bigint): bigint {
    const target = gasLimit / GETH_ELASTICITY;
    if (gasUsed === target) return prevBaseFee;
    if (gasUsed > target) {
        const delta =
            (prevBaseFee * (gasUsed - target)) / target / GETH_BASE_FEE_CHANGE_DENOMINATOR;
        return prevBaseFee + (delta > 0n ? delta : 1n);
    }
    const delta = (prevBaseFee * (target - gasUsed)) / target / GETH_BASE_FEE_CHANGE_DENOMINATOR;
    const next = prevBaseFee - delta;
    return next > 0n ? next : 0n;
}

/**
 * A block's number + gas accounting + base fee as native types; the canonical reader
 * shared by the fee-market specs (eth_feeHistory / eth_gasPrice). Accepts a height or
 * the `latest` tag.
 */
export async function blockGasInfo(
    provider: ethers.JsonRpcProvider,
    n: number | 'latest',
): Promise<{ number: number; gasUsed: bigint; gasLimit: bigint; baseFee: bigint }> {
    const tag = typeof n === 'number' ? ethers.toQuantity(n) : n;
    const b = await provider.send('eth_getBlockByNumber', [tag, false]);
    return {
        number: Number(b.number),
        gasUsed: BigInt(b.gasUsed),
        gasLimit: BigInt(b.gasLimit),
        baseFee: BigInt(b.baseFeePerGas ?? '0x0'),
    };
}
