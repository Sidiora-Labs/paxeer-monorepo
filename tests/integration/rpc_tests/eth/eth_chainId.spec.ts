import { expect } from 'chai';
import { ethers } from 'ethers';
import { bothProviders, rawPax, rawGeth, expectJsonRpcError } from '../utils/chainUtils';
import { readRuntimeState, RuntimeState, expectSameError } from '../utils/testUtils';
import { HEX_QUANTITY } from '../utils/format';

const MERGED_EVM_CHAIN_ID = 125n;
const EXPECTED_EVM_CHAIN_ID = BigInt(process.env.RPC_EXPECTED_EVM_CHAIN_ID ?? '125');
const NETWORK_MODE = process.env.RPC_NETWORK_MODE ?? 'mainnet-beta';

describe('eth_chainId', function () {
    this.timeout(60 * 1000);

    const { pax, geth } = bothProviders();
    let runtime: RuntimeState;

    before(() => {
        runtime = readRuntimeState();
        expect(EXPECTED_EVM_CHAIN_ID > 0n, 'expected chain id must be positive').to.equal(true);
        if (NETWORK_MODE === 'mainnet-beta') {
            expect(EXPECTED_EVM_CHAIN_ID, 'mainnet-beta requires chain 125').to.equal(MERGED_EVM_CHAIN_ID);
        }
        expect(BigInt(runtime.chainIds.pax), 'bootstrap must match the expected network').to.equal(EXPECTED_EVM_CHAIN_ID);
    });

    it('returns a canonical 0x-prefixed quantity on Pax', async () => {
        const hex = await pax.send('eth_chainId', []);
        expect(hex).to.match(HEX_QUANTITY);
        expect(Number(BigInt(hex))).to.equal(runtime.chainIds.pax);
    });

    it('matches the explicitly configured network identity', async () => {
        const hex = await pax.send('eth_chainId', []);
        expect(hex).to.equal(ethers.toQuantity(EXPECTED_EVM_CHAIN_ID));
        if (NETWORK_MODE === 'mainnet-beta') {
            expect(hex, 'merged mainnet-beta chain id').to.equal('0x7d');
        }
    });

    it('ethers Provider.getNetwork() agrees with raw eth_chainId on Pax', async () => {
        const network = await pax.getNetwork();
        const hex = await pax.send('eth_chainId', []);
        expect(network.chainId).to.equal(BigInt(hex));
    });

    it('net_version returns the same chain id in decimal form on Pax', async () => {
        const [hex, netVersion] = await Promise.all([
            pax.send('eth_chainId', []),
            pax.send('net_version', []),
        ]);
        expect(netVersion).to.match(/^[0-9]+$/, 'net_version must be a decimal string');
        expect(BigInt(netVersion)).to.equal(BigInt(hex));
    });

    it('rejects extra positional parameters identically to geth (-32602 error code)', async () => {
        const [s, g] = await Promise.all([
            rawPax('eth_chainId', ['latest']),
            rawGeth('eth_chainId', ['latest']),
        ]);
        expectJsonRpcError(s, -32602, /too many arguments, want at most 0/);
        expectSameError(s, g);
    });
});
