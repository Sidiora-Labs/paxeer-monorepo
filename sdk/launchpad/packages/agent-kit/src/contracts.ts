import {
  getAddress,
  getAbiItem,
  parseAbi,
  toFunctionSelector,
  toFunctionSignature,
  type Address,
} from 'viem';
import { requireThat } from './errors.js';
import { routerAbi, quoterAbi, protocolConfigAbi } from '../../sidiora-launchpad-v3/dist/abis.js';

export { routerAbi, quoterAbi, protocolConfigAbi };

export const DEPLOYMENT = Object.freeze({
  chainId: 125,
  router: '0x78FB06f525cD964388f2305A489035263E31C6e4' as Address,
  quoter: '0x793006f4F321b8863F26DBD75b11bAa01DA181dc' as Address,
  protocolConfig: '0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6' as Address,
  usdl: '0x85fcd13735f4309833a503ee804ea32395851479' as Address,
  spender: '0x0000000000000000000000000000000000001017' as Address,
  creationFeeRaw: '1000000',
});

export interface LaunchpadDeployment {
  chainId: number;
  router: Address;
  quoter: Address;
  protocolConfig: Address;
  usdl: Address;
  spender: Address;
  creationFeeRaw: string;
}

export interface EvmDeployment extends LaunchpadDeployment {
  poolRegistry: Address;
}

export function checkedDeployment(value: LaunchpadDeployment): Readonly<LaunchpadDeployment> {
  requireThat(Number.isSafeInteger(value.chainId) && value.chainId > 0, 'invalid_deployment_chain');
  requireThat(/^[1-9][0-9]{0,77}$/.test(value.creationFeeRaw), 'invalid_creation_fee');
  for (const address of [
    value.router,
    value.quoter,
    value.protocolConfig,
    value.usdl,
    value.spender,
  ])
    requireThat(BigInt(getAddress(address)) !== 0n, 'invalid_deployment_address');
  return Object.freeze({ ...value });
}

export const EVM_POOL_ABI = parseAbi([
  'function isRegisteredPool(address pool) view returns (bool)',
  'function tokenAddress() view returns (address)',
]);

export const HOME_CHAIN_ABI = parseAbi([
  'function homeChainOf(address pool) view returns (uint64)',
]);
export const VIRTUAL_CHAIN_ID_BASE = 125000000;
export const QUOTE_TTL_MS = 30000;
export const SIMULATION_TTL_MS = 10000;
export const TRADE_DEADLINE_SECONDS = 120;

function routerCall(
  name: 'buy' | 'sell' | 'createMarket',
  deployment: LaunchpadDeployment = DEPLOYMENT,
) {
  const item = getAbiItem({ abi: routerAbi, name });
  return Object.freeze({
    contract: deployment.router,
    method: toFunctionSignature(item),
    selector: toFunctionSelector(item),
  });
}

export function executionCalls(deployment: LaunchpadDeployment = DEPLOYMENT) {
  return Object.freeze({
    buy: routerCall('buy', deployment),
    sell: routerCall('sell', deployment),
    launch: routerCall('createMarket', deployment),
    approve: Object.freeze({
      method: 'approve(address,uint256)',
      selector: toFunctionSelector('approve(address,uint256)'),
      spender: deployment.spender,
    }),
  });
}

export const EXECUTION_CALLS = executionCalls();
