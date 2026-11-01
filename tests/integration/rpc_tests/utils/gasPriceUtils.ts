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
        const b1 = await provider.getBlockNumber();
        const price = await gasPrice(provider);
        const b2 = await provider.getBlockNumber();
        if (b1 === b2) return { gasPrice: price, block: b1 };
    }
    throw new Error('gasPriceAtStableBlock: block kept advancing across the gas price call');
}

/**
 * Assert a Pax gas price reading is exactly (baseFee * 110) / 100 (rounded down, matching
 * the node's integer Mul/Div) plus the 1 gwei default priority fee when uncongested. The base
 * fee must come from a block in the immediate neighbourhood of B; congested readings
 * must at least cover the base fee.
 */
export async function assertPaxGasPriceTracks(
    provider: ethers.JsonRpcProvider,
    gasPriceWei: bigint,
    block: number,
): Promise<void> {
    await waitUntil(async () => ((await provider.getBlockNumber()) >= block + 2 ? true : null), {
        timeoutMs: 15_000,
        label: 'two blocks after sample',
    });
    const head = await provider.getBlockNumber();
    const heights = [block - 1, block, block + 1, block + 2].filter(h => h >= 0 && h <= head);
    const infos = await Promise.all(heights.map(h => blockGasInfo(provider, h)));

    assertPaxGasPriceTracksBlocks(gasPriceWei, block, infos);
}

export function assertPaxGasPriceTracksBlocks(
    gasPriceWei: bigint,
    block: number,
    infos: Awaited<ReturnType<typeof blockGasInfo>>[],
): void {
    if (infos.some(b => (b.baseFee * 110n) / 100n + DEFAULT_PRIORITY_FEE_WEI === gasPriceWei)) return;

    const congested = infos.some(b => b.gasUsed > (b.gasLimit * 80n) / 100n);
    const minBase = infos.reduce((m, b) => (b.baseFee < m ? b.baseFee : m), infos[0].baseFee);
    expect(
        congested && gasPriceWei >= minBase,
        `gasPrice ${gasPriceWei} is not 1.1x any base fee plus the default priority fee near block ${block} ` +
            `(bases: ${infos.map(b => b.baseFee).join(', ')})`,
    ).to.equal(true);
}
