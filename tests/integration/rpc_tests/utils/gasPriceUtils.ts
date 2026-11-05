import { ethers } from 'ethers';
import { expect } from 'chai';
import { waitUntil, blockGasInfo } from './chainUtils';

/**
 * Helpers for the eth_gasPrice parity spec: read the current gas price, sample it
 * against a stable block height, and assert it tracks Pax's base fee plus buffer
 * and default priority fee.
 */

export async function gasPrice(provider: ethers.JsonRpcProvider): Promise<bigint> {
    return BigInt(await provider.send('eth_gasPrice', []));
}

/** Pax's idle-chain default priority fee (1 gwei), returned when the latest block is uncongested. */
export const DEFAULT_PRIORITY_FEE_WEI = ethers.parseUnits('1', 'gwei');

/** Pax treats a block as congested once it burns > 80% of the block gas limit. */
export const CONGESTION_THRESHOLD = 0.8;

export async function maxPriorityFeePerGas(provider: ethers.JsonRpcProvider): Promise<bigint> {
    return BigInt(await provider.send('eth_maxPriorityFeePerGas', []));
}

/**
 * Read eth_gasPrice with the latest height pinned: only accept a reading where no new
 * block landed across the call, so the returned price provably derives from
 * GetNextBaseFeePerGas(B). Returns the price and that block height B.
 */
export async function gasPriceAtStableBlock(
    provider: ethers.JsonRpcProvider,
): Promise<{ gasPrice: bigint; block: number }> {
    for (let i = 0; i < 20; i++) {
        const b1 = await provider.send('eth_blockNumber', []).then(Number);
        const price = await gasPrice(provider);
        const b2 = await provider.send('eth_blockNumber', []).then(Number);
        if (b1 === b2) return { gasPrice: price, block: b1 };
    }
    throw new Error('gasPriceAtStableBlock: block kept advancing across the gas price call');
}

/** Assert the sample using B's congestion and the base fee in its direct successor B+1. */
export async function assertPaxGasPriceTracks(
    provider: ethers.JsonRpcProvider,
    gasPriceWei: bigint,
    block: number,
): Promise<void> {
    await waitUntil(
        async () =>
            (await provider.send('eth_blockNumber', []).then(Number)) >= block + 1 ? true : null,
        {
            timeoutMs: 15_000,
            label: 'successor of gas-price sample',
        },
    );
    const infos = await Promise.all([
        blockGasInfo(provider, block),
        blockGasInfo(provider, block + 1),
    ]);
    assertPaxGasPriceTracksBlocks(gasPriceWei, block, infos);
}

export function assertPaxGasPriceTracksBlocks(
    gasPriceWei: bigint,
    block: number,
    infos: Awaited<ReturnType<typeof blockGasInfo>>[],
): void {
    const samples = infos.filter((b) => b.number === block);
    const successors = infos.filter((b) => b.number === block + 1);
    expect(samples.length, `exactly one sampled block ${block}`).to.equal(1);
    expect(successors.length, `exactly one successor block ${block + 1}`).to.equal(1);
    const sample = samples[0];
    const nextBase = successors[0].baseFee;
    const congested = sample.gasUsed * 100n > sample.gasLimit * 80n;
    if (congested) {
        expect(
            gasPriceWei >= nextBase,
            `gasPrice covers the next base fee at block ${block}`,
        ).to.equal(true);
    } else {
        const expected = (nextBase * 110n) / 100n + DEFAULT_PRIORITY_FEE_WEI;
        expect(
            gasPriceWei,
            `gasPrice at block ${block} equals buffered successor fee plus default priority fee`,
        ).to.equal(expected);
    }
}
