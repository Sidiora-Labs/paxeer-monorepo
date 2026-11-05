import { expect } from 'chai';
import { assertPaxGasPriceTracksBlocks, DEFAULT_PRIORITY_FEE_WEI } from './gasPriceUtils';

function busyUncongestedBlockRequiresBufferedBaseFeeAndDefaultPriorityFee(): void {
    const blocks = [
        { number: 42, baseFee: 9_000_000_000n, gasUsed: 79n, gasLimit: 100n },
        { number: 43, baseFee: 2_000_000_009n, gasUsed: 99n, gasLimit: 100n },
    ];
    const price = 3_200_000_009n;

    expect(() => assertPaxGasPriceTracksBlocks(price, 42, blocks)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_200_000_009n, 42, blocks)).to.throw(
        'gasPrice at block 42 equals buffered successor fee plus default priority fee',
    );
    expect(() => assertPaxGasPriceTracksBlocks(price - 1n, 42, blocks)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(price + 1n, 42, blocks)).to.throw();
}

function emptyBlockRequiresIdleFloorIncludingDefaultPriorityFee(): void {
    const blocks = [
        { number: 42, baseFee: 2_000_000_000n, gasUsed: 0n, gasLimit: 100n },
        { number: 43, baseFee: 1_000_000_000n, gasUsed: 100n, gasLimit: 100n },
    ];

    expect(DEFAULT_PRIORITY_FEE_WEI).to.equal(1_000_000_000n);
    expect(() => assertPaxGasPriceTracksBlocks(2_100_000_000n, 42, blocks)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(1_100_000_000n, 42, blocks)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_099_999_999n, 42, blocks)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_100_000_001n, 42, blocks)).to.throw();
}

function congestionBoundaryPreservesExactFormulaAndMinimumBaseFee(): void {
    const boundary = [
        { number: 42, baseFee: 9_000_000_000n, gasUsed: 80n, gasLimit: 100n },
        { number: 43, baseFee: 2_000_000_000n, gasUsed: 100n, gasLimit: 100n },
    ];
    const congested = [{ ...boundary[0], gasUsed: 81n }, boundary[1]];
    const fractionalBoundary = [{ ...boundary[0], gasLimit: 101n }, boundary[1]];
    const fractionalCongested = [{ ...fractionalBoundary[0], gasUsed: 81n }, boundary[1]];

    expect(() => assertPaxGasPriceTracksBlocks(3_200_000_000n, 42, boundary)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, boundary)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_200_000_000n, 42, boundary)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(3_200_000_001n, 42, boundary)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, congested)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_001n, 42, congested)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(1_999_999_999n, 42, congested)).to.throw();
    expect(() =>
        assertPaxGasPriceTracksBlocks(3_200_000_000n, 42, fractionalBoundary),
    ).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, fractionalBoundary)).to.throw();
    expect(() =>
        assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, fractionalCongested),
    ).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(1_999_999_999n, 42, fractionalCongested)).to.throw();
}

function sampledNextFeeRejectsEveryWrongNeighbourPrice(): void {
    const blocks = [
        { number: 41, baseFee: 1_000_000_000n, gasUsed: 100n, gasLimit: 100n },
        { number: 42, baseFee: 2_000_000_000n, gasUsed: 50n, gasLimit: 100n },
        { number: 43, baseFee: 3_000_000_000n, gasUsed: 100n, gasLimit: 100n },
        { number: 44, baseFee: 4_000_000_000n, gasUsed: 100n, gasLimit: 100n },
    ];
    expect(() => assertPaxGasPriceTracksBlocks(4_300_000_000n, 42, blocks)).not.to.throw();
    for (const price of [
        2_100_000_000n,
        3_200_000_000n,
        5_400_000_000n,
        3_300_000_000n,
        4_299_999_999n,
        4_300_000_001n,
    ]) {
        expect(() => assertPaxGasPriceTracksBlocks(price, 42, blocks)).to.throw();
    }
    expect(() =>
        assertPaxGasPriceTracksBlocks(
            4_300_000_000n,
            42,
            blocks.filter((b) => b.number !== 43),
        ),
    ).to.throw();
    expect(() =>
        assertPaxGasPriceTracksBlocks(
            4_300_000_000n,
            42,
            blocks.filter((b) => b.number !== 42),
        ),
    ).to.throw();
    expect(() =>
        assertPaxGasPriceTracksBlocks(4_300_000_000n, 42, [...blocks, blocks[2]]),
    ).to.throw();
}

function integerArithmeticPreservesLargeFeesAndTruncation(): void {
    const baseFee = 9_007_199_254_740_999n;
    const blocks = [
        { number: 42, baseFee: 1n, gasUsed: 80n, gasLimit: 100n },
        { number: 43, baseFee, gasUsed: 100n, gasLimit: 100n },
    ];
    const price = (baseFee * 110n) / 100n + DEFAULT_PRIORITY_FEE_WEI;
    expect(() => assertPaxGasPriceTracksBlocks(price, 42, blocks)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(price - 1n, 42, blocks)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(price + 1n, 42, blocks)).to.throw();
}

describe('gasPriceUtils', function () {
    it(
        'keeps integer arithmetic exact above the number precision limit',
        integerArithmeticPreservesLargeFeesAndTruncation,
    );
    it(
        'requires the buffered base fee plus the default priority fee on a busy uncongested block',
        busyUncongestedBlockRequiresBufferedBaseFeeAndDefaultPriorityFee,
    );
    it(
        'requires the idle floor plus the default priority fee on an empty block',
        emptyBlockRequiresIdleFloorIncludingDefaultPriorityFee,
    );
    it(
        'keeps the exact formula through the congestion boundary and the minimum base fee above it',
        congestionBoundaryPreservesExactFormulaAndMinimumBaseFee,
    );
    it(
        'pins the direct successor fee and rejects every wrong neighboring formula',
        sampledNextFeeRejectsEveryWrongNeighbourPrice,
    );
});
