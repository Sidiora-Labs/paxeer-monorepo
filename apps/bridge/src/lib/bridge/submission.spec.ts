import { padHex, zeroAddress, type Address } from 'viem';
import { expect, test } from 'vitest';

import { erc20Abi, paxeerXVaultAbi } from './contracts';
import { planDeposit } from './submission';

const vault: Address = '0x1111111111111111111111111111111111111111';
const recipient: Address = '0x2222222222222222222222222222222222222222';
const token = {
  address: '0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48' as const,
  isNative: false,
};
const amount = 1234567890123456789n;

function planDepositSkipsApproveWhenAllowanceCovers() {
  for (const allowance of [ amount, amount + 1n ]) {
    const plan = planDeposit({ asset: token, amount, allowance, vault, recipient });
    expect(plan).toEqual([ {
      abi: paxeerXVaultAbi,
      address: vault,
      functionName: 'deposit',
      args: [ token.address, amount, padHex(recipient, { size: 32 }) ],
    } ]);
  }
}

function planDepositApprovesTheExactAmount() {
  for (const allowance of [ 0n, amount - 1n ]) {
    const plan = planDeposit({ asset: token, amount, allowance, vault, recipient });
    expect(plan).toEqual([ {
      abi: erc20Abi,
      address: token.address,
      functionName: 'approve',
      args: [ vault, amount ],
    }, {
      abi: paxeerXVaultAbi,
      address: vault,
      functionName: 'deposit',
      args: [ token.address, amount, padHex(recipient, { size: 32 }) ],
    } ]);
  }
}

function planDepositUsesDepositNativeForTheNativeAsset() {
  for (const allowance of [ 0n, amount ]) {
    const plan = planDeposit({
      asset: { address: zeroAddress, isNative: true },
      amount,
      allowance,
      vault,
      recipient,
    });
    expect(plan).toEqual([ {
      abi: paxeerXVaultAbi,
      address: vault,
      functionName: 'depositNative',
      args: [ padHex(recipient, { size: 32 }) ],
      value: amount,
    } ]);
  }
}

test(planDepositSkipsApproveWhenAllowanceCovers.name, planDepositSkipsApproveWhenAllowanceCovers);
test(planDepositApprovesTheExactAmount.name, planDepositApprovesTheExactAmount);
test(planDepositUsesDepositNativeForTheNativeAsset.name, planDepositUsesDepositNativeForTheNativeAsset);
