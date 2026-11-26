import {
  decodeFunctionData,
  encodeFunctionData,
  parseAbi,
  toFunctionSelector,
  type Hex,
} from 'viem';

export const LAUNCHPAD_BUY_ABI = parseAbi([
  'function buy(address pool,uint256 amountIn,uint256 minimum,uint256 deadline) returns (uint256)',
  'function usdlAddress() view returns (address)',
]);
export const LAUNCHPAD_BUY_SELECTOR = toFunctionSelector('buy(address,uint256,uint256,uint256)');

export function decodeLaunchpadBuy(data: Hex): { pool: Hex; amount: bigint } {
  const decoded = decodeFunctionData({ abi: LAUNCHPAD_BUY_ABI, data });
  if (
    decoded.functionName !== 'buy' ||
    encodeFunctionData({
      abi: LAUNCHPAD_BUY_ABI,
      functionName: 'buy',
      args: decoded.args,
    }).toLowerCase() !== data.toLowerCase()
  )
    throw new Error('invalid launchpad buy');
  return { pool: decoded.args[0].toLowerCase() as Hex, amount: decoded.args[1] };
}
