import { expect } from 'chai';
import { assertPaxGasPriceTracksBlocks, DEFAULT_PRIORITY_FEE_WEI } from './gasPriceUtils';

function busyUncongestedBlockRequiresBufferedBaseFeeAndDefaultPriorityFee(): void {
    const blocks = [{ number: 42, baseFee: 2_000_000_009n, gasUsed: 79n, gasLimit: 100n }];
    const price = 3_200_000_009n;

    expect(() => assertPaxGasPriceTracksBlocks(price, 42, blocks)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_200_000_009n, 42, blocks)).to.throw(
        'gasPrice 2200000009 is not 1.1x any base fee plus the default priority fee near block 42',
    );
    expect(() => assertPaxGasPriceTracksBlocks(price - 1n, 42, blocks)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(price + 1n, 42, blocks)).to.throw();
}

function emptyBlockRequiresIdleFloorIncludingDefaultPriorityFee(): void {
    const blocks = [{ number: 42, baseFee: 1_000_000_000n, gasUsed: 0n, gasLimit: 100n }];

    expect(DEFAULT_PRIORITY_FEE_WEI).to.equal(1_000_000_000n);
    expect(() => assertPaxGasPriceTracksBlocks(2_100_000_000n, 42, blocks)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(1_100_000_000n, 42, blocks)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_099_999_999n, 42, blocks)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_100_000_001n, 42, blocks)).to.throw();
}

function congestionBoundaryPreservesExactFormulaAndMinimumBaseFee(): void {
    const boundary = [{ number: 42, baseFee: 2_000_000_000n, gasUsed: 80n, gasLimit: 100n }];
    const congested = [{ ...boundary[0], gasUsed: 81n }];
    const fractionalBoundary = [{ ...boundary[0], gasLimit: 101n }];
    const fractionalCongested = [{ ...fractionalBoundary[0], gasUsed: 81n }];

    expect(() => assertPaxGasPriceTracksBlocks(3_200_000_000n, 42, boundary)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, boundary)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_200_000_000n, 42, boundary)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(3_200_000_001n, 42, boundary)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, congested)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_001n, 42, congested)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(1_999_999_999n, 42, congested)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(3_200_000_000n, 42, fractionalBoundary)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, fractionalBoundary)).to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(2_000_000_000n, 42, fractionalCongested)).not.to.throw();
    expect(() => assertPaxGasPriceTracksBlocks(1_999_999_999n, 42, fractionalCongested)).to.throw();
}

function sampledNeighbourhoodAcceptsOnlyAnExactNeighbourFormula(): void {
    const blocks = [
        { number: 41, baseFee: 1_000_000_000n, gasUsed: 0n, gasLimit: 100n },
        { number: 42, baseFee: 2_000_000_000n, gasUsed: 50n, gasLimit: 100n },
        { number: 43, baseFee: 3_000_000_000n, gasUsed: 79n, gasLimit: 100n },
        { number: 44, baseFee: 4_000_000_000n, gasUsed: 80n, gasLimit: 100n },
    ];

    for (const price of [2_100_000_000n, 3_200_000_000n, 4_300_000_000n, 5_400_000_000n]) {
        expect(() => assertPaxGasPriceTracksBlocks(price, 42, blocks)).not.to.throw();
    }
    expect(() => assertPaxGasPriceTracksBlocks(3_300_000_000n, 42, blocks)).to.throw();
}

describe('gasPriceUtils', function () {
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
        'matches the exact formula for any block in the sampled neighbourhood',
        sampledNeighbourhoodAcceptsOnlyAnExactNeighbourFormula,
    );
});
