import util from 'node:util';
import fs from 'node:fs';
import { createHash } from 'node:crypto';
import { DirectSecp256k1HdWallet } from '@cosmjs/proto-signing';
import { SigningCosmWasmClient } from '@cosmjs/cosmwasm-stargate';
import { stringToPath } from '@cosmjs/crypto';
import { Endpoints } from '../config/endpoints';
import { waitUntil } from './chainUtils';
import {
    PAX_HD_PATH,
    DOCKER_NODE,
    PAXD_ENV,
    DOCKER_KEY_PASSWORD,
    DOCKER_EVM_RPC,
    CW20_WASM_PATH,
    CW20_WASM_SHA256,
    WASM_FEE,
    EXEC_FEE,
} from './constants';

const exec = util.promisify(require('node:child_process').exec);

export interface Cw20InitMsg {
    name: string;
    symbol: string;
    decimals: number;
    initial_balances: { address: string; amount: string }[];
    mint?: { minter: string };
}

export type WasmCapability = 'enabled' | 'disabled';

/** Decode the selected chain's validated policy; malformed or missing policy is an error. */
export function wasmCapability(params: any): WasmCapability {
    if (!params || !params.code_upload_access) throw new Error('wasm params: missing access policy');
    const permissions = ['Nobody', 'OnlyAddress', 'Everybody'];
    const upload = params.code_upload_access.permission;
    const instantiate = params.instantiate_default_permission;
    if (!permissions.includes(upload) || !permissions.includes(instantiate)) {
        throw new Error('wasm params: invalid access permission');
    }
    if (upload === 'OnlyAddress' && (typeof params.code_upload_access.address !== 'string' || !/^pax1[0-9a-z]+$/.test(params.code_upload_access.address))) {
        throw new Error('wasm params: invalid upload address');
    }
    return upload === 'Everybody' && instantiate === 'Everybody' ? 'enabled' : 'disabled';
}

/** Transport and decoding failures remain failures; only an explicit policy disables this path. */
export async function isWasmEnabled(): Promise<boolean> {
    const query = async (key: string): Promise<any> => {
        const response = await fetch(new URL(`/cosmos/params/v1beta1/params?subspace=wasm&key=${key}`, Endpoints.pax.rest), {
            signal: AbortSignal.timeout(10_000),
        });
        if (!response.ok) throw new Error(`wasm params query failed: HTTP ${response.status}`);
        const body = await response.json();
        if (body.param?.subspace !== 'wasm' || body.param?.key !== key || typeof body.param?.value !== 'string') {
            throw new Error('wasm params query returned an invalid parameter');
        }
        return JSON.parse(body.param.value);
    };
    const [upload, instantiate] = await Promise.all([query('uploadAccess'), query('instantiateAccess')]);
    const capability = wasmCapability({ code_upload_access: upload, instantiate_default_permission: instantiate });
    if (capability === 'disabled') console.info('CW20 fixtures disabled by selected-chain wasm access policy');
    return capability === 'enabled';
}

type AdminConnection = { client: SigningCosmWasmClient; address: string };
interface AdminCacheEntry {
    key: string;
    connection: Promise<AdminConnection>;
    users: number;
    discarded: boolean;
}
let adminClient: AdminCacheEntry | undefined;

/** Dispose a superseded connection only after its last active operation finishes. */
function discardAdmin(entry: AdminCacheEntry): void {
    entry.discarded = true;
    if (entry.users === 0) void entry.connection.then(({ client }) => client.disconnect(), () => undefined);
}

/** Cache by the selected endpoint and derived public signer identity, never by mnemonic. */
async function adminCosmWasm(mnemonic: string): Promise<AdminConnection & { release: () => void }> {
    const endpoint = Endpoints.pax.cosmosRpc;
    const wallet = await DirectSecp256k1HdWallet.fromMnemonic(mnemonic, {
        prefix: 'pax', hdPaths: [stringToPath(PAX_HD_PATH)],
    });
    const [account] = await wallet.getAccounts();
    if (!account) throw new Error('CosmWasm signer has no account');
    return leaseAdminConnection(endpoint, account.address, () =>
        SigningCosmWasmClient.connectWithSigner(endpoint, wallet)
            .then(client => ({ client, address: account.address })),
    );
}

