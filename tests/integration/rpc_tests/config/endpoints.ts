const env = (key: string, fallback: string): string => {
    const v = process.env[key];
    return v && v.length > 0 ? v : fallback;
};

function accountlessEndpoint(value = env('RPC_ACCOUNTLESS', '')) {
    if (!value || value.trim() !== value) {
        throw new Error(
            'Set RPC_ACCOUNTLESS to a verified keyless local fixture or mainnet-beta root RPC; verify eth_chainId and eth_accounts before running the suite.',
        );
    }
    const local = /^https?:\/\/(?:localhost|127\.0\.0\.1|\[::1\])(?::([0-9]{1,5}))?\/?$/.exec(value);
    const publicRoot = /^https:\/\/(?:api(?:[1-9]|1[0-4])\.mainnet-beta\.paxeer\.network|api-mainnet-beta\.paxeer\.network)\/?$/.test(value);
    if ((!local && !publicRoot) || (local && local[1] && (Number(local[1]) === 0 || Number(local[1]) > 65535))) {
        throw new Error(
            'RPC_ACCOUNTLESS must use a loopback fixture or an HTTPS mainnet-beta root RPC (api1-api14 or api-mainnet-beta), without credentials, query or fragment.',
        );
    }
    return value;
}

export const Endpoints = {
    pax: {
        evmRpc: env('PAX_EVM_RPC', 'http://localhost:8545'),
        evmWs: env('PAX_EVM_WS', 'ws://localhost:8546'),
        cosmosRpc: env('PAX_COSMOS_RPC', 'http://localhost:26657'),
        rest: env('PAX_REST', 'http://localhost:1317'),
    },
    eth: {
        geth: env('RPC_ETH_GETH', 'http://127.0.0.1:9547'),
        fork: env('RPC_ETH_FORK', 'http://127.0.0.1:9546'),
    },
    get accountless() {
        return accountlessEndpoint();
    },
} as const;

export const AdminMnemonic = env('PAX_ADMIN_MNEMONIC', '');

export const RuntimeStatePath = env(
    'RPC_TESTS_RUNTIME_STATE',
    'runtime/runtime.json',
);
