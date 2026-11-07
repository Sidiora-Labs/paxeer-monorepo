import { expect } from 'chai';
import { ethers } from 'ethers';
import { bothProviders, rawPax, rawGeth, expectJsonRpcError, blockGasInfo, Eip1559Params, queryEip1559Params, waitUntil } from '../utils/chainUtils';
import { readRuntimeState, RuntimeState, expectSameError, claimPool } from '../utils/testUtils';
import { EvmAccount } from '../utils/evmUtils';
import { burnGasBurst } from '../utils/txUtils';
import { HEX_QUANTITY } from '../utils/format';
import { gasPrice, DEFAULT_PRIORITY_FEE_WEI } from '../utils/gasPriceUtils';

interface PinnedGasPrice {
    gasPrice: bigint;
    block: number;
    hash: string;
    nextBaseFee: bigint;
    gasUsed: bigint;
    gasLimit: bigint;
    medianReward: bigint;
}

async function samplePinnedGasPrice(provider: ethers.JsonRpcProvider): Promise<PinnedGasPrice> {
    for (let attempt = 0; attempt < 20; attempt++) {
        // Raw RPC avoids ethers' short-lived getBlockNumber cache hiding a commit.
        const head = await provider.send('eth_getBlockByNumber', ['latest', false]);
        const pending = await provider.send('eth_getBlockByNumber', ['pending', false]);
        const price = await gasPrice(provider);
        const history = await provider.send('eth_feeHistory', ['0x1', head.number, [50]]);
        const pendingAfter = await provider.send('eth_getBlockByNumber', ['pending', false]);
        const after = await provider.send('eth_getBlockByNumber', ['latest', false]);
        if (head.hash !== after.hash || pending.number !== pendingAfter.number ||
            pending.baseFeePerGas !== pendingAfter.baseFeePerGas) continue;
        expect(BigInt(pending.number), 'open block is the sampled head successor').to.equal(BigInt(head.number) + 1n);
        expect(pending.baseFeePerGas).to.match(HEX_QUANTITY);
        expect(history.baseFeePerGas[1], 'forecast and open block fix the same successor fee').to.equal(pending.baseFeePerGas);
        return {
            gasPrice: price,
            block: Number(BigInt(head.number)),
            hash: head.hash,
            nextBaseFee: BigInt(pending.baseFeePerGas),
            gasUsed: BigInt(head.gasUsed),
            gasLimit: BigInt(head.gasLimit),
            medianReward: BigInt(history.reward[0][0]),
        };
    }
    throw new Error('gas price sample could not pin a committed head and its open successor');
}

async function assertPinnedGasPrice(provider: ethers.JsonRpcProvider, sample: PinnedGasPrice): Promise<void> {
    const uncongested = sample.gasUsed <= sample.gasLimit * 80n / 100n;
    const expected = uncongested
        ? sample.nextBaseFee * 110n / 100n + DEFAULT_PRIORITY_FEE_WEI
        : sample.nextBaseFee + sample.medianReward;
    expect(sample.gasPrice, `exact gas price at block ${sample.block} (${sample.hash})`).to.equal(expected);
    const successor = await waitUntil(async () => {
        const block = await provider.send('eth_getBlockByNumber', [ethers.toQuantity(sample.block + 1), false]);
        return block ?? null;
    }, { timeoutMs: 15_000, label: `committed successor of gas price sample ${sample.block}` });
    expect(successor.parentHash, 'successor belongs to the pinned sample').to.equal(sample.hash);
    expect(BigInt(successor.baseFeePerGas), 'committed successor preserves the sampled fee').to.equal(sample.nextBaseFee);
}