async function leaseAdminConnection(
    endpoint: string,
    address: string,
    connect: () => Promise<AdminConnection>,
): Promise<AdminConnection & { release: () => void }> {
    const key = JSON.stringify([endpoint, address]);
    if (!adminClient || adminClient.key !== key) {
        if (adminClient) discardAdmin(adminClient);
        adminClient = {
            key, users: 0, discarded: false,
            connection: connect(),
        };
    }
    const entry = adminClient;
    entry.users++;
    let released = false;
    const release = (): void => {
        if (released) return;
        released = true;
        entry.users--;
        if (entry.discarded && entry.users === 0) discardAdmin(entry);
    };
    try {
        return { ...await entry.connection, release };
    } catch {
        if (adminClient === entry) adminClient = undefined;
        release();
        throw new Error('CosmWasm signing connection failed');
    }
}

function readVerifiedCw20Wasm(): Buffer {
    const wasm = fs.readFileSync(CW20_WASM_PATH);
    const digest = createHash('sha256').update(wasm).digest('hex');
    if (digest !== CW20_WASM_SHA256) {
        throw new Error(
            `cw20_base.wasm checksum mismatch: expected ${CW20_WASM_SHA256}, got ${digest}. ` +
                `Re-copy from contracts/wasm/cw20_base.wasm and update CW20_WASM_SHA256 in the same commit.`,
        );
    }
    return wasm;
}

/**
 * Store cw20_base and instantiate it with `initMsg`, signed client-side by the admin.
 * Returns the instantiated CW20 `pax1…` contract address (and its code id).
 */
export async function deployCw20(
    initMsg: Cw20InitMsg,
    adminMnemonic: string,
    label = 'rpc_tests cw20',
): Promise<{ codeId: number; address: string }> {
    const { client, address, release } = await adminCosmWasm(adminMnemonic);
    try {
    const wasm = readVerifiedCw20Wasm();
    const uploaded = await client.upload(address, wasm, WASM_FEE);
    const instantiated = await client.instantiate(
        address,
        uploaded.codeId,
        initMsg,
        label,
        WASM_FEE,
        { admin: address },
    );
    return { codeId: uploaded.codeId, address: instantiated.contractAddress };
    } finally { release(); }
}

/** Query the CW20's ERC20 pointer; returns '' until it is registered on-chain. */
async function queryCw20Pointer(cw20Address: string): Promise<string> {
    const { stdout } = await exec(
        `docker exec ${DOCKER_NODE} /bin/bash -c '${PAXD_ENV} && paxd query evm pointer CW20 ${cw20Address} -o json'`,
    );
    const res = JSON.parse(stdout);
    return res.exists && res.pointer ? res.pointer : '';
}

/**
 * Register an ERC20 pointer for a CW20 via the in-container `admin` key and return the
 * pointer's `0x…` EVM address. `register-evm-pointer` broadcasts in sync mode (modern
 * paxd no longer blocks until commit), so we poll the pointer query until it appears
 * rather than reading it once right after broadcast.
 */
export async function registerCw20Pointer(cw20Address: string): Promise<string> {
    await exec(
        `docker exec ${DOCKER_NODE} /bin/bash -c '${PAXD_ENV} && printf "${DOCKER_KEY_PASSWORD}\\n" | ` +
            `paxd tx evm register-evm-pointer CW20 ${cw20Address} --evm-rpc=${DOCKER_EVM_RPC} ` +
            `--from admin -y --gas-limit 4900000 --fees 800000uhpx -b sync'`,
    );
    return waitUntil<string>(
        async () => (await queryCw20Pointer(cw20Address)) || null,
        { timeoutMs: 60_000, intervalMs: 1_000, label: `CW20 pointer for ${cw20Address}` },
    );
}

export interface Cw20ExecResult {
    hash: string;
    height: number;
    code: number;
    /** Gas the Cosmos tx consumed — the same amount its shell receipt reports on-chain. */
    gasUsed: bigint;
}

/**
 * Sign and broadcast a CW20 `transfer` from the admin and wait for inclusion. Returns
 * the block height so the rich-block builder can require it to co-locate with the EVM
 * batch. This is a pure Cosmos tx — it must NOT surface through any EVM JSON-RPC.
 */
export async function cw20Transfer(
    cw20Address: string,
    recipientPax: string,
    amount: string,
    adminMnemonic: string,
): Promise<Cw20ExecResult> {
    const { client, address, release } = await adminCosmWasm(adminMnemonic);
    try {
    const res = await client.execute(
        address,
        cw20Address,
        { transfer: { recipient: recipientPax, amount } },
        EXEC_FEE,
        'rpc_tests dual-vm cw20 transfer',
    );
    return {
        hash: res.transactionHash,
        height: Number(res.height),
        code: 0,
        gasUsed: BigInt(res.gasUsed),
    };
    } finally { release(); }
}