describe('eth_gasPrice', function () {
    this.timeout(240 * 1000);

    const { pax, geth } = bothProviders();

    let runtime: RuntimeState;
    let spammers: EvmAccount[];
    let paxParams: Eip1559Params | null;
    let floorBase: bigint;
    let floorGasPrice: bigint;

    before(async () => {
        runtime = readRuntimeState();
        spammers = claimPool(runtime, pax, 5, 'eth_gasPrice');
        paxParams = await queryEip1559Params();
        expect(paxParams, 'required live fee-market parameters').to.not.equal(null);
        floorBase = BigInt(paxParams!.minFeePerGas);
        floorGasPrice = (floorBase * 110n) / 100n + DEFAULT_PRIORITY_FEE_WEI;
    });

    describe('happy path / schema', () => {
        it('returns a positive canonical quantity on Pax', async () => {
            const body = await rawPax<string>('eth_gasPrice', []);
            expect(body.error, JSON.stringify(body.error)).to.equal(undefined);
            expect(body.result).to.match(HEX_QUANTITY);
            expect(BigInt(body.result!) > 0n).to.equal(true);
        });

        it('returns a positive canonical quantity on geth', async () => {
            const body = await rawGeth<string>('eth_gasPrice', []);
            expect(body.error, JSON.stringify(body.error)).to.equal(undefined);
            expect(body.result).to.match(HEX_QUANTITY);
            expect(BigInt(body.result!) > 0n).to.equal(true);
        });

        it('is at least the current base fee, so a tx priced at it is includable (both)', async () => {
            const [sGas, sBlk, gGas, gBlk] = await Promise.all([
                gasPrice(pax),
                blockGasInfo(pax, 'latest'),
                gasPrice(geth),
                blockGasInfo(geth, 'latest'),
            ]);
            expect(sGas >= sBlk.baseFee, `pax gasPrice ${sGas} < base ${sBlk.baseFee}`).to.equal(true);
            expect(gGas >= gBlk.baseFee, `geth gasPrice ${gGas} < base ${gBlk.baseFee}`).to.equal(true);
        });
    });

    describe('relationship to base fee and priority fee', () => {
        it('[Pax] an uncongested gas price is exactly the base fee plus the 10% buffer plus the default priority fee', async () => {
            const sample = await samplePinnedGasPrice(pax);
            expect(sample.gasUsed <= sample.gasLimit * 80n / 100n, 'required uncongested sample').to.equal(true);
            await assertPinnedGasPrice(pax, sample);
            expect(sample.gasPrice >= floorGasPrice, 'gas price respects the idle floor').to.equal(true);
        });

        it('[geth] the gas price equals the base fee plus the suggested priority fee (exact)', async () => {
            const [price, tip, blk] = await Promise.all([
                gasPrice(geth),
                BigInt(await geth.send('eth_maxPriorityFeePerGas', [])),
                blockGasInfo(geth, 'latest'),
            ]);
            expect(price, 'geth gasPrice = baseFee + tip').to.equal(blk.baseFee + tip);
        });

        it('[Pax] maxPriorityFeePerGas defaults to 1 gwei while the chain is uncongested', async () => {
            // Quiescent runs sit well under the 80% congestion threshold.
            const tip = BigInt(await pax.send('eth_maxPriorityFeePerGas', []));
            expect(tip).to.equal(1_000_000_000n);
        });
    });

    describe('reflects base fee increases (Pax)', () => {
        it('gas price rises with the base fee and keeps tracking nextBaseFee * 1.1 plus the default priority fee', async function () {
            const samples: PinnedGasPrice[] = [];
            const { maxBlock } = await burnGasBurst(pax, runtime, spammers, {
                rounds: 10,
                onRound: async () => {
                    samples.push(await samplePinnedGasPrice(pax));
                },
            });
            expect(maxBlock, 'required workload must commit').to.be.greaterThan(0);
            expect(samples.length, 'required gas-price samples').to.be.greaterThan(0);
            const peakGasPrice = samples.reduce((m, s) => (s.gasPrice > m ? s.gasPrice : m), 0n);

            expect(
                peakGasPrice > floorGasPrice,
                `gas price should rise above the idle ${floorGasPrice}`,
            ).to.equal(true);

            // Every reading must track the live base fee through formula.
            for (const s of samples) {
                await assertPinnedGasPrice(pax, s);
            }
        });

        it('gas price decays once the load stops', async function () {
            const start = await gasPrice(pax);
            expect(start > floorGasPrice, 'required post-burst gas price exceeds the idle floor').to.equal(true);
            let lower: bigint | null = null;
            lower = await waitUntil<bigint>(
                async () => {
                    const now = await gasPrice(pax);
                    return now < start ? now : null;
                },
                { timeoutMs: 60_000, intervalMs: 500, label: 'gas price decays' },
            );

            expect(lower! < start, `gas price ${lower} should drop below post-burst ${start}`).to.equal(true);
            expect(lower! >= floorGasPrice, `gas price ${lower} never falls below the floor`).to.equal(true);
        });
    });

    describe('wrong params / error handling', () => {
        it('an extra positional argument fails identically (-32602, want at most 0)', async () => {
            const [s, g] = await Promise.all([
                rawPax('eth_gasPrice', ['latest']),
                rawGeth('eth_gasPrice', ['latest']),
            ]);
            expectJsonRpcError(s, -32602, /too many arguments, want at most 0/);
            expectSameError(s, g);
        });

        it('an object argument fails identically (-32602, want at most 0)', async () => {
            const [s, g] = await Promise.all([
                rawPax('eth_gasPrice', [{}]),
                rawGeth('eth_gasPrice', [{}]),
            ]);
            expectJsonRpcError(s, -32602, /too many arguments, want at most 0/);
            expectSameError(s, g);
        });
    });
});
